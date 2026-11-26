package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/logs"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/workers"
	"github.com/gorilla/websocket"
)

// CFTailMessage represents the JSON payload pushed by Cloudflare's Tail WebSocket.
type CFTailMessage struct {
	Outcome          string             `json:"outcome"`
	ScriptName       string             `json:"scriptName"`
	EventTimestampMs int64              `json:"eventTimestampMs"`
	EventType        string             `json:"eventType"`
	Logs             []CFTailLogEntry   `json:"logs"`
	Exceptions       []CFTailException  `json:"exceptions"`
	Event            *CFTailEventDetail `json:"event"`
	CPUTimeMs        float64            `json:"cpuTimeMs"`
	WallTimeMs       float64            `json:"wallTimeMs"`
}

type CFTailLogEntry struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Message   any    `json:"message"`
}

type CFTailException struct {
	Name      string `json:"name"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type CFTailEventDetail struct {
	Request  *CFTailRequest  `json:"request"`
	Response *CFTailResponse `json:"response"`
}

type CFTailRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type CFTailResponse struct {
	Status int `json:"status"`
}

// TailOptions configures filtering, fault-ownership formatting, and NDJSON export during live tailing.
type TailOptions struct {
	RouteFilter string
	ErrorsOnly  bool
	OwnerFilter logs.Owner
	Raw         bool
	QuietAssets bool
	NDJSONOut   io.Writer
}

// TailLogs streams live logs from a Cloudflare Worker using Cloudflare's Tail WebSocket API natively.
// Automatically reconnects on connection drops or session expiration without requiring external tools like Wrangler.
func (p *CloudflareProvider) TailLogs(ctx context.Context, cfg *config.NextDeployConfig, routeFilter string, out io.Writer) error {
	return p.TailLogsWithOptions(ctx, cfg, TailOptions{
		RouteFilter: routeFilter,
		QuietAssets: true,
	}, out)
}

// TailLogsWithOptions streams live logs from a Cloudflare Worker with fault-ownership classification and filtering.
func (p *CloudflareProvider) TailLogsWithOptions(ctx context.Context, cfg *config.NextDeployConfig, opts TailOptions, out io.Writer) error {
	workerName := p.getWorkerName(cfg)

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := p.runTailSession(ctx, workerName, opts, out)
		if ctx.Err() != nil {
			return nil
		}

		if err != nil {
			p.log.Warn("Tail connection closed (%v). Reconnecting in 2s...", err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (p *CloudflareProvider) runTailSession(ctx context.Context, workerName string, opts TailOptions, out io.Writer) error {
	p.log.Info("Creating native Cloudflare Workers tail session for %s...", workerName)

	tail, err := p.cf.Workers.Scripts.Tail.New(ctx, workerName, workers.ScriptTailNewParams{
		AccountID: cloudflare.F(p.accountID),
		Body:      map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("failed to create tail session: %w", err)
	}

	defer func() {
		delCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = p.cf.Workers.Scripts.Tail.Delete(delCtx, workerName, tail.ID, workers.ScriptTailDeleteParams{
			AccountID: cloudflare.F(p.accountID),
		})
	}()

	wsURL := tail.URL
	if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	} else if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	}

	header := http.Header{}
	if p.apiToken != "" {
		header.Set("Authorization", "Bearer "+p.apiToken)
	}

	dialer := websocket.Dialer{
		Subprotocols:     []string{"trace-v1"},
		HandshakeTimeout: 15 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket handshake failed (%d %s): %w", resp.StatusCode, resp.Status, err)
		}
		return fmt.Errorf("failed to connect to tail websocket: %w", err)
	}
	defer conn.Close()

	p.log.Success("Connected to %s. Streaming live logs (Ctrl+C to stop)...", workerName)

	go func() {
		<-ctx.Done()
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
		_ = conn.Close()
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		var event CFTailMessage
		if err := json.Unmarshal(msgBytes, &event); err != nil {
			continue
		}

		formatTailEvent(out, &event, opts)
	}
}

func formatTailEvent(out io.Writer, event *CFTailMessage, opts TailOptions) {
	if event == nil {
		return
	}

	t := time.Now()
	if event.EventTimestampMs > 0 {
		t = time.UnixMilli(event.EventTimestampMs)
	}
	timeStr := fmt.Sprintf("\033[90m%s\033[0m", t.Format("15:04:05"))

	method := "GET"
	reqPath := "/"
	rawURL := ""
	status := 200
	var headers map[string]string

	if event.Event != nil && event.Event.Request != nil {
		if event.Event.Request.Method != "" {
			method = event.Event.Request.Method
		}
		rawURL = event.Event.Request.URL
		headers = event.Event.Request.Headers
		if u, err := url.Parse(rawURL); err == nil && u.RequestURI() != "" {
			reqPath = u.RequestURI()
		} else if rawURL != "" {
			reqPath = rawURL
		}
	}
	if event.Event != nil && event.Event.Response != nil && event.Event.Response.Status != 0 {
		status = event.Event.Response.Status
	}

	if opts.RouteFilter != "" {
		if !strings.Contains(rawURL, opts.RouteFilter) && !strings.Contains(reqPath, opts.RouteFilter) {
			return
		}
	}

	hasErrorLog := false
	rawLogMessages := make([]string, 0, len(event.Logs))
	for _, l := range event.Logs {
		if strings.EqualFold(l.Level, "error") || strings.EqualFold(l.Level, "fatal") {
			hasErrorLog = true
		}
		rawLogMessages = append(rawLogMessages, tailLogMessageText(l.Message))
	}

	exceptions := make([]string, 0, len(event.Exceptions))
	for _, ex := range event.Exceptions {
		exceptions = append(exceptions, fmt.Sprintf("%s: %s", ex.Name, ex.Message))
	}

	isError := status >= 400 ||
		(event.Outcome != "" && event.Outcome != "ok") ||
		len(exceptions) > 0 ||
		hasErrorLog

	if opts.ErrorsOnly && !isError {
		return
	}

	if !opts.Raw && opts.QuietAssets && !isError && opts.RouteFilter == "" && isQuietAssetPath(reqPath) {
		return
	}

	if !isError && opts.OwnerFilter != "" {
		return
	}

	var diag logs.Diagnosis
	var hasDiag bool
	if isError {
		sig := logs.Signal{
			Method:     method,
			Path:       reqPath,
			URL:        rawURL,
			Status:     status,
			Outcome:    event.Outcome,
			Headers:    headers,
			Messages:   logs.DedupeMessages(rawLogMessages),
			Exceptions: exceptions,
		}
		diag = logs.Classify(sig)
		hasDiag = true
		if opts.OwnerFilter != "" && diag.Owner != opts.OwnerFilter {
			return
		}
	}

	statusColor := "\033[32m" // Green
	if status >= 400 && status < 500 {
		statusColor = "\033[33m" // Yellow
	} else if status >= 500 || (event.Outcome != "" && event.Outcome != "ok") || len(exceptions) > 0 {
		statusColor = "\033[31m" // Red
	}

	durationStr := ""
	if event.WallTimeMs > 0 {
		durationStr = fmt.Sprintf(" \033[90m(%.1fms)\033[0m", event.WallTimeMs)
	}

	outcomeLabel := ""
	if event.Outcome != "ok" && event.Outcome != "" {
		outcomeLabel = fmt.Sprintf(" \033[31m[%s]\033[0m", event.Outcome)
	}

	ownerPrefix := ""
	if hasDiag {
		ownerPrefix = " " + logs.OwnerBadge(diag.Owner)
	}

	fmt.Fprintf(out, "%s%s \033[1;36m%-6s\033[0m %s %s%d\033[0m%s%s\n",
		timeStr, ownerPrefix, method, reqPath, statusColor, status, durationStr, outcomeLabel)

	if hasDiag && !opts.Raw {
		fmt.Fprintf(out, "       \033[90mCause:\033[0m  %s\n", diag.Summary)
		if diag.Hint != "" {
			fmt.Fprintf(out, "       \033[32mHint:\033[0m   %s\n", diag.Hint)
		}
		var detailSrc []string
		detailSrc = append(detailSrc, exceptions...)
		detailSrc = append(detailSrc, rawLogMessages...)
		if detail := logs.BestDetail(detailSrc); detail != "" {
			fmt.Fprintf(out, "       \033[90mDetail:\033[0m %s\n", detail)
		}
	} else {
		for _, l := range event.Logs {
			logColor := "\033[37m"
			switch l.Level {
			case "warn":
				logColor = "\033[33m"
			case "error":
				logColor = "\033[31m"
			}
			fmt.Fprintf(out, "       %s[%s]\033[0m %s\n", logColor, l.Level, tailLogMessageText(l.Message))
		}
		for _, ex := range event.Exceptions {
			fmt.Fprintf(out, "       \033[1;31m[exception]\033[0m %s: %s\n", ex.Name, ex.Message)
		}
	}

	if opts.NDJSONOut != nil {
		var detailSrc []string
		detailSrc = append(detailSrc, exceptions...)
		detailSrc = append(detailSrc, rawLogMessages...)
		rec := logs.ErrRecord{
			TS:              t.UTC().Format(time.RFC3339Nano),
			Method:          method,
			URL:             rawURL,
			Path:            reqPath,
			Status:          status,
			Outcome:         event.Outcome,
			SegmentPrefetch: tailHeaderLookup(headers, "next-router-segment-prefetch"),
			Owner:           diag.Owner,
			Code:            diag.Code,
			Summary:         diag.Summary,
			Hint:            diag.Hint,
			Message:         logs.BestDetail(detailSrc),
			WallTimeMs:      event.WallTimeMs,
			CPUTimeMs:       event.CPUTimeMs,
		}
		_ = json.NewEncoder(opts.NDJSONOut).Encode(rec)
	}
}

func tailLogMessageText(msg any) string {
	if arr, ok := msg.([]any); ok {
		parts := make([]string, 0, len(arr))
		for _, p := range arr {
			parts = append(parts, fmt.Sprintf("%v", p))
		}
		return strings.Join(parts, " ")
	}
	if msg == nil {
		return ""
	}
	return fmt.Sprintf("%v", msg)
}

func isQuietAssetPath(reqPath string) bool {
	p := reqPath
	if idx := strings.IndexByte(p, '?'); idx >= 0 {
		p = p[:idx]
	}
	if strings.HasPrefix(p, "/_next/static/") || strings.HasPrefix(p, "/_next/image") {
		return true
	}
	switch p {
	case "/favicon.ico", "/robots.txt":
		return true
	}
	return false
}

func tailHeaderLookup(headers map[string]string, name string) string {
	if len(headers) == 0 {
		return ""
	}
	if v, ok := headers[name]; ok {
		return v
	}
	want := strings.ToLower(name)
	for k, v := range headers {
		if strings.ToLower(k) == want {
			return v
		}
	}
	return ""
}
