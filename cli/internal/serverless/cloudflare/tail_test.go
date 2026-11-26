package cloudflare

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aynaash/nextdeploy/cli/internal/logs"
	"github.com/cloudflare/cloudflare-go/v6/workers"
)

func TestFormatTailEvent_SimpleVsRawAndFilters(t *testing.T) {
	ev := &CFTailMessage{
		Outcome:          "ok",
		EventTimestampMs: 1795699996836,
		WallTimeMs:       129,
		Event: &CFTailEventDetail{
			Request: &CFTailRequest{
				Method: "GET",
				URL:    "https://nextdeploy.org/docs/concepts/security?_rsc=abc",
				Headers: map[string]string{
					"next-router-prefetch":         "1",
					"next-router-segment-prefetch": "/_tree",
				},
			},
			Response: &CFTailResponse{Status: 500},
		},
		Logs: []CFTailLogEntry{
			{Level: "error", Message: "GET https://nextdeploy.org/docs/concepts/security?_rsc=abc"},
			{Level: "error", Message: "⨯     at Context.externalRequire [as x] (worker.mjs:3:12269)\n    at module evaluation (worker.mjs:3062:22882)"},
			{Level: "error", Message: "    at Context.externalRequire [as x] (worker.mjs:3:12269)\n    at module evaluation (worker.mjs:3062:22882)"},
		},
	}

	// Simple mode: shows [APP] badge, Cause, Hint, and 1 deduplicated Detail line.
	var simpleBuf, ndjsonBuf bytes.Buffer
	formatTailEvent(&simpleBuf, ev, TailOptions{
		QuietAssets: true,
		NDJSONOut:   &ndjsonBuf,
	})
	simpleOut := simpleBuf.String()
	for _, want := range []string{"[APP]", "Cause:", "Hint:", "Detail:", "externalRequire"} {
		if !strings.Contains(simpleOut, want) {
			t.Errorf("simple tail output missing %q:\n%s", want, simpleOut)
		}
	}
	if strings.Count(simpleOut, "Detail:") != 1 || strings.Count(simpleOut, "worker.mjs:3:12269") != 1 {
		t.Errorf("expected duplicate stack fragments to be collapsed into 1 Detail line in simple mode:\n%s", simpleOut)
	}

	var rec logs.ErrRecord
	if err := json.Unmarshal(bytes.TrimSpace(ndjsonBuf.Bytes()), &rec); err != nil {
		t.Fatalf("invalid NDJSON output: %v", err)
	}
	if rec.Owner != logs.OwnerApp || rec.Status != 500 {
		t.Errorf("unexpected NDJSON record: %+v", rec)
	}

	// Raw mode prints all log entries verbatim alongside the owner badge.
	var rawBuf bytes.Buffer
	formatTailEvent(&rawBuf, ev, TailOptions{Raw: true})
	if strings.Count(rawBuf.String(), "[error]") != 3 {
		t.Errorf("raw mode should print all 3 log entries, got:\n%s", rawBuf.String())
	}

	// OwnerFilter excludes non-matching owners.
	var filteredBuf bytes.Buffer
	formatTailEvent(&filteredBuf, ev, TailOptions{OwnerFilter: logs.OwnerNextDeploy})
	if filteredBuf.Len() != 0 {
		t.Errorf("expected OwnerFilter=NEXTDEPLOY to skip APP event, got:\n%s", filteredBuf.String())
	}

	// QuietAssets suppresses 200 /_next/static/* noise in non-raw mode.
	assetEv := &CFTailMessage{
		Outcome: "ok",
		Event: &CFTailEventDetail{
			Request:  &CFTailRequest{Method: "GET", URL: "https://nextdeploy.org/_next/static/chunks/app.js"},
			Response: &CFTailResponse{Status: 200},
		},
	}
	var assetBuf bytes.Buffer
	formatTailEvent(&assetBuf, assetEv, TailOptions{QuietAssets: true})
	if assetBuf.Len() != 0 {
		t.Errorf("expected QuietAssets to hide static 200, got:\n%s", assetBuf.String())
	}
}

func TestTelemetryEventToExportEntry(t *testing.T) {
	ev := workers.ObservabilityTelemetryQueryResponseEventsEvent{
		Timestamp: 1795699996836,
		Dataset:   "cloudflare-workers",
		Source: map[string]any{
			"level":   "error",
			"message": "TypeError: boom",
		},
		Metadata: workers.ObservabilityTelemetryQueryResponseEventsEventsMetadata{
			ID:        "evt-1",
			RequestID: "req-1",
			Level:     "error",
			Error:     "TypeError: boom",
		},
		Workers: workers.ObservabilityTelemetryQueryResponseEventsEventsWorkers{
			ScriptName: "nextdeploy-frontend-production",
			Outcome:    "exception",
			WallTimeMs: 42.5,
			CPUTimeMs:  8.2,
			Event: map[string]any{
				"request": map[string]any{
					"method": "GET",
					"url":    "https://nextdeploy.org/api/boom",
					"cf": map[string]any{
						"colo":    "OTP",
						"country": "KE",
						"asn":     33771,
					},
				},
				"response": map[string]any{
					"status": 500,
				},
			},
		},
	}

	entry := telemetryEventToExportEntry(ev)
	if entry.Metadata.RequestID != "req-1" || entry.Source.Level != "error" || entry.Source.MessageString() != "TypeError: boom" {
		t.Fatalf("unexpected mapped entry metadata/source: %+v", entry)
	}
	if entry.Workers.Event.Response.Status != 500 || entry.Workers.Event.Request.CF.Country != "KE" {
		t.Fatalf("unexpected mapped worker event: %+v", entry.Workers)
	}
}
