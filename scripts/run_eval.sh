#!/bin/bash
set -e

# Usage: ./scripts/run_eval.sh <scenario_name> <agent_command>

SCENARIO=$1
AGENT_CMD=$2

if [ -z "$SCENARIO" ] || [ -z "$AGENT_CMD" ]; then
  echo "Usage: $0 <scenario_name> <agent_command>"
  exit 1
fi

REPO_ROOT=$(git rev-parse --show-toplevel)
EVAL_DIR="$REPO_ROOT/evaluations/$SCENARIO"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RUN_DIR="$REPO_ROOT/runs/${SCENARIO}_${TIMESTAMP}"

echo ">>> Setting up evaluation for $SCENARIO in $RUN_DIR..."

# 1. Prepare Workspace
mkdir -p "$RUN_DIR"
cp -r "$EVAL_DIR/workspace/"* "$RUN_DIR/"
cp "$EVAL_DIR/README.md" "$RUN_DIR/PROMPT.md"

# 2. Run Agent
cd "$RUN_DIR"
echo ">>> Running Agent..."
START_TIME=$(date +%s)

# Pipe prompt to agent
cat PROMPT.md | eval "$AGENT_CMD" || true

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
echo ">>> Agent finished in ${DURATION}s."

# 3. Functional Verification
echo ">>> Running Verification Tests..."
if [ -f "$EVAL_DIR/solution/verification_test.go" ]; then
  cp "$EVAL_DIR/solution/verification_test.go" .
  
  # Ensure we can run tests
  # If go.mod exists, use it.
  if [ -f "go.mod" ]; then
    go test ./... -v > verification.log 2>&1 || true
  else
    go test -v *.go > verification.log 2>&1 || true
  fi
  
  if grep -q "FAIL" verification.log; then
    echo "❌ Verification Tests FAILED"
    grep "FAIL" verification.log
  elif grep -q "PASS" verification.log; then
    echo "✅ Verification Tests PASSED"
  else
    echo "⚠️ Verification Tests did not run or output PASS/FAIL properly."
  fi
else
  echo "⚠️ No verification_test.go found for this scenario."
fi

# 4. Linting
if command -v golangci-lint &> /dev/null; then
    golangci-lint run > lint.log 2>&1 || true
    if [ -s lint.log ]; then
        echo "⚠️ Linting issues found (see lint.log)"
    else
        echo "✅ Linting Passed"
    fi
else
    echo "ℹ️ golangci-lint not installed, skipping."
fi

echo ">>> Evaluation complete. Results in $RUN_DIR"