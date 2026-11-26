# NextDeploy — Pending Fixes & TODOs

---

## 1. Secret value leaking into audit log (security bug)

**Severity:** High — plaintext secrets (database URLs, API keys) are written to `/var/log/nextdeployd/audit.log` on every `secrets get` call.

**Root cause chain:**

- `daemon/internal/daemon/secrets_handler.go` — `getSecret()` returns the raw secret value as `resp.Message` (this is correct for the CLI round-trip, the caller needs it).
- `daemon/internal/daemon/command_handler.go` line ~293 — `HandleCommand` unconditionally copies `resp.Message` into `AuditEntry.ErrorDetails` for every command type, including `secrets get`.

```
// current code — the problem
ch.auditLogger.Log(AuditEntry{
    CommandType:    cmd.Type,
    ClientIdentity: clientIdentity,
    Result:         fmt.Sprintf("%v", resp.Success),
    ErrorDetails:   resp.Message,  // ← raw secret ends up here
    Args:           cmd.Args,
})
```

**Suggested fix — `command_handler.go`**

Before the `auditLogger.Log(...)` call, build a sanitised `auditMessage` string:

- If `cmd.Type == "secrets"` and the action arg is `"get"` or `"list"`, set `auditMessage` to a fixed string like `"secret value retrieved"` (or just `""` to omit the field entirely via `omitempty`).
- For every other command, use `resp.Message` as today.

```go
auditMessage := resp.Message
if cmd.Type == "secrets" {
    action, _ := StringArg(cmd.Args, "action")
    if action == "get" || action == "list" {
        auditMessage = "secret value retrieved"
    }
}

ch.auditLogger.Log(AuditEntry{
    CommandType:    cmd.Type,
    ClientIdentity: clientIdentity,
    Result:         fmt.Sprintf("%v", resp.Success),
    ErrorDetails:   auditMessage,
    Args:           cmd.Args,
})
```

Also check `Args` — the audit log already serialises `cmd.Args` which includes the `key` name (acceptable) but should never include the `value`. Confirm `setSecret` does not pass the value in `cmd.Args` before the daemon receives it; if it does, strip the `value` key from `cmd.Args` before logging.

**Immediate action:** rotate the Neon database password `npg_Sd6p9zReMrZW` — it was written to the audit log in plaintext and is now in your terminal history from this session.

---

## 2. Log disk management (issue #75 / TODO in `cli/cmd/logs.go` line 180)

**Current state:** Logs in `/var/log/nextdeployd/` (audit.log, nextdeployd.log) grow unbounded. The Caddy logs already have logrotate configured via `daemon/internal/prepare/plan.go` → `templates.go` (`logrotateCaddy`). The nextdeployd logs do not.

**Suggested fix — mirror what Caddy already does**

### Step A — add a `logrotateNextdeployd` template to `daemon/internal/prepare/templates.go`

Pattern it after `logrotateCaddy` but use `postrotate` instead of `copytruncate` because nextdeployd can be signalled to reopen its log file (or just restarted):

```
/var/log/nextdeployd/*.log {
  daily
  rotate 14
  compress
  delaycompress
  missingok
  notifempty
  postrotate
    systemctl reload nextdeployd 2>/dev/null || true
  endscript
}
```

Rotate 14 is a reasonable default (two weeks). Adjust to taste.

### Step B — install the logrotate policy in `daemon/internal/prepare/plan.go`

Add a new plan step after the existing Caddy logrotate step:

```go
{
    Name: "Install logrotate policy for /var/log/nextdeployd",
    Done: func(h Host) bool { return h.Exists("/etc/logrotate.d/nextdeployd") },
    Apply: func(h Host) error {
        return h.WriteFile("/etc/logrotate.d/nextdeployd", []byte(logrotateNextdeployd), 0o644)
    },
},
```

This runs during `nextdeploy plan --apply` (provisioning), so existing servers will pick it up on the next plan run.

### Step C — remove the TODO comment from `cli/cmd/logs.go` line 180

Once the logrotate policy is installed by the plan, the TODO is resolved. Delete the comment line.

### Optional — disk threshold warning in `nextdeploy logs`

If you want the proactive warning the issue mentions (alert before the disk fills, not just rotate after), add a check at the top of `logsVPS()` before streaming:

- SSH to the server and run `df -k /var/log` to get used/total bytes.
- If used > 80% of total, print a yellow warning to stderr before tailing.
- This is non-blocking — it warns but still streams logs.

This is optional and can be a follow-up. The logrotate policy above is the essential fix.

---

## Files to change

| File | Change |
|------|--------|
| `daemon/internal/daemon/command_handler.go` | Sanitise `auditMessage` before `auditLogger.Log()` for `secrets get/list` |
| `daemon/internal/prepare/templates.go` | Add `logrotateNextdeployd` const |
| `daemon/internal/prepare/plan.go` | Add plan step to install `/etc/logrotate.d/nextdeployd` |
| `cli/cmd/logs.go` | Remove TODO comment at line 180 once logrotate step lands |
