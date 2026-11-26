package logs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FlexTimestamp parses either an ISO-8601 string or a Unix-millisecond integer/float.
type FlexTimestamp string

func (f *FlexTimestamp) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*f = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*f = FlexTimestamp(s)
		return nil
	}
	var ms float64
	if err := json.Unmarshal(trimmed, &ms); err != nil {
		return err
	}
	if ms <= 0 {
		*f = ""
		return nil
	}
	*f = FlexTimestamp(time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339Nano))
	return nil
}

// CFExportEntry is the minimal shape of a Cloudflare Workers Observability event.
// Unknown fields are skipped during streaming decode.
type CFExportEntry struct {
	Source    CFExportSource   `json:"source"`
	Timestamp FlexTimestamp    `json:"timestamp"`
	Workers   CFExportWorkers  `json:"$workers"`
	Metadata  CFExportMetadata `json:"$metadata"`
}

type CFExportSource struct {
	Level   string `json:"level"`
	Message any    `json:"message"`
}

// MessageString converts Source.Message (string, array, or object) into a string.
func (s CFExportSource) MessageString() string {
	switch v := s.Message.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, " ")
	default:
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
		return fmt.Sprint(v)
	}
}

type CFExportWorkers struct {
	Event         CFExportWorkersEvent `json:"event"`
	ScriptName    string               `json:"scriptName"`
	ScriptVersion struct {
		ID string `json:"id"`
	} `json:"scriptVersion"`
	Outcome    string  `json:"outcome"`
	WallTimeMs float64 `json:"wallTimeMs"`
	CPUTimeMs  float64 `json:"cpuTimeMs"`
}

type CFExportWorkersEvent struct {
	Request struct {
		URL     string            `json:"url"`
		Path    string            `json:"path"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		CF      struct {
			Colo    string `json:"colo"`
			Country string `json:"country"`
			ASN     int    `json:"asn"`
		} `json:"cf"`
	} `json:"request"`
	Response struct {
		Status int `json:"status"`
	} `json:"response"`
}

type CFExportMetadata struct {
	ID        string `json:"id"`
	RequestID string `json:"requestId"`
	RayID     string `json:"rayId"`
	TraceID   string `json:"traceId"`
	Level     string `json:"level"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Trigger   string `json:"trigger"`
	Service   string `json:"service"`
}

// ErrRecord is the compact, merged, classified representation of one failing request.
type ErrRecord struct {
	TS              string  `json:"ts"`
	RequestID       string  `json:"requestId"`
	RayID           string  `json:"rayId"`
	Method          string  `json:"method,omitempty"`
	URL             string  `json:"url"`
	Path            string  `json:"path"`
	Status          int     `json:"status"`
	Outcome         string  `json:"outcome,omitempty"`
	SegmentPrefetch string  `json:"segmentPrefetch,omitempty"`
	Colo            string  `json:"colo,omitempty"`
	Country         string  `json:"country,omitempty"`
	ASN             int     `json:"asn,omitempty"`
	ScriptVer       string  `json:"scriptVersion,omitempty"`
	Owner           Owner   `json:"owner"`
	Code            string  `json:"code"`
	Summary         string  `json:"summary"`
	Hint            string  `json:"hint"`
	Message         string  `json:"message"`
	WallTimeMs      float64 `json:"wallTimeMs,omitempty"`
	CPUTimeMs       float64 `json:"cpuTimeMs,omitempty"`
}

// CountItem holds a label and occurrence count for ranking tables.
type CountItem struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// IssueGroup aggregates distinct failing requests sharing the same (Owner, Code).
type IssueGroup struct {
	Owner        Owner    `json:"owner"`
	Code         string   `json:"code"`
	Summary      string   `json:"summary"`
	Hint         string   `json:"hint"`
	Count        int      `json:"count"`
	SampleRoutes []string `json:"sampleRoutes"`
	ExtraRoutes  int      `json:"extraRoutes,omitempty"`
	SampleDetail string   `json:"sampleDetail,omitempty"`
}

// AnalysisReport summarizes a log stream or exported log file.
type AnalysisReport struct {
	TotalRawEvents   int           `json:"totalRawEvents"`
	DistinctRequests int           `json:"distinctRequests"`
	Truncated        bool          `json:"truncated,omitempty"`
	ByOwner          map[Owner]int `json:"byOwner"`
	Groups           []IssueGroup  `json:"groups"`
	TopPathStatus    []CountItem   `json:"topPathStatus"`
	TopCountries     []CountItem   `json:"topCountries"`
	ByMinute         []CountItem   `json:"byMinute"`
	Records          []ErrRecord   `json:"records,omitempty"`
}

// AnalyzeOptions configures streaming filtering and optional NDJSON output.
type AnalyzeOptions struct {
	RouteFilter     string
	OwnerFilter     Owner
	AllLevels       bool
	RawNDJSONOut    io.Writer
	MergedNDJSONOut io.Writer
}

// StreamCFEntries streams a JSON array ([...]) or NDJSON stream of Cloudflare log objects
// in constant memory, tolerating a truncated tail cleanly.
func StreamCFEntries(r io.Reader, fn func(CFExportEntry) error) (truncated bool, err error) {
	br := bufio.NewReaderSize(r, 1<<20) // 1 MB read buffer

	firstByte, err := peekFirstNonSpace(br)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}

	dec := json.NewDecoder(br)

	if firstByte == '[' {
		tok, err := dec.Token()
		if err != nil {
			if isTruncatedJSONErr(err, false) {
				return true, nil
			}
			return false, err
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '[' {
			return false, fmt.Errorf("expected '[' array start, got %v", tok)
		}

		decodedAny := false
		for dec.More() {
			var e CFExportEntry
			if err := dec.Decode(&e); err != nil {
				if isTruncatedJSONErr(err, decodedAny) {
					return true, nil
				}
				return false, err
			}
			decodedAny = true
			if fn != nil {
				if err := fn(e); err != nil {
					return false, err
				}
			}
		}

		// Consume closing ']' if present; if missing, treat as truncated tail.
		if _, err := dec.Token(); err != nil && decodedAny {
			if isTruncatedJSONErr(err, true) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}

	// NDJSON (line-delimited JSON objects)
	decodedAny := false
	for {
		var e CFExportEntry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return false, nil
			}
			if isTruncatedJSONErr(err, decodedAny) {
				return true, nil
			}
			return false, err
		}
		decodedAny = true
		if fn != nil {
			if err := fn(e); err != nil {
				return false, err
			}
		}
	}
}

func peekFirstNonSpace(br *bufio.Reader) (byte, error) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			if err := br.UnreadByte(); err != nil {
				return 0, err
			}
			return b, nil
		}
	}
}

func isTruncatedJSONErr(err error, decodedAny bool) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if decodedAny {
		var synErr *json.SyntaxError
		if errors.As(err, &synErr) {
			return true
		}
	}
	return false
}

type pendingRequest struct {
	rec      ErrRecord
	headers  map[string]string
	messages []string
	seenMsg  map[string]struct{}
}

// StreamCollector merges multi-fragment Cloudflare log events by requestId and produces an AnalysisReport.
type StreamCollector struct {
	opts           AnalyzeOptions
	totalRawEvents int
	truncated      bool
	syntheticSeq   int
	order          []string
	byID           map[string]*pendingRequest
	rawEnc         *json.Encoder
}

// NewStreamCollector constructs a StreamCollector with the provided AnalyzeOptions.
func NewStreamCollector(opts AnalyzeOptions) *StreamCollector {
	var rawEnc *json.Encoder
	if opts.RawNDJSONOut != nil {
		rawEnc = json.NewEncoder(opts.RawNDJSONOut)
	}
	return &StreamCollector{
		opts:   opts,
		byID:   make(map[string]*pendingRequest),
		rawEnc: rawEnc,
	}
}

// SetTruncated marks whether the underlying input stream terminated mid-JSON.
func (c *StreamCollector) SetTruncated(t bool) {
	c.truncated = t
}

// Add ingests a single CFExportEntry into the collector.
func (c *StreamCollector) Add(e CFExportEntry) {
	if !c.opts.AllLevels && !isErrorEntry(e) {
		return
	}

	c.totalRawEvents++

	reqID := firstNonEmpty(e.Metadata.RequestID, e.Workers.RequestIDCompat())
	if reqID == "" {
		c.syntheticSeq++
		reqID = "_anon_" + strconv.Itoa(c.syntheticSeq)
	}

	msg := firstNonEmpty(e.Source.MessageString(), e.Metadata.Error, e.Metadata.Message)

	if c.rawEnc != nil {
		req := e.Workers.Event.Request
		flat := ErrRecord{
			TS:              string(e.Timestamp),
			RequestID:       reqID,
			RayID:           e.Metadata.RayID,
			Method:          req.Method,
			URL:             req.URL,
			Path:            req.Path,
			Status:          e.Workers.Event.Response.Status,
			Outcome:         e.Workers.Outcome,
			SegmentPrefetch: headerLookup(req.Headers, "next-router-segment-prefetch"),
			Colo:            req.CF.Colo,
			Country:         req.CF.Country,
			ASN:             req.CF.ASN,
			ScriptVer:       e.Workers.ScriptVersion.ID,
			Message:         msg,
			WallTimeMs:      e.Workers.WallTimeMs,
			CPUTimeMs:       e.Workers.CPUTimeMs,
		}
		_ = c.rawEnc.Encode(flat)
	}

	req := e.Workers.Event.Request
	pr, exists := c.byID[reqID]
	if !exists {
		pr = &pendingRequest{
			rec: ErrRecord{
				TS:              string(e.Timestamp),
				RequestID:       reqID,
				RayID:           e.Metadata.RayID,
				Method:          req.Method,
				URL:             req.URL,
				Path:            req.Path,
				Status:          e.Workers.Event.Response.Status,
				Outcome:         e.Workers.Outcome,
				SegmentPrefetch: headerLookup(req.Headers, "next-router-segment-prefetch"),
				Colo:            req.CF.Colo,
				Country:         req.CF.Country,
				ASN:             req.CF.ASN,
				ScriptVer:       e.Workers.ScriptVersion.ID,
				WallTimeMs:      e.Workers.WallTimeMs,
				CPUTimeMs:       e.Workers.CPUTimeMs,
			},
			headers: copyHeaders(req.Headers),
			seenMsg: make(map[string]struct{}),
		}
		c.byID[reqID] = pr
		c.order = append(c.order, reqID)
	} else {
		if pr.rec.TS == "" && e.Timestamp != "" {
			pr.rec.TS = string(e.Timestamp)
		}
		if pr.rec.RayID == "" && e.Metadata.RayID != "" {
			pr.rec.RayID = e.Metadata.RayID
		}
		if pr.rec.URL == "" && req.URL != "" {
			pr.rec.URL = req.URL
		}
		if pr.rec.Path == "" && req.Path != "" {
			pr.rec.Path = req.Path
		}
		if pr.rec.Method == "" && req.Method != "" {
			pr.rec.Method = req.Method
		}
		if pr.rec.Status == 0 && e.Workers.Event.Response.Status != 0 {
			pr.rec.Status = e.Workers.Event.Response.Status
		}
		if pr.rec.Outcome == "" && e.Workers.Outcome != "" {
			pr.rec.Outcome = e.Workers.Outcome
		}
		if pr.rec.Colo == "" && req.CF.Colo != "" {
			pr.rec.Colo = req.CF.Colo
		}
		if pr.rec.Country == "" && req.CF.Country != "" {
			pr.rec.Country = req.CF.Country
		}
		if pr.rec.ASN == 0 && req.CF.ASN != 0 {
			pr.rec.ASN = req.CF.ASN
		}
		if pr.rec.WallTimeMs == 0 && e.Workers.WallTimeMs != 0 {
			pr.rec.WallTimeMs = e.Workers.WallTimeMs
		}
		if pr.rec.CPUTimeMs == 0 && e.Workers.CPUTimeMs != 0 {
			pr.rec.CPUTimeMs = e.Workers.CPUTimeMs
		}
		if pr.rec.SegmentPrefetch == "" {
			pr.rec.SegmentPrefetch = headerLookup(req.Headers, "next-router-segment-prefetch")
		}
		if len(req.Headers) > 0 {
			if pr.headers == nil {
				pr.headers = make(map[string]string, len(req.Headers))
			}
			for k, v := range req.Headers {
				if _, ok := pr.headers[k]; !ok {
					pr.headers[k] = v
				}
			}
		}
	}

	if msg != "" && len(pr.messages) < 16 {
		if _, seen := pr.seenMsg[msg]; !seen {
			pr.seenMsg[msg] = struct{}{}
			pr.messages = append(pr.messages, msg)
		}
	}
}

// RequestIDCompat returns an empty string (Workers metadata requestId lives under $metadata.requestId in exports).
func (w CFExportWorkers) RequestIDCompat() string {
	return ""
}

func isErrorEntry(e CFExportEntry) bool {
	lvl := strings.ToLower(firstNonEmpty(e.Source.Level, e.Metadata.Level))
	if lvl == "error" || lvl == "fatal" {
		return true
	}
	if e.Workers.Event.Response.Status >= 400 {
		return true
	}
	if e.Workers.Outcome != "" && e.Workers.Outcome != "ok" {
		return true
	}
	if e.Metadata.Error != "" {
		return true
	}
	return false
}

// Finalize classifies all merged requests, applies filters, emits MergedNDJSONOut, and returns the report.
func (c *StreamCollector) Finalize() *AnalysisReport {
	rep := &AnalysisReport{
		TotalRawEvents: c.totalRawEvents,
		Truncated:      c.truncated,
		ByOwner:        make(map[Owner]int),
	}

	type groupAcc struct {
		group      IssueGroup
		routeCount map[string]int
	}
	groupsMap := make(map[string]*groupAcc)
	pathStatusMap := make(map[string]int)
	countryMap := make(map[string]int)
	minuteMap := make(map[string]int)

	var mergedEnc *json.Encoder
	if c.opts.MergedNDJSONOut != nil {
		mergedEnc = json.NewEncoder(c.opts.MergedNDJSONOut)
	}

	for _, id := range c.order {
		pr := c.byID[id]
		rec := pr.rec

		if rec.Path == "" && rec.URL != "" {
			rec.Path = extractPathFromURL(rec.URL)
		}

		if c.opts.RouteFilter != "" {
			if !strings.Contains(rec.URL, c.opts.RouteFilter) && !strings.Contains(rec.Path, c.opts.RouteFilter) {
				continue
			}
		}

		deduped := DedupeMessages(pr.messages)
		sig := Signal{
			Method:   rec.Method,
			Path:     rec.Path,
			URL:      rec.URL,
			Status:   rec.Status,
			Outcome:  rec.Outcome,
			Headers:  pr.headers,
			Messages: deduped,
		}
		diag := Classify(sig)

		if c.opts.OwnerFilter != "" && diag.Owner != c.opts.OwnerFilter {
			continue
		}

		rec.Owner = diag.Owner
		rec.Code = diag.Code
		rec.Summary = diag.Summary
		rec.Hint = diag.Hint
		rec.Message = BestDetail(deduped)
		if rec.Message == "" && len(deduped) > 0 {
			rec.Message = deduped[0]
		}

		if mergedEnc != nil {
			_ = mergedEnc.Encode(rec)
		}

		rep.Records = append(rep.Records, rec)
		rep.DistinctRequests++
		rep.ByOwner[diag.Owner]++

		gKey := string(diag.Owner) + "|" + diag.Code
		ga, ok := groupsMap[gKey]
		if !ok {
			ga = &groupAcc{
				group: IssueGroup{
					Owner:        diag.Owner,
					Code:         diag.Code,
					Summary:      diag.Summary,
					Hint:         diag.Hint,
					SampleDetail: rec.Message,
				},
				routeCount: make(map[string]int),
			}
			groupsMap[gKey] = ga
		}
		ga.group.Count++
		if ga.group.SampleDetail == "" && rec.Message != "" {
			ga.group.SampleDetail = rec.Message
		}
		routeLabel := firstNonEmpty(rec.Path, rec.URL, "(unknown)")
		ga.routeCount[routeLabel]++

		psKey := fmt.Sprintf("%s %d", routeLabel, rec.Status)
		pathStatusMap[psKey]++

		country := firstNonEmpty(rec.Country, "(unknown)")
		countryMap[country]++

		if len(rec.TS) >= 16 {
			minuteMap[rec.TS[:16]]++
		}
	}

	for _, ga := range groupsMap {
		routes := rankCounts(ga.routeCount, 5)
		sample := make([]string, len(routes))
		for i, r := range routes {
			sample[i] = r.Key
		}
		ga.group.SampleRoutes = sample
		if len(ga.routeCount) > len(routes) {
			ga.group.ExtraRoutes = len(ga.routeCount) - len(routes)
		}
		rep.Groups = append(rep.Groups, ga.group)
	}
	sort.Slice(rep.Groups, func(i, j int) bool {
		if rep.Groups[i].Count != rep.Groups[j].Count {
			return rep.Groups[i].Count > rep.Groups[j].Count
		}
		return rep.Groups[i].Code < rep.Groups[j].Code
	})

	rep.TopPathStatus = rankCounts(pathStatusMap, 0)
	rep.TopCountries = rankCounts(countryMap, 0)
	rep.ByMinute = rankMinutesChrono(minuteMap)

	return rep
}

// AnalyzeReader streams JSON/NDJSON Cloudflare logs from r and returns an AnalysisReport.
func AnalyzeReader(r io.Reader, opts AnalyzeOptions) (*AnalysisReport, error) {
	col := NewStreamCollector(opts)
	truncated, err := StreamCFEntries(r, func(e CFExportEntry) error {
		col.Add(e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	col.SetTruncated(truncated)
	return col.Finalize(), nil
}

// AnalyzeFile opens path and analyzes it in constant streaming memory.
func AnalyzeFile(path string, opts AnalyzeOptions) (*AnalysisReport, error) {
	f, err := os.Open(path) // #nosec G304 — user-requested log file path
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return AnalyzeReader(f, opts)
}

// FormatReport writes a concise, ownership-focused triage report to w.
func FormatReport(w io.Writer, rep *AnalysisReport, topN int) {
	if topN <= 0 {
		topN = 15
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "\033[1;36mNextDeploy Log Triage\033[0m — %d failing requests (from %d raw error events)\n",
		rep.DistinctRequests, rep.TotalRawEvents)
	if rep.Truncated {
		fmt.Fprintln(w, "\033[33mNote: input JSON had a truncated tail; recovered all complete events before EOF.\033[0m")
	}
	fmt.Fprintln(w, "\033[90m────────────────────────────────────────────────────────────────────────\033[0m")

	if rep.DistinctRequests == 0 {
		fmt.Fprintln(w, "No matching errors found.")
		return
	}

	fmt.Fprintln(w, "\033[1mWHO OWNS THESE ERRORS?\033[0m")
	ownerRows := []struct {
		owner Owner
		desc  string
	}{
		{OwnerApp, "Your Next.js app code / imports / route logic"},
		{OwnerNextDeploy, "NextDeploy Worker runtime / adapter / R2 or KV glue"},
		{OwnerPlatform, "Cloudflare Workers CPU / memory / edge limits"},
		{OwnerUnknown, "Unclassified errors (inspect with --raw)"},
	}
	for _, row := range ownerRows {
		cnt := rep.ByOwner[row.owner]
		if cnt == 0 {
			continue
		}
		pct := float64(cnt) / float64(rep.DistinctRequests) * 100
		fmt.Fprintf(w, "  %-22s %6d (%5.1f%%)  %s\n", OwnerBadge(row.owner), cnt, pct, row.desc)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "\033[1mTOP ISSUES (grouped by cause)\033[0m")
	limit := len(rep.Groups)
	if limit > topN {
		limit = topN
	}
	for i := 0; i < limit; i++ {
		g := rep.Groups[i]
		routeStr := strings.Join(g.SampleRoutes, ", ")
		if g.ExtraRoutes > 0 {
			routeStr += fmt.Sprintf(" (+%d more)", g.ExtraRoutes)
		}
		fmt.Fprintf(w, "  %s %5dx  %s\n", OwnerBadge(g.Owner), g.Count, g.Summary)
		if routeStr != "" {
			fmt.Fprintf(w, "     \033[90mRoutes:\033[0m %s\n", routeStr)
		}
		if g.SampleDetail != "" {
			fmt.Fprintf(w, "     \033[90mDetail:\033[0m %s\n", g.SampleDetail)
		}
		if g.Hint != "" {
			fmt.Fprintf(w, "     \033[32mFix:\033[0m    %s\n", g.Hint)
		}
	}

	printCountSection(w, "TOP FAILING ROUTES", rep.TopPathStatus, topN)
	printCountSection(w, "TOP COUNTRIES", rep.TopCountries, topN)
	printCountSection(w, "ERROR TIMELINE (UTC minute)", rep.ByMinute, topN)
	fmt.Fprintln(w)
}

func printCountSection(w io.Writer, title string, items []CountItem, topN int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "\033[1m%s\033[0m\n", title)
	n := len(items)
	if n > topN {
		n = topN
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(w, "  %6d  %s\n", items[i].Count, items[i].Key)
	}
}

func rankCounts(m map[string]int, limit int) []CountItem {
	out := make([]CountItem, 0, len(m))
	for k, v := range m {
		out = append(out, CountItem{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func rankMinutesChrono(m map[string]int) []CountItem {
	out := make([]CountItem, 0, len(m))
	for k, v := range m {
		out = append(out, CountItem{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func headerLookup(headers map[string]string, name string) string {
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

func copyHeaders(h map[string]string) map[string]string {
	if len(h) == 0 {
		return nil
	}
	cp := make(map[string]string, len(h))
	for k, v := range h {
		cp[k] = v
	}
	return cp
}

func extractPathFromURL(raw string) string {
	idx := strings.Index(raw, "://")
	if idx >= 0 {
		rest := raw[idx+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			p := rest[slash:]
			if q := strings.IndexByte(p, '?'); q >= 0 {
				return p[:q]
			}
			return p
		}
		return "/"
	}
	if q := strings.IndexByte(raw, '?'); q >= 0 {
		return raw[:q]
	}
	return raw
}
