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
