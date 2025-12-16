# Evaluation Plan: GoDoctor MCP Server

This document outlines the evaluation strategy to benchmark the `godoctor` MCP server against a vanilla LLM agent. The goal is to measure improvements in accuracy, safety, and efficiency for typical software engineering tasks.

## Objectives
*   **Safety:** Does the MCP agent avoid deleting code or introducing syntax errors?
*   **Precision:** Can the MCP agent modify code without hallucinating context?
*   **Context:** Does the MCP agent correctly identify dependencies and usages?
*   **Recovery:** Can the MCP agent self-correct using tool feedback (e.g., build warnings)?

## Scenarios

### Scenario A: "The API Evolution" (Refactoring & Feature Add)
*   **Description:** A simple REST API for managing `Products`. The user requests adding a `Category` field to the struct.
*   **Challenge:** The agent must update the struct definition, the JSON parsing logic, the SQL/Mock database storage method, and the response handler.
*   **Metrics:** Number of files correctly updated, build stability, retention of existing logic.

### Scenario B: "The Concurrency Bug" (Debugging)
*   **Description:** A worker pool implementation that has a data race or deadlock (e.g., missing mutex lock or unbuffered channel misuse).
*   **Challenge:** The agent must identify the bug (potentially using `inspect` or reading docs) and apply a fix.
*   **Metrics:** Correct identification of the bug, minimal code change to fix it.

### Scenario C: "The Migration" (Batch Refactoring)
*   **Description:** A legacy codebase uses `io/ioutil` (deprecated) and an old logging interface.
*   **Challenge:** Replace all `ioutil.ReadFile` with `os.ReadFile` and `ioutil.ReadAll` with `io.ReadAll` across multiple files.
*   **Metrics:** Completeness of replacement (using `replace_all`), handling of imports (`goimports` auto-fix).

### Scenario D: "Test & Document" (Greenfield/Maintenance)
*   **Description:** A complex utility function `CalculateRiskScore` exists without tests or docs.
*   **Challenge:** Generate a table-driven test covering edge cases and add GoDoc comments.
*   **Metrics:** Validity of the generated test, coverage of edge cases, correct use of the `edit_code` tool to create new files.

## Structure
Each scenario will be organized as follows:
```
evaluations/
  <scenario_name>/
    README.md       # Instructions for the human evaluator and the initial prompt for the agent.
    workspace/      # The initial state of the code (sandboxed environment).
    solution/       # Reference implementation of the desired state.
```
