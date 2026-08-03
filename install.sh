#!/usr/bin/env bash

# install.sh - Installs GoDoctor for Antigravity CLI, Antigravity 2.0, or Other Agents (Skills Only)

set -euo pipefail

show_help() {
  echo "GoDoctor Custom Installer"
  echo "========================="
  echo "Usage: ./install.sh [options]"
  echo ""
  echo "Target Options:"
  echo "  -t, --target <mode>  Installation target mode: cli | agy2 | skills (Default: agy2)"
  echo ""
  echo "Scope Options:"
  echo "  -g, --global         Install globally (Default)"
  echo "  -w, --workspace      Install locally to the active workspace"
  echo ""
  echo "General Options:"
  echo "  -f, --overwrite      Overwrite existing target directory or skills if they exist"
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
  cli|agy2|skills)
    ;;
  *)
    echo "❌ Error: Unsupported target mode '${TARGET_MODE}'." >&2
    echo "Valid options for --target are: cli, agy2, skills" >&2
    exit 1
    ;;
esac

# 1. Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${OS}" in
  darwin)  OS="darwin" ;;
  linux)   OS="linux" ;;
  *)
    echo "❌ Error: OS '${OS}' is not supported by this installer." >&2
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
  skills)
    if [ "${INSTALL_SCOPE}" = "workspace" ]; then
      INSTALL_DIR="${WORKSPACE_ROOT}/.agents/skills"
    else
      INSTALL_DIR="${HOME}/.agents/skills"
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

# 6. Perform Installation based on TARGET_MODE
if [ "${TARGET_MODE}" = "skills" ]; then
  # Skills-only mode
  TEMP_DIR="$(mktemp -d)"
  trap 'rm -rf "${TEMP_DIR}"' EXIT

  echo "📥 Downloading release package for skills extraction..."
  if ! curl -sSL "${DOWNLOAD_URL}" | tar -xzf - -C "${TEMP_DIR}"; then
    echo "❌ Error: Failed to download or extract the release asset." >&2
    exit 1
  fi

  if [ ! -d "${TEMP_DIR}/skills" ]; then
    echo "❌ Error: No 'skills' directory found in the release package." >&2
    exit 1
  fi

  # Check if any skill folder already exists in INSTALL_DIR
  EXISTING_SKILLS=()
  for skill_path in "${TEMP_DIR}/skills"/*; do
    if [ -d "${skill_path}" ]; then
      skill_name="$(basename "${skill_path}")"
      if [ -d "${INSTALL_DIR}/${skill_name}" ]; then
        EXISTING_SKILLS+=("${skill_name}")
      fi
    fi
  done

  if [ ${#EXISTING_SKILLS[@]} -gt 0 ]; then
    if [ "${OVERWRITE}" = "true" ]; then
      echo "⚠️  Existing skills detected (${EXISTING_SKILLS[*]}). Overwriting as requested..."
    else
      echo "❌ Error: The following skill directories already exist in '${INSTALL_DIR}':" >&2
      for s in "${EXISTING_SKILLS[@]}"; do
        echo "  - ${s}" >&2
      done
      echo "Please use the -f/--overwrite flag or remove them before running the installer." >&2
      exit 1
    fi
  fi

  mkdir -p "${INSTALL_DIR}"
  for skill_path in "${TEMP_DIR}/skills"/*; do
    if [ -d "${skill_path}" ]; then
      skill_name="$(basename "${skill_path}")"
      rm -rf "${INSTALL_DIR}/${skill_name}"
      cp -r "${skill_path}" "${INSTALL_DIR}/"
      echo "  ✓ Installed skill: ${skill_name}"
    fi
  done

  echo "✅ Success! GoDoctor skills have been successfully installed to [${INSTALL_DIR}]."

else
  # Plugin modes (agy-cli or agy-2)
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

  echo "🔧 Dynamically resolving plugin paths..."
  replace_path() {
    local file="$1"
    local target_dir="$2"
    if [ -f "${file}" ]; then
      sed 's|__PLUGIN_PATH__|'"${target_dir}"'|g' "${file}" > "${file}.tmp"
      mv "${file}.tmp" "${file}"
    fi
  }

  replace_path "${INSTALL_DIR}/mcp_config.json" "${INSTALL_DIR}"

  echo "✅ Success! GoDoctor has been successfully installed in '${TARGET_MODE}' mode (${INSTALL_SCOPE}) to [${INSTALL_DIR}]."
fi

