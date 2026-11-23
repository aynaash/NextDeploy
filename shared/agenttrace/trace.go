package agenttrace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	NextDeployTraceDir = ".nextdeploy/trace"
	LegacyHandoffDir   = ".handoff"
	CheckpointFile     = "checkpoint.json"
	EventsFile         = "events.json"
	TakeoverFile       = "takeover.md"
	LatestBriefFile    = "latest.md"
)

// DetectHandoff checks if either .nextdeploy/trace or .handoff exists.
func DetectHandoff(projectDir string) bool {
	return DetectTrace(projectDir)
}

// DetectTrace returns true if an active trace directory exists in the workspace.
func DetectTrace(projectDir string) bool {
	ndDir := filepath.Join(projectDir, NextDeployTraceDir)
	if info, err := os.Stat(ndDir); err == nil && info.IsDir() {
		return true
	}
	hDir := filepath.Join(projectDir, LegacyHandoffDir)
	if info, err := os.Stat(hDir); err == nil && info.IsDir() {
		return true
	}
	return false
}

// GetTraceDir resolves the active trace directory (.nextdeploy/trace or .handoff).
func GetTraceDir(projectDir string) string {
	ndDir := filepath.Join(projectDir, NextDeployTraceDir)
	if info, err := os.Stat(ndDir); err == nil && info.IsDir() {
		return ndDir
	}
	hDir := filepath.Join(projectDir, LegacyHandoffDir)
	if info, err := os.Stat(hDir); err == nil && info.IsDir() {
		return hDir
	}
	return ndDir
}

// EnsureTraceDir ensures the trace directory exists, preferring .nextdeploy/trace.
func EnsureTraceDir(projectDir string) (string, error) {
	dir := GetTraceDir(projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create trace directory: %w", err)
	}
	return dir, nil
}

// ReadLatestCheckpoint reads and parses checkpoint.json from the active trace directory.
func ReadLatestCheckpoint(projectDir string) (*HandoffCheckpoint, error) {
	dir := GetTraceDir(projectDir)
	path := filepath.Join(dir, CheckpointFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cp HandoffCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("malformed checkpoint at %s: %w", path, err)
	}
	return &cp, nil
}

// SaveCheckpoint snapshots current agent intent, model, and git modified files.
func SaveCheckpoint(projectDir string, cp HandoffCheckpoint) error {
	dir, err := EnsureTraceDir(projectDir)
	if err != nil {
		return err
	}

	if cp.Timestamp.IsZero() {
		cp.Timestamp = time.Now().UTC()
	}
	if len(cp.ModifiedFiles) == 0 {
		cp.ModifiedFiles, _ = GetGitModifiedFiles(projectDir)
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}

	cpPath := filepath.Join(dir, CheckpointFile)
	if err := os.WriteFile(cpPath, data, 0o644); err != nil {
		return err
	}

	// Update takeover brief alongside checkpoint
	return UpdateTakeoverBrief(projectDir, "", "", "CHECKPOINT_SAVED", "", nil)
}

// ReadEvents reads all events from events.json in the trace directory.
func ReadEvents(projectDir string) ([]HandoffEvent, error) {
	dir := GetTraceDir(projectDir)
	eventsPath := filepath.Join(dir, EventsFile)
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		return nil, err
	}
	var events []HandoffEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// RecordEvent appends an infrastructure event to events.json.
func RecordEvent(projectDir string, event HandoffEvent) error {
	dir, err := EnsureTraceDir(projectDir)
	if err != nil {
		return err
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Source == "" {
		event.Source = "nextdeploy"
	}

	eventsPath := filepath.Join(dir, EventsFile)
	var events []HandoffEvent

	if data, err := os.ReadFile(eventsPath); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &events)
	}

	events = append(events, event)

	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(eventsPath, data, 0o644)
}

// UpdateTakeoverBrief appends or updates deployment status section in takeover.md and latest.md.
func UpdateTakeoverBrief(projectDir string, target, environment, status, liveURL string, deployErr error) error {
	dir, err := EnsureTraceDir(projectDir)
	if err != nil {
		return err
	}

	briefPath := filepath.Join(dir, TakeoverFile)
	latestPath := filepath.Join(dir, LatestBriefFile)

	cp, _ := ReadLatestCheckpoint(projectDir)
	modFiles, _ := GetGitModifiedFiles(projectDir)

	var sb strings.Builder
	sb.WriteString("# Developer Takeover Brief\n\n")

	sb.WriteString("## Active AI Coding Agent\n")
	if cp != nil {
		sb.WriteString(fmt.Sprintf("- **Agent & Model**: %s (%s)\n", cp.Agent, cp.Model))
		sb.WriteString(fmt.Sprintf("- **Session ID**: `%s`\n", cp.SessionID))
		sb.WriteString(fmt.Sprintf("- **Objective**: %s\n", cp.Objective))
		if cp.UnfinishedWork != "" {
			sb.WriteString(fmt.Sprintf("- **Unfinished Work**: %s\n", cp.UnfinishedWork))
		}
	} else {
		sb.WriteString("- **Status**: Interactive session in progress\n")
	}

	if target != "" || status != "" {
		sb.WriteString("\n## NextDeploy Infrastructure Status\n")
		sb.WriteString(fmt.Sprintf("- **Target**: `%s` (environment: `%s`)\n", target, environment))
		sb.WriteString(fmt.Sprintf("- **Status**: `%s` (at %s)\n", status, time.Now().UTC().Format(time.RFC3339)))
		if liveURL != "" {
			sb.WriteString(fmt.Sprintf("- **Deployment URL**: %s\n", liveURL))
		}
		if deployErr != nil {
			sb.WriteString(fmt.Sprintf("- **Failure**: `%v`\n", deployErr))
		}
	}

	sb.WriteString("\n## Modified Files & Git State\n")
	if len(modFiles) > 0 {
		for _, f := range modFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	} else {
		sb.WriteString("- Clean working directory (no uncommitted diffs)\n")
	}

	sb.WriteString("\n## Quick Actions\n")
	sb.WriteString("- Open touched files: `nextdeploy trace takeover -o`\n")
	sb.WriteString("- View local dashboard: `nextdeploy trace ui` (http://localhost:4044)\n")
	sb.WriteString("- Deploy changes: `nextdeploy ship`\n")

	content := sb.String()
	_ = os.WriteFile(briefPath, []byte(content), 0o644)
	return os.WriteFile(latestPath, []byte(content), 0o644)
}

// GenerateTakeoverBrief returns the scannable takeover brief markdown.
func GenerateTakeoverBrief(projectDir string) (string, error) {
	dir := GetTraceDir(projectDir)
	briefPath := filepath.Join(dir, TakeoverFile)
	data, err := os.ReadFile(briefPath)
	if err == nil && len(data) > 0 {
		return string(data), nil
	}
	latestPath := filepath.Join(dir, LatestBriefFile)
	data, err = os.ReadFile(latestPath)
	if err == nil && len(data) > 0 {
		return string(data), nil
	}

	// Generate fresh if neither exists
	_ = UpdateTakeoverBrief(projectDir, "", "", "IN_PROGRESS", "", nil)
	fresh, err := os.ReadFile(briefPath)
	if err != nil {
		return "", err
	}
	return string(fresh), nil
}

// OpenTakeoverInEditor launches $EDITOR with all modified files.
func OpenTakeoverInEditor(projectDir string, editorOverride string) error {
	files, err := GetGitModifiedFiles(projectDir)
	if err != nil || len(files) == 0 {
		cp, _ := ReadLatestCheckpoint(projectDir)
		if cp != nil && len(cp.ModifiedFiles) > 0 {
			files = cp.ModifiedFiles
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no modified files found to open")
	}

	editor := editorOverride
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		// Fallbacks: nvim, code, vim, nano
		for _, candidate := range []string{"nvim", "code", "vim", "nano"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found (set $EDITOR or specify --editor)")
	}

	args := append([]string{}, files...)
	cmd := exec.Command(editor, args...)
	cmd.Dir = projectDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetGitModifiedFiles parses `git status --porcelain` to return changed file paths.
func GetGitModifiedFiles(projectDir string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectDir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var files []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) > 3 {
			// Status is first 2 chars, followed by space, followed by filename
			filePart := strings.TrimSpace(l[2:])
			// In case of rename: orig -> new
			if idx := strings.Index(filePart, "->"); idx != -1 {
				filePart = strings.TrimSpace(filePart[idx+2:])
			}
			files = append(files, filePart)
		}
	}
	return files, nil
}

// GetGitBranch returns the current active git branch.
func GetGitBranch(projectDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "main", err
	}
	return strings.TrimSpace(string(out)), nil
}
