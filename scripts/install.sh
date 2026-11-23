#!/bin/sh
# NextDeploy CLI installer — POSIX sh compatible (works with `| sh` and `| bash`).
#
# Usage:
#   curl -fsSL https://nextdeploy.org/install.sh | sh
#   NEXTDEPLOY_VERSION=v0.20.2 curl -fsSL https://nextdeploy.org/install.sh | sh
#   NEXTDEPLOY_INSTALL_DIR=/usr/local/bin curl -fsSL https://nextdeploy.org/install.sh | sh

set -eu

REPO="aynaash/NextDeploy"
BINARY_NAME="nextdeploy"
TMP_DIR=""
INSTALL_SUCCESS=0

say() {
  printf '%s\n' "$1"
}

err() {
  printf 'Error: %s\n' "$1" >&2
}

cleanup() {
  exit_code=$?
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
  if [ "$exit_code" -ne 0 ] && [ "$INSTALL_SUCCESS" -eq 0 ]; then
    printf 'Installation failed (exit code: %d)\n' "$exit_code" >&2
  fi
}

trap cleanup EXIT

detect_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    darwin|linux) printf '%s' "$os" ;;
    *)
      err "Unsupported operating system: $os"
      exit 1
      ;;
  esac
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *)
      err "Unsupported architecture: $arch"
      exit 1
      ;;
  esac
}

fetch_latest_version() {
  # Resolve the latest tag via GitHub's redirect (releases/latest →
  # releases/tag/<tag>) instead of the REST API. The API is rate-limited to 60
  # unauthenticated requests/hour per IP and returns 403 once exhausted, which
  # breaks `curl | sh` installs on shared or CI IPs. The redirect is a plain
  # github.com page fetch, not subject to the API limit.
  releases_url="https://github.com/${REPO}/releases/latest"
  final_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$releases_url")
  tag=${final_url##*/}
  if [ -z "$tag" ]; then
    err "Could not resolve latest release version from ${releases_url}"
    exit 1
  fi
  printf '%s' "$tag"
}

verify_checksum() {
  archive_path=$1
  archive_name=$2
  checksums_path=$3

  expected=$(awk -v f="$archive_name" '$2 == f { print $1; exit }' "$checksums_path")
  if [ -z "$expected" ]; then
    err "Archive ${archive_name} not found in checksums.txt"
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$archive_path" | awk '{ print $1 }')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$archive_path" | awk '{ print $1 }')
  else
    say "Warning: neither sha256sum nor shasum found — skipping checksum verification"
    return 0
  fi

  if [ "$actual" != "$expected" ]; then
    err "Checksum mismatch for ${archive_name} (expected ${expected}, got ${actual})"
    exit 1
  fi
}

choose_install_dir() {
  if [ -n "${NEXTDEPLOY_INSTALL_DIR:-}" ]; then
    printf '%s' "$NEXTDEPLOY_INSTALL_DIR"
    return
  fi
  # Prefer an existing user-writable bin directory on PATH (no sudo needed).
  for dir in /opt/homebrew/bin /usr/local/bin; do
    if [ -d "$dir" ] && [ -w "$dir" ]; then
      printf '%s' "$dir"
      return
    fi
  done
  printf '%s' "$HOME/.local/bin"
}

ensure_in_path() {
  dir=$1
  case ":${PATH:-}:" in
    *":${dir}:"*) return 0 ;;
  esac

  shell_name=$(basename "${SHELL:-sh}")
  case "$shell_name" in
    zsh)  rc_file="$HOME/.zshrc" ;;
    bash)
      if [ -f "$HOME/.bash_profile" ] && [ "$(detect_os)" = "darwin" ]; then
        rc_file="$HOME/.bash_profile"
      else
        rc_file="$HOME/.bashrc"
      fi
      ;;
    *)    rc_file="$HOME/.profile" ;;
  esac

  export_line="export PATH=\"${dir}:\$PATH\""
  if [ ! -f "$rc_file" ] || ! grep -F "$dir" "$rc_file" >/dev/null 2>&1; then
    printf '\n# NextDeploy CLI\n%s\n' "$export_line" >> "$rc_file"
    say "Added ${dir} to PATH in ${rc_file}"
  fi
  say "Run 'export PATH=\"${dir}:\$PATH\"' or restart your shell to use '${BINARY_NAME}'."
}

main() {
  if ! command -v curl >/dev/null 2>&1; then
    err "curl is required to install ${BINARY_NAME}"
    exit 1
  fi
  if ! command -v tar >/dev/null 2>&1; then
    err "tar is required to install ${BINARY_NAME}"
    exit 1
  fi

  say "==========================================="
  say "   Installing NextDeploy CLI"
  say "==========================================="
  say ""

  OS=$(detect_os)
  ARCH=$(detect_arch)
  say "Platform: ${OS}/${ARCH}"

  if [ -n "${NEXTDEPLOY_VERSION:-}" ]; then
    VERSION="$NEXTDEPLOY_VERSION"
  else
    say "Fetching latest release info..."
    VERSION=$(fetch_latest_version)
  fi
  VERSION_NUM="${VERSION#v}"
  say "Version: ${VERSION}"

  ARCHIVE="${BINARY_NAME}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

  TMP_DIR=$(mktemp -d)
  ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE}"
  CHECKSUMS_PATH="${TMP_DIR}/checksums.txt"

  say "Downloading ${ARCHIVE}..."
  curl -fsSL "${BASE_URL}/${ARCHIVE}" -o "$ARCHIVE_PATH"

  say "Verifying checksum..."
  curl -fsSL "${BASE_URL}/checksums.txt" -o "$CHECKSUMS_PATH"
  verify_checksum "$ARCHIVE_PATH" "$ARCHIVE" "$CHECKSUMS_PATH"
  say "Checksum OK"

  say "Extracting binary..."
  tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"
  BIN_SRC="${TMP_DIR}/${BINARY_NAME}"
  if [ ! -f "$BIN_SRC" ]; then
    err "Binary '${BINARY_NAME}' not found in archive"
    exit 1
  fi
  chmod +x "$BIN_SRC"

  INSTALL_DIR=$(choose_install_dir)
  TARGET="${INSTALL_DIR}/${BINARY_NAME}"
  say "Installing to ${TARGET}..."

  if mkdir -p "$INSTALL_DIR" 2>/dev/null && [ -w "$INSTALL_DIR" ]; then
    mv "$BIN_SRC" "$TARGET"
  else
    if [ ! -r /dev/tty ]; then
      err "No write permission to ${INSTALL_DIR} and no interactive terminal for sudo. Re-run with NEXTDEPLOY_INSTALL_DIR=\$HOME/.local/bin"
      exit 1
    fi
    say "Elevated permissions required for ${INSTALL_DIR} (set NEXTDEPLOY_INSTALL_DIR=\$HOME/.local/bin to avoid sudo)..."
    sudo mkdir -p "$INSTALL_DIR" </dev/tty
    sudo mv "$BIN_SRC" "$TARGET" </dev/tty
  fi

  INSTALL_SUCCESS=1
  say ""
  say "Installed ${BINARY_NAME} ${VERSION} to ${TARGET}"
  ensure_in_path "$INSTALL_DIR"
}

main "$@"
