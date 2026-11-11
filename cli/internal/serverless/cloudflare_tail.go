package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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

// TailLogs streams live logs from a Cloudflare Worker using Cloudflare's Tail WebSocket API natively.
// Automatically reconnects on connection drops or session expiration without requiring external tools like Wrangler.
func (p *CloudflareProvider) TailLogs(ctx context.Context, cfg *config.NextDeployConfig, routeFilter string, out io.Writer) error {
	workerName := p.getWorkerName(cfg)

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := p.runTailSession(ctx, workerName, routeFilter, out)
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

func (p *CloudflareProvider) runTailSession(ctx context.Context, workerName, routeFilter string, out io.Writer) error {
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

		// Apply route filter if present
		if routeFilter != "" {
			reqURL := ""
			if event.Event != nil && event.Event.Request != nil {
				reqURL = event.Event.Request.URL
			}
			if !strings.Contains(reqURL, routeFilter) {
				continue
			}
		}

		formatAndPrintTailEvent(out, &event)
	}
}

func formatAndPrintTailEvent(out io.Writer, event *CFTailMessage) {
	t := time.Now()
	if event.EventTimestampMs > 0 {
		t = time.UnixMilli(event.EventTimestampMs)
	}
	timeStr := fmt.Sprintf("\033[90m%s\033[0m", t.Format("15:04:05"))

	method := "GET"
	reqPath := "/"
	status := 200

	if event.Event != nil && event.Event.Request != nil {
		method = event.Event.Request.Method
		if u, err := url.Parse(event.Event.Request.URL); err == nil {
			reqPath = u.RequestURI()
		} else {
			reqPath = event.Event.Request.URL
		}
	}
	if event.Event != nil && event.Event.Response != nil {
		status = event.Event.Response.Status
	}

	statusColor := "\033[32m" // Green
	if status >= 400 && status < 500 {
		statusColor = "\033[33m" // Yellow
	} else if status >= 500 || event.Outcome == "exception" {
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

	fmt.Fprintf(out, "%s \033[1;36m%-6s\033[0m %s %s%d\033[0m%s%s\n",
		timeStr, method, reqPath, statusColor, status, durationStr, outcomeLabel)

	// Print console logs if present
	for _, l := range event.Logs {
		logColor := "\033[37m"
		switch l.Level {
		case "warn":
			logColor = "\033[33m"
		case "error":
			logColor = "\033[31m"
		}
		msgStr := ""
		if arr, ok := l.Message.([]any); ok {
			var parts []string
			for _, p := range arr {
				parts = append(parts, fmt.Sprintf("%v", p))
			}
			msgStr = strings.Join(parts, " ")
		} else {
			msgStr = fmt.Sprintf("%v", l.Message)
		}
		fmt.Fprintf(out, "       %s[%s]\033[0m %s\n", logColor, l.Level, msgStr)
	}

	// Print exceptions if present
	for _, ex := range event.Exceptions {
		fmt.Fprintf(out, "       \033[1;31m[exception]\033[0m %s: %s\n", ex.Name, ex.Message)
	}
}
