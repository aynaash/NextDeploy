---
title: Agent Trace Architecture
lede: The context continuity layer that preserves AI agent thought processes, prompts, git diffs, and deployment lifecycle events.
status: ready
sourceRef: shared/agenttrace/types.go, shared/agenttrace/trace.go
related:
  - title: Agent-First Infrastructure
    href: /docs/agent-trace/agent-first-infra
  - title: CLI & Human Takeover
    href: /docs/agent-trace/cli-takeover
  - title: Trace Command Reference
    href: /docs/cli/trace
---

## System Overview

> **"Your coding agent should be replaceable. Your engineering context and production deployments shouldn't be."**

When collaborating with AI coding agents (Antigravity IDE, Claude Code, Cursor, Windsurf, Kiro), developer sessions frequently hit token limits, get caught in loops, or complete the boilerplate so a human developer wants to take over.

**Agent Trace** acts as a provider-independent continuity layer that captures:
1. **Developer Prompts & Objectives**: The chronological trail of goals and instructions.
2. **Mental Model & Thoughts**: The model's reasoning steps and model transitions (Claude ➔ Gemini ➔ Cursor).
3. **Repository & Git Evidence**: Touched files and uncommitted diffs.
4. **Execution & Failure Evidence**: Exit codes, build errors, and live deployment statuses.

```
┌────────────────────────────────────────────────────────────────────────┐
│                        AI CODING AGENT                                │
│       (Google Antigravity, Claude Code, Cursor, Windsurf, Kiro)        │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │ 1. Code Edits + Intent         │ 2. Infra Invocations
                    ▼                                ▼
       ┌─────────────────────────┐      ┌─────────────────────────┐
       │   AGENT TRACE ENGINE    │◄────►│       NEXTDEPLOY        │
       │                         │      │                         │
       │ • Prompt & Objectives   │      │ • nextcompile (CF/AWS)  │
       │ • Step-by-Step Thoughts │      │ • nextdeployd (VPS)     │
       │ • Workspace Git Diffs   │      │ • Atomic Zero-Downtime  │
       │ • takeover.md           │      │ • Synthetic Probing     │
       └────────────┬────────────┘      └────────────┬────────────┘
                    │                                │
                    └────────────────┬───────────────┘
                                     │ Continuous Telemetry
                                     ▼
        ┌────────────────────────────────────────────────────────┐
        │     LOCAL WEB DASHBOARD (http://localhost:4044)        │
        │                                                        │
        │  [Session: "Add Next.js 15 Server Action & Deploy"]    │
        │  ├─ Model: Antigravity IDE (Gemini 3.8 Flash)          │
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
│  - View 20-line scannable     │               │   - Inherits mental model     │
│    mental model & failures    │               │   - Reads touched diffs       │
│  - Opens touched files in IDE │               │   - Continues unfinished work │
└───────────────────────────────┘               └───────────────────────────────┘
```

---

## File Storage & Schema

Agent Trace stores its evidence in `.nextdeploy/trace/` (with automatic fallback to `.handoff/` for backward compatibility):

```
.nextdeploy/trace/
├── checkpoint.json   # Machine-readable snapshot of the active agent state
├── events.json       # Chronological event stream of tool & infra events
└── takeover.md       # Scannable 20-line Developer Takeover Brief
```

### 1. `checkpoint.json`
```json
{
  "session_id": "sess_9821",
  "agent": "antigravity",
  "model": "gemini-3.8-flash",
  "objective": "Build Next.js 15 Server Actions and deploy to Cloudflare Workers",
  "timestamp": "2026-11-23T15:37:00Z",
  "modified_files": [
    "app/checkout/page.tsx",
    "app/checkout/actions.ts"
  ],
  "unfinished_work": "Add input validation for international phone numbers"
}
```

### 2. `events.json`
```json
[
  {
    "type": "nextdeploy_ship_start",
    "timestamp": "2026-11-23T15:36:50Z",
    "source": "nextdeploy",
    "target": "serverless/cloudflare",
    "environment": "production",
    "details": "Initiated deployment for checkout-app"
  },
  {
    "type": "nextdeploy_ship_success",
    "timestamp": "2026-11-23T15:37:00Z",
    "source": "nextdeploy",
    "target": "serverless/cloudflare",
    "environment": "production",
    "live_url": "https://checkout-app.workers.dev",
    "details": "Serverless deployment succeeded"
  }
]
```

### 3. `takeover.md`
```markdown
# Developer Takeover Brief

## Active AI Coding Agent
- **Agent & Model**: antigravity (gemini-3.8-flash)
- **Session ID**: `sess_9821`
- **Objective**: Build Next.js 15 Server Actions and deploy to Cloudflare Workers
- **Unfinished Work**: Add input validation for international phone numbers

## NextDeploy Infrastructure Status
- **Target**: `serverless/cloudflare` (environment: `production`)
- **Status**: `SUCCESS` (at 2026-11-23T15:37:00Z)
- **Deployment URL**: https://checkout-app.workers.dev

## Modified Files & Git State
- `app/checkout/page.tsx`
- `app/checkout/actions.ts`

## Quick Actions
- Open touched files: `nextdeploy trace takeover -o`
- View local dashboard: `nextdeploy trace ui` (http://localhost:4044)
- Deploy changes: `nextdeploy ship`
```
