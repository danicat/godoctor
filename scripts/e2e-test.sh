#!/usr/bin/env bash

# e2e-test.sh - Automated End-to-End Test Pipeline for GoDoctor Plugin
# Validates installation, custom agent, skills, all 10 MCP tools, physical disk state, and lifecycle hooks using agy CLI.

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

# Pre-flight prerequisite check
check_prerequisites() {
  local missing=()
  for cmd in agy go python3 curl make; do
    if ! command -v "$cmd" &> /dev/null; then
      missing+=("$cmd")
    fi
  done
  if [ ${#missing[@]} -ne 0 ]; then
    echo -e "${RED}❌ Error: Missing required tools: ${missing[*]}${NC}" >&2
    exit 1
  fi
}
check_prerequisites

TIMEOUT_CMD=""
if command -v timeout &> /dev/null; then
  TIMEOUT_CMD="timeout 90"
elif command -v gtimeout &> /dev/null; then
  TIMEOUT_CMD="gtimeout 90"
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
  (cd "${TEST_WORKSPACE}" && curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target agy2 -w -f)
else
  echo -e "🔨 Building local GoDoctor binary..."
  (cd "${ROOT_DIR}" && make build)
  
  echo -e "📦 Staging local plugin files to test workspace..."
  cp "${ROOT_DIR}/plugin.json" "${PLUGIN_DIR}/"
  cp "${ROOT_DIR}/mcp.json" "${PLUGIN_DIR}/"
  cp "${ROOT_DIR}/hooks.json" "${PLUGIN_DIR}/"
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

# Configure workspace hooks.json pointing to the installed hook script
cat << EOF > "${TEST_WORKSPACE}/.agents/hooks.json"
{
  "godoctor-hooks": {
    "enabled": true,
    "PreToolUse": [
      {
        "matcher": "run_command|view_file|write_to_file|replace_file_content|multi_replace_file_content",
        "hooks": [
          {
            "type": "command",
            "command": "python3 ${PLUGIN_DIR}/hooks/godoctor-hook.py",
            "timeout": 15
          }
        ]
      }
    ]
  }
}
EOF

# 4. Verify physical plugin directory structure
echo -e "🔍 Verifying installed plugin structure..."
assert_file() {
  if [ ! -e "$1" ]; then
    echo -e "${RED}❌ Missing required plugin file: $1${NC}"
    exit 1
  fi
}
assert_disk_contains() {
  local target_file="$1"
  local expected_text="$2"
  if ! grep -q "${expected_text}" "${target_file}" 2>/dev/null; then
    echo -e "${RED}❌ Physical disk verification failed: '${expected_text}' not found in ${target_file}${NC}"
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
  
  local output=""
  local attempts=0
  local max_attempts=2
  
  while [ "${attempts}" -lt "${max_attempts}" ]; do
    attempts=$((attempts + 1))
    set +e
    output=$(cd "${TEST_WORKSPACE}" && ${TIMEOUT_CMD} agy --agent godoctor --dangerously-skip-permissions -p "${prompt}" 2>&1)
    set -e
    
    if echo "${output}" | grep -Ei "${pattern}" > /dev/null; then
      echo -e "  ${GREEN}✓ PASSED${NC} (Matched: '${pattern}')"
      PASSED_TESTS=$((PASSED_TESTS + 1))
      echo ""
      return 0
    elif echo "${output}" | grep -Ei "network issue|timeout|deadline" > /dev/null && [ "${attempts}" -lt "${max_attempts}" ]; then
      echo -e "  ${YELLOW}⚠️  Transient connection error, retrying (${attempts}/${max_attempts})...${NC}"
      sleep 2
    else
      break
    fi
  done
  
  echo -e "  ${RED}✗ FAILED${NC} (Expected pattern: '${pattern}')"
  echo -e "  ${YELLOW}Output snippet:${NC}"
  echo "${output}" | head -n 15 | sed 's/^/    /'
  FAILED_TESTS=$((FAILED_TESTS + 1))
  echo ""
}

run_hook_test() {
  local test_name="$1"
  local json_payload="$2"
  local expected_decision="$3"
  local pattern="$4"

  TOTAL_TESTS=$((TOTAL_TESTS + 1))
  echo -e "${BLUE}▶ [Test ${TOTAL_TESTS}] ${test_name}${NC}"

  local output
  set +e
  output=$(echo "${json_payload}" | python3 "${PLUGIN_DIR}/hooks/godoctor-hook.py" 2>&1)
  local exit_code=$?
  set -e

  if [ ${exit_code} -eq 0 ] && echo "${output}" | grep -q "\"decision\": \"${expected_decision}\"" && echo "${output}" | grep -Ei "${pattern}" > /dev/null; then
    echo -e "  ${GREEN}✓ PASSED${NC} (Decision: '${expected_decision}', Matched: '${pattern}')"
    PASSED_TESTS=$((PASSED_TESTS + 1))
  else
    echo -e "  ${RED}✗ FAILED${NC} (Expected decision '${expected_decision}', pattern '${pattern}')"
    echo -e "  ${YELLOW}Output:${NC} ${output}"
    FAILED_TESTS=$((FAILED_TESTS + 1))
  fi
  echo ""
}

echo -e "${YELLOW}Starting non-interactive 'agy' test execution across all features in workspace...${NC}\n"

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
assert_disk_contains "${TEST_WORKSPACE}/calc.go" "func Multiply"

# Feature 4: MCP smart_multi_edit (multi-file atomic edit)
run_agy_test \
  "MCP Tool: smart_multi_edit" \
  "Use smart_multi_edit to add a Subtract(a, b int) int function to '${TEST_WORKSPACE}/calc.go' and add a TestSubtract test to '${TEST_WORKSPACE}/calc_test.go'." \
  "Subtract"
assert_disk_contains "${TEST_WORKSPACE}/calc.go" "func Subtract"
assert_disk_contains "${TEST_WORKSPACE}/calc_test.go" "TestSubtract"

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
assert_file "${TEST_WORKSPACE}/testquery.db"

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
assert_disk_contains "${TEST_WORKSPACE}/go.mod" "github.com/google/uuid"

# Feature 10: MCP read_docs
run_agy_test \
  "MCP Tool: read_docs" \
  "Use read_docs to fetch documentation for package 'math' symbol 'Sqrt'." \
  "Sqrt|float64|math"

echo -e "${YELLOW}Testing lifecycle hook interceptor contract and enforcement...${NC}\n"

# Hook Test 1: Intercept raw 'go build'
run_hook_test \
  "Hook: Intercept 'go build'" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "go build ./..."}}}' \
  "deny" \
  "GoDoctor guidance.*smart_build"

# Hook Test 2: Intercept raw shell edit on .go files
run_hook_test \
  "Hook: Intercept shell edit on .go" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "cat calc.go"}}}' \
  "deny" \
  "GoDoctor guidance.*smart_read|smart_edit"

# Hook Test 3: Intercept view_file on .go files
run_hook_test \
  "Hook: Intercept view_file on .go" \
  '{"toolCall": {"name": "view_file", "args": {"AbsolutePath": "/path/to/calc.go"}}}' \
  "deny" \
  "GoDoctor guidance.*smart_read"

# Hook Test 4: Intercept write_to_file on .go files
run_hook_test \
  "Hook: Intercept write_to_file on .go" \
  '{"toolCall": {"name": "write_to_file", "args": {"TargetFile": "/path/to/calc.go"}}}' \
  "deny" \
  "GoDoctor guidance.*smart_edit"

# Hook Test 5: Intercept replace_file_content on .go files
run_hook_test \
  "Hook: Intercept replace_file_content on .go" \
  '{"toolCall": {"name": "replace_file_content", "args": {"TargetFile": "/path/to/calc.go"}}}' \
  "deny" \
  "GoDoctor guidance.*smart_edit"

# Hook Test 6: Intercept multi_replace_file_content on .go files
run_hook_test \
  "Hook: Intercept multi_replace_file_content on .go" \
  '{"toolCall": {"name": "multi_replace_file_content", "args": {"TargetFile": "/path/to/calc.go"}}}' \
  "deny" \
  "GoDoctor guidance.*smart_edit"

# Hook Test 7: Intercept direct go get / go mod
run_hook_test \
  "Hook: Intercept direct go get / mod tidy" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "go get github.com/stretchr/testify"}}}' \
  "deny" \
  "GoDoctor guidance.*add_dependencies"

# Hook Test 8: Intercept unflagged go test
run_hook_test \
  "Hook: Intercept unflagged go test" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "go test ./..."}}}' \
  "deny" \
  "GoDoctor guidance.*smart_test"

# Hook Test 9: Allow 'go test -race'
run_hook_test \
  "Hook: Permit 'go test -race'" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "go test -race ./..."}}}' \
  "allow" \
  "GoDoctor hook: operation permitted"

# Hook Test 10: Allow 'go test -fuzz'
run_hook_test \
  "Hook: Permit 'go test -fuzz'" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "go test -fuzz=FuzzTarget ./..."}}}' \
  "allow" \
  "GoDoctor hook: operation permitted"

# Hook Test 11: Allow 'go test -bench'
run_hook_test \
  "Hook: Permit 'go test -bench'" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "go test -bench=. ./..."}}}' \
  "allow" \
  "GoDoctor hook: operation permitted"

# Hook Test 12: Allow non-Go shell command
run_hook_test \
  "Hook: Permit non-Go shell command" \
  '{"toolCall": {"name": "run_command", "args": {"CommandLine": "git status"}}}' \
  "allow" \
  "GoDoctor hook: operation permitted"

# 6. Summary Report
echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}                  E2E Test Summary                    ${NC}"
echo -e "${BLUE}======================================================${NC}"
echo -e "Total Features Tested: ${TOTAL_TESTS}"
echo -e "Passed: ${GREEN}${PASSED_TESTS}${NC}"
echo -e "Failed: ${RED}${FAILED_TESTS}${NC}"

if [ "${FAILED_TESTS}" -eq 0 ]; then
  echo -e "\n${GREEN}🎉 All GoDoctor plugin features, agent directives, skills, tools, physical disk states, and hooks passed!${NC}"
  exit 0
else
  echo -e "\n${RED}❌ Some tests failed. Review output above.${NC}"
  exit 1
fi
