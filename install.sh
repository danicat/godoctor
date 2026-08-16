#!/usr/bin/env bash

# install.sh - GoDoctor Installer
# 1. Downloads prebuilt release binary (via GoReleaser) or falls back to 'go install'
# 2. Automatically executes 'godoctor init' to configure MCP server and Agent Skills

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

REPO="danicat/godoctor"
VERSION="latest"
BUILD_FROM_SOURCE="false"
INIT_ARGS=()

print_usage() {
  cat << 'EOF'
GoDoctor Installer

Usage:
  install.sh [components] [options]
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- [components] [options]

Components (default: all):
  --mcp                    Register the MCP server in mcp_config.json
  --skills                 Unpack embedded skills (@godoctor, @selene, @testquery)

Options:
  -g, --global             Initialize in global user config (Default: ~/.gemini/config)
  -w, --workspace          Initialize in workspace scope (.agents/)
  -v, --version <v>        Target GoDoctor release version (Default: latest)
      --build              Build from source via 'go install' instead of prebuilt binary
  -f, --force              Force overwrite of existing skill files
  -q, --quiet              Quiet / script-friendly output
  -c, --config <path>      Explicit path to mcp_config.json
  -s, --skills-dir <dir>   Explicit directory for skills installation
  -h, --help               Show this help message

Examples:
  ./install.sh                      # Install binary + configure MCP & Skills globally
  ./install.sh -w                   # Install binary + configure MCP & Skills in workspace (.agents/)
  ./install.sh --mcp                # Install binary + configure MCP server only
  ./install.sh --skills             # Install binary + configure Skills only
EOF
}

# Parse installer arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      print_usage
      exit 0
      ;;
    --build)
      BUILD_FROM_SOURCE="true"
      shift
      ;;
    -v|--version)
      VERSION="$2"
      shift 2
      ;;
    *)
      INIT_ARGS+=("$1")
      shift
      ;;
  esac
done

echo -e "${BLUE}===============================================${NC}"
echo -e "${BLUE}           GoDoctor Installer                  ${NC}"
echo -e "${BLUE}===============================================${NC}"
echo -e "Version: ${BOLD}${VERSION}${NC}"
echo ""

# Determine target install directory for binary
GOPATH_BIN=""
if command -v go &> /dev/null; then
  GOBIN="$(go env GOBIN)"
  if [ -n "${GOBIN}" ]; then
    INSTALL_BIN_DIR="${GOBIN}"
  else
    INSTALL_BIN_DIR="$(go env GOPATH)/bin"
  fi
else
  INSTALL_BIN_DIR="${HOME}/.local/bin"
fi

mkdir -p "${INSTALL_BIN_DIR}"
BIN_PATH="${INSTALL_BIN_DIR}/godoctor"
BINARY_INSTALLED="false"

# 1. Attempt prebuilt release binary download (GoReleaser)
if [ "${BUILD_FROM_SOURCE}" != "true" ]; then
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  case "${ARCH}" in
    x86_64|amd64) ARCH="x64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) ARCH="" ;;
  esac

  if [ -n "${ARCH}" ] && [[ "${OS}" =~ ^(darwin|linux)$ ]]; then
    echo -e "📦 ${BLUE}[Binary] Fetching prebuilt binary for ${OS}.${ARCH}...${NC}"

    if [ "${VERSION}" = "latest" ]; then
      RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${OS}.${ARCH}.godoctor.tar.gz"
    else
      CLEAN_VER="${VERSION#v}"
      RELEASE_URL="https://github.com/${REPO}/releases/download/v${CLEAN_VER}/${OS}.${ARCH}.godoctor.tar.gz"
    fi

    TMP_DIR="$(mktemp -d)"
    TAR_FILE="${TMP_DIR}/godoctor.tar.gz"

    if curl -fsSL "${RELEASE_URL}" -o "${TAR_FILE}" 2>/dev/null; then
      if tar -xzf "${TAR_FILE}" -C "${TMP_DIR}" 2>/dev/null; then
        if [ -f "${TMP_DIR}/bin/godoctor" ]; then
          mv "${TMP_DIR}/bin/godoctor" "${BIN_PATH}"
          chmod +x "${BIN_PATH}"
          BINARY_INSTALLED="true"
          echo -e "  ${GREEN}✓ Downloaded and installed to ${BIN_PATH}${NC}"
        elif [ -f "${TMP_DIR}/godoctor" ]; then
          mv "${TMP_DIR}/godoctor" "${BIN_PATH}"
          chmod +x "${BIN_PATH}"
          BINARY_INSTALLED="true"
          echo -e "  ${GREEN}✓ Downloaded and installed to ${BIN_PATH}${NC}"
        fi
      fi
    fi
    rm -rf "${TMP_DIR}"
  fi
fi

# 2. Fallback to 'go install' if release download was not used or failed
if [ "${BINARY_INSTALLED}" != "true" ]; then
  echo -e "🔨 ${BLUE}[Binary] Compiling from source via 'go install'...${NC}"
  if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Error: 'go' toolchain is required to build from source.${NC}" >&2
    exit 1
  fi

  INSTALL_TARGET="github.com/${REPO}/cmd/godoctor@${VERSION}"
  go install "${INSTALL_TARGET}"
  if [ -f "${BIN_PATH}" ]; then
    echo -e "  ${GREEN}✓ Installed to ${BIN_PATH}${NC}"
  else
    echo -e "${RED}❌ Error: Binary not found at ${BIN_PATH} after go install.${NC}" >&2
    exit 1
  fi
fi

# 3. Check if binary directory is in PATH
if ! command -v godoctor &> /dev/null; then
  echo -e "${YELLOW}⚠️  Note: '${INSTALL_BIN_DIR}' is not currently in your \$PATH.${NC}"
  echo "  Add it to your shell profile (~/.zshrc or ~/.bashrc):"
  echo -e "  ${BLUE}export PATH=\"${INSTALL_BIN_DIR}:\$PATH\"${NC}"
  echo ""
fi

# 4. Delegate surface initialization to 'godoctor install'
echo -e "⚙️  ${BLUE}[Surfaces] Running 'godoctor install'...${NC}"
"${BIN_PATH}" install "${INIT_ARGS[@]+"${INIT_ARGS[@]}"}"
