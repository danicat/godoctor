# RFC-0001: Experimental `smart_test` Tool (Decoupled Design)

- **Status**: Draft
- **Date**: 2026-08-12
- **Author**: Antigravity & GoDoctor Team
- **Deciders/Reviewers**: Danicat / GoDoctor Maintainers
- **ADR Reference**: N/A (Pending RFC Review)

---

## 1. Executive Summary

This proposal outlines the creation of **`smart_test`**, an experimental, standalone MCP tool dedicated to Go test execution, diagnostic output, test result indexing (`testquery.db`), and AST mutation testing (`Selene`).

To maintain strict modularity and prevent regressions, **`smart_test` and `smart_build` will remain completely decoupled**. `smart_build` will retain its existing self-contained build/test pipeline, while `smart_test` will operate as an independent tool optimized for fast, granular developer test loops.

---

## 2. Context & Problem Statement

Currently, running tests via GoDoctor requires either calling `smart_build` or running CLI commands:
- **`smart_build`** executes a full workspace health check (`go mod tidy` -> `modernize` -> `gofmt` -> `deadcode` -> `go build` -> `go test -v -coverprofile` -> `linter`). Rerunning a single failing test forces the entire multi-stage build pipeline to execute.
- **`go test ./...`** lacks structured output formatting, automatic integration with `testquery.db` (for SQL queries), granular filtering flags, or mutation testing triggers.

Developers need a fast, specialized test runner that supports targeted test filtering, race detection, coverage gap visualization, and test history database synchronization—without altering or depending on `smart_build`.

---

## 3. Guiding Principles & Architectural Decoupling

```
                  ┌─────────────────────────────────┐
                  │       GoDoctor MCP Server       │
                  └───────────────┬─────────────────┘
                                  │
         ┌────────────────────────┴────────────────────────┐
         │ (Independent Tool Registration)                │
         ▼                                                ▼
┌──────────────────┐                            ┌──────────────────┐
│   smart_build    │                            │    smart_test    │
│  (Stable Tool)   │                            │  (Experimental)  │
└────────┬─────────┘                            └────────┬─────────┘
         │                                               │
         │ Runs:                                         │ Runs:
         ├─ go mod tidy                                  ├─ go test (-run, -race, -bench)
         ├─ modernize                                    ├─ testquery.db sync
         ├─ gofmt / deadcode                             └─ selene mutation testing
         ├─ go build                                             │
         ├─ go test                                              ▼
         └─ golangci-lint                               ┌──────────────────┐
                                                        │    test_query    │
                                                        │   (SQL Engine)   │
                                                        └──────────────────┘
```

1. **Zero Import Dependency**: `internal/tools/smart_build` will not import `internal/tools/smart_test`, and `internal/tools/smart_test` will not import `internal/tools/smart_build`.
2. **Independent Lifecycles**: `smart_build` remains stable and untouched. `smart_test` can evolve, experiment with output schemas, or be disabled without impacting `smart_build`.
3. **Shared Primitives Only**: Both tools leverage common low-level infrastructure packages (`internal/roots` for path validation and `internal/safeshell` for safe command execution).

---

## 4. `smart_test` Tool Specification

### MCP Tool Schema

- **Tool Name**: `smart_test`
- **Package Location**: `internal/tools/smart_test/test.go`
- **Experimental Notice**: Marked as `🧪 [Experimental]` in its MCP description.

```json
{
  "name": "smart_test",
  "description": "GoDoctor's specialized test runner. Executes Go tests across packages or specific functions, delivering structured failure diagnostics, coverage gap analysis, benchmark metrics, and automated test history tracking.",
  "parameters": {
    "type": "object",
    "properties": {
      "dir": {
        "type": "string",
        "description": "The absolute directory path to test in. Always pass absolute paths in multi-root workspaces."
      },
      "packages": {
        "type": "string",
        "description": "Packages to test (default: ./...)."
      },
      "level": {
        "type": "string",
        "enum": ["fast", "basic", "benchmark", "complete"],
        "description": "Testing depth/mode: 'fast' (unit tests only), 'basic' (tests + coverage + testquery.db sync, default), 'benchmark' (benchmarks with sensible defaults), 'complete' (tests + coverage + Selene mutation testing)."
      },
      "run": {
        "type": "string",
        "description": "Regex pattern to filter specific tests or benchmark functions (maps to -run or -bench)."
      }
    },
    "required": ["dir"]
  }
}
```

---

## 5. Key Features & Pipeline Workflow

When `smart_test` is invoked, it executes the following pipeline:

```
                  ┌───────────────────────────────┐
                  │ 1. Validate Dir & Target Pkgs │
                  └───────────────┬───────────────┘
                                  │
      ┌───────────────────┬───────┴───────────┬───────────────────┐
      │                   │                   │                   │
level = "fast"      level = "basic"    level = "benchmark"   level = "complete"
 (Tests Only)      (Tests+Coverage)       (Benchmarks)      (Tests+Cov+Selene)
      │                   │                   │                   │
      │                   ▼                   │                   ▼
      │            [Sync testquery]           │            [Sync testquery]
      │                   │                   │                   │
      │                   │                   │                   ▼
      │                   │                   │            [Selene Mutation]
      │                   │                   │                   │
      └───────────────────┴─────────┬─────────┴───────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────────┐
                    │    Return Markdown Report     │
                    └───────────────────────────────┘
```

### Execution Levels

1. **`level: "fast"`**:
   - Runs `go test -json` with target packages and `run` filter.
   - Skips coverage profiling (`-coverprofile`) and Selene mutation testing.
   - Ideal for sub-second TDD iterations while refactoring code or debugging a single failing test function.

2. **`level: "basic"`** *(Default)*:
   - Runs `go test -v -coverprofile=...`.
   - Generates statement-level coverage reports and pinpoints untested function ranges.
   - Automatically synchronizes test runs and coverage records into `testquery.db` for SQL query analysis.

3. **`level: "benchmark"`**:
   - Executes benchmarks using sensible Go defaults (`go test -bench=<run|.> -benchmem -run=^$`).
   - Automatically skips normal unit tests (`-run=^$`) so execution is fast and benchmarking isolated.
   - Parses output lines (`BenchmarkParse-12 1000000 1052 ns/op 128 B/op 4 allocs/op`) and formats them into a clean performance table:

     | Benchmark | Iterations | Time / Op | Memory / Op | Allocs / Op |
     | :--- | :--- | :--- | :--- | :--- |
     | `BenchmarkParse-12` | 1,000,000 | `1052 ns/op` | `128 B/op` | `4 allocs/op` |

4. **`level: "complete"`**:
   - Executes everything in `level: "basic"`.
   - After unit tests pass successfully, triggers Selene AST mutation testing (`go run github.com/danicat/selene@latest`).
   - Identifies surviving mutants and flags weak test assertions or missing boundary conditions.

---

## 6. Sample Response Output

```markdown
# 🧪 Smart Test Report (`internal/auth`)

### ⚡ Status: ❌ FAILED (1 failed, 14 passed in 180ms)

#### 🔴 Test Failures
* **`TestValidateToken/ExpiredToken`** ([token_test.go:48](file:///path/to/token_test.go#L48)):
  ```text
  --- FAIL: TestValidateToken/ExpiredToken (0.01s)
      token_test.go:48: expected ErrExpired, got nil
  ```

---

#### 📊 Coverage & Gap Summary
* **Package Coverage**: `82.4%`
* **Uncovered Statements**:
  * `internal/auth/token.go:72-85` (`RefreshToken`) - 0 hits

---

#### 💾 TestQuery Sync
* Synchronized `15` test results to `testquery.db`.
* *Query history using `test_query(query="SELECT * FROM tests WHERE status = 'FAIL'")`.*
```

---

## 7. Tool Lifecycle Strategy

`smart_test` joins GoDoctor as a core member of the tool suite alongside `smart_build`, `smart_read`, and `smart_edit`.

1. **Standard Registration**: Registered in `internal/server/server.go` alongside other `smart_*` tools.
2. **Instruction Alignment**: Featured in `internal/instructions/instructions.go` as the primary tool for testing Go packages and running TDD loops.

---

## 8. Discarded Alternatives

| Alternative | Why Discarded |
| :--- | :--- |
| **Coupling `smart_build` to call `smart_test`** | Rejected per requirement. Tight coupling creates risk of regressions in `smart_build` and prevents isolated experimentation. |
| **Adding all test flags directly to `smart_build`** | Rejected. Overloads `smart_build` schema, causing SRP violation and making basic test iterations slow. |

---

---

## 10. Architectural Review & Counter-Critique Matrix

An initial critical review identified potential edge cases, which were subsequently stress-tested by a pragmatic counter-review:

| Topic | Initial Critique | Counter-Critique & Final Resolution |
| :--- | :--- | :--- |
| **Tool Schema & Flags** | Suggested adding a raw `flags` string for `-race`, `-tags`, `-count=1`. | **Resolution**: Exposing raw CLI strings to LLMs introduces shell injection risks, escaping bugs, and prompt confusion. Keep schema strictly typed with optional `race` (bool) or `tags` (string) fields if needed. |
| **`testquery.db` Sync in `fast` Mode** | Claimed skipping DB sync in `fast` mode creates "fragmented test history". | **Resolution**: `fast` mode exists for sub-millisecond TDD iterations. Disk I/O and SQLite writes defeat this purpose. `testquery.db` is an analytics engine, not a ledger; skipping micro-TDD runs is the correct trade-off. |
| **Output Parsing** | Demanded using `golang.org/x/tools/benchmark/parse`. | **Resolution**: `x/tools/benchmark/parse` was deprecated years ago in favor of `x/perf`. Standard library `encoding/json` and basic line scanners handle `go test -json` and benchmark lines cleanly without extra dependencies. |
| **Timeout Handling** | Recommended a blanket 5-minute timeout. | **Resolution**: A static 5-minute limit breaks large benchmark suites or deep mutation testing runs. Timeouts must be **level-specific** (e.g. 30s for `fast`, 3m for `basic`, 15m for `benchmark` / `complete`). |
| **Database Concurrency** | Noted potential `SQLITE_BUSY` lock errors on `testquery.db`. | **Valid & Retained**: Enable SQLite WAL mode (`PRAGMA journal_mode=WAL;`) and wrap DB writes in exponential backoff retries. |

---

## 11. Next Steps

1. Review and approve RFC-0001.
2. Implement `internal/tools/smart_test/test.go` incorporating WAL mode, stdlib JSON streaming, and level-specific timeout contexts.
3. Register `smart_test` in `internal/server/server.go`.
4. Validate execution with unit tests and empirical run on workspace packages.
