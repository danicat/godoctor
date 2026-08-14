#!/usr/bin/env bash

# e2e-test.sh - Automated End-to-End Test Pipeline for GoDoctor Plugin
# Validates installation, custom agent, skills, all 10 MCP tools, and lifecycle hooks using agy CLI.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

MODE="local"
KEEP_WORKSPACE="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release)
      MODE="release"
      shift
      ;;
    --local)
      MODE="local"
      shift
      ;;
    --keep)
      KEEP_WORKSPACE="true"
      shift
      ;;
    -h|--help)
      echo "Usage: ./scripts/e2e-test.sh [options]"
      echo "  --local    Test using current local build (Default)"
      echo "  --release  Test using latest published release via installer"
      echo "  --keep     Keep temporary test workspace after test completion"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." &> /dev/null && pwd)"

echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}        GoDoctor Automated E2E Test Pipeline          ${NC}"
echo -e "${BLUE}======================================================${NC}"
echo -e "Mode: ${YELLOW}${MODE}${NC}"

# Check that agy CLI is installed and available
if ! command -v agy &> /dev/null; then
  echo -e "${RED}❌ Error: 'agy' CLI is not installed or not in PATH.${NC}" >&2
  exit 1
fi

# 1. Create a clean isolated temporary workspace
TEST_WORKSPACE="$(mktemp -d -t godoctor-e2e-XXXXXX)"
echo -e "📁 Clean workspace: ${YELLOW}${TEST_WORKSPACE}${NC}"

cleanup() {
  if [ "${KEEP_WORKSPACE}" = "false" ]; then
    echo -e "🧹 Cleaning up test workspace..."
    rm -rf "${TEST_WORKSPACE}"
  else
    echo -e "💾 Workspace preserved at: ${TEST_WORKSPACE}"
  fi
}
trap cleanup EXIT

# 2. Seed starter Go module
cd "${TEST_WORKSPACE}"
go mod init example.com/calc > /dev/null 2>&1

cat << 'EOF' > calc.go
package calc

func Add(a, b int) int {
	return a + b
}

func IsPositive(n int) bool {
	return n > 0
}
EOF

cat << 'EOF' > calc_test.go
package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
}

func TestIsPositive(t *testing.T) {
	if !IsPositive(1) {
		t.Errorf("IsPositive(1) = false, want true")
	}
}
EOF

# 3. Install Plugin into workspace
PLUGIN_DIR="${TEST_WORKSPACE}/.agents/plugins/godoctor"
mkdir -p "${PLUGIN_DIR}"

if [ "${MODE}" = "release" ]; then
  echo -e "📦 Installing latest GoDoctor release via install.sh..."
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target agy2 -w -f
else
  echo -e "🔨 Building local GoDoctor binary..."
  (cd "${ROOT_DIR}" && make build)
  
  echo -e "📦 Staging local plugin files to test workspace..."
  cp "${ROOT_DIR}/plugin.json" "${PLUGIN_DIR}/"
  cp "${ROOT_DIR}/mcp.json" "${PLUGIN_DIR}/"
  cp "${ROOT_DIR}/README.md" "${PLUGIN_DIR}/"
  cp "${ROOT_DIR}/LICENSE" "${PLUGIN_DIR}/"
  
  mkdir -p "${PLUGIN_DIR}/bin"
  cp "${ROOT_DIR}/bin/godoctor" "${PLUGIN_DIR}/bin/godoctor"
  chmod +x "${PLUGIN_DIR}/bin/godoctor"
  
  mkdir -p "${PLUGIN_DIR}/hooks"
  cp "${ROOT_DIR}/hooks/godoctor-hook.py" "${PLUGIN_DIR}/hooks/godoctor-hook.py"
  chmod +x "${PLUGIN_DIR}/hooks/godoctor-hook.py"
  
  mkdir -p "${PLUGIN_DIR}/agents"
  cp "${ROOT_DIR}/agents/godoctor.md" "${PLUGIN_DIR}/agents/godoctor.md"
  
  mkdir -p "${PLUGIN_DIR}/skills"
  cp -r "${ROOT_DIR}/skills/"* "${PLUGIN_DIR}/skills/"
fi

# 4. Verify physical plugin directory structure
echo -e "🔍 Verifying installed plugin structure..."
assert_file() {
  if [ ! -e "$1" ]; then
    echo -e "${RED}❌ Missing required plugin file: $1${NC}"
    exit 1
  fi
}
assert_file "${PLUGIN_DIR}/plugin.json"
assert_file "${PLUGIN_DIR}/mcp.json"
assert_file "${PLUGIN_DIR}/bin/godoctor"
assert_file "${PLUGIN_DIR}/agents/godoctor.md"
assert_file "${PLUGIN_DIR}/hooks/godoctor-hook.py"
assert_file "${PLUGIN_DIR}/skills/selene/SKILL.md"
assert_file "${PLUGIN_DIR}/skills/testquery/SKILL.md"
echo -e "${GREEN}✓ Plugin package integrity verified.${NC}\n"

# 5. Non-interactive agy CLI Test Runner
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

run_agy_test() {
  local test_name="$1"
  local prompt="$2"
  local pattern="$3"
  
  TOTAL_TESTS=$((TOTAL_TESTS + 1))
  echo -e "${BLUE}▶ [Test ${TOTAL_TESTS}] ${test_name}${NC}"
  
  local output
  set +e
  output=$(agy --agent godoctor --dangerously-skip-permissions -p "${prompt}" 2>&1)
  local exit_code=$?
  set -e
  
  if echo "${output}" | grep -Ei "${pattern}" > /dev/null; then
    echo -e "  ${GREEN}✓ PASSED${NC} (Matched: '${pattern}')"
    PASSED_TESTS=$((PASSED_TESTS + 1))
  else
    echo -e "  ${RED}✗ FAILED${NC} (Expected pattern: '${pattern}')"
    echo -e "  ${YELLOW}Output snippet:${NC}"
    echo "${output}" | head -n 15 | sed 's/^/    /'
    FAILED_TESTS=$((FAILED_TESTS + 1))
  fi
  echo ""
}

echo -e "${YELLOW}Starting non-interactive 'agy' test execution across all features...${NC}\n"

# Feature 1: MCP list_files & Agent invocation
run_agy_test \
  "Custom Agent & list_files" \
  "Use the list_files tool on '${TEST_WORKSPACE}' to list all files." \
  "calc.go|calc_test.go"

# Feature 2: MCP smart_read
run_agy_test \
  "MCP Tool: smart_read" \
  "Use smart_read to inspect '${TEST_WORKSPACE}/calc.go'." \
  "Add|IsPositive|package calc"

# Feature 3: MCP smart_edit (single-file compiler checked)
run_agy_test \
  "MCP Tool: smart_edit" \
  "Use smart_edit on '${TEST_WORKSPACE}/calc.go' to add a Multiply(a, b int) int function." \
  "Multiply"

# Feature 4: MCP smart_multi_edit (multi-file atomic edit)
run_agy_test \
  "MCP Tool: smart_multi_edit" \
  "Use smart_multi_edit to add a Subtract(a, b int) int function to '${TEST_WORKSPACE}/calc.go' and add a TestSubtract test to '${TEST_WORKSPACE}/calc_test.go'." \
  "Subtract"

# Feature 5: MCP smart_build (tidy, format, compile, vet, lint)
run_agy_test \
  "MCP Tool: smart_build" \
  "Run smart_build on directory '${TEST_WORKSPACE}'." \
  "PASS|build|success|verified|ok"

# Feature 6: MCP smart_test (test execution & testquery sync)
run_agy_test \
  "MCP Tool: smart_test" \
  "Run smart_test on directory '${TEST_WORKSPACE}' with level='basic'." \
  "PASS|PASS: Test"

# Feature 7: MCP test_query & testquery skill
run_agy_test \
  "MCP Tool: test_query & testquery skill" \
  "Use test_query on directory '${TEST_WORKSPACE}' with SQL query 'SELECT package, name, status FROM tests;'." \
  "TestAdd|TestIsPositive|PASS"

# Feature 8: MCP mutation_test & selene skill
run_agy_test \
  "MCP Tool: mutation_test & selene skill" \
  "Following the selene skill, run mutation_test on directory '${TEST_WORKSPACE}'." \
  "mutations|Mutation Score|Killed"

# Feature 9: MCP add_dependencies
run_agy_test \
  "MCP Tool: add_dependencies" \
  "Use add_dependencies to add 'github.com/google/uuid@latest' to '${TEST_WORKSPACE}'." \
  "github.com/google/uuid|go.mod|added|installed"

# Feature 10: MCP read_docs
run_agy_test \
  "MCP Tool: read_docs" \
  "Use read_docs to fetch documentation for package 'math' symbol 'Sqrt'." \
  "Sqrt|float64|math"

# Feature 11: Lifecycle Hook - Block raw 'go build'
run_agy_test \
  "Lifecycle Hook: Intercept 'go build'" \
  "Run 'go build ./...' using run_command." \
  "BLOCKED by GoDoctor|smart_build"

# Feature 12: Lifecycle Hook - Block raw shell inspection of .go files
run_agy_test \
  "Lifecycle Hook: Intercept shell cat on .go" \
  "Run 'cat ${TEST_WORKSPACE}/calc.go' using run_command." \
  "BLOCKED by GoDoctor|smart_read"

# Feature 13: Lifecycle Hook - Allow 'go test -race'
run_agy_test \
  "Lifecycle Hook: Permit 'go test -race'" \
  "Run 'go test -race ./...' using run_command." \
  "PASS|ok"

# 6. Summary Report
echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}                  E2E Test Summary                    ${NC}"
echo -e "${BLUE}======================================================${NC}"
echo -e "Total Features Tested: ${TOTAL_TESTS}"
echo -e "Passed: ${GREEN}${PASSED_TESTS}${NC}"
echo -e "Failed: ${RED}${FAILED_TESTS}${NC}"

if [ "${FAILED_TESTS}" -eq 0 ]; then
  echo -e "\n${GREEN}🎉 All GoDoctor plugin features, agent directives, skills, tools, and hooks passed!${NC}"
  exit 0
else
  echo -e "\n${RED}❌ Some tests failed. Review output above.${NC}"
  exit 1
fi
