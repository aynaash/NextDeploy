# Embedded AWS Lambda Binaries

This directory holds the compiled `bootstrap` binaries embedded into the
`nextdeploy` CLI via `//go:embed lambda/*` (see `assets/assets.go`) for the
auxiliary AWS serverless Lambdas:

- `imgopt/bootstrap` (`./cmd/imgopt`) — on-the-fly Next.js image optimization
- `revalidator/bootstrap` (`./cmd/revalidator`) — SQS-driven ISR revalidation

Compiled binaries are intentionally **not** committed to Git (they are ignored
via `.gitignore`). Instead:

- Official releases compile them automatically from the release commit via
  `.goreleaser.yml` (`before.hooks: ./build-lambdas.sh`).
- For local AWS builds with `imageOptimization` or `isrRevalidation` enabled,
  run `./build-lambdas.sh` or `mage buildLambdas` before building the CLI.

This `README.md` stays tracked so `//go:embed lambda/*` always matches at least
one file on a fresh clone (allowing `go test ./...` and `go build ./cli` to
succeed without pre-building the Lambdas).
