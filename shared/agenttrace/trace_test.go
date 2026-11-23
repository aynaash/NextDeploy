package agenttrace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectHandoff(t *testing.T) {
	tmp := t.TempDir()
	if DetectHandoff(tmp) {
		t.Fatal("expected DetectHandoff to be false for empty directory")
	}

	handoffDir := filepath.Join(tmp, LegacyHandoffDir)
	if err := os.Mkdir(handoffDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if !DetectHandoff(tmp) {
		t.Fatal("expected DetectHandoff to be true when .handoff exists")
	}
}

func TestDetectTraceNative(t *testing.T) {
	tmp := t.TempDir()
	ndDir := filepath.Join(tmp, NextDeployTraceDir)
	if err := os.MkdirAll(ndDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if !DetectTrace(tmp) {
		t.Fatal("expected DetectTrace to be true when .nextdeploy/trace exists")
	}
}

func TestReadLatestCheckpoint(t *testing.T) {
	tmp := t.TempDir()
	handoffDir := filepath.Join(tmp, LegacyHandoffDir)
	_ = os.Mkdir(handoffDir, 0o755)

	cp := HandoffCheckpoint{
		SessionID: "sess-123",
		Agent:     "antigravity",
		Model:     "gemini-3.8-flash",
		Objective: "Build and deploy Next.js auth routes",
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}

	data, err := json.Marshal(cp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	cpFile := filepath.Join(handoffDir, CheckpointFile)
	if err := os.WriteFile(cpFile, data, 0o644); err != nil {
		t.Fatalf("write checkpoint failed: %v", err)
	}

	read, err := ReadLatestCheckpoint(tmp)
	if err != nil {
		t.Fatalf("ReadLatestCheckpoint failed: %v", err)
	}

	if read.SessionID != "sess-123" || read.Model != "gemini-3.8-flash" {
		t.Errorf("unexpected checkpoint content: %+v", read)
	}
}

func TestSaveCheckpointNative(t *testing.T) {
	tmp := t.TempDir()
	cp := HandoffCheckpoint{
		SessionID: "sess-456",
		Agent:     "claude",
		Model:     "claude-3-7-sonnet",
		Objective: "Implement Stripe webhooks",
	}

	if err := SaveCheckpoint(tmp, cp); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	read, err := ReadLatestCheckpoint(tmp)
	if err != nil {
		t.Fatalf("ReadLatestCheckpoint failed: %v", err)
	}

	if read.SessionID != "sess-456" || read.Agent != "claude" {
		t.Errorf("unexpected read checkpoint: %+v", read)
	}

	brief, err := GenerateTakeoverBrief(tmp)
	if err != nil {
		t.Fatalf("GenerateTakeoverBrief failed: %v", err)
	}
	if !strings.Contains(brief, "Implement Stripe webhooks") {
		t.Errorf("expected brief to contain objective: %s", brief)
	}
}

func TestRecordEvent(t *testing.T) {
	tmp := t.TempDir()
	handoffDir := filepath.Join(tmp, LegacyHandoffDir)
	_ = os.Mkdir(handoffDir, 0o755)

	event := HandoffEvent{
		Type:        "nextdeploy_ship_success",
		Target:      "cloudflare",
		Environment: "production",
		LiveURL:     "https://app.workers.dev",
	}

	if err := RecordEvent(tmp, event); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	eventsPath := filepath.Join(handoffDir, EventsFile)
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events failed: %v", err)
	}

	var events []HandoffEvent
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshal events failed: %v", err)
	}

	if len(events) != 1 || events[0].LiveURL != "https://app.workers.dev" {
		t.Errorf("unexpected events: %+v", events)
	}
}

func TestUpdateTakeoverBrief(t *testing.T) {
	tmp := t.TempDir()
	handoffDir := filepath.Join(tmp, LegacyHandoffDir)
	_ = os.Mkdir(handoffDir, 0o755)

	initialBrief := "# Takeover Brief\n\nAgent working on Stripe webhook.\n"
	_ = os.WriteFile(filepath.Join(handoffDir, LatestBriefFile), []byte(initialBrief), 0o644)

	err := UpdateTakeoverBrief(tmp, "cloudflare", "preview", "FAILED", "", errors.New("missing STRIPE_SECRET"))
	if err != nil {
		t.Fatalf("UpdateTakeoverBrief failed: %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(handoffDir, LatestBriefFile))
	if err != nil {
		t.Fatalf("read latest.md failed: %v", err)
	}

	content := string(updated)
	if !strings.Contains(content, "NextDeploy Infrastructure Status") {
		t.Errorf("missing section header in latest.md: %s", content)
	}
	if !strings.Contains(content, "missing STRIPE_SECRET") {
		t.Errorf("missing failure text in latest.md: %s", content)
	}
}
