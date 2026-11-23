# Agent Trace & Handoff Integration

NextDeploy features native integration with **Handoff (`.handoff/`)**, the provider-independent agent trace and continuity layer for AI coding agents.

This subsystem resides in [`shared/agenttrace`](file:///Users/yussufhersi/Music/workspace/NextDeploy/shared/agenttrace/trace.go) and bridges the gap between **autonomous AI code generation** and **autonomous infrastructure deployment**.

---

## The Problem Solved

When AI coding agents (such as Google Antigravity, Claude Code, Cursor, or Kiro) work on Next.js repositories:
1. They iterate on code diffs, prompts, and thought processes stored in `.handoff/`.
2. When deploying with NextDeploy (`nextdeploy ship`), deployments historically ran in isolation from the agent's reasoning session.
3. If an SSR build or cloud cutover failed, or if an agent ran out of context/tokens mid-deployment, the human developer running `handoff takeover` or the next agent running `handoff resume` had no visibility into what broke on the infrastructure layer.

With **Agent Trace Integration**:
* NextDeploy detects active Handoff sessions automatically.
* It prints the agent's session ID, model, and active intent during builds.
* It writes structured infrastructure lifecycle events into `.handoff/events.json`.
* It updates `.handoff/latest.md` with live preview URLs or exact deployment failure diagnostics for instant human takeover (`handoff takeover -o`).

---

## Code Architecture

### 1. The `shared/agenttrace` Package

Located at [`shared/agenttrace`](file:///Users/yussufhersi/Music/workspace/NextDeploy/shared/agenttrace):

```
shared/agenttrace/
├── types.go       # Checkpoint, Event, and DeploymentProvenance structs
├── trace.go       # Detection, event appending, and takeover brief rendering
└── trace_test.go  # Unit test suite verifying Handoff lifecycle contracts
```

#### Core Structs ([`types.go`](file:///Users/yussufhersi/Music/workspace/NextDeploy/shared/agenttrace/types.go)):
```go
type HandoffCheckpoint struct {
    SessionID      string    `json:"session_id"`
    Agent          string    `json:"agent"`
    Model          string    `json:"model"`
    Objective      string    `json:"objective"`
    Timestamp      time.Time `json:"timestamp"`
    ModifiedFiles  []string  `json:"modified_files,omitempty"`
}

type HandoffEvent struct {
    Type        string    `json:"type"`        // "nextdeploy_ship_start", "nextdeploy_ship_success", "nextdeploy_deploy_error"
    Timestamp   time.Time `json:"timestamp"`
    Source      string    `json:"source"`      // "nextdeploy"
    Target      string    `json:"target"`      // "cloudflare", "vps", "aws"
    Environment string    `json:"environment"` // "production", "preview"
    LiveURL     string    `json:"live_url,omitempty"`
    ErrorMsg    string    `json:"error_msg,omitempty"`
}
```

---

### 2. Lifecycle Hooks in CLI Ship Flow ([`cli/cmd/ship.go`](file:///Users/yussufhersi/Music/workspace/NextDeploy/cli/cmd/ship.go))

#### A. Pre-Flight Trace Detection:
Before building, NextDeploy checks if `.handoff/` is present:
```go
if agenttrace.DetectHandoff(".") {
    if cp, err := agenttrace.ReadLatestCheckpoint("."); err == nil && cp != nil {
        log.Info("🔍 Agent Trace Active: Session %s (%s / %s)", cp.SessionID, cp.Agent, cp.Model)
        if cp.Objective != "" {
            log.Info("   Intent: %q", cp.Objective)
        }
    }
    _ = agenttrace.RecordEvent(".", agenttrace.HandoffEvent{
        Type:        "nextdeploy_ship_start",
        Target:      cfg.TargetType,
        Environment: cfg.App.Environment,
        Details:     fmt.Sprintf("Initiated deployment for %s", cfg.App.Name),
    })
}
```

#### B. Build Failure Interception:
If `buildflow.Run` fails (e.g. Next.js prerender error, TypeScript compilation error):
```go
if err != nil {
    if agenttrace.DetectHandoff(".") {
        _ = agenttrace.RecordEvent(".", agenttrace.HandoffEvent{
            Type:        "nextdeploy_build_error",
            Target:      cfg.TargetType,
            Environment: cfg.App.Environment,
            ErrorMsg:    err.Error(),
        })
        _ = agenttrace.UpdateTakeoverBrief(".", cfg.TargetType, cfg.App.Environment, "BUILD_FAILED", "", err)
    }
    log.Error("Build flow failed: %v", err)
    os.Exit(1)
}
```

#### C. Cloudflare & VPS Cutover Recording:
Upon successful cutover or deployment failure:
```go
// Success:
_ = agenttrace.RecordEvent(".", agenttrace.HandoffEvent{
    Type:        "nextdeploy_ship_success",
    Target:      "serverless/" + cfg.Serverless.Provider,
    Environment: cfg.App.Environment,
    LiveURL:     liveURL,
    Details:     fmt.Sprintf("Serverless deployment succeeded for %s", cfg.App.Name),
})
_ = agenttrace.UpdateTakeoverBrief(".", "serverless/"+cfg.Serverless.Provider, cfg.App.Environment, "SUCCESS", liveURL, nil)

// Failure:
_ = agenttrace.RecordEvent(".", agenttrace.HandoffEvent{
    Type:        "nextdeploy_deploy_error",
    Target:      "serverless/" + cfg.Serverless.Provider,
    Environment: cfg.App.Environment,
    ErrorMsg:    err.Error(),
})
_ = agenttrace.UpdateTakeoverBrief(".", "serverless/"+cfg.Serverless.Provider, cfg.App.Environment, "DEPLOY_FAILED", "", err)
```

---

## Resulting Developer Experience

### 1. In the Terminal:
```
$ nextdeploy ship
🚀 SHIP: Starting NextDeploy ship process...
🔍 Agent Trace Active: Session sess_01HZX8 (antigravity / gemini-3.8-flash)
   Intent: "Refactor Next.js App Router checkout page and verify Server Actions"
📦 NextDeploy: Building Next.js standalone bundle...
🚀 SHIP: Uploading artifact to VPS / Cloudflare...
✨ SHIP: Deployment complete!
```

### 2. In `handoff takeover` Brief:
When running `handoff takeover`, the developer immediately sees the live deployment status injected by NextDeploy:

```markdown
# Takeover Brief

## Active Session
- Agent: Antigravity IDE (gemini-3.8-flash)
- Objective: Refactor Next.js App Router checkout page and verify Server Actions

## NextDeploy Infrastructure Status
- **Target**: `cloudflare` (environment: `production`)
- **Status**: `SUCCESS` (at 2026-11-23T15:37:00Z)
- **Deployment URL**: https://app-production.workers.dev

## Modified Files
- app/checkout/page.tsx
- app/checkout/actions.ts
```

### 3. In Handoff Local Web UI (`:4044`):
The local dashboard reads `.handoff/events.json` and renders the NextDeploy deployment timeline alongside the agent's code diffs and reasoning steps.
