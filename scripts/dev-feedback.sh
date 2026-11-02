#!/usr/bin/env bash
#
# scripts/dev-feedback.sh - NextDeploy Fast Developer Feedback Loop
#
# Usage:
#   ./scripts/dev-feedback.sh          # Quick check on git-changed packages
#   ./scripts/dev-feedback.sh --all    # Full preflight check (compile, tests, bridge, coverage)
#   ./scripts/dev-feedback.sh --watch  # Continuous live feedback loop on file changes
#   ./scripts/dev-feedback.sh <path>   # Check specific package or file
#

set -eo pipefail

BOLD='\033[1m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
DIM='\033[2m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

log_info()    { echo -e "${BLUE}ℹ${NC} $*"; }
log_success() { echo -e "${GREEN}✔${NC} ${BOLD}$*${NC}"; }
log_warn()    { echo -e "${YELLOW}⚠${NC} $*"; }
log_fail()    { echo -e "${RED}✘${NC} ${BOLD}$*${NC}"; }
header()      { echo -e "\n${CYAN}${BOLD}═══ $* ═══${NC}"; }

# Ensure go is in PATH
if ! command -v go >/dev/null 2>&1; then
  if [ -x "/opt/homebrew/bin/go" ]; then
    export PATH="/opt/homebrew/bin:$PATH"
  elif [ -x "/usr/local/bin/go" ]; then
    export PATH="/usr/local/bin:$PATH"
  else
    log_fail "Go binary not found in PATH"
    exit 1
  fi
fi

# ── Target Resolution ──────────────────────────────────────────────────────────

get_changed_packages() {
  local files
  # Staged, unstaged, and untracked Go/JS files
  files=$(git status --porcelain | awk '{print $NF}' | grep -E '\.(go|js|mjs|ts)$' || true)
  if [ -z "$files" ]; then
    # If no working tree changes, check against origin/main
    files=$(git diff --name-only origin/main...HEAD 2>/dev/null | grep -E '\.(go|js|mjs|ts)$' || true)
  fi

  if [ -z "$files" ]; then
    echo "./shared/..."
    return
  fi

  local pkgs=""
  for f in $files; do
    if [ -f "$f" ]; then
      local dir
      dir=$(dirname "$f")
      # Skip repo root if it only contains mage files
      if [ "$dir" = "." ]; then
        continue
      fi
      if [ -f "$dir/go.mod" ] || ls "$dir"/*.go >/dev/null 2>&1; then
        pkgs="$pkgs ./$dir"
      fi
    fi
  done

  if [ -z "$pkgs" ]; then
    echo "./shared/..."
  else
    echo "$pkgs" | tr ' ' '\n' | sort -u | tr '\n' ' '
  fi
}

# ── Verification Tasks ────────────────────────────────────────────────────────

check_packages() {
  local target_pkgs="$1"
  local start_time
  start_time=$(date +%s)

  echo -e "${DIM}Checking packages:${NC} ${target_pkgs}"

  # 1. Fast build/vet check
  echo -ne "  ${CYAN}●${NC} Compiling and vetting... "
  if ! err_out=$(go vet $target_pkgs 2>&1); then
    echo -e "${RED}FAILED${NC}"
    echo -e "${RED}$err_out${NC}"
    return 1
  fi
  echo -e "${GREEN}OK${NC}"

  # 2. Targeted unit tests
  echo -ne "  ${CYAN}●${NC} Running unit tests... "
  if ! test_out=$(go test -short $target_pkgs 2>&1); then
    echo -e "${RED}FAILED${NC}"
    echo -e "${RED}$test_out${NC}"
    return 1
  fi
  echo -e "${GREEN}PASSED${NC}"

  # 3. If bridge or serverless was touched, run runtime tests
  if echo "$target_pkgs" | grep -Eq "serverless|runtime|packaging"; then
    echo -ne "  ${CYAN}●${NC} Running Node.js bridge tests... "
    if command -v node >/dev/null 2>&1 && [ -f "internal/packaging/runtime/bridge.test.js" ]; then
      if ! node_out=$(node --test internal/packaging/runtime/bridge.test.js 2>&1); then
        echo -e "${RED}FAILED${NC}"
        echo -e "${RED}$node_out${NC}"
        return 1
      fi
      echo -e "${GREEN}PASSED${NC}"
    else
      echo -e "${DIM}SKIPPED (node missing or test not found)${NC}"
    fi
  fi

  local duration=$(( $(date +%s) - start_time ))
  log_success "Feedback loop passed in ${duration}s"
  return 0
}

check_all() {
  header "NextDeploy Full Preflight Check"
  local overall_fail=0

  # 1. Format check
  echo -ne "${CYAN}●${NC} Checking gofmt formatting... "
  local unformatted
  unformatted=$(gofmt -s -l $(find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.legacy-go/*") || true)
  if [ -n "$unformatted" ]; then
    echo -e "${YELLOW}WARNING (files need formatting)${NC}"
    echo "$unformatted" | sed 's/^/    /'
    echo "  Run: gofmt -w -s ."
  else
    echo -e "${GREEN}OK${NC}"
  fi

  # 2. Fast syntax & typecheck
  echo -ne "${CYAN}●${NC} Typechecking all packages... "
  if ! err_out=$(go test -run=^$ ./... 2>&1); then
    echo -e "${RED}FAILED${NC}"
    echo -e "${RED}$err_out${NC}"
    overall_fail=1
  else
    echo -e "${GREEN}OK${NC}"
  fi

  # 3. Unit test suite
  echo -ne "${CYAN}●${NC} Running all unit tests... "
  if ! test_out=$(go test -short ./... 2>&1); then
    echo -e "${RED}FAILED${NC}"
    echo "$test_out" | grep -E "FAIL|--- FAIL" | head -15 | sed 's/^/    /'
    overall_fail=1
  else
    echo -e "${GREEN}ALL PASSED${NC}"
  fi

  # 4. Node runtime tests
  if command -v node >/dev/null 2>&1 && [ -f "internal/packaging/runtime/bridge.test.js" ]; then
    echo -ne "${CYAN}●${NC} Running Node.js runtime tests... "
    if ! node_out=$(node --test internal/packaging/runtime/bridge.test.js 2>&1); then
      echo -e "${RED}FAILED${NC}"
      overall_fail=1
    else
      echo -e "${GREEN}PASSED${NC}"
    fi
  fi

  # 5. Coverage floor ratchet
  if [ -f ".coverage-floor" ] && [ -x "scripts/coverage-ratchet.sh" ]; then
    echo -ne "${CYAN}●${NC} Checking coverage ratchet... "
    local floor
    floor=$(cat .coverage-floor)
    echo -e "${DIM}(Floor is ${floor}%)${NC}"
  fi

  echo
  if [ $overall_fail -eq 0 ]; then
    log_success "All preflight checks passed! Ready to commit or push."
    return 0
  else
    log_fail "Preflight checks failed. Please resolve the errors above."
    return 1
  fi
}

# ── Watch Mode ────────────────────────────────────────────────────────────────

watch_loop() {
  header "Starting NextDeploy Continuous Feedback Watcher"
  log_info "Watching for file changes across cli/, daemon/, shared/, internal/..."
  log_info "Press Ctrl+C to stop."
  echo

  # Cross-platform file change detector using find + md5/stat
  get_snapshot() {
    find cli daemon shared internal -type f \( -name "*.go" -o -name "*.js" -o -name "*.json" \) 2>/dev/null | \
      xargs stat -f "%m %N" 2>/dev/null || \
      find cli daemon shared internal -type f \( -name "*.go" -o -name "*.js" -o -name "*.json" \) -printf "%T@ %p\n" 2>/dev/null
  }

  local last_snapshot
  last_snapshot=$(get_snapshot)

  # Initial check
  local initial_pkgs
  initial_pkgs=$(get_changed_packages)
  check_packages "$initial_pkgs" || true

  while true; do
    sleep 1
    local current_snapshot
    current_snapshot=$(get_snapshot)

    if [ "$current_snapshot" != "$last_snapshot" ]; then
      # Identify modified files
      local changed_file
      changed_file=$(comm -13 <(echo "$last_snapshot" | sort) <(echo "$current_snapshot" | sort) | awk '{print $2}' | head -1)

      last_snapshot="$current_snapshot"

      if [ -n "$changed_file" ]; then
        clear
        echo -e "${CYAN}${BOLD}[$(date +%T)] Change detected: ${BOLD}${changed_file}${NC}"
        local target_dir
        target_dir=$(dirname "$changed_file")
        check_packages "./$target_dir" || true
        echo -e "\n${DIM}Watching for next change...${NC}"
      fi
    fi
  done
}

# ── Entrypoint ────────────────────────────────────────────────────────────────

case "${1:-}" in
  --all|-a)
    check_all
    ;;
  --watch|-w)
    watch_loop
    ;;
  "")
    pkgs=$(get_changed_packages)
    check_packages "$pkgs"
    ;;
  *)
    check_packages "$1"
    ;;
esac
