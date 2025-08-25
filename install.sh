#!/bin/bash

set -euo pipefail

# Configuration
REPO_OWNER="aynaash"
REPO_NAME="NextDeploy"
BINARY_NAME="nextdeployd-linux-amd64"
GITHUB_API_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

# Determine install path based on permissions
if [ -w "/usr/local/bin" ] || [ "$(id -u)" -eq 0 ]; then
    INSTALL_PATH="/usr/local/bin/nextdeployd"
    INSTALL_TYPE="system"
else
    # Create local bin directory if it doesn't exist
    mkdir -p "$HOME/.local/bin"
    INSTALL_PATH="$HOME/.local/bin/nextdeployd"
    INSTALL_TYPE="user"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[+]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1" >&2
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    local missing_deps=()
    
    if ! command_exists curl; then
        missing_deps+=("curl")
    fi
    
    if ! command_exists jq; then
        missing_deps+=("jq")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_error "Please install them using:"
        log_error "  Ubuntu/Debian: sudo apt update && sudo apt install -y ${missing_deps[*]}"
        log_error "  CentOS/RHEL: sudo yum install -y ${missing_deps[*]}"
        log_error "  Alpine: sudo apk add --no-cache ${missing_deps[*]}"
        exit 1
    fi
    
    log_success "All dependencies are available"
}

# Check and create installation directory
check_permissions() {
    local install_dir
    install_dir=$(dirname "$INSTALL_PATH")
    
    if [ ! -w "$install_dir" ]; then
        if [ "$INSTALL_TYPE" = "system" ]; then
            log_error "Cannot write to $install_dir. Please run with sudo or as root."
            exit 1
        else
            log_error "Cannot write to $install_dir"
            exit 1
        fi
    fi
    
    if [ "$INSTALL_TYPE" = "user" ]; then
        log_info "Installing to user directory: $INSTALL_PATH"
        log_warning "Make sure $HOME/.local/bin is in your PATH"
    else
        log_info "Installing to system directory: $INSTALL_PATH"
    fi
}

# Fetch latest release metadata
fetch_release_metadata() {
    log_info "Fetching latest release metadata from GitHub..."
    
    local response
    local http_code
    
    # Create a temporary file to store response
    local temp_file
    temp_file=$(mktemp)
    
    # Fetch with curl and capture HTTP status code
    http_code=$(curl -s -w "%{http_code}" -o "$temp_file" "$GITHUB_API_URL")
    
    if [ "$http_code" -ne 200 ]; then
        log_error "Failed to fetch release metadata. HTTP status: $http_code"
        if [ "$http_code" -eq 404 ]; then
            log_error "Repository not found or no releases available"
        elif [ "$http_code" -eq 403 ]; then
            log_error "API rate limit exceeded. Please try again later"
        fi
        rm -f "$temp_file"
        exit 1
    fi
    
    response=$(cat "$temp_file")
    rm -f "$temp_file"
    
    # Validate JSON response
    if ! echo "$response" | jq . >/dev/null 2>&1; then
        log_error "Invalid JSON response from GitHub API"
        exit 1
    fi
    
    echo "$response"
}

# Parse download URL from release metadata
parse_download_url() {
    local release_data="$1"
    
    log_info "Parsing download URL for binary: $BINARY_NAME"
    
    local download_url
    download_url=$(echo "$release_data" | jq -r ".assets[] | select(.name == \"$BINARY_NAME\") | .browser_download_url")
    
    if [ -z "$download_url" ] || [ "$download_url" = "null" ]; then
        log_error "Binary asset '$BINARY_NAME' not found in latest release"
        log_error "Available assets:"
        echo "$release_data" | jq -r '.assets[].name' | sed 's/^/  - /'
        exit 1
    fi
    
    log_success "Found download URL: $download_url"
    echo "$download_url"
}

# Download and install binary
download_and_install() {
    local download_url="$1"
    local temp_binary
    
    log_info "Downloading binary from: $download_url"
    
    # Create temporary file for download
    temp_binary=$(mktemp)
    
    # Download with progress bar and follow redirects
    if ! curl -L --progress-bar --fail --output "$temp_binary" "$download_url"; then
        log_error "Failed to download binary from $download_url"
        rm -f "$temp_binary"
        exit 1
    fi
    
    # Verify the downloaded file is not empty
    if [ ! -s "$temp_binary" ]; then
        log_error "Downloaded file is empty"
        rm -f "$temp_binary"
        exit 1
    fi
    
    log_success "Binary downloaded successfully"
    
    # Make binary executable
    log_info "Making binary executable..."
    chmod +x "$temp_binary"
    
    # Move to installation path
    log_info "Installing binary to $INSTALL_PATH..."
    if ! mv "$temp_binary" "$INSTALL_PATH"; then
        log_error "Failed to move binary to $INSTALL_PATH"
        rm -f "$temp_binary"
        exit 1
    fi
    
    log_success "Binary installed successfully at $INSTALL_PATH"
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    if [ ! -f "$INSTALL_PATH" ]; then
        log_error "Installation verification failed: $INSTALL_PATH not found"
        exit 1
    fi
    
    if [ ! -x "$INSTALL_PATH" ]; then
        log_error "Installation verification failed: $INSTALL_PATH is not executable"
        exit 1
    fi
    
    # Try to get version (if the binary supports --version)
    local version_output
    if version_output=$("$INSTALL_PATH" --version 2>/dev/null); then
        log_success "Installation verified. Version: $version_output"
    else
        log_success "Installation verified. Binary is executable at $INSTALL_PATH"
    fi
}

# Cleanup function for trap
cleanup() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        log_error "Installation failed"
    fi
    exit $exit_code
}

# Main installation function
main() {
    echo "NextDeploy Installation Script"
    echo "=============================="
    echo
    
    # Set trap for cleanup on exit
    trap cleanup EXIT
    
    # Check dependencies
    check_dependencies
    
    # Check permissions
    check_permissions
    
    # Fetch release metadata
    local release_data
    release_data=$(fetch_release_metadata)
    
    # Parse download URL
    local download_url
    download_url=$(parse_download_url "$release_data")
    
    # Download and install
    download_and_install "$download_url"
    
    # Verify installation
    verify_installation
    
    echo
    log_success "NextDeploy daemon installed successfully!"
    
    if [ "$INSTALL_TYPE" = "user" ]; then
        log_info "Binary installed at: $INSTALL_PATH"
        log_info "To use nextdeploy-daemon from anywhere, add this to your shell profile:"
        log_info "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc"
        log_info "  source ~/.bashrc"
        log_info ""
        log_info "Or run directly with: $INSTALL_PATH"
    else
        log_info "You can now use: nextdeployd"
    fi
    
    # Show usage hint if binary supports help
    if "$INSTALL_PATH" --help >/dev/null 2>&1; then
        log_info "Run 'nextdeployd --help' for usage information"
    fi
}

# Run main function
main "$@"
