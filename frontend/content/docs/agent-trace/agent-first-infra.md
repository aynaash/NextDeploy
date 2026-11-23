---
title: Agent-First Infrastructure
lede: Accelerating Next.js development in the era of AI coding agents through autonomous deployment, machine-readable diagnostics, and self-healing rollbacks.
status: ready
sourceRef: shared/agenttrace, cli/cmd/ship.go
related:
  - title: Agent Trace Architecture
    href: /docs/agent-trace/architecture
  - title: CLI & Human Takeover
    href: /docs/agent-trace/cli-takeover
  - title: Trace Command Reference
    href: /docs/cli/trace
---

## The Paradigm Shift

In the era of AI coding agents (**Google Antigravity IDE, Anthropic Claude Code, Cursor, Windsurf, Devin, Kiro**), **code generation is no longer the bottleneck**. AI agents can generate complex Next.js applications with React Server Components, Server Actions, dynamic routes, and database models in minutes.

However, traditional cloud infrastructure and deployment workflows were designed for **humans clicking consoles or watching human-formatted terminal logs**. When an AI agent attempts to build, configure, deploy, or debug cloud infrastructure, it hits severe friction:
- **Interactive TTY Traps**: Headless agents hang when CLIs prompt for interactive confirmation.
- **Unstructured Terminal Noise**: When Next.js builds fail, agents get 2,000 lines of ANSI-colored terminal text instead of structured remediation clues.
- **Missing Autonomous Data Plane**: Agents write Prisma or Drizzle code, but provisioning and binding databases (D1, Postgres, SQLite) requires human console intervention.
- **Lack of Verification**: Agents hallucinate success based on exit code 0, even if runtime SSR routes crash with HTTP 500 or hydration mismatches.

NextDeploy turns this upside down by serving as an **Agent-First Infrastructure Engine**.

---

## The 5 Pillars of Agent-First Infrastructure

### 1. Model Context Protocol (MCP) Tooling
Instead of requiring agents to invoke fragile bash subshells, NextDeploy exposes native MCP tool schemas:
- `nextdeploy_inspect`: Inspects Next.js version (14/15/16), App Router structure, and Server Actions.
- `nextdeploy_plan`: Runs headless pre-flight dry runs and emits structured resource drift.
- `nextdeploy_ship`: Executes compilation, atomic cutover, and live verification.
- `nextdeploy_diagnose`: Filters recent SSR runtime errors by request ID for direct LLM ingestion.

### 2. Machine-Readable Diagnostic Engine (`--format=agent-json`)
When Next.js compiler errors occur, NextDeploy outputs structured JSON remediation clues:
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
  "actionable_suggestion": "Add `export const dynamic = 'force-dynamic'` or wrap dynamic data fetching in Suspense."
}
```

### 3. Ephemeral "Branch-and-Burn" Previews
Agents iterate best in tight feedback loops:
- **Cloudflare Workers**: Instant isolated worker subdomains (`preview-<hash>.workers.dev`) generated in under 3 seconds.
- **VPS Target**: Caddy + `nextdeployd` dynamically generate reverse-proxy rules for subdomains pointing to an ephemeral port.

### 4. Autonomous Data Plane & Storage Binding
NextDeploy provisions databases and injects connection strings automatically:
- `target: cloudflare` ➔ Binds Cloudflare **D1** (SQL), **KV**, or **R2** storage.
- `target: vps` ➔ Binds embedded SQLite or orchestrated Postgres with automated secret injection.

### 5. Autonomous Verification Sentinel & Self-Healing Rollback
- Immediately after cutover, NextDeploy sends synthetic HTTP probes to detected routes.
- If an unhandled exception or 500 status is detected, NextDeploy executes an atomic rollback to the previous active release symlink (`current`) or worker script, and passes the stack trace directly to the agent for auto-repair.

---

## Next.js Runtime Telemetry Integration

To feed runtime errors from Next.js Server Components, Server Actions, and API routes back into the NextDeploy trace stream, configure Next.js `instrumentation.ts` and `app/global-error.tsx`.

### 1. `instrumentation.ts`
```typescript
import fs from 'node:fs';
import path from 'node:path';

export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    appendTraceEvent({
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
  appendTraceEvent({
    type: 'server_error',
    digest: err.digest,
    message: err.message,
    stack: err.stack,
    route: request.path,
    method: request.method,
    timestamp: new Date().toISOString(),
  });
}

function appendTraceEvent(event: Record<string, unknown>) {
  try {
    const traceDir = path.join(process.cwd(), '.nextdeploy', 'trace');
    if (fs.existsSync(traceDir)) {
      const eventsFile = path.join(traceDir, 'events.json');
      let events: unknown[] = [];
      if (fs.existsSync(eventsFile)) {
        events = JSON.parse(fs.readFileSync(eventsFile, 'utf8'));
      }
      events.push(event);
      fs.writeFileSync(eventsFile, JSON.stringify(events, null, 2), 'utf8');
    }
  } catch {
    // Graceful no-op in production
  }
}
```

### 2. Global Error Boundary (`app/global-error.tsx`)
```tsx
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
      <body className="p-8 text-center bg-slate-950 text-white">
        <h2 className="text-xl font-bold mb-4">Application Runtime Error</h2>
        <button
          onClick={() => reset()}
          className="px-4 py-2 bg-emerald-600 rounded-lg text-sm font-semibold"
        >
          Retry
        </button>
      </body>
    </html>
  );
}
```

### 3. Agent Instructions (`AGENTS.md` / `.cursorrules`)
Teach any coding agent how to operate autonomously:
```markdown
# Agent Instructions: Continuous Development & Deployment

1. Checkpointing Context:
   - Run `nextdeploy trace checkpoint -m "<summary>"` before major refactors and after milestones.
2. Validating & Deploying:
   - Verify plan: `nextdeploy plan`
   - Deploy: `nextdeploy ship --non-interactive`
3. Autonomous Handoff Protocol:
   - If blocked or failing SSR probes for 2 iterations, save a checkpoint with `nextdeploy trace checkpoint` so the developer can take over via `nextdeploy takeover -o`.
```
