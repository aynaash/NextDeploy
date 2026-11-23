# Agent-First Infrastructure & Handoff (Agent Trace) for NextDeploy

> **"Your coding agent should be replaceable. Your engineering context and production deployments shouldn't be."**

---

## Executive Summary

The paradigm of software development has shifted. In the era of AI coding agents (**Google Antigravity IDE, Anthropic Claude Code, Cursor, Windsurf, Devin, Kiro, v0, Bolt**), code generation is no longer the bottleneck. An AI agent can scaffold an entire full-stack Next.js application, implement React Server Components, write Server Actions, and configure database schemas in minutes.

The new bottleneck is **Autonomous Infrastructure Execution & Context Continuity**:
1. **Infrastructure Blindness**: Traditional platforms (Vercel, AWS Console, generic cloud PaaS) were designed for human developers clicking web dashboards or deciphering ANSI-colored terminal logs. When an AI agent attempts to build, configure, deploy, or debug, it hits interactive TTY traps, missing database bindings, and unstructured error walls.
2. **Context Fragmentation**: When an agent hits a token limit, starts looping, or finishes the boilerplate so a human wants to step in, the thought process, reasoning steps, uncommitted diffs, and deployment failures are lost.

By coupling **NextDeploy** (the high-performance, single-binary Next.js deployment engine) with **Handoff** (the provider-independent agent trace and continuity layer), we create the world's first **Agent-Native Continuous Delivery (ACD) Platform**.

---

## System Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                        AI CODING AGENT                                │
│       (Google Antigravity, Claude Code, Cursor, Windsurf, Kiro)        │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │ 1. Code Edits + Intent         │ 2. Infra Invocations
                    ▼                                ▼
       ┌─────────────────────────┐      ┌─────────────────────────┐
       │   HANDOFF (Agent Trace) │◄────►│       NEXTDEPLOY        │
       │                         │      │                         │
       │ • Objectives & Prompts  │      │ • nextcompile (CF/AWS)  │
       │ • Step-by-Step Thoughts │      │ • nextdeployd (VPS)     │
       │ • Workspace Git Diffs   │      │ • Atomic Zero-Downtime  │
       │ • .handoff/latest.md    │      │ • Synthetic Probing     │
       └────────────┬────────────┘      └────────────┬────────────┘
                    │                                │
                    └────────────────┬───────────────┘
                                     │ Continuous Telemetry
                                     ▼
        ┌────────────────────────────────────────────────────────┐
        │      HANDOFF LOCAL DASHBOARD (http://localhost:4044)   │
        │                                                        │
        │  [Session: "Add Next.js 15 Server Action & Deploy"]    │
        │  ├─ Agent: Antigravity IDE (Gemini 3.8 Flash)          │
        │  ├─ Diffs: 5 files (+112, -18)                         │
        │  ├─ Target: Cloudflare Workers (nextcompile)           │
        │  ├─ Status: SSR Probing Succeeded (HTTP 200)           │
        │  └─ Live URL: https://preview-7b91.workers.dev         │
        └────────────────────────────┬───────────────────────────┘
                                     │
           ┌─────────────────────────┴─────────────────────────┐
           ▼                                                   ▼
┌───────────────────────────────┐               ┌───────────────────────────────┐
│  HUMAN TAKEOVER (`takeover`)  │               │   NEXT AGENT RESUME (`resume`) │
│  - View 20-line scannable     │               │   - Reads mental model        │
│    mental model & failures    │               │   - Inspects touched diffs    │
│  - Opens files directly in    │               │   - Resumes execution without │
│    Neovim / VSCode ($EDITOR)  │               │     prompt repetition         │
└───────────────────────────────┘               └───────────────────────────────┘
```

---

## The 5 Pillars of Agent-First Infrastructure

### 1. Model Context Protocol (MCP) Native Engine
Instead of forcing coding agents to run opaque bash commands and parse regex outputs, NextDeploy exposes first-class tool definitions via an embedded MCP server:
* `nextdeploy_inspect`: Analyzes the Next.js version (14/15/16), App Router structure, Server Actions, and database dependencies.
* `nextdeploy_env_sync`: Deterministically validates or configures environment secrets (`.env.production`).
* `nextdeploy_sandbox_create`: Provisions an ephemeral preview sandbox in < 3 seconds.
* `nextdeploy_ship`: Executes the build, compilation, deployment, and live verification.
* `nextdeploy_diagnose`: Feeds production/preview logs filtered by request ID directly to the LLM context.

### 2. Machine-Readable Diagnostics (`--format=agent-json`)
When builds fail, agents need structured error payloads that point directly to the culprit code:
```json
{
  "status": "error",
  "stage": "nextcompile_static_analysis",
  "culprit": {
    "file": "app/dashboard/page.tsx",
    "line": 42,
    "error_type": "DYNAMIC_SERVER_USAGE",
    "message": "Route /dashboard used headers() without dynamic = 'force-dynamic'"
  },
  "actionable_suggestion": "Add `export const dynamic = 'force-dynamic'` or wrap dynamic data fetching in Suspense.",
  "raw_context": "Error occurred prerendering page \"/dashboard\"..."
}
```

### 3. Ephemeral "Branch-and-Burn" Previews
Agents iterate best in tight feedback loops:
* **Cloudflare Workers**: Deploy directly to an instant isolated worker subdomain (`preview-<hash>.workers.dev`).
* **VPS Target**: Caddy + `nextdeployd` dynamically generate reverse-proxy rules for subdomains (e.g., `https://agent-task-42.app.internal`) pointing to an ephemeral port.

### 4. Autonomous Data Plane & Storage Binding
Agents frequently build apps that require database storage. NextDeploy automates database provisioning and binding:
* `target: cloudflare` ➔ Automatically bind Cloudflare **D1** (SQL), **KV** (caching), or **R2** (blob storage).
* `target: vps` ➔ `nextdeployd` provisions embedded SQLite or orchestrated Postgres with automatic connection string injection.

### 5. Autonomous Verification Sentinel & Self-Healing Rollback
* **Post-Ship Synthetic Prober**: Immediately after traffic cutover, NextDeploy sends HTTP probes to detected routes.
* **Automatic Rollback**: If an unhandled exception or 500 status is detected:
  1. NextDeploy instantly triggers an atomic rollback to the previous active release symlink (`current`) or worker version.
  2. Extracts the exact exception stack trace.
  3. Returns a failure report directly to the agent: *"Deployment rejected: /api/checkout crashed with ReferenceError: Stripe key missing. Rolled back to v12. Fix the missing environment variable."*

---

## What is Handoff (Agent Trace)?

**Handoff** is a provider-independent CLI, local web dashboard (`:4044`), and continuity layer for AI-assisted software engineering. It allows transferring an unfinished coding session from an AI agent to another agent OR directly to yourself (human developer takeover) without losing context.

### Observable Engineering Evidence Captured:
1. **Objective & Intent**: Every developer prompt in sequence.
2. **Thought Process & Mental Model**: The step-by-step reasoning trail and model transitions (Claude Sonnet ➔ Gemini Flash, etc.).
3. **Repository & File Evidence**: Exact files modified by the agent and live git diff state.
4. **Execution & Failure Evidence**: Terminal commands run, their exit codes, and error outputs.

### Key Handoff Files in Workspace:
* `.handoff/checkpoint.json`: Machine-readable snapshot of the active agent session.
* `.handoff/events.json`: Chronological event stream of tool invocations, bash commands, and deployment events.
* `.handoff/latest.md`: Scannable Developer Takeover Brief.

---

## Implementation Inside NextDeploy Codebase

The integration is built directly into NextDeploy's Go core:

### 1. `shared/agenttrace` Package

Located at `shared/agenttrace/`:

```
shared/agenttrace/
├── types.go       # Checkpoint, Event, and DeploymentProvenance structs
├── trace.go       # Detection, event appending, and takeover brief rendering
└── trace_test.go  # Unit test suite verifying Handoff lifecycle contracts
```

#### Core Structs:
```go
package agenttrace

import "time"

type HandoffCheckpoint struct {
    SessionID      string    `json:"session_id"`
    Agent          string    `json:"agent"`
    Model          string    `json:"model"`
    Objective      string    `json:"objective"`
    Timestamp      time.Time `json:"timestamp"`
    ModifiedFiles  []string  `json:"modified_files,omitempty"`
    LastCommand    string    `json:"last_command,omitempty"`
    LastExitCode   int       `json:"last_exit_code,omitempty"`
    UnfinishedWork string    `json:"unfinished_work,omitempty"`
}

type HandoffEvent struct {
    Type        string    `json:"type"`                  // "nextdeploy_ship_start", "nextdeploy_ship_success", "nextdeploy_deploy_error"
    Timestamp   time.Time `json:"timestamp"`
    Source      string    `json:"source"`                // "nextdeploy"
    Target      string    `json:"target,omitempty"`      // "vps", "cloudflare", "aws"
    Environment string    `json:"environment,omitempty"` // "production", "preview"
    Details     string    `json:"details,omitempty"`
    LiveURL     string    `json:"live_url,omitempty"`
    ErrorCode   string    `json:"error_code,omitempty"`
    ErrorMsg    string    `json:"error_msg,omitempty"`
    Route       string    `json:"route,omitempty"`
    FixHint     string    `json:"fix_hint,omitempty"`
}
```

### 2. Integration into `nextdeploy ship` (`cli/cmd/ship.go`)

NextDeploy hooks into Handoff at three critical phases:

```go
// 1. Pre-Flight Trace Detection
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

// 2. Build Failure Interception
result, err := buildflow.Run(...)
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

// 3. Cutover Success & Failure Telemetry
if err := serverless.Deploy(...); err != nil {
    if agenttrace.DetectHandoff(".") {
        _ = agenttrace.RecordEvent(".", agenttrace.HandoffEvent{
            Type:        "nextdeploy_deploy_error",
            Target:      "serverless/" + cfg.Serverless.Provider,
            Environment: cfg.App.Environment,
            ErrorMsg:    err.Error(),
        })
        _ = agenttrace.UpdateTakeoverBrief(".", "serverless/"+cfg.Serverless.Provider, cfg.App.Environment, "DEPLOY_FAILED", "", err)
    }
    os.Exit(1)
}

// On Success:
if agenttrace.DetectHandoff(".") {
    _ = agenttrace.RecordEvent(".", agenttrace.HandoffEvent{
        Type:        "nextdeploy_ship_success",
        Target:      "serverless/" + cfg.Serverless.Provider,
        Environment: cfg.App.Environment,
        LiveURL:     liveURL,
        Details:     fmt.Sprintf("Serverless deployment succeeded for %s", cfg.App.Name),
    })
    _ = agenttrace.UpdateTakeoverBrief(".", "serverless/"+cfg.Serverless.Provider, cfg.App.Environment, "SUCCESS", liveURL, nil)
}
```

---

## Next.js Project Implementation

### 1. `nextdeploy.yml` Configuration
```yaml
version: 1
app: my-nextjs-app

target:
  type: cloudflare # or 'vps' (nextdeployd) / 'aws' (lambda)
  domain: app.example.com

build:
  engine: standalone # or nextcompile for edge
  clean: true

automation:
  non_interactive: true
  diagnostics_format: json
  trace:
    provider: handoff
    record_events: true

health:
  probes:
    - path: /
      expected_status: 200
      timeout_seconds: 5
    - path: /api/health
      expected_status: 200
      timeout_seconds: 3
  auto_rollback: true
```

### 2. Next.js Runtime Telemetry (`instrumentation.ts`)
```typescript
// instrumentation.ts
import fs from 'node:fs';
import path from 'node:path';

export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    appendHandoffEvent({
      type: 'runtime_boot',
      runtime: 'nodejs',
      timestamp: new Date().toISOString(),
    });
  }
}

export function onRequestError(
  err: { digest: string } & Error,
  request: {
    path: string;
    method: string;
    headers: { [key: string]: string };
  }
) {
  appendHandoffEvent({
    type: 'server_error',
    digest: err.digest,
    message: err.message,
    stack: err.stack,
    route: request.path,
    method: request.method,
    timestamp: new Date().toISOString(),
  });
}

function appendHandoffEvent(event: Record<string, unknown>) {
  try {
    const handoffDir = path.join(process.cwd(), '.handoff');
    if (fs.existsSync(handoffDir)) {
      const eventsFile = path.join(handoffDir, 'events.json');
      let events: unknown[] = [];
      if (fs.existsSync(eventsFile)) {
        events = JSON.parse(fs.readFileSync(eventsFile, 'utf8'));
      }
      events.push(event);
      fs.writeFileSync(eventsFile, JSON.stringify(events, null, 2), 'utf8');
    }
  } catch {
    // Gracefully ignore in production
  }
}
```

### 3. Agent Instructions (`AGENTS.md` / `.cursorrules`)
```markdown
# Agent Instructions: Continuous Development & Deployment

1. Checkpoint Frequently:
   - Before starting complex refactors or after key milestones:
     `handoff checkpoint`

2. Deploy & Verify with NextDeploy:
   - Verify plan: `nextdeploy plan`
   - Ship release: `nextdeploy ship --non-interactive`

3. Autonomous Handoff Protocol:
   - If blocked or failing SSR probes for 2 iterations:
     1. Run `handoff checkpoint`
     2. Explain the root cause in the summary
     3. Allow human takeover via `handoff takeover -o`
```

---

## Developer Workflows

### 1. Human Developer Takeover (`handoff takeover -o`)
When an AI agent finishes generating routes or hits a blocker:
```bash
# View concise 20-line brief in terminal
handoff takeover

# Open touched files directly in $EDITOR (Neovim / VS Code)
handoff takeover -o
```

**Takeover Brief Output:**
```markdown
# Takeover Brief

## Active Session
- Agent: Antigravity IDE (gemini-3.8-flash)
- Objective: Build Next.js 15 Server Actions and deploy to Cloudflare Workers

## NextDeploy Infrastructure Status
- **Target**: `serverless/cloudflare` (environment: `production`)
- **Status**: `SUCCESS` (at 2026-11-23T15:37:00Z)
- **Deployment URL**: https://my-app-production.workers.dev

## Modified Files
- app/checkout/page.tsx
- app/checkout/actions.ts
```

### 2. Local Web Dashboard on Port 4044 (`handoff ui`)
Launch the interactive web UI:
```bash
handoff ui
```
Browse sessions, inspect agent mental models, expand syntax-highlighted git diffs, and view live NextDeploy deployment status and preview URLs side-by-side.

### 3. Agent-to-Agent Handoff (`handoff resume`)
Switch models or tools without re-explaining context:
```bash
# 1. Snapshot session
handoff checkpoint

# 2. Resume in Claude Code or another agent
handoff resume --agent claude
```

---

## Command Reference

| Utility | Command | Purpose |
| :--- | :--- | :--- |
| **Handoff** | `handoff ui` | Interactive local dashboard at `http://localhost:4044` |
| **Handoff** | `handoff takeover` | Output 20-line terminal takeover brief |
| **Handoff** | `handoff takeover -o` | Open modified files directly in `$EDITOR` |
| **Handoff** | `handoff checkpoint` | Snapshot prompts, reasoning, and diffs to `.handoff/` |
| **Handoff** | `handoff resume --agent <name>` | Transfer session to another AI agent |
| **NextDeploy** | `nextdeploy plan` | Validate configuration and compute resource diff |
| **NextDeploy** | `nextdeploy ship` | Build, bundle, deploy, and verify live endpoints |
| **NextDeploy** | `nextdeploy rollback` | Atomic zero-downtime cutover to previous release |
| **NextDeploy** | `nextdeploy logs -f` | Tail live serverless or VPS daemon logs |
