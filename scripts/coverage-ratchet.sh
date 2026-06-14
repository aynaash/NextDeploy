#!/usr/bin/env bash
#
# coverage-ratchet.sh — enforce a non-regressing coverage floor.
#
# Measures total statement coverage across all packages and compares it to the
# committed floor in .coverage-floor. The build FAILS if coverage drops below the
# floor (minus a small slack for float noise). As coverage climbs, raise the floor
# with `--bump` and commit the change, so coverage can only go up over time.
#
# Goal: 45% by 2026-09-02. This script reports the current gap to that target.
#
# Usage:
#   scripts/coverage-ratchet.sh          # measure + enforce floor
#   scripts/coverage-ratchet.sh --bump   # raise .coverage-floor to current coverage
#
set -euo pipefail

FLOOR_FILE=".coverage-floor"
PROFILE="coverage.out"
TARGET="45.0"
SLACK="0.1"   # allow 0.1% float jitter before failing

cd "$(dirname "$0")/.."

echo "==> Measuring coverage across all packages..."
go test -covermode=atomic -coverprofile="$PROFILE" ./... >/dev/null 2>&1 || true

if [[ ! -f "$PROFILE" ]]; then
  echo "FAIL: no coverage profile produced ($PROFILE)"
  exit 1
fi

TOTAL=$(go tool cover -func="$PROFILE" | awk '/^total:/ {gsub(/%/,"",$NF); print $NF}')
FLOOR=$(cat "$FLOOR_FILE" 2>/dev/null || echo "0")

if [[ -z "${TOTAL:-}" ]]; then
  echo "FAIL: could not parse total coverage"
  exit 1
fi

GAP=$(awk -v t="$TARGET" -v c="$TOTAL" 'BEGIN { printf "%.1f", (t - c) }')

printf 'coverage: %s%%   floor: %s%%   target: %s%% (gap %s%%)\n' \
  "$TOTAL" "$FLOOR" "$TARGET" "$GAP"

if [[ "${1:-}" == "--bump" ]]; then
  # Only ever raise the floor, never lower it.
  if awk -v c="$TOTAL" -v f="$FLOOR" 'BEGIN { exit !(c > f) }'; then
    printf '%s\n' "$TOTAL" > "$FLOOR_FILE"
    echo "==> Raised floor: ${FLOOR}% -> ${TOTAL}% (commit $FLOOR_FILE)"
  else
    echo "==> Floor unchanged (current ${TOTAL}% not above floor ${FLOOR}%)"
  fi
  exit 0
fi

# Regression gate: fail if coverage fell below floor - slack.
if awk -v c="$TOTAL" -v f="$FLOOR" -v s="$SLACK" 'BEGIN { exit !(c < f - s) }'; then
  echo "FAIL: coverage ${TOTAL}% dropped below floor ${FLOOR}%."
  echo "      Add tests, or if this is intentional, lower $FLOOR_FILE deliberately."
  exit 1
fi

echo "OK: coverage holds at or above the floor."
if awk -v c="$TOTAL" -v t="$TARGET" 'BEGIN { exit !(c >= t) }'; then
  echo "🎯 Target of ${TARGET}% reached!"
fi
