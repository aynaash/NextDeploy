# Test Specs — hand-writing guide

These are **implementation-ready test specifications**. Each file describes the
tests to write for one package: the function under test, setup, table rows with
concrete inputs/expected outputs, and design notes. You write the Go; this tells
you exactly what to write.

Grounded in the real source (signatures, fields, error sentinels) as of the
priority sweep — not guesses. Where a function isn't unit-testable as written, the
spec says so and gives the minimal refactor seam.

## Order (highest risk first)
1. [secrets.md](secrets.md) — AES-GCM crypto, **zero tests today**, found a real round-trip footgun 🔴
2. [config.md](config.md) — Load/Save, only domain logic tested 🔴
3. [revalidator.md](revalidator.md) — ISR revalidation, **zero tests** 🔴
4. [nextbuild.md](nextbuild.md) — `Run` + flag logic untested 🔴
5. [updater.md](updater.md) — self-update core untested (version cmp, checksums, extract) 🔴

See [testinfrareboot.md](../../testinfrareboot.md) at repo root for the why/priority
rationale across the whole codebase.

## House rules (match the repo)
- Pure Go stdlib `testing`. **No** testify/gomock. Use `t.Run` subtests + table rows.
- `t.TempDir()` for any filesystem; never touch the real CWD/HOME without restoring.
- Mock via interfaces or injected seams (runner/HTTP client/clock), never concrete SDK types.
- Hermetic by default. Network/cred tests go behind `//go:build integration`.
- Name tests `TestФn_Scenario`; name table fields `name`, `in`, `want`, `wantErr`.
- Assert errors with `errors.Is`/`errors.As`, never `strings.Contains(err.Error(), …)`.
- Run: `mage testUnit`; coverage: `scripts/coverage-ratchet.sh` then `--bump`.

## Definition of done per package
- Every exported symbol has at least one test.
- Every `if err != nil` branch and every `switch`/`case` has a row.
- Round-trip (encode→decode, save→load) proven where applicable.
- `go test -race ./<pkg>/...` green; `go test -cover` shows the package at/near 100%.
