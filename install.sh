#!/usr/bin/env bash

# install.sh - GoDoctor Installer
# Installs any combination of GoDoctor MCP server, Agent, and Skills:
# - MCP Server: 'go install' + mcp_config.json registration
# - Agent: Custom named agent definition (@godoctor)
# - Skills: Agent Skills (@selene, @testquery) via 'npx skills'

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

REPO="danicat/godoctor"
VERSION="latest"
SCOPE="global"
OVERWRITE="false"
YES="false"

INSTALL_MCP="false"
INSTALL_AGENT="false"
INSTALL_SKILLS="false"
EXPLICIT_COMPONENT="false"

print_usage() {
  cat << 'EOF'
GoDoctor Installer

Usage:
  install.sh [components] [options]
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- [components] [options]

Components (install any combination; default: all):
  --mcp              Install the GoDoctor MCP server binary via 'go install' and configure mcp_config.json
  --agent            Install the GoDoctor Agent definition (@godoctor)
  --skills           Install GoDoctor Agent Skills (@selene, @testquery) via 'npx skills'
  --all              Install all components (MCP, Agent, Skills)

Options:
  -g, --global       Install to global scope (Default: ~/.gemini/config)
  -w, --workspace    Install to workspace scope (.agents/)
  -v, --version <v>  Target GoDoctor release version (Default: latest)
  -f, --overwrite    Overwrite existing agent and skill files
  -y, --yes          Non-interactive mode
  -h, --help         Show this help message

Examples:
  ./install.sh                        # Install MCP, Agent, and Skills globally
  ./install.sh -w                     # Install MCP, Agent, and Skills to current workspace (.agents/)
  ./install.sh --mcp                  # Install MCP server only
  ./install.sh --agent -w             # Install Agent definition to workspace
  ./install.sh --skills               # Install Skills only globally
  ./install.sh --mcp --skills         # Install MCP and Skills
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mcp)
      INSTALL_MCP="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    --agent)
      INSTALL_AGENT="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    --skills)
      INSTALL_SKILLS="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    --all)
      INSTALL_MCP="true"
      INSTALL_AGENT="true"
      INSTALL_SKILLS="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    -g|--global)
      SCOPE="global"
      shift
      ;;
    -w|--workspace)
      SCOPE="workspace"
      shift
      ;;
    -v|--version)
      VERSION="$2"
      shift 2
      ;;
    -f|--overwrite)
      OVERWRITE="true"
      shift
      ;;
    -y|--yes)
      YES="true"
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}" >&2
      print_usage
      exit 1
      ;;
  esac
done

# If no specific component was selected, default to installing all three
if [ "${EXPLICIT_COMPONENT}" = "false" ]; then
  INSTALL_MCP="true"
  INSTALL_AGENT="true"
  INSTALL_SKILLS="true"
fi

# Determine target root paths
if [ "${SCOPE}" = "global" ]; then
  TARGET_ROOT="${GEMINI_CONFIG_DIR:-${HOME}/.gemini/config}"
else
  TARGET_ROOT="$(pwd)/.agents"
fi

AGENTS_DIR="${TARGET_ROOT}/agents"
SKILLS_DIR="${TARGET_ROOT}/skills"
MCP_CONFIG="${TARGET_ROOT}/mcp_config.json"

VERSION_REF="${VERSION}"
if [ "${VERSION}" = "latest" ]; then
  VERSION_REF="main"
fi

echo -e "${BLUE}===============================================${NC}"
echo -e "${BLUE}           GoDoctor Installer                  ${NC}"
echo -e "${BLUE}===============================================${NC}"
echo -e "Scope:      ${BOLD}${SCOPE}${NC} (${TARGET_ROOT})"
echo -e "Version:    ${BOLD}${VERSION}${NC}"
echo -e "Components: MCP=${BOLD}${INSTALL_MCP}${NC}, Agent=${BOLD}${INSTALL_AGENT}${NC}, Skills=${BOLD}${INSTALL_SKILLS}${NC}"
echo ""

# -----------------------------------------------------------------------------
# 1. MCP Server Installation
# -----------------------------------------------------------------------------
if [ "${INSTALL_MCP}" = "true" ]; then
  echo -e "🔨 ${BLUE}[MCP] Installing GoDoctor MCP Server...${NC}"

  # Verify Go toolchain
  if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Error: 'go' toolchain is not found in PATH.${NC}" >&2
    echo "Please install Go (https://go.dev/dl/) and retry." >&2
    exit 1
  fi

  GOPATH_BIN="$(go env GOPATH)/bin"
  GOBIN="$(go env GOBIN)"
  if [ -n "${GOBIN}" ]; then
    INSTALL_BIN_DIR="${GOBIN}"
  else
    INSTALL_BIN_DIR="${GOPATH_BIN}"
  fi

  echo "  Running 'go install github.com/${REPO}/cmd/godoctor@${VERSION}'..."
  go install "github.com/${REPO}/cmd/godoctor@${VERSION}"

  BIN_PATH="${INSTALL_BIN_DIR}/godoctor"
  if [ -f "${BIN_PATH}" ]; then
    echo -e "  ${GREEN}✓ Binary installed to ${BIN_PATH}${NC}"
  else
    echo -e "${RED}❌ Error: Binary not found in ${BIN_PATH} after go install.${NC}" >&2
    exit 1
  fi

  # Check PATH
  if ! command -v godoctor &> /dev/null; then
    echo -e "${YELLOW}⚠️  Note: '${INSTALL_BIN_DIR}' is not currently in your \$PATH.${NC}"
    echo "  Consider adding it to your shell configuration (~/.zshrc or ~/.bashrc):"
    echo -e "  ${BLUE}export PATH=\"${INSTALL_BIN_DIR}:\$PATH\"${NC}"
  fi

  # Register MCP in mcp_config.json
  echo "  Configuring ${MCP_CONFIG}..."
  mkdir -p "${TARGET_ROOT}"
  python3 -c '
import json, os, sys

config_path = sys.argv[1]
bin_path = sys.argv[2]
data = {}

if os.path.exists(config_path):
    try:
        with open(config_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception:
        data = {}

if not isinstance(data, dict):
    data = {}

if "mcpServers" not in data or not isinstance(data["mcpServers"], dict):
    data["mcpServers"] = {}

data["mcpServers"]["godoctor"] = {
    "command": bin_path,
    "args": []
}

os.makedirs(os.path.dirname(os.path.abspath(config_path)), exist_ok=True)
with open(config_path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
' "${MCP_CONFIG}" "${BIN_PATH}"

  echo -e "  ${GREEN}✓ Registered 'godoctor' MCP server in ${MCP_CONFIG}${NC}"
  echo ""
fi

# -----------------------------------------------------------------------------
# 2. Agent Definition Installation
# -----------------------------------------------------------------------------
if [ "${INSTALL_AGENT}" = "true" ]; then
  echo -e "🤖 ${BLUE}[Agent] Installing GoDoctor Named Agent (@godoctor)...${NC}"
  mkdir -p "${AGENTS_DIR}"
  AGENT_DEST="${AGENTS_DIR}/godoctor.md"

  if [ -e "${AGENT_DEST}" ] && [ "${OVERWRITE}" != "true" ]; then
    echo "  ⚠️  ${AGENT_DEST} already exists. Skipping (use -f/--overwrite to replace)."
  else
    rm -f "${AGENT_DEST}"
    RAW_AGENT_URL="https://raw.githubusercontent.com/${REPO}/${VERSION_REF}/agent/godoctor.md"
    curl -fsSL "${RAW_AGENT_URL}" -o "${AGENT_DEST}"
    echo -e "  ${GREEN}✓ Installed agent definition to ${AGENT_DEST}${NC}"
  fi
  echo ""
fi

# -----------------------------------------------------------------------------
# 3. Skills Installation via 'npx skills'
# -----------------------------------------------------------------------------
if [ "${INSTALL_SKILLS}" = "true" ]; then
  echo -e "📚 ${BLUE}[Skills] Installing GoDoctor Skills (@selene, @testquery)...${NC}"

  SKILLS_INSTALLED="false"

  if command -v npx &> /dev/null; then
    SKILL_FLAGS=("-y")
    if [ "${SCOPE}" = "global" ]; then
      SKILL_FLAGS+=("-g")
    fi
    echo "  Using standard 'npx skills add ${REPO} ${SKILL_FLAGS[*]}'..."
    if npx -y skills add "${REPO}" "${SKILL_FLAGS[@]}"; then
      SKILLS_INSTALLED="true"
      echo -e "  ${GREEN}✓ Skills installed via 'npx skills add'${NC}"
    else
      echo -e "  ${YELLOW}⚠️  'npx skills add' exited with non-zero status. Falling back to direct download...${NC}"
    fi
  fi

  # Fallback to direct file download if npx is not installed or failed
  if [ "${SKILLS_INSTALLED}" != "true" ]; then
    mkdir -p "${SKILLS_DIR}"

    install_skill_file() {
      local skill_name="$1"
      local skill_target_dir="${SKILLS_DIR}/${skill_name}"
      local skill_target_file="${skill_target_dir}/SKILL.md"

      mkdir -p "${skill_target_dir}"
      if [ -e "${skill_target_file}" ] && [ "${OVERWRITE}" != "true" ]; then
        echo "  ⚠️  ${skill_target_file} already exists. Skipping."
      else
        rm -f "${skill_target_file}"
        RAW_SKILL_URL="https://raw.githubusercontent.com/${REPO}/${VERSION_REF}/skills/${skill_name}/SKILL.md"
        curl -fsSL "${RAW_SKILL_URL}" -o "${skill_target_file}"
        echo -e "  ${GREEN}✓ Installed skill @${skill_name} to ${skill_target_file}${NC}"
      fi
    }

    install_skill_file "selene"
    install_skill_file "testquery"
  fi
  echo ""
fi

# Clean up stale legacy plugin directories if global install
if [ "${SCOPE}" = "global" ]; then
  rm -rf "${HOME}/.gemini/config/plugins/godoctor" 2>/dev/null || true
  rm -rf "${HOME}/.gemini/antigravity-cli/plugins/godoctor" 2>/dev/null || true
fi

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo -e "${GREEN}===============================================${NC}"
echo -e "${GREEN}🎉 GoDoctor installation completed successfully!${NC}"
echo -e "${GREEN}===============================================${NC}"

if [ "${INSTALL_MCP}" = "true" ]; then
  echo -e "  • ${BOLD}MCP Server:${NC}  Registered in ${MCP_CONFIG}"
fi
if [ "${INSTALL_AGENT}" = "true" ]; then
  echo -e "  • ${BOLD}Agent:${NC}       @godoctor (${AGENTS_DIR}/godoctor.md)"
fi
if [ "${INSTALL_SKILLS}" = "true" ]; then
  echo -e "  • ${BOLD}Skills:${NC}      @selene, @testquery"
fi
echo ""
