#!/usr/bin/env bash

# install.sh - Installs GoDoctor Agent Plugin (Agent Plugins Spec v1.0.0)

set -euo pipefail

show_help() {
  echo "GoDoctor Plugin Installer (Agent Plugins v1.0.0)"
  echo "================================================"
  echo "Usage: ./install.sh [options]"
  echo ""
  echo "Target Options:"
  echo "  -t, --target <mode>  Target runtime: agy2 | cli (Default: agy2)"
  echo ""
  echo "Scope Options:"
  echo "  -g, --global         Install globally (Default)"
  echo "  -w, --workspace      Install locally to the active workspace"
  echo ""
  echo "General Options:"
  echo "  -f, --overwrite      Overwrite existing target directory if it exists"
  echo "  -h, --help           Show this help message"
  echo ""
}

TARGET_MODE="agy2"
INSTALL_SCOPE="global"
OVERWRITE="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -t|--target)
      if [[ -z "${2:-}" ]]; then
        echo "❌ Error: --target requires a mode argument." >&2
        exit 1
      fi
      TARGET_MODE="$2"
      shift 2
      ;;
    -g|--global)
      INSTALL_SCOPE="global"
      shift
      ;;
    -w|--workspace)
      INSTALL_SCOPE="workspace"
      shift
      ;;
    -f|--overwrite)
      OVERWRITE="true"
      shift
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    *)
      echo "❌ Error: Unknown option $1" >&2
      show_help
      exit 1
      ;;
  esac
done

# Validate target mode
case "${TARGET_MODE}" in
  cli|agy2)
    ;;
  *)
    echo "❌ Error: Unsupported target mode '${TARGET_MODE}'." >&2
    echo "Valid options for --target are: cli, agy2" >&2
    exit 1
    ;;
esac

# 1. Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${OS}" in
  darwin)  OS="darwin" ;;
  linux)   OS="linux" ;;
  *)
    echo "❌ Error: OS '${OS}' is not supported by this installer script. For Windows, download and extract the release zip." >&2
    exit 1
    ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) ARCH="x64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "❌ Error: Architecture '${ARCH}' is not supported." >&2
    exit 1
    ;;
esac

echo "🔍 Detected platform: ${OS}/${ARCH}"
echo "🎯 Target mode: ${TARGET_MODE} (${INSTALL_SCOPE})"

# 3. Determine target installation directory
if [ "${INSTALL_SCOPE}" = "workspace" ]; then
  if git rev-parse --show-toplevel >/dev/null 2>&1; then
    WORKSPACE_ROOT="$(git rev-parse --show-toplevel)"
  else
    WORKSPACE_ROOT="$(pwd)"
  fi
fi

case "${TARGET_MODE}" in
  cli)
    if [ "${INSTALL_SCOPE}" = "workspace" ]; then
      INSTALL_DIR="${WORKSPACE_ROOT}/.agents/plugins/godoctor"
    else
      INSTALL_DIR="${HOME}/.gemini/antigravity-cli/plugins/godoctor"
    fi
    ;;
  agy2)
    if [ "${INSTALL_SCOPE}" = "workspace" ]; then
      INSTALL_DIR="${WORKSPACE_ROOT}/.agents/plugins/godoctor"
    else
      INSTALL_DIR="${HOME}/.gemini/config/plugins/godoctor"
    fi
    ;;
esac

echo "📂 Target destination: [${INSTALL_DIR}]"

# 4. Fetch latest release version from GitHub API
echo "🌐 Fetching latest release tag..."
LATEST_RELEASE=$(curl -s https://api.github.com/repos/danicat/godoctor/releases | grep -o '"tag_name": "[^"]*' | head -n1 | cut -d'"' -f4)

if [ -z "${LATEST_RELEASE}" ]; then
  echo "❌ Error: Failed to fetch the latest release tag. Please try again." >&2
  exit 1
fi

echo "🏷️  Latest release: ${LATEST_RELEASE}"

# 5. Construct download URL
FILENAME="${OS}.${ARCH}.godoctor.tar.gz"
DOWNLOAD_URL="https://github.com/danicat/godoctor/releases/download/${LATEST_RELEASE}/${FILENAME}"

# 6. Perform Installation
if [ -d "${INSTALL_DIR}" ]; then
  if [ "${OVERWRITE}" = "true" ]; then
    echo "⚠️  Target installation directory '${INSTALL_DIR}' already exists. Overwriting as requested..."
    rm -rf "${INSTALL_DIR}"
  else
    echo "❌ Error: Target installation directory '${INSTALL_DIR}' already exists." >&2
    echo "Please use the -f/--overwrite flag or remove it manually before running the installer again." >&2
    exit 1
  fi
fi

mkdir -p "${INSTALL_DIR}"

echo "📥 Downloading and extracting ${FILENAME}..."
if ! curl -sSL "${DOWNLOAD_URL}" | tar -xzf - -C "${INSTALL_DIR}"; then
  echo "❌ Error: Failed to download or extract the release asset." >&2
  exit 1
fi

# 7. Set execution permissions on bundled binaries and hooks
if [ -f "${INSTALL_DIR}/bin/godoctor" ]; then
  chmod +x "${INSTALL_DIR}/bin/godoctor"
fi

if [ -f "${INSTALL_DIR}/hooks/godoctor-hook.py" ]; then
  chmod +x "${INSTALL_DIR}/hooks/godoctor-hook.py"
fi

# 8. Register custom named agent for Antigravity 2.0 Desktop / IDE
if [ "${TARGET_MODE}" = "agy2" ] && [ -f "${INSTALL_DIR}/agents/godoctor.md" ]; then
  if [ "${INSTALL_SCOPE}" = "workspace" ]; then
    AGENTS_DIR="${WORKSPACE_ROOT}/.agents/agents"
  else
    AGENTS_DIR="${HOME}/.gemini/config/agents"
  fi
  mkdir -p "${AGENTS_DIR}"
  ln -sf "${INSTALL_DIR}/agents/godoctor.md" "${AGENTS_DIR}/godoctor.md"
  echo "🔗 Registered custom agent to [${AGENTS_DIR}/godoctor.md]"
fi

echo "✅ Success! GoDoctor plugin (v${LATEST_RELEASE}) has been successfully installed in '${TARGET_MODE}' mode (${INSTALL_SCOPE}) to [${INSTALL_DIR}]."
