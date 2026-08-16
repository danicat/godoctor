---
name: godoctor
description: Activate this skill whenever developing, building, editing, testing, documenting, or verifying Go code, or managing GoDoctor CLI and MCP surfaces. Trigger when the user wants to run Go build/lint pipelines, perform AST-aware edits with compiler rollback gates, run multi-tier tests, inspect Go docs/types, run Selene mutation tests, or query test analytics. Trigger even for general Go tasks like "verify my Go code", "fix compiler errors", or "check Go package docs".
---

# GoDoctor Developer Intelligence Guide

GoDoctor provides AST-aware Go developer tooling available both as a standalone Command Line Interface (CLI) and as a Model Context Protocol (MCP) server.

## Tool Selection Matrix

| Task / Goal | CLI Command (`godoctor call`) | MCP Tool Name | Behavior / Safeguards |
| :--- | :--- | :--- | :--- |
| **Safe Code Edits** | `godoctor call edit` | `smart_edit` | Coordinate matching + AST formatting + compiler gate (`go vet`) with auto-rollback. |
| **Full Build & Hygiene Gate** | `godoctor call build` | `smart_build` | 6-phase pipeline: `go mod tidy` $\to$ modernize $\to$ `gofmt` $\to$ deadcode $\to$ build $\to$ test $\to$ linter. |
| **Test & Benchmark Runner** | `godoctor call test` | `smart_test` | Multi-tier runner (`fast`, `basic`, `benchmark`, `complete`) + indexes into `testquery.db`. |
| **AST Documentation Lookup** | `godoctor call docs` | `read_docs` | Fetches package docs, exported symbols, types, and function signatures. |
| **Mutation Testing** | `godoctor call selene` | `selene` | Evaluates test suite quality by mutating AST operators and checking for test failures. |
| **SQL Test Analytics** | `godoctor call tq` | `test_query` | Executes SQLite queries against test history and statement coverage in `testquery.db`. |

---

## Critical Gotchas & Rules

> [!IMPORTANT]
> 1. **Absolute Paths Required**: All directory (`dir`) and file (`filename`) parameters **MUST be absolute paths** (e.g. `/Users/.../project`). Relative paths (`.`, `./...`, `main.go`) are strictly rejected.
> 2. **Compiler Gate on Edit**: `edit` / `smart_edit` automatically reverts file changes if the edit introduces compilation or syntax errors (`go vet`).
> 3. **Automatic DB Synchronization**: Running `test` / `smart_test` automatically syncs statement coverage and run logs into `testquery.db`.
> 4. **No Ad-Hoc Build Scripts**: Always prefer `build` / `smart_build` over running manual shell commands for formatting, vet, or linting.

---

## Surface Management (`godoctor install` & `uninstall`)

Manage MCP server registration in `mcp_config.json` and agent skills unpacking (`@godoctor`, `@selene`, `@testquery`):

```bash
# Configure MCP and skills globally (default: ~/.gemini/config)
godoctor install

# Configure in workspace scope (.agents/)
godoctor install -w

# Modular configuration
godoctor install --mcp        # MCP server registration only
godoctor install --skills     # Skills unpacking only

# Clean removal
godoctor uninstall
godoctor uninstall -w
```

---

## Direct CLI Invocation (`godoctor call`)

Tools can be invoked from the command line using standard JSON or CLI flags:

### 1. `edit` (Safe Coordinate Editing)
```bash
godoctor call edit '{"filename": "/absolute/path/to/main.go", "old_content": "fmt.Println(\"old\")", "new_content": "fmt.Println(\"new\")"}'
```

### 2. `build` (Comprehensive Hygiene & Lint Pipeline)
```bash
godoctor call build '{"dir": "/absolute/path/to/project"}'
```

### 3. `test` (Multi-Tier Test Runner)
```bash
# Levels: fast, basic, benchmark, complete
godoctor call test '{"dir": "/absolute/path/to/project", "level": "basic"}'
```

### 4. `docs` (AST Symbol Documentation)
```bash
godoctor call docs '{"import_path": "net/http", "symbol_name": "Client"}'
```

### 5. `selene` (Mutation Testing)
```bash
godoctor call selene '{"dir": "/absolute/path/to/project"}'
```

### 6. `tq` (SQL Test & Coverage Analytics)
```bash
godoctor call tq '{"dir": "/absolute/path/to/project", "query": "SELECT package, test, elapsed FROM all_tests WHERE action = '\''fail'\''"}'
```

---

## Agent Verification Workflow Loop

When implementing Go features or resolving bug fixes:

1. **Inspect Documentation**: Use `docs` / `read_docs` to check standard library or third-party APIs.
2. **Apply Coordinate Edits**: Use `edit` / `smart_edit` to ensure edits compile cleanly without syntax errors.
3. **Execute Fast Tests**: Use `test` with `level: fast` for immediate feedback.
4. **Run Full Quality Gate**: Run `build` / `smart_build` to ensure formatting, tidy dependencies, deadcode elimination, and linter compliance.
5. **Audit Test Gaps**: Run `selene` to ensure unit tests catch introduced defects.
