#!/bin/bash

set -euo pipefail

# Configuration
REPO_OWNER="aynaash"
REPO_NAME="NextDeploy"
GITHUB_API_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Global variables
PLATFORM=""
CLI_BINARY_NAME=""
CLI_INSTALL_PATH=""
INSTALL_TYPE=""

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

log_header() {
    echo -e "${PURPLE}[★]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[→]${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Platform detection
detect_platform() {
    local os arch
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux";;
        Darwin*)    os="darwin";;
        MINGW*|CYGWIN*|MSYS*) os="windows";;
        *)          
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64";;
        arm64|aarch64)  arch="arm64";;
        i386|i686)      arch="386";;
        *)              
            log_warning "Architecture $(uname -m) may not be supported, trying amd64"
            arch="amd64"
            ;;
    esac
    
    PLATFORM="${os}-${arch}"
    log_success "Detected platform: $PLATFORM"
}

# Get binary name based on platform
get_binary_name() {
    case "$PLATFORM" in
        windows-*)
            CLI_BINARY_NAME="nextdeploy-${PLATFORM}.exe"
            ;;
        *)
            CLI_BINARY_NAME="nextdeploy-${PLATFORM}"
            ;;
    esac
    log_info "Looking for binary: $CLI_BINARY_NAME"
}

# Determine installation paths based on platform and privileges
setup_install_paths() {
    case "$PLATFORM" in
        windows-*)
            # Windows installation paths
            local program_files="/c/Program Files"
            if [ ! -d "$program_files" ]; then
                program_files="/mnt/c/Program Files"  # WSL2
            fi
            
            if [ -w "$program_files" ] 2>/dev/null; then
                # System-wide installation (Admin)
                mkdir -p "$program_files/NextDeploy"
                CLI_INSTALL_PATH="$program_files/NextDeploy/nextdeploy.exe"
                INSTALL_TYPE="system"
            else
                # User installation
                local appdata="${APPDATA:-$HOME/AppData/Roaming}"
                if [ ! -d "$(dirname "$appdata")" ]; then
                    appdata="$HOME/.nextdeploy"  # Fallback for Unix-like shells
                fi
                mkdir -p "$appdata/NextDeploy"
                CLI_INSTALL_PATH="$appdata/NextDeploy/nextdeploy.exe"
                INSTALL_TYPE="user"
            fi
            ;;
        *)
            # Unix-like systems (Linux, macOS)
            if [ -w "/usr/local/bin" ] || [ "$(id -u)" -eq 0 ]; then
                # System-wide installation
                CLI_INSTALL_PATH="/usr/local/bin/nextdeploy"
                INSTALL_TYPE="system"
            else
                # User installation
                mkdir -p "$HOME/.local/bin"
                CLI_INSTALL_PATH="$HOME/.local/bin/nextdeploy"
                INSTALL_TYPE="user"
            fi
            ;;
    esac
    
    log_info "Installation type: $INSTALL_TYPE"
    log_info "Install path: $CLI_INSTALL_PATH"
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
        case "$(uname -s)" in
            Linux*)
                log_error "  Ubuntu/Debian: sudo apt update && sudo apt install -y ${missing_deps[*]}"
                log_error "  CentOS/RHEL/Fedora: sudo dnf install -y ${missing_deps[*]}"
                log_error "  Alpine: sudo apk add --no-cache ${missing_deps[*]}"
                log_error "  Arch: sudo pacman -S ${missing_deps[*]}"
                ;;
            Darwin*)
                log_error "  Homebrew: brew install ${missing_deps[*]}"
                log_error "  MacPorts: sudo port install ${missing_deps[*]}"
                ;;
            *)
                log_error "  Please install curl and jq for your system"
                ;;
        esac
        exit 1
    fi
    
    log_success "All dependencies are available"
}

# Check installation directory permissions
check_permissions() {
    local install_dir
    install_dir=$(dirname "$CLI_INSTALL_PATH")
    
    if [ ! -w "$install_dir" ]; then
        if [ "$INSTALL_TYPE" = "system" ]; then
            log_error "Cannot write to $install_dir. Please run with sudo or as administrator."
            log_error "Alternatively, the script will attempt user installation."
            
            # Fallback to user installation
            case "$PLATFORM" in
                windows-*)
                    local appdata="${APPDATA:-$HOME/AppData/Roaming}"
                    if [ ! -d "$(dirname "$appdata")" ]; then
                        appdata="$HOME/.nextdeploy"
                    fi
                    mkdir -p "$appdata/NextDeploy"
                    CLI_INSTALL_PATH="$appdata/NextDeploy/nextdeploy.exe"
                    ;;
                *)
                    mkdir -p "$HOME/.local/bin"
                    CLI_INSTALL_PATH="$HOME/.local/bin/nextdeploy"
                    ;;
            esac
            INSTALL_TYPE="user"
            log_warning "Switched to user installation: $CLI_INSTALL_PATH"
        else
            log_error "Cannot write to $install_dir"
            exit 1
        fi
    fi
    
    # Create directory if it doesn't exist
    mkdir -p "$(dirname "$CLI_INSTALL_PATH")"
}

# Fetch latest release metadata
fetch_release_metadata() {
    log_info "Fetching latest release metadata from GitHub..."
    
    local response http_code temp_file
    temp_file=$(mktemp)
    
    # Fetch with curl and capture HTTP status code
    http_code=$(curl -s -w "%{http_code}" -o "$temp_file" "$GITHUB_API_URL")
    
    if [ "$http_code" -ne 200 ]; then
        log_error "Failed to fetch release metadata. HTTP status: $http_code"
        case "$http_code" in
            404) log_error "Repository not found or no releases available" ;;
            403) log_error "API rate limit exceeded. Please try again later" ;;
            *) log_error "GitHub API error. Please check your internet connection" ;;
        esac
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
    
    log_success "Successfully fetched release metadata"
    echo "$response"
}

# Parse download URL from release metadata
parse_download_url() {
    local release_data="$1"
    
    log_step "Parsing download URL for binary: $CLI_BINARY_NAME"
    
    local download_url tag_name
    download_url=$(echo "$release_data" | jq -r ".assets[] | select(.name == \"$CLI_BINARY_NAME\") | .browser_download_url")
    tag_name=$(echo "$release_data" | jq -r ".tag_name")
    
    if [ -z "$download_url" ] || [ "$download_url" = "null" ]; then
        log_error "CLI binary '$CLI_BINARY_NAME' not found in latest release ($tag_name)"
        log_error "Available assets:"
        echo "$release_data" | jq -r '.assets[].name' | grep -E "^nextdeploy-" | sed 's/^/  - /'
        
        # Suggest alternative if exact match not found
        local alternatives
        alternatives=$(echo "$release_data" | jq -r '.assets[].name' | grep -E "^nextdeploy-.*$(echo "$PLATFORM" | cut -d- -f1)" || true)
        if [ -n "$alternatives" ]; then
            log_warning "Similar binaries found:"
            echo "$alternatives" | sed 's/^/  - /'
        fi
        exit 1
    fi
    
    log_success "Found download URL for release $tag_name"
    log_info "Download URL: $download_url"
    echo "$download_url"
}

# Download and install binary
download_and_install() {
    local download_url="$1"
    local temp_binary
    
    log_step "Downloading NextDeploy CLI..."
    log_info "From: $download_url"
    log_info "To: $CLI_INSTALL_PATH"
    
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
    
    local file_size
    file_size=$(stat -c%s "$temp_binary" 2>/dev/null || stat -f%z "$temp_binary" 2>/dev/null || echo "unknown")
    log_success "Binary downloaded successfully ($file_size bytes)"
    
    # Make binary executable (Unix-like systems)
    if [[ "$PLATFORM" != windows-* ]]; then
        log_step "Making binary executable..."
        chmod +x "$temp_binary"
    fi
    
    # Move to installation path
    log_step "Installing binary..."
    if ! mv "$temp_binary" "$CLI_INSTALL_PATH"; then
        log_error "Failed to move binary to $CLI_INSTALL_PATH"
        rm -f "$temp_binary"
        exit 1
    fi
    
    log_success "NextDeploy CLI installed at $CLI_INSTALL_PATH"
}

# Verify installation
verify_installation() {
    log_step "Verifying installation..."
    
    if [ ! -f "$CLI_INSTALL_PATH" ]; then
        log_error "Installation verification failed: $CLI_INSTALL_PATH not found"
        exit 1
    fi
    
    if [[ "$PLATFORM" != windows-* ]] && [ ! -x "$CLI_INSTALL_PATH" ]; then
        log_error "Installation verification failed: $CLI_INSTALL_PATH is not executable"
        exit 1
    fi
    
    # Try to get version (if the binary supports --version)
    local version_output
    if version_output=$("$CLI_INSTALL_PATH" version 2>/dev/null || "$CLI_INSTALL_PATH" --version 2>/dev/null || echo ""); then
        if [ -n "$version_output" ]; then
            log_success "Installation verified. Version info:"
            echo "$version_output" | sed 's/^/  /'
        else
            log_success "Installation verified. Binary is executable at $CLI_INSTALL_PATH"
        fi
    else
        log_success "Installation verified. Binary is available at $CLI_INSTALL_PATH"
    fi
}

# Setup PATH and shell integration
setup_shell_integration() {
    if [ "$INSTALL_TYPE" = "system" ]; then
        log_success "NextDeploy CLI is installed system-wide and should be available in your PATH"
        return
    fi
    
    log_step "Setting up shell integration..."
    
    local bin_dir
    bin_dir=$(dirname "$CLI_INSTALL_PATH")
    
    case "$PLATFORM" in
        windows-*)
            log_warning "For Windows user installation:"
            log_warning "Add to your PATH: $bin_dir"
            log_warning "Or run directly: $CLI_INSTALL_PATH"
            ;;
        *)
            # Unix-like systems - update shell profiles
            local shell_files=()
            [ -f "$HOME/.bashrc" ] && shell_files+=("$HOME/.bashrc")
            [ -f "$HOME/.zshrc" ] && shell_files+=("$HOME/.zshrc")
            [ -f "$HOME/.profile" ] && shell_files+=("$HOME/.profile")
            
            local path_export="export PATH=\"$bin_dir:\$PATH\""
            local updated_files=()
            
            for shell_file in "${shell_files[@]}"; do
                if ! grep -q "$bin_dir" "$shell_file" 2>/dev/null; then
                    echo "" >> "$shell_file"
                    echo "# NextDeploy CLI" >> "$shell_file"
                    echo "$path_export" >> "$shell_file"
                    updated_files+=("$(basename "$shell_file")")
                fi
            done
            
            if [ ${#updated_files[@]} -gt 0 ]; then
                log_success "Updated shell profiles: ${updated_files[*]}"
                log_warning "Run 'source ~/.bashrc' (or restart your terminal) to update your PATH"
            else
                log_info "Shell profiles already configured or not found"
            fi
            
            # Check if directory is already in PATH
            if [[ ":$PATH:" == *":$bin_dir:"* ]]; then
                log_success "$bin_dir is already in your PATH"
            else
                log_warning "To use 'nextdeploy' command, add to your PATH:"
                log_warning "  export PATH=\"$bin_dir:\$PATH\""
            fi
            ;;
    esac
}

# Show usage information
show_usage() {
    log_success "NextDeploy CLI installed successfully! 🎉"
    echo
    log_header "Quick Start:"
    echo "  nextdeploy init     # Initialize a new project"
    echo "  nextdeploy build    # Build your application"
    echo "  nextdeploy ship     # Deploy to your server"
    echo "  nextdeploy --help   # Show all available commands"
    echo
    
    case "$INSTALL_TYPE" in
        system)
            log_info "CLI is available system-wide as 'nextdeploy'"
            ;;
        user)
            log_info "CLI installed in user directory: $CLI_INSTALL_PATH"
            case "$PLATFORM" in
                windows-*)
                    log_warning "Add to PATH or run directly: $CLI_INSTALL_PATH"
                    ;;
                *)
                    log_warning "Restart your terminal or run: source ~/.bashrc"
                    ;;
            esac
            ;;
    esac
    
    echo
    log_info "Documentation: https://github.com/aynaash/NextDeploy"
    log_info "Issues: https://github.com/aynaash/NextDeploy/issues"
}

# Cleanup function for trap
cleanup() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        log_error "Installation failed with exit code $exit_code"
    fi
    exit $exit_code
}

# Main installation function
main() {
    echo
    log_header "NextDeploy CLI Installation Script"
    echo "====================================="
    echo
    
    # Set trap for cleanup on exit
    trap cleanup EXIT
    
    # Platform detection and setup
    detect_platform
    get_binary_name
    setup_install_paths
    
    # Pre-installation checks
    check_dependencies
    check_permissions
    
    # Download and install
    local release_data download_url
    release_data=$(fetch_release_metadata)
    download_url=$(parse_download_url "$release_data")
    download_and_install "$download_url"
    
    # Post-installation
    verify_installation
    setup_shell_integration
    show_usage
    
    echo
    log_success "Installation completed successfully! 🚀"
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "NextDeploy CLI Installation Script"
        echo
        echo "Usage: $0 [options]"
        echo
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo "  --version     Show version information"
        echo
        echo "Environment variables:"
        echo "  INSTALL_DIR   Override installation directory"
        echo
        echo "Examples:"
        echo "  curl -fsSL https://raw.githubusercontent.com/aynaash/NextDeploy/main/install-nextdeploy-cli.sh | bash"
        echo "  wget -qO- https://raw.githubusercontent.com/aynaash/NextDeploy/main/install-nextdeploy-cli.sh | bash"
        exit 0
        ;;
    --version)
        echo "NextDeploy CLI Installation Script v1.0"
        exit 0
        ;;
esac

# Run main function
main "$@"
