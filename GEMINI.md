# Gemini Go Doctor Guidelines

This document outlines the best practices and operational parameters for the Gemini AI assistant when working on this Go project. The primary goal is to ensure code quality, maintainability, and adherence to idiomatic Go conventions.

## Error Budget

To maintain a high standard of quality, the AI will operate under an error budget system. The purpose of this budget is not merely to track failures, but to encourage a more cautious and rigorous development process when confidence is low.

- **Initial Budget:** 100 points.
- **Successful Task:** +1 point. A successful task is a `go test` or `go build` command that passes without errors.
- **Failed Task:** -1 point. A failed task is a `go test` or `go build` command that fails. This includes regressions where a previously passing test fails after a code change.
- **Neutral Task:** No change. A test that fails as an expected part of a Test-Driven Development (TDD) cycle does not affect the budget. This only applies when a failing test is written *before* the implementation code.
- **Budget Interpretation:**
    - **> 120 (High Confidence):** The AI is performing well. It can rely more on its internal knowledge and be more proactive in its suggestions and changes.
    - **80 - 120 (Normal Operations):** The AI should follow standard procedures, balancing autonomy with verification.
    - **< 80 (Low Confidence):** The AI is making frequent mistakes. It must become extremely cautious. Before making any code change, it **must** consult external documentation, use web search to verify APIs and language features, and double-check its work against established patterns in the codebase.
    - **0 (Full Stop):** The error budget is depleted. The AI must cease all operations, report its status, and hand over control to the user for a manual review and reset.

- **Budget Reporting:** The AI **must, without exception, report** the current error budget at the beginning of every message, in the format: `ERROR BUDGET: <value>`. Failure to report the budget is a critical violation of these operational guidelines.

The AI is responsible for tracking and reporting its current error budget upon request.

## Go Programming Best Practices

All code contributed to this project must adhere to the following principles.

### 1. Formatting

All Go code **must** be formatted with `gofmt` before being submitted. No exceptions. This ensures a consistent and readable codebase.

### 2. Naming Conventions

- **Packages:** Use short, concise, all-lowercase names. Avoid `under_scores` or `mixedCaps`.
- **Variables, Functions, and Methods:** Use `camelCase` for unexported (internal) identifiers and `PascalCase` for exported (public) identifiers.
- **Interfaces:** Interface types should not have a prefix like `I`. Name them for what they do (e.g., `io.Reader`).

### 3. Error Handling

- Errors are values. Do not discard errors using the blank identifier (`_`).
- Handle errors explicitly. The `if err != nil` pattern is the standard.
- Provide context to errors. Use `fmt.Errorf("context: %w", err)` to wrap errors and build a meaningful error chain.

### 4. Simplicity and Clarity

- "Clear is better than clever." Write code that is easy for other developers to understand.
- Avoid unnecessary complexity and abstractions.
- Prefer returning concrete types, not interfaces.

### 5. Concurrency

- "Don't communicate by sharing memory, share memory by communicating."
- Use channels to manage communication between goroutines.
- Be mindful of race conditions. Use the `-race` flag during testing to detect them.

### 6. Packages and Project Structure

- Keep packages focused on a single purpose.
- Circular dependencies are a design flaw and are not allowed.
- Follow the project's existing directory structure.

### 7. Testing

- Write unit tests for new functionality.
- Place tests in `_test.go` files alongside the code they are testing.
- Use the standard `testing` package.
- Aim for reasonable test coverage, focusing on critical paths and edge cases.
- After `make test` passes, consider using the `code_review` tool as a final verification step before considering a task complete.

### 8. Documentation

- All exported identifiers (`PascalCase`) **must** have a doc comment.
- Comments should explain the *why*, not the *what*.
- Follow the conventions outlined in [Effective Go](https://go.dev/doc/effective_go).

By adhering to these guidelines, we aim to build a robust, maintainable, and high-quality Go application.
# The gopls MCP server

These instructions describe how to efficiently work in the Go programming language using the gopls MCP server. You can load this file directly into a session where the gopls MCP server is connected.

## Detecting a Go workspace

At the start of every session, you MUST use the `go_workspace` tool to learn about the Go workspace. The rest of these instructions apply whenever that tool indicates that the user is in a Go workspace.

## Go programming workflows

These guidelines MUST be followed whenever working in a Go workspace. There are two workflows described below: the 'Read Workflow' must be followed when the user asks a question about a Go workspace. The 'Edit Workflow' must be followed when the user edits a Go workspace.

You may re-do parts of each workflow as necessary to recover from errors. However, you must not skip any steps.

### Read workflow

The goal of the read workflow is to understand the codebase.

1. **Understand the workspace layout**: Start by using `go_workspace` to understand the overall structure of the workspace, such as whether it's a module, a workspace, or a GOPATH project.

2. **Find relevant symbols**: If you're looking for a specific type, function, or variable, use `go_search`. This is a fuzzy search that will help you locate symbols even if you don't know the exact name or location.
   EXAMPLE: search for the 'Server' type: `go_search({"query":"server"})`

3. **Understand a file and its intra-package dependencies**: When you have a file path and want to understand its contents and how it connects to other files *in the same package*, use `go_file_context`. This tool will show you a summary of the declarations from other files in the same package that are used by the current file. `go_file_context` MUST be used immediately after reading any Go file for the first time, and MAY be re-used if dependencies have changed.
   EXAMPLE: to understand `server.go`'s dependencies on other files in its package: `go_file_context({"file":"/path/to/server.go"})`

4. **Understand a package's public API**: When you need to understand what a package provides to external code (i.e., its public API), use `go_package_api`. This is especially useful for understanding third-party dependencies or other packages in the same monorepo.
   EXAMPLE: to see the API of the `storage` package: `go_package_api({"packagePaths":["example.com/internal/storage"]})`

### Editing workflow

The editing workflow is iterative. You should cycle through these steps until the task is complete.

1. **Read first**: Before making any edits, follow the Read Workflow to understand the user's request and the relevant code.

2. **Find references**: Before modifying the definition of any symbol, use the `go_symbol_references` tool to find all references to that identifier. This is critical for understanding the impact of your change. Read the files containing references to evaluate if any further edits are required.
   EXAMPLE: `go_symbol_references({"file":"/path/to/server.go","symbol":"Server.Run"})`

3. **Make edits**: Make the required edits, including edits to references you identified in the previous step. Don't proceed to the next step until all planned edits are complete.

4. **Check for errors**: After every code modification, you MUST call the `go_diagnostics` tool. Pass the paths of the files you have edited. This tool will report any build or analysis errors.
   EXAMPLE: `go_diagnostics({"files":["/path/to/server.go"]})`

5. **Fix errors**: If `go_diagnostics` reports any errors, fix them. The tool may provide suggested quick fixes in the form of diffs. You should review these diffs and apply them if they are correct. Once you've applied a fix, re-run `go_diagnostics` to confirm that the issue is resolved. It is OK to ignore 'hint' or 'info' diagnostics if they are not relevant to the current task. Note that Go diagnostic messages may contain a summary of the source code, which may not match its exact text.

6. **Run tests**: Once `go_diagnostics` reports no errors (and ONLY once there are no errors), run the tests for the packages you have changed. You can do this with `go test [packagePath...]`. Don't run `go test ./...` unless the user explicitly requests it, as doing so may slow down the iteration loop.


