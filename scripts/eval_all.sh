#!/bin/bash
set -e

AGENT_CMD=$1

if [ -z "$AGENT_CMD" ]; then
  echo "Usage: $0 <agent_command>"
  exit 1
fi

SCENARIOS=(
  "scenario_a_api_evolution"
  "scenario_b_concurrency_bug"
  "scenario_c_migration"
  "scenario_d_test_and_doc"
)

for scenario in "${SCENARIOS[@]}"; do
  echo "========================================================"
  echo "Running Eval: $scenario"
  echo "========================================================"
  ./scripts/run_eval.sh "$scenario" "$AGENT_CMD"
  echo ""
done
