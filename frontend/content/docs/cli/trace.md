---
title: nextdeploy trace
lede: Command reference for NextDeploy Agent Trace, checkpointing, and human takeover.
status: ready
sourceRef: cli/cmd/trace.go
related:
  - title: Agent-First Infrastructure
    href: /docs/agent-trace/agent-first-infra
  - title: Agent Trace Architecture
    href: /docs/agent-trace/architecture
  - title: CLI & Human Takeover
    href: /docs/agent-trace/cli-takeover
---

## Usage

```bash
nextdeploy trace [command] [flags]
# or root alias for takeover:
nextdeploy takeover [flags]
```

---

## Subcommands

| Subcommand | Description |
| :--- | :--- |
| `nextdeploy trace checkpoint` | Snapshot current agent thought process, intent, and git diffs to `.nextdeploy/trace/`. |
| `nextdeploy trace status` | Display the active agent session, modified files, and deployment state. |
| `nextdeploy trace takeover` | Output the scannable 20-line Developer Takeover Brief and optionally open touched files. |
| `nextdeploy trace ui` | Launch the interactive local web dashboard at `http://localhost:4044`. |
| `nextdeploy trace resume` | Output a context-preserving handoff prompt to continue in another agent. |
| `nextdeploy takeover` | Direct root alias for `nextdeploy trace takeover`. |

---

## Flags & Examples

### `nextdeploy trace checkpoint`
```bash
nextdeploy trace checkpoint \
  --agent antigravity \
  --model gemini-3.8-flash \
  --objective "Add Stripe webhook signature verification" \
  --unfinished "Run integration tests"
```

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--objective` | `-m` | string | Intent or goal of the agent session. |
| `--agent` | | string | Agent name (e.g. `antigravity`, `claude`, `cursor`). |
| `--model` | | string | Model identifier (e.g. `gemini-3.8-flash`, `claude-3-7-sonnet`). |
| `--unfinished` | | string | Summary of remaining work. |

### `nextdeploy takeover` (or `nextdeploy trace takeover`)
```bash
# Print takeover brief
nextdeploy takeover

# Print brief and open all touched files in $EDITOR
nextdeploy takeover -o
```

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--open` | `-o` | boolean | Open all modified files directly in `$EDITOR`. |
| `--editor` | | string | Specific editor binary (default: `$EDITOR`, `nvim`, `code`, `vim`, `nano`). |

### `nextdeploy trace ui`
```bash
nextdeploy trace ui --port 4044
```

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--port` | `-p` | int | Local port for the web dashboard (default: `4044`). |

### `nextdeploy trace resume`
```bash
nextdeploy trace resume --agent claude
```

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--agent` | `-a` | string | Target AI agent (`claude`, `cursor`, `antigravity`). |
