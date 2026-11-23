# `nextdeploy trace` & `nextdeploy takeover`

Native agent tracing and context continuity layer for AI coding agents (Google Antigravity, Claude Code, Cursor, Windsurf, Kiro) and human developers.

---

## Overview

The `nextdeploy trace` command family captures and coordinates:
- The **intent and mental model** of the AI agent.
- The **modified files and live git diffs**.
- The **deployment and infrastructure state** (target, URLs, SSR health probe results).
- Fast **human takeover** via terminal briefs or directly launching `$EDITOR`.
- An embedded local web dashboard on port `4044`.

---

## Usage

```bash
nextdeploy trace [command] [flags]
```

### Subcommands

| Command | Description |
| :--- | :--- |
| `nextdeploy trace status` | Display the active agent session, model, modified files, and deployment status. |
| `nextdeploy trace checkpoint` | Snapshot the current thought process, intent, and git diffs to `.nextdeploy/trace/`. |
| `nextdeploy trace takeover` | Output the scannable 20-line Developer Takeover Brief and optionally open touched files. |
| `nextdeploy trace ui` | Launch the interactive local web dashboard at `http://localhost:4044`. |
| `nextdeploy trace resume` | Output a context-preserving handoff prompt to continue in another agent. |
| `nextdeploy takeover` | Direct alias for `nextdeploy trace takeover`. |

---

## Subcommand Reference

### 1. `nextdeploy trace checkpoint`

Saves an atomic snapshot of the current engineering context:

```bash
nextdeploy trace checkpoint \
  --agent antigravity \
  --model gemini-3.8-flash \
  --objective "Implement Next.js 15 Server Actions and deploy to Cloudflare"
```

#### Flags

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--objective` | `-m` | string | Intent or goal of the agent session. |
| `--agent` | | string | Agent name (e.g. `antigravity`, `claude`, `cursor`). |
| `--model` | | string | Model identifier (e.g. `gemini-3.8-flash`, `claude-3-7-sonnet`). |
| `--unfinished` | | string | Summary of remaining work to be completed. |

---

### 2. `nextdeploy trace takeover` (Alias: `nextdeploy takeover`)

When an agent hits a token limit, loops, or finishes boilerplate, step in instantly:

```bash
# Print scannable 20-line takeover brief
nextdeploy takeover

# Print brief AND open all touched files in your editor (Neovim / VS Code / Cursor)
nextdeploy takeover -o
```

#### Flags

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--open` | `-o` | boolean | Open all modified files directly in `$EDITOR`. |
| `--editor` | | string | Specific editor binary (default: `$EDITOR`, `nvim`, `code`, `vim`, `nano`). |

---

### 3. `nextdeploy trace ui`

Starts the embedded local web dashboard served directly from the `nextdeploy` binary:

```bash
nextdeploy trace ui --port 4044
```

Navigate to `http://localhost:4044`:
- **Active Agent Session**: See model, intent, and session timestamp.
- **Infrastructure Card**: View active target (Cloudflare / VPS / AWS), live preview URL, and SSR probe health.
- **Git Evidence**: View touched files and live git diffs.
- **Event Timeline**: Stream of agent prompts, build milestones, and cutover results.

---

### 4. `nextdeploy trace resume`

Transfer context cleanly to another AI coding agent:

```bash
nextdeploy trace resume --agent claude
```
Outputs a prompt payload containing the full reasoning trail, touched files, and deployment status.
