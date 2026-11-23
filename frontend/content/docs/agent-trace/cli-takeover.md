---
title: CLI & Human Takeover
lede: How human developers step into an autonomous AI session and inspect sessions via the local web dashboard.
status: ready
sourceRef: cli/cmd/trace.go, shared/agenttrace/webui.go
related:
  - title: Agent-First Infrastructure
    href: /docs/agent-trace/agent-first-infra
  - title: Agent Trace Architecture
    href: /docs/agent-trace/architecture
  - title: Trace Command Reference
    href: /docs/cli/trace
---

## Human Takeover Workflow

When an AI coding agent hits an obstacle, runs out of tokens, or finishes the repetitive scaffolding, you shouldn't have to reverse-engineer what happened.

### 1. View the Takeover Brief
```bash
nextdeploy takeover
# or
nextdeploy trace takeover
```
Prints a scannable 20-line summary showing:
- The agent's intent and prompt objective.
- Remaining unfinished work.
- Touched files and uncommitted git status.
- Live deployment state and any failing commands.

### 2. Open Files Directly in Your Editor
```bash
nextdeploy takeover -o
```
Opens every modified file directly in `$EDITOR` (Neovim, VS Code, Cursor, Vim).

---

## Local Web Dashboard on Port 4044 (`nextdeploy trace ui`)

NextDeploy includes a self-contained local web dashboard served directly from the Go binary:

```bash
nextdeploy trace ui --port 4044
```

Navigate to `http://localhost:4044` to access:
- **Active Agent Card**: Inspect active prompt, model transitions, and session timestamps.
- **Infrastructure Card**: View active target (Cloudflare / VPS / AWS), live preview URL, and SSR probe health.
- **Git Evidence**: View syntax-highlighted diffs of touched files.
- **Event Stream**: Chronological timeline of prompts, builds, and cutover attempts.
- **1-Click Takeover**: Copy takeover commands with one click.

---

## Agent-to-Agent Handoff (`nextdeploy trace resume`)

When switching between AI agents (e.g. from Google Antigravity to Anthropic Claude Code):

```bash
# 1. Snapshot current agent session
nextdeploy trace checkpoint -m "Refactor authentication Server Actions"

# 2. Output the context-preserving resume payload
nextdeploy trace resume --agent claude
```

The output can be injected into the next agent with zero prompt repetition or context loss.
