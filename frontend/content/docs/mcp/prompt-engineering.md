---
title: Agent Rules & Prompt Engineering
lede: Production prompt templates, system instructions, and rule files for Cursor, Claude Code, and Antigravity IDE.
status: ready
sourceRef: AGENTS.md, .cursorrules
related:
  - title: MCP Server Overview
    href: /docs/mcp/server
  - title: MCP Tool Schemas
    href: /docs/mcp/tools
  - title: Agent-First Infrastructure
    href: /docs/agent-trace/agent-first-infra
---

## Overview

To enable autonomous coding agents to build and deploy Next.js applications reliably without human micro-management, repositories should provide clear instructions via **agent context files**:
- `.cursorrules` (Cursor IDE)
- `CLAUDE.md` (Anthropic Claude Code / Claude Desktop)
- `AGENTS.md` (Standardized agent instructions)

---

## The Universal `AGENTS.md` Specification

Place this file in the root of any Next.js project using NextDeploy:

```markdown
# Agent Directives: Next.js Engineering with NextDeploy

You are an autonomous senior software engineer operating on this Next.js repository.

## 1. Context Continuity & Checkpointing (Handoff)
- Before beginning a complex refactor or after completing a significant milestone, snapshot your context:
  `nextdeploy trace checkpoint -m "<what you did and what remains>"`
- Never leave an unfinished feature without an updated checkpoint. The human developer may take over via `nextdeploy takeover -o`.

## 2. Infrastructure & Deployment Protocol
- NextDeploy manages all infrastructure across Linux VPS, Cloudflare Workers, and AWS.
- Configuration lives in `nextdeploy.yml`. Do not create separate `wrangler.toml` or `Dockerfile` configs unless explicitly instructed.
- Always validate infrastructure changes before deploying:
  `nextdeploy plan --json`
- Deploy releases non-interactively:
  `nextdeploy ship --non-interactive --verify`
- If deployment fails, inspect the structured error output. Fix the root cause in application code or configuration rather than guessing.

## 3. Runtime Safety & Self-Healing
- If runtime SSR probes return 500 or hydration mismatches, NextDeploy automatically keeps the previous release active.
- Read the failure details from the trace:
  `nextdeploy trace status`
- If you cannot resolve a deployment error within 2 iterations:
  1. Record the blocking issue: `nextdeploy trace checkpoint -m "Blocked: <error>"`
  2. Stop and notify the human developer to run `nextdeploy takeover -o`.
```

---

## Cursor IDE Rules (`.cursorrules`)

```markdown
# NextDeploy Project Rules

- When modifying Next.js routes, verify Server Actions and App Router conventions.
- Use `nextdeploy plan` to dry-run deployments.
- Use `nextdeploy ship` to deploy.
- Call `nextdeploy trace checkpoint` after completing substantial multi-file edits.
- Consult `/llms.txt` or `/llms-full.txt` for the complete NextDeploy API.
```

---

## Best Practices for Agent Autonomy

1. **Leverage `llms.txt`**: When prompting models that accept URLs or documentation files, reference `https://nextdeploy.org/llms.txt` to ground the model in accurate syntax.
2. **Never Hardcode Secrets**: Instruct agents to use `nextdeploy secrets set KEY=VALUE` rather than writing credentials into `.env.local` or committing them to git.
3. **Use Ephemeral Environments**: In PR reviews, direct agents to deploy to isolated preview workers (`nextdeploy ship -e pr-$ID`).
