# Agent-First Infrastructure & Handoff (Agent Trace)

In the era of AI coding agents (such as Google Antigravity, Claude Code, Cursor, and Kiro), **code generation is no longer the bottleneck**. AI agents can generate complex Next.js applications with React Server Components, Server Actions, dynamic routes, and database models in minutes.

However, traditional cloud infrastructure and deployment workflows were designed for **humans clicking consoles or watching human-formatted terminal logs**. When an AI agent attempts to build, configure, deploy, or debug cloud infrastructure, it hits severe friction:
- Interactive TTY prompts block headless agent execution.
- Build and runtime failures dump walls of ANSI terminal text rather than structured, repairable errors.
- If an agent hits a token limit, loops, or crashes mid-feature, its engineering context, mental model, and deployment intent are lost.

By coupling **NextDeploy** (the agent-first Next.js deployment engine) with **Handoff** (the provider-independent agent trace and continuity layer), developers achieve a continuous autonomous loop:
> **Your coding agent should be replaceable. Your engineering context and production deployments shouldn't be.**

---

## Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────┐
│                        AI CODING AGENT                                │
│       (Google Antigravity, Claude Code, Cursor, Windsurf, Kiro)        │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │ 1. Code Edits + Intent         │ 2. Infra Commands
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

## Quickstart: Installing the Agent-First Toolchain

Install both `nextdeploy` and `handoff` into your user space:

```bash
# 1. Install NextDeploy
curl -fsSL https://nextdeploy.org/install.sh | bash

# 2. Install Handoff (Agent Trace)
curl -fsSL https://raw.githubusercontent.com/aynaash/handoff/main/install.sh | sh

# 3. Install the Handoff agent skills for your coding agent
handoff skill install
```

Verify both binaries are available in your path:

```bash
nextdeploy version
handoff version
```

---

## Step-by-Step Implementation in Next.js

### 1. Configure `nextdeploy.yml` for Agent-First Deployments

In your Next.js project root, define your `nextdeploy.yml`. Enable headless, machine-readable validation and health probing:

```yaml
version: 1
app: my-nextjs-app

target:
  type: cloudflare # or 'vps' (nextdeployd) / 'aws' (lambda)
  domain: app.example.com

build:
  engine: standalone # or nextcompile for edge
  clean: true

# Agent & Automation Settings
automation:
  non_interactive: true
  diagnostics_format: json # Structured error payloads for AI agents
  trace:
    provider: handoff # Links deployments directly to .handoff/
    record_events: true

# Verification Sentinel
health:
  probes:
    - path: /
      expected_status: 200
      timeout_seconds: 5
    - path: /api/health
      expected_status: 200
      timeout_seconds: 3
  auto_rollback: true # Instantly cut back to previous active release on SSR probe failure
```

---

### 2. Next.js Runtime Telemetry & Error Boundary Hook

To feed runtime errors from Next.js Server Components, Server Actions, and API routes back into Handoff's event stream (`.handoff/events.json`), set up Next.js `instrumentation.ts` and a root error boundary.

#### A. Next.js Instrumentation (`instrumentation.ts`)
Next.js supports the `instrumentation.ts` hook (App Router / Pages Router) to capture server-side crashes:

```typescript
// instrumentation.ts
import fs from 'node:fs';
import path from 'node:path';

export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    // Record boot event in local preview runs
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
    // Gracefully ignore logging errors in production
  }
}
```

#### B. Global Error Boundary (`app/global-error.tsx`)
Catch unhandled client or SSR React hydration failures:

```tsx
// app/global-error.tsx
'use client';

import { useEffect } from 'react';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log client crash telemetry during preview sessions
    if (process.env.NODE_ENV === 'development') {
      fetch('/api/__agent_trace', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'client_hydration_error',
          message: error.message,
          stack: error.stack,
          digest: error.digest,
        }),
      }).catch(() => {});
    }
  }, [error]);

  return (
    <html>
      <body>
        <h2>Application encountered a runtime error</h2>
        <button onClick={() => reset()}>Retry</button>
      </body>
    </html>
  );
}
```

---

### 3. Agent Rules Configuration (`AGENTS.md` / `.cursorrules`)

Add an `AGENTS.md` or `.cursorrules` file to your repository so any coding agent (Antigravity, Claude Code, Cursor, etc.) automatically knows how to work with Handoff and NextDeploy:

```markdown
# Agent Instructions: Continuous Development & Deployment

You are an autonomous coding agent operating on this Next.js repository.

## Infrastructure & Trace Guidelines:
1. **Checkpointing Context**:
   - Before starting complex refactors or after every major milestone, run:
     `handoff checkpoint`
   - This records your intent, mental model, and modified files into `.handoff/`.

2. **Validating & Shipping with NextDeploy**:
   - Always run non-interactive verification before declaring a task complete:
     `nextdeploy plan`
   - Deploy preview/production releases:
     `nextdeploy ship --non-interactive`
   - If `nextdeploy ship` fails, inspect the structured error output. Fix the root cause in the Next.js routes or config instead of guessing.

3. **Autonomous Handoff Rule**:
   - If you encounter a blocking issue, rate limit, or cannot resolve an SSR failure within 2 iterations:
     1. Run `handoff checkpoint`
     2. Explain the blocking issue in the checkpoint summary
     3. Allow the human developer or another model to take over cleanly via `handoff takeover`.
```

---

## Developer Workflows & Scenarios

### Scenario 1: The Human Takeover (`handoff takeover -o`)
When an AI agent finishes generating routes or gets interrupted mid-feature:

1. **View the 20-line Takeover Brief**:
   ```bash
   handoff takeover
   ```
   Outputs:
   - Primary objective and intent of the session.
   - The agent's thought trail and reasoning.
   - Touched files and uncommitted git diffs.
   - The last command run and its exit code.

2. **Open directly in your editor**:
   ```bash
   handoff takeover -o
   ```
   Instantly opens all files modified by the agent directly in your `$EDITOR` (Neovim, VS Code, Cursor).

3. **Verify with NextDeploy**:
   ```bash
   nextdeploy plan
   nextdeploy ship
   ```

---

### Scenario 2: The Local Web Dashboard (`handoff ui` on `:4044`)
To inspect concurrent agent sessions, diffs, and deployment logs visually:

```bash
handoff ui
```

Navigate to `http://localhost:4044`:
* **Sessions Panel**: Inspect timeline of developer prompts and model switches (e.g., Claude ➔ Gemini).
* **Code Diffs**: Expand line-by-line syntax-highlighted git diffs.
* **NextDeploy Execution Card**: View the target environment, active Caddy/Cloudflare URL, and SSR probe results.
* **1-Click Takeover**: Copy the exact takeover command or resume string.

---

### Scenario 3: Agent-to-Agent Seamless Continuity (`handoff resume`)
When switching from one coding agent to another (for example, handing off from Antigravity IDE to Claude Code):

```bash
# 1. Save the checkpoint from the current agent session
handoff checkpoint

# 2. Inspect the session state
handoff inspect

# 3. Resume seamlessly inside Claude Code without repeating the prompt history
handoff resume --agent claude
```

Claude Code initializes with:
- The full objective and reasoning history.
- The list of modified files.
- The latest NextDeploy build status.
- Zero context loss.

---

## Summary of Commands

| Tool | Command | Description |
| :--- | :--- | :--- |
| **Handoff** | `handoff ui` | Launch interactive web dashboard at `http://localhost:4044` |
| **Handoff** | `handoff takeover` | Print concise terminal takeover brief for human developer |
| **Handoff** | `handoff takeover -o` | Print brief and open all modified files in `$EDITOR` |
| **Handoff** | `handoff checkpoint` | Snapshot current thought process, prompts, and diffs to `.handoff/` |
| **Handoff** | `handoff resume --agent claude` | Resume session with another AI agent |
| **NextDeploy** | `nextdeploy plan` | Validate configuration and build dry-run |
| **NextDeploy** | `nextdeploy ship` | Build, bundle, deploy, and verify live endpoints |
| **NextDeploy** | `nextdeploy rollback` | Atomic cutover back to previous active release |
| **NextDeploy** | `nextdeploy logs` | Stream live server and SSR runtime logs |
