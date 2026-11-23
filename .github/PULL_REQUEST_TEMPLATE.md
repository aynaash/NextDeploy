## Summary

<!-- Describe what this PR changes and why. Remember: your PR title becomes the
     squash-merge commit message and feeds CHANGELOG.md via release-please, so
     use Conventional Commits (e.g., `fix(cloudflare): ...`). -->

## Related Issue

<!-- Closes # -->

## Areas Affected

- [ ] CLI (`cli/`)
- [ ] Daemon (`daemon/`)
- [ ] VPS target
- [ ] AWS Serverless target
- [ ] Cloudflare Workers target / `nextcompile`
- [ ] Documentation (`docs/`)
- [ ] CI / Release / Tooling

## Verification

<!-- Commands run and results (e.g., `go test ./...`, `mage lint`, manual deploy test). -->

## Checklist

- [ ] PR title follows Conventional Commits (`feat`, `fix`, `docs`, `ci`, `chore`, `refactor`, `perf`, `test`)
- [ ] Tests added or updated (regression test included for bug fixes)
- [ ] No secrets, compiled binaries, real account IDs, or personal paths committed
- [ ] Code adheres to [CODE_QUALITY.md](../CODE_QUALITY.md):
  - [ ] No new file > 300 LOC or function > 40 LOC (unless justified)
  - [ ] No new SDK clients constructed inside business logic
  - [ ] No new `strings.Contains(err.Error(), ...)` or silent `_ =` error ignores
  - [ ] Exported symbols documented
  - [ ] Boy-scout rule applied when touching legacy files > 300 LOC
