#!/usr/bin/env bash

# uninstall.sh - GoDoctor Uninstaller
# Uninstalls any combination of GoDoctor MCP server, Agent, and Skills:
# - MCP Server: Removes binary from Go bin path and unregisters from mcp_config.json
# - Agent: Removes custom named agent definition (@godoctor)
# - Skills: Removes Agent Skills (@selene, @testquery) via 'npx skills' and directory cleanup

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

SCOPE="global"
YES="false"

UNINSTALL_MCP="false"
UNINSTALL_AGENT="false"
UNINSTALL_SKILLS="false"
EXPLICIT_COMPONENT="false"

MCP_REMOVED="false"
AGENT_REMOVED="false"
SKILLS_REMOVED="false"

print_usage() {
  cat << 'EOF'
GoDoctor Uninstaller

Usage:
  uninstall.sh [components] [options]
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/uninstall.sh | bash -s -- [components] [options]

Components (uninstall any combination; default: all):
  --mcp              Remove GoDoctor binary and unregister from mcp_config.json
  --agent            Remove GoDoctor Agent definition (@godoctor)
  --skills           Remove GoDoctor Agent Skills (@selene, @testquery)
  --all              Uninstall all components (MCP, Agent, Skills)

Options:
  -g, --global       Uninstall from global scope (Default: ~/.gemini/config)
  -w, --workspace    Uninstall from workspace scope (.agents/)
  -y, --yes          Non-interactive mode
  -h, --help         Show this help message

Examples:
  ./uninstall.sh                        # Uninstall MCP, Agent, and Skills globally
  ./uninstall.sh -w                     # Uninstall MCP, Agent, and Skills from workspace (.agents/)
  ./uninstall.sh --mcp                  # Uninstall MCP server only
  ./uninstall.sh --agent -w             # Uninstall Agent definition from workspace
  ./uninstall.sh --skills               # Uninstall Skills only
  ./uninstall.sh --mcp --skills         # Uninstall MCP and Skills
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mcp)
      UNINSTALL_MCP="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    --agent)
      UNINSTALL_AGENT="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    --skills)
      UNINSTALL_SKILLS="true"
      EXPLICIT_COMPONENT="true"
      shift
      ;;
    --all)
      UNINSTALL_MCP="true"
      UNINSTALL_AGENT="true"
      UNINSTALL_SKILLS="true"
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

# If no specific component was selected, default to uninstalling all three
if [ "${EXPLICIT_COMPONENT}" = "false" ]; then
  UNINSTALL_MCP="true"
  UNINSTALL_AGENT="true"
  UNINSTALL_SKILLS="true"
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

echo -e "${BLUE}===============================================${NC}"
echo -e "${BLUE}          GoDoctor Uninstaller                 ${NC}"
echo -e "${BLUE}===============================================${NC}"
echo -e "Scope:      ${BOLD}${SCOPE}${NC} (${TARGET_ROOT})"
echo -e "Components: MCP=${BOLD}${UNINSTALL_MCP}${NC}, Agent=${BOLD}${UNINSTALL_AGENT}${NC}, Skills=${BOLD}${UNINSTALL_SKILLS}${NC}"
echo ""

# -----------------------------------------------------------------------------
# 1. MCP Server Uninstallation
# -----------------------------------------------------------------------------
if [ "${UNINSTALL_MCP}" = "true" ]; then
  echo -e "🗑️  ${BLUE}[MCP] Checking GoDoctor MCP Server...${NC}"

  # Unregister from mcp_config.json
  MCP_CONFIG_STATUS="NOT_FOUND"
  if [ -f "${MCP_CONFIG}" ]; then
    MCP_CONFIG_STATUS=$(python3 -c '
import json, os, sys

config_path = sys.argv[1]
if os.path.exists(config_path):
    try:
        with open(config_path, "r", encoding="utf-8") as f:
            data = json.load(f)
        if isinstance(data, dict) and "mcpServers" in data and isinstance(data["mcpServers"], dict):
            if "godoctor" in data["mcpServers"]:
                del data["mcpServers"]["godoctor"]
                with open(config_path, "w", encoding="utf-8") as f:
                    json.dump(data, f, indent=2)
                    f.write("\n")
                print("REMOVED")
                sys.exit(0)
    except Exception:
        pass
print("NOT_FOUND")
' "${MCP_CONFIG}" 2>/dev/null || echo "NOT_FOUND")
  fi

  if [ "${MCP_CONFIG_STATUS}" = "REMOVED" ]; then
    echo -e "  ${GREEN}✓ Unregistered 'godoctor' from ${MCP_CONFIG}${NC}"
    MCP_REMOVED="true"
  else
    echo "  ℹ️  'godoctor' entry not present in ${MCP_CONFIG}."
  fi

  # Remove compiled binary (global scope only)
  if [ "${SCOPE}" = "global" ]; then
    BIN_FOUND="false"
    if command -v go &> /dev/null; then
      GOPATH_BIN="$(go env GOPATH 2>/dev/null || echo "")/bin"
      GOBIN="$(go env GOBIN 2>/dev/null || echo "")"
      
      if [ -n "${GOBIN}" ] && [ -f "${GOBIN}/godoctor" ]; then
        rm -f "${GOBIN}/godoctor"
        echo -e "  ${GREEN}✓ Removed binary from ${GOBIN}/godoctor${NC}"
        BIN_FOUND="true"
        MCP_REMOVED="true"
      fi
      if [ -n "${GOPATH_BIN}" ] && [ -f "${GOPATH_BIN}/godoctor" ]; then
        rm -f "${GOPATH_BIN}/godoctor"
        echo -e "  ${GREEN}✓ Removed binary from ${GOPATH_BIN}/godoctor${NC}"
        BIN_FOUND="true"
        MCP_REMOVED="true"
      fi
    fi

    if [ "${BIN_FOUND}" = "false" ]; then
      CMD_PATH="$(command -v godoctor 2>/dev/null || echo "")"
      if [ -n "${CMD_PATH}" ] && [ -f "${CMD_PATH}" ]; then
        rm -f "${CMD_PATH}" 2>/dev/null || true
        echo -e "  ${GREEN}✓ Removed binary from ${CMD_PATH}${NC}"
        MCP_REMOVED="true"
      else
        echo "  ℹ️  GoDoctor binary not found in Go bin paths."
      fi
    fi
  fi
  echo ""
fi

# -----------------------------------------------------------------------------
# 2. Agent Definition Uninstallation
# -----------------------------------------------------------------------------
if [ "${UNINSTALL_AGENT}" = "true" ]; then
  echo -e "🗑️  ${BLUE}[Agent] Checking GoDoctor Named Agent (@godoctor)...${NC}"
  AGENT_DEST="${AGENTS_DIR}/godoctor.md"

  if [ -f "${AGENT_DEST}" ]; then
    rm -f "${AGENT_DEST}"
    echo -e "  ${GREEN}✓ Removed ${AGENT_DEST}${NC}"
    AGENT_REMOVED="true"
  else
    echo "  ℹ️  Agent definition not found (${AGENT_DEST})."
  fi

  # Clean up legacy plugin directories (global scope only)
  if [ "${SCOPE}" = "global" ]; then
    if [ -d "${HOME}/.gemini/config/plugins/godoctor" ] || [ -d "${HOME}/.gemini/antigravity-cli/plugins/godoctor" ]; then
      rm -rf "${HOME}/.gemini/config/plugins/godoctor" 2>/dev/null || true
      rm -rf "${HOME}/.gemini/antigravity-cli/plugins/godoctor" 2>/dev/null || true
      echo -e "  ${GREEN}✓ Removed legacy plugin directory${NC}"
      AGENT_REMOVED="true"
    fi
  fi
  echo ""
fi

# -----------------------------------------------------------------------------
# 3. Skills Uninstallation
# -----------------------------------------------------------------------------
if [ "${UNINSTALL_SKILLS}" = "true" ]; then
  echo -e "🗑️  ${BLUE}[Skills] Checking GoDoctor Skills (@selene, @testquery)...${NC}"

  SKILLS_EXIST="false"
  if [ -d "${SKILLS_DIR}/selene" ] || [ -d "${SKILLS_DIR}/testquery" ]; then
    SKILLS_EXIST="true"
  fi

  if [ "${SKILLS_EXIST}" = "false" ] && command -v npx &> /dev/null; then
    SKILL_LIST_FLAGS=()
    if [ "${SCOPE}" = "global" ]; then
      SKILL_LIST_FLAGS+=("-g")
    fi
    if npx -y skills list ${SKILL_LIST_FLAGS[@]+"${SKILL_LIST_FLAGS[@]}"} 2>/dev/null | grep -qE "(selene|testquery)"; then
      SKILLS_EXIST="true"
    fi
  fi

  if [ "${SKILLS_EXIST}" = "true" ]; then
    if command -v npx &> /dev/null; then
      SKILL_FLAGS=("-y")
      if [ "${SCOPE}" = "global" ]; then
        SKILL_FLAGS+=("-g")
      fi
      npx -y skills remove selene testquery "${SKILL_FLAGS[@]}" >/dev/null 2>&1 || true
    fi

    if [ -d "${SKILLS_DIR}/selene" ]; then
      rm -rf "${SKILLS_DIR}/selene" 2>/dev/null || true
    fi
    if [ -d "${SKILLS_DIR}/testquery" ]; then
      rm -rf "${SKILLS_DIR}/testquery" 2>/dev/null || true
    fi
    echo -e "  ${GREEN}✓ Removed skills (@selene, @testquery)${NC}"
    SKILLS_REMOVED="true"
  else
    echo "  ℹ️  Skills (@selene, @testquery) not found."
  fi
  echo ""
fi

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo -e "${GREEN}===============================================${NC}"
echo -e "${GREEN}🎉 GoDoctor uninstallation check completed!    ${NC}"
echo -e "${GREEN}===============================================${NC}"
if [ "${UNINSTALL_MCP}" = "true" ]; then
  if [ "${MCP_REMOVED}" = "true" ]; then
    echo -e "  • ${BOLD}MCP Server:${NC}  ${GREEN}Removed${NC}"
  else
    echo -e "  • ${BOLD}MCP Server:${NC}  ${YELLOW}Not found (already uninstalled)${NC}"
  fi
fi
if [ "${UNINSTALL_AGENT}" = "true" ]; then
  if [ "${AGENT_REMOVED}" = "true" ]; then
    echo -e "  • ${BOLD}Agent:${NC}       ${GREEN}Removed (@godoctor)${NC}"
  else
    echo -e "  • ${BOLD}Agent:${NC}       ${YELLOW}Not found (already uninstalled)${NC}"
  fi
fi
if [ "${UNINSTALL_SKILLS}" = "true" ]; then
  if [ "${SKILLS_REMOVED}" = "true" ]; then
    echo -e "  • ${BOLD}Skills:${NC}      ${GREEN}Removed (@selene, @testquery)${NC}"
  else
    echo -e "  • ${BOLD}Skills:${NC}      ${YELLOW}Not found (already uninstalled)${NC}"
  fi
fi
echo ""
