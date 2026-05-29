#!/bin/sh
# agent-fleet installer
# Usage:
#   curl -sSL https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | sh
#
# For private repos:
#   curl -sSL -H "Authorization: token $GITHUB_TOKEN" \
#     https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | GITHUB_TOKEN=$GITHUB_TOKEN sh

set -e

REPO="donbader/agent-fleet"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="agent-fleet"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { printf "${GREEN}▸${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}▸${NC} %s\n" "$1"; }
error() { printf "${RED}✗${NC} %s\n" "$1" >&2; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)      error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)             error "Unsupported architecture: $ARCH" ;;
    esac

    info "Detected platform: ${OS}/${ARCH}"
}

# Resolve GitHub token: gh CLI > GITHUB_TOKEN env var
resolve_token() {
    if [ -n "$GITHUB_TOKEN" ]; then
        return
    fi

    if command -v gh >/dev/null 2>&1; then
        GITHUB_TOKEN=$(gh auth token 2>/dev/null || true)
        if [ -n "$GITHUB_TOKEN" ]; then
            info "Using token from gh CLI"
        fi
    fi
}

# Get the latest release version
get_latest_version() {
    local url="https://api.github.com/repos/${REPO}/releases/latest"
    local auth_header=""

    resolve_token

    if [ -n "$GITHUB_TOKEN" ]; then
        auth_header="Authorization: token ${GITHUB_TOKEN}"
    fi

    if [ -n "$auth_header" ]; then
        VERSION=$(curl -sSL -H "$auth_header" "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    else
        VERSION=$(curl -sSL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    fi

    if [ -z "$VERSION" ]; then
        error "Failed to get latest version. If this is a private repo, set GITHUB_TOKEN."
    fi

    info "Latest version: ${VERSION}"
}

# Download and install
download_and_install() {
    local version_no_v="${VERSION#v}"
    local filename="${BINARY_NAME}_${version_no_v}_${OS}_${ARCH}.tar.gz"
    local url="https://github.com/${REPO}/releases/download/${VERSION}/${filename}"
    local auth_header=""

    if [ -n "$GITHUB_TOKEN" ]; then
        auth_header="Authorization: token ${GITHUB_TOKEN}"
    fi

    info "Downloading ${filename}..."

    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT

    if [ -n "$auth_header" ]; then
        # For private repos, need to follow redirects with auth
        curl -sSL -H "$auth_header" -H "Accept: application/octet-stream" \
            "$url" -o "${tmp_dir}/${filename}" || error "Download failed"
    else
        curl -sSL "$url" -o "${tmp_dir}/${filename}" || error "Download failed"
    fi

    info "Extracting..."
    tar -xzf "${tmp_dir}/${filename}" -C "$tmp_dir"

    # Install binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    info "Installed ${BINARY_NAME} ${VERSION} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Verify installation
verify() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        info "Verification: $(${BINARY_NAME} --version)"
    else
        warn "${BINARY_NAME} installed but not in PATH. Add ${INSTALL_DIR} to your PATH."
    fi
}

main() {
    info "Installing agent-fleet..."
    detect_platform
    get_latest_version
    download_and_install
    verify
    printf "\n${GREEN}✓${NC} Installation complete!\n"
}

main
