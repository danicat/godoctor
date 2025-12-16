# Scenario D: Test & Document

## Background
You have inherited a utility file `risk.go` with a core business logic function `CalculateRiskScore`. It has no tests and minimal documentation.

## Task
1.  Read `risk.go` to understand the logic.
2.  Create a new file `risk_test.go` with a table-driven unit test covering all branches (Minor, Low Income, Senior, Default).
3.  Run the test to ensure it passes.
4.  Update the documentation comment for `CalculateRiskScore` to be more detailed about the thresholds (e.g., mention age < 18 returns 100).

## Instructions for Agent
*   Use `edit_code` (strategy: `overwrite_file`) to create the test file.
*   Use `edit_code` (strategy: `single_match`) to update the doc comment.
