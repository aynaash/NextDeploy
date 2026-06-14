package shared

// Version is the released version of the binary. Populated via ldflags at
// build time (see .goreleaser.yml: -X .../shared.Version={{.Tag}}). Left as a
// var, not a const, so -X can rewrite it; falls back to "dev" for local builds
// without the ldflag.
var Version = "dev"

// Commit is the short git commit the binary was built from. Populated via
// ldflags at build time (see Makefile / magefile.go / .goreleaser.yml).
// Left as a var rather than const so -X can rewrite it.
var Commit = "unknown"
