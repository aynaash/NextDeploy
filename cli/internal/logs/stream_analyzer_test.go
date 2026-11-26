package logs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const sampleCFExportWithTruncatedTail = `[
  {
    "source": {
      "level": "error",
      "message": "GET https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb"
    },
    "timestamp": "2026-11-26T13:33:16.836Z",
    "$workers": {
      "event": {
        "request": {
          "cf": { "colo": "OTP", "country": "KE", "asn": 33771 },
          "headers": {
            "next-router-prefetch": "1",
            "next-router-segment-prefetch": "/__PAGE__"
          },
          "method": "GET",
          "url": "https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb"
        },
        "path": "/",
        "response": { "status": 500 }
      },
      "scriptVersion": { "id": "f3bfb4fd" },
      "outcome": "ok",
      "wallTimeMs": 129,
      "cpuTimeMs": 9
    },
    "$metadata": {
      "id": "01M8C0YY540000000000000008",
      "requestId": "req-rsc-1",
      "rayId": "ray-1"
    }
  },
  {
    "source": {
      "level": "error",
      "message": "⨯ \n    at worker.mjs:5804:5403"
    },
    "timestamp": "2026-11-26T13:33:16.836Z",
    "$workers": {
      "event": { "request": { "method": "GET", "url": "https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb", "path": "/" } }
    },
    "$metadata": {
      "id": "01M8C0YY540000000000000006",
      "requestId": "req-rsc-1",
      "rayId": "ray-1"
    }
  },
  {
    "source": {
      "level": "error",
      "message": "⨯     at worker.mjs:5539:9298\n    at async NextNodeServer.renderToResponseWithComponentsImpl (worker.mjs:5791:42997)"
    },
    "timestamp": "2026-11-26T13:33:16.836Z",
    "$workers": {
      "event": { "request": { "method": "GET", "url": "https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb", "path": "/" } }
    },
    "$metadata": {
      "id": "01M8C0YY540000000000000005",
      "requestId": "req-rsc-1",
      "rayId": "ray-1"
    }
  },
  {
    "source": {
      "level": "error",
      "message": "    at worker.mjs:5539:9298\n    at async NextNodeServer.renderToResponseWithComponentsImpl (worker.mjs:5791:42997)"
    },
    "timestamp": "2026-11-26T13:33:16.836Z",
    "$workers": {
      "event": { "request": { "method": "GET", "url": "https://nextdeploy.org/?_rsc=LJGu_tk5iPI2Zyzb", "path": "/" } }
    },
    "$metadata": {
      "id": "01M8C0YY540000000000000004",
      "requestId": "req-rsc-1",
      "rayId": "ray-1"
    }
  },
  {
    "source": {
      "level": "error",
      "message": "GET https://nextdeploy.org/docs/concepts/security?_rsc=CKPo2T6QqhkmtnA0"
    },
    "timestamp": "2026-11-26T13:32:59.863Z",
    "$workers": {
      "event": {
        "request": {
          "cf": { "colo": "OTP", "country": "KE", "asn": 33771 },
          "headers": {
            "next-router-prefetch": "1",
            "next-router-segment-prefetch": "/_tree"
          },
          "method": "GET",
          "url": "https://nextdeploy.org/docs/concepts/security?_rsc=CKPo2T6QqhkmtnA0"
        },
        "response": { "status": 500 }
      },
      "outcome": "ok",
      "wallTimeMs": 168,
      "cpuTimeMs": 7
    },
    "$metadata": {
      "id": "01M8C0YDJQ000000000000000C",
      "requestId": "req-docs-1",
      "rayId": "ray-2"
    }
  },
  {
    "source": {
      "level": "error",
      "message": "⨯     at Context.externalRequire [as x] (worker.mjs:3:12269)\n    at module evaluation (worker.mjs:3062:22882)"
    },
    "timestamp": "2026-11-26T13:32:59.863Z",
    "$workers": {
      "event": { "request": { "method": "GET", "url": "https://nextdeploy.org/docs/concepts/security?_rsc=CKPo2T6QqhkmtnA0" } }
    },
    "$metadata": {
      "id": "01M8C0YDJQ0000000000000007",
      "requestId": "req-docs-1",
      "rayId": "ray-2"
    }
  },
  {
    "source": {
      "level": "error",
      "message": "GET https://nextdeploy.org/api/health"
    },
    "timestamp": 1795699999000,
    "$workers": {
      "event": { "request": { "method": "GET", "url": "https://nextdeploy.org/api/health" }, "response": { "status": 200 } },
      "outcome": "ok"
    },
    "$metadata": {
      "requestId": "req-truncated-unfinished"`

func TestAnalyzeReader_CorrelatesAndSurvivesTruncatedTail(t *testing.T) {
	var ndjson bytes.Buffer
	rep, err := AnalyzeReader(strings.NewReader(sampleCFExportWithTruncatedTail), AnalyzeOptions{
		MergedNDJSONOut: &ndjson,
	})
	if err != nil {
		t.Fatalf("AnalyzeReader error: %v", err)
	}

	if !rep.Truncated {
		t.Errorf("expected Truncated=true for cut-off JSON array")
	}
	if rep.TotalRawEvents != 6 {
		t.Errorf("TotalRawEvents = %d, want 6", rep.TotalRawEvents)
	}
	if rep.DistinctRequests != 2 {
		t.Fatalf("DistinctRequests = %d, want 2", rep.DistinctRequests)
	}
	if rep.ByOwner[OwnerNextDeploy] != 1 || rep.ByOwner[OwnerApp] != 1 {
		t.Errorf("unexpected ByOwner: %+v", rep.ByOwner)
	}

	// Verify merged records carry URL, status, colo/country/ASN, and classified owner.
	byID := map[string]ErrRecord{}
	for _, r := range rep.Records {
		byID[r.RequestID] = r
	}
	if rsc := byID["req-rsc-1"]; rsc.Owner != OwnerNextDeploy || rsc.Status != 500 || rsc.Country != "KE" || rsc.ASN != 33771 {
		t.Errorf("unexpected merged req-rsc-1 record: %+v", rsc)
	}
	if docs := byID["req-docs-1"]; docs.Owner != OwnerApp || docs.Path != "/docs/concepts/security" || !strings.Contains(docs.Message, "externalRequire") {
		t.Errorf("unexpected merged req-docs-1 record: %+v", docs)
	}

	// Verify NDJSON output has exactly 2 valid JSON lines.
	lines := strings.Split(strings.TrimSpace(ndjson.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d:\n%s", len(lines), ndjson.String())
	}
	for i, ln := range lines {
		var rec ErrRecord
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			t.Errorf("line %d invalid JSON: %v", i, err)
		}
	}

	var reportOut bytes.Buffer
	FormatReport(&reportOut, rep, 10)
	text := reportOut.String()
	for _, want := range []string{"WHO OWNS THESE ERRORS?", "[APP]", "[NEXTDEPLOY]", "TOP ISSUES", "/docs/concepts/security"} {
		if !strings.Contains(text, want) {
			t.Errorf("FormatReport missing %q in:\n%s", want, text)
		}
	}
}

func TestAnalyzeReader_NDJSONAndFilters(t *testing.T) {
	ndjsonInput := strings.Join([]string{
		`{"source":{"level":"info","message":"ok"},"timestamp":1795699980000,"$workers":{"event":{"request":{"method":"GET","url":"https://nextdeploy.org/ok","cf":{"country":"US"}},"response":{"status":200}},"outcome":"ok"},"$metadata":{"requestId":"r1"}}`,
		`{"source":{"level":"error","message":"TypeError: boom"},"timestamp":1795699985000,"$workers":{"event":{"request":{"method":"POST","url":"https://nextdeploy.org/api/submit","cf":{"country":"DE"}},"response":{"status":500}},"outcome":"exception"},"$metadata":{"requestId":"r2"}}`,
		`{"source":{"level":"error","message":"[nextcompile cacheHandler R2 put failed] err"},"timestamp":"2026-11-26T13:33:16.836Z","$workers":{"event":{"request":{"method":"GET","url":"https://nextdeploy.org/routes/isr","cf":{"country":"KE"}},"response":{"status":500}},"outcome":"ok"},"$metadata":{"requestId":"r3"}}`,
	}, "\n")

	rep, err := AnalyzeReader(strings.NewReader(ndjsonInput), AnalyzeOptions{
		OwnerFilter: OwnerApp,
	})
	if err != nil {
		t.Fatalf("AnalyzeReader NDJSON error: %v", err)
	}
	if rep.Truncated {
		t.Errorf("expected Truncated=false for valid NDJSON")
	}
	if rep.DistinctRequests != 1 {
		t.Fatalf("expected 1 APP error after OwnerFilter, got %d", rep.DistinctRequests)
	}
	if rep.Records[0].RequestID != "r2" || !strings.HasPrefix(rep.Records[0].TS, "2026-") {
		t.Errorf("unexpected filtered record: %+v", rep.Records[0])
	}

	routeRep, err := AnalyzeReader(strings.NewReader(ndjsonInput), AnalyzeOptions{
		RouteFilter: "/routes/isr",
	})
	if err != nil {
		t.Fatalf("AnalyzeReader RouteFilter error: %v", err)
	}
	if routeRep.DistinctRequests != 1 || routeRep.Records[0].Owner != OwnerNextDeploy {
		t.Errorf("unexpected RouteFilter result: %+v", routeRep.Records)
	}
}
