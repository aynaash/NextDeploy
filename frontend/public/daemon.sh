#!/bin/bash
# NextDeploy Daemon Installation Script
# For Linux servers only - production version

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="aynaash/NextDeploy"
BIN_NAME="nextdeployd"
INSTALL_DIR="/usr/local/bin"
DOWNLOAD_DIR="/tmp/nextdeployd-install"
SYSTEMD_DIR="/etc/systemd/system"

# Parse command line arguments
SKIP_SYSTEMD=false
for arg in "$@"; do
    case $arg in
        --skip-systemd) SKIP_SYSTEMD=true ;;
        --help) echo "Usage: $0 [--skip-systemd]"; exit 0 ;;
    esac
done

# Cleanup on exit
cleanup() {
    local exit_code=$?
    if [[ -d "$DOWNLOAD_DIR" ]]; then
        rm -rf "$DOWNLOAD_DIR"
    fi
    if [[ $exit_code -ne 0 ]]; then
        echo -e "${RED} Installation failed with exit code $exit_code${NC}" >&2
    fi
    exit $exit_code
}
trap cleanup EXIT INT TERM

# Print banner
echo -e "${BLUE}===========================================${NC}"
echo -e "${GREEN}   Installing NextDeploy Daemon for Linux${NC}"
echo -e "${BLUE}===========================================${NC}"
echo ""

# Check if running on Linux
if [[ "$(uname -s)" != "Linux" ]]; then
    echo -e "${RED}Error: The NextDeploy Daemon only runs on Linux servers.${NC}" >&2
    exit 1
fi

# Check for root/sudo
if [[ $EUID -ne 0 ]]; then
    echo -e "${YELLOW}⚠️  This script requires root privileges for system-wide installation.${NC}"
    if command -v sudo >/dev/null 2>&1; then
        echo "Re-running with sudo..."
        exec sudo bash "$0" "$@"
    else
        echo -e "${RED}Error: Please run as root or install sudo.${NC}" >&2
        exit 1
    fi
fi

# Check dependencies
check_dependencies() {
    local missing=()

    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        missing+=("curl or wget")
    fi

    if ! command -v sha256sum >/dev/null 2>&1; then
        missing+=("sha256sum")
    fi

    if ! command -v tar >/dev/null 2>&1; then
        missing+=("tar")
    fi

    if [[ "$SKIP_SYSTEMD" == false ]] && ! command -v systemctl >/dev/null 2>&1; then
        echo -e "${YELLOW} systemctl not found. Use --skip-systemd for non-systemd systems.${NC}" >&2
        missing+=("systemd (or use --skip-systemd)")
    fi

    if [[ ${#missing[@]} -gt 0 ]]; then
        echo -e "${RED}Error: Missing required dependencies:${NC}" >&2
        printf '  - %s\n' "${missing[@]}" >&2
        exit 1
    fi
}

# Detect platform
detect_platform() {
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) echo -e "${RED}Unsupported architecture: $ARCH${NC}" >&2; exit 1 ;;
    esac

    echo -e "${GREEN}✓ Detected:${NC} Linux/${ARCH}"
}

# Download function
download_file() {
    local url="$1"
    local output="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 30 --retry 3 --retry-delay 2 -o "$output" "$url"
    else
        wget -q --timeout=30 --tries=3 --retry-connrefused -O "$output" "$url"
    fi
}

# Get latest release
get_latest_release() {
    local api_url="https://api.github.com/repos/$REPO/releases/latest"
    local response

    echo -e "${BLUE}📡 Fetching latest release info...${NC}"

    if command -v curl >/dev/null 2>&1; then
        response=$(curl -fsSL -H "Accept: application/vnd.github.v3+json" "$api_url")
    else
        response=$(wget -q -O - --header="Accept: application/vnd.github.v3+json" "$api_url")
    fi

    if [[ -z "$response" ]]; then
        echo -e "${RED}Error: Failed to fetch from GitHub API${NC}" >&2
        exit 1
    fi

    # Extract tag_name using grep/sed
    local tag
    tag=$(echo "$response" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)

    if [[ -z "$tag" ]]; then
        echo -e "${RED}Error: Could not parse GitHub response${NC}" >&2
        exit 1
    fi

    echo "$tag"
}

# Get checksums
get_checksums() {
    local version="$1"
    local clean_version="${version#v}"
    local checksums_url="https://github.com/$REPO/releases/download/$version/nextdeploy_${clean_version}_checksums.txt"
    local checksums_file="$DOWNLOAD_DIR/checksums.txt"

    if download_file "$checksums_url" "$checksums_file" 2>/dev/null; then
        cat "$checksums_file"
        return 0
    fi
    return 1
}

# Create systemd service file
create_systemd_service() {
    local user="${SUDO_USER:-$USER}"
    local home
    home=$(eval echo "~$user")

    cat > "$SYSTEMD_DIR/nextdeployd.service" << EOF
[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
Type=simple
User=$user
Group=$user
WorkingDirectory=$home
ExecStart=$INSTALL_DIR/nextdeployd serve
Restart=always
RestartSec=10
Environment="PATH=/usr/local/bin:/usr/bin:/bin:$home/.nextdeploy/bin"

[Install]
WantedBy=multi-user.target
EOF

    echo -e "${GREEN}✓ Systemd service created${NC}"
}

# Main installation
main() {
    # Check dependencies
    check_dependencies

    # Detect platform
    detect_platform

    # Get version
    VERSION=$(get_latest_release)
    echo -e "${GREEN}✓ Latest version:${NC} $VERSION"

    local CLEAN_VERSION="${VERSION#v}"

    # Create download directory
    mkdir -p "$DOWNLOAD_DIR"

    # Download archive (GoReleaser format: nextdeployd_VERSION_Linux_ARCH.tar.gz)
    ARCHIVE_NAME="nextdeployd_${CLEAN_VERSION}_Linux_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE_NAME"
    ARCHIVE_PATH="$DOWNLOAD_DIR/$ARCHIVE_NAME"

    echo -e "${BLUE}📥 Downloading ${ARCHIVE_NAME}...${NC}"
    if ! download_file "$DOWNLOAD_URL" "$ARCHIVE_PATH"; then
        echo -e "${RED}Error: Download failed${NC}" >&2
        exit 1
    fi

    # Verify download size
    local size
    size=$(stat -c%s "$ARCHIVE_PATH" 2>/dev/null || stat -f%z "$ARCHIVE_PATH" 2>/dev/null)
    if [[ $size -lt 1000000 ]]; then  # 1MB minimum for compressed archive
        echo -e "${RED}Error: Downloaded file too small (${size} bytes)${NC}" >&2
        exit 1
    fi
    echo -e "${GREEN}✓ Download size: $(numfmt --to=iec $size)${NC}"

    # Verify checksums
    echo -e "${BLUE}🔒 Verifying checksum...${NC}"
    if CHECKSUMS=$(get_checksums "$VERSION"); then
        EXPECTED=$(echo "$CHECKSUMS" | grep "$ARCHIVE_NAME" | awk '{print $1}')
        if [[ -n "$EXPECTED" ]]; then
            ACTUAL=$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')
            if [[ "$ACTUAL" != "$EXPECTED" ]]; then
                echo -e "${RED}Error: Checksum verification failed${NC}" >&2
                echo "Expected: $EXPECTED" >&2
                echo "Actual:   $ACTUAL" >&2
                exit 1
            fi
            echo -e "${GREEN}✓ Checksum verified${NC}"
        else
            echo -e "${YELLOW}⚠️  No checksum found for $ARCHIVE_NAME${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  Could not download checksums, skipping verification${NC}"
    fi

    # Extract binary from archive
    echo -e "${BLUE}📦 Extracting binary...${NC}"
    tar -xzf "$ARCHIVE_PATH" -C "$DOWNLOAD_DIR"

    BINARY_PATH="$DOWNLOAD_DIR/$BIN_NAME"
    if [[ ! -f "$BINARY_PATH" ]]; then
        echo -e "${RED}Error: Binary '$BIN_NAME' not found in archive${NC}" >&2
        exit 1
    fi

    # Make binary executable
    chmod +x "$BINARY_PATH"

    # Verify binary
    echo -e "${BLUE}🔍 Verifying binary...${NC}"
    if "$BINARY_PATH" version | grep -qF "$CLEAN_VERSION"; then
        echo -e "${GREEN}✓ Binary verified${NC}"
    else
        echo -e "${YELLOW}⚠️  Warning: Binary version mismatch${NC}"
        "$BINARY_PATH" version
    fi

    # Stop existing service if running
    if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet nextdeployd 2>/dev/null; then
        echo -e "${YELLOW}⏸️  Stopping existing nextdeployd service...${NC}"
        systemctl stop nextdeployd
    fi

    # Backup existing binary
    if [[ -f "$INSTALL_DIR/$BIN_NAME" ]]; then
        local backup_path="$INSTALL_DIR/$BIN_NAME.backup.$(date +%Y%m%d_%H%M%S)"
        echo -e "${YELLOW}⚠️  Backing up existing binary to $backup_path${NC}"
        mv "$INSTALL_DIR/$BIN_NAME" "$backup_path"
    fi

    # Install binary
    echo -e "${BLUE}📦 Installing to $INSTALL_DIR/$BIN_NAME...${NC}"
    mv "$BINARY_PATH" "$INSTALL_DIR/$BIN_NAME"
    chmod 755 "$INSTALL_DIR/$BIN_NAME"

    # Create systemd service if not skipped
    if [[ "$SKIP_SYSTEMD" == false ]] && command -v systemctl >/dev/null 2>&1; then
        echo -e "${BLUE}🔧 Configuring systemd service...${NC}"
        create_systemd_service
        systemctl daemon-reload
        echo -e "${GREEN}✓ Service configured. Start with: systemctl start nextdeployd${NC}"
    fi

    # Final verification
    echo -e "${BLUE}🔍 Final verification...${NC}"
    INSTALLED_VERSION=$("$INSTALL_DIR/$BIN_NAME" version 2>/dev/null | head -1)

    echo ""
    echo -e "${GREEN}===========================================${NC}"
    echo -e "${GREEN}✅ NextDeploy Daemon ${INSTALLED_VERSION} installed!${NC}"
    echo -e "${GREEN}===========================================${NC}"
    echo ""
    echo "📍 Location: $INSTALL_DIR/$BIN_NAME"
    echo ""

    if [[ "$SKIP_SYSTEMD" == false ]] && command -v systemctl >/dev/null 2>&1; then
        echo "🚀 To start the daemon:"
        echo "   sudo systemctl start nextdeployd"
        echo "   sudo systemctl enable nextdeployd"
        echo ""
        echo "📊 To check status:"
        echo "   sudo systemctl status nextdeployd"
        echo "   sudo journalctl -u nextdeployd -f"
    else
        echo "🚀 To start the daemon manually:"
        echo "   sudo nextdeployd serve &"
        echo ""
        echo "💡 For production, consider setting up a process supervisor."
    fi
    echo ""
    echo -e "${YELLOW}💡 The daemon will be managed by 'nextdeploy prepare' and 'nextdeploy ship' commands.${NC}"
    echo ""
}

# Run main
main "$@"
