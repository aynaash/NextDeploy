#!/usr/bin/env bash
#
# scaffold-tests.sh — drop a skipped test stub into every Go package that has no
# test file yet. Stubs compile and pass (they Skip), so the build stays green
# while marking exactly which packages still need real tests.
#
# Each stub uses the package's real name (read from its source) and is named
# <pkg>_scaffold_test.go. Re-running is idempotent: existing stubs are left alone,
# and packages that already have any *_test.go are skipped entirely.
#
# Fill in real tests, then delete the t.Skip line (or the whole stub file).
#
set -euo pipefail
cd "$(dirname "$0")/.."

created=0
skipped=0

# All dirs containing buildable .go files, excluding vendored / generated trees.
while IFS= read -r dir; do
  case "$dir" in
    *vendor*|*/.next/*|*test-serverless-app*|*/tmp/*) continue ;;
  esac

  # Skip if the dir already has any test file.
  if compgen -G "$dir/*_test.go" >/dev/null; then
    skipped=$((skipped+1))
    continue
  fi

  # Read the package name from the first non-test .go file.
  src=$(find "$dir" -maxdepth 1 -name '*.go' ! -name '*_test.go' | head -1)
  [[ -z "$src" ]] && continue
  pkg=$(awk '/^package /{print $2; exit}' "$src")
  [[ -z "$pkg" ]] && continue

  out="$dir/${pkg}_scaffold_test.go"
  [[ -f "$out" ]] && { skipped=$((skipped+1)); continue; }

  cat > "$out" <<EOF
package ${pkg}

import "testing"

// TODO(coverage): replace this scaffold with real tests for the ${pkg} package.
// Goal: 45% total coverage by 2026-09-02. See todo.md and docs/testing.md.
// Delete the t.Skip below once you add a real assertion.
func TestScaffold(t *testing.T) {
	t.Skip("TODO: write tests for ${pkg}")
}
EOF
  echo "created $out"
  created=$((created+1))
done < <(go list -f '{{.Dir}}' ./... 2>/dev/null)

echo "---"
echo "scaffolds created: $created   packages skipped (already have tests): $skipped"
echo "Next: open the new *_scaffold_test.go files and write real tests."
