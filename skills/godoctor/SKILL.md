---
name: godoctor
description: Activate when working with Go (golang) code. Includes tooling and best practices to generate code efficiently and increase code quality.
metadata:
  author: daniela@danicat.dev
  version: 0.1.0
---

# Go Quality & Tooling Guide (GoDoctor)

## Go Version
Current go version: 1.26.5

## Coding Guidelines

### Google Go Style & Standards
Follow the official Google Go Style Guide and Go Code Review Comments. Code must be idiomatic, clear, maintainable, and strictly checked using standard Go tooling (`gofmt`, `go vet`, `goimports`).

### Package Architecture & Layout
- **Flat Package Structure**: Prefer flat package layouts over deep enterprise layered modeling (such as `adapters/`, `ports/`, `entities/`, `controllers/`, `repositories/`, `services/`, `usecases/`). Keep code flat in the root or logically grouped by feature/domain.
- **Private vs. Public API**: Use `internal/` for private packages that should not be imported by external modules. Do not use `pkg/` unless it is a cloud native project in the K8S ecossystem.
- **Test Fixtures**: Store test fixtures, golden files, mock datasets, and external test inputs in `testdata/` directories. The Go toolchain ignores `testdata/` folders during normal package builds.
- **Avoid Monolithic Files**: Avoid monolithic single-file packages. Split package logic into clear, focused files named after their primary responsibility (e.g., `server.go`, `handler.go`, `config.go`).
- **No Stuttering in Naming**: Avoid repeating package names in exported types or functions. For example, use `user.Service` instead of `user.UserService`, `http.Server` instead of `http.HttpServer`, and `config.Load` instead of `config.LoadConfig`.
- **Prohibition of Generic Packages**: NEVER create generic `util`, `shared`, `common`, or `helpers` packages. These act as catch-all dumping grounds that destroy dependency boundaries. Place functionality in specific, domain-named packages or close to its site of use.

### API Design & HTTP Architecture
- **API Design Standards**: Keep interfaces small and consumer-defined (*accept interfaces, return structs*). Do not create premature interfaces with single implementations. Expose concrete types from producer packages.
- **HTTP Service Architecture**: Follow modern Go HTTP service design patterns as described in [How I write HTTP services in Go after 13 years](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/). Key principles include constructor-based dependency injection, grouping HTTP routes and handlers on a single server struct, and writing explicit HTTP middleware.

## Installation & Setup

### Installing GoDoctor as a Plugin in Antigravity

To install the GoDoctor plugin, run:
```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh
```

### Installing GoDoctor as an MCP Server

1. Install `godoctor` using `go install`:
   ```bash
   go install github.com/danicat/godoctor/cmd/godoctor@latest
   ```
2. Ensure `$GOPATH/bin` (or `$(go env GOPATH)/bin`) is included in your system `PATH`.
3. Add `godoctor` to your MCP client configuration (`mcp_config.json`):
   ```json
   {
     "mcpServers": {
       "godoctor": {
         "command": "godoctor",
         "args": []
       }
     }
   }
   ```
   *Alternatively, run directly via `go run` without pre-compiling:*
   ```json
   {
     "mcpServers": {
       "godoctor": {
         "command": "go",
         "args": ["run", "github.com/danicat/godoctor/cmd/godoctor@latest"]
       }
     }
   }
   ```

## Advanced Tools & Tool Suite

The GoDoctor suite offers both MCP tools for AI agents and direct CLI/fallback commands for non-MCP shell environments.

### GoDoctor MCP Tool Suite
- `smart_read`: Reads Go source files with structure awareness and appends `<types>` metadata blocks defining referenced symbols. Supports AST outline mode.
- `smart_edit`: Performs single-file code edits verified by `gofmt`, `goimports`, and `go vet` before committing to disk. Automatically rolls back edits on compiler error.
- `smart_multi_edit`: Performs atomic, multi-file batch code edits across the workspace verified by `gofmt`, `goimports`, and `go vet` before committing to disk.
- `smart_build`: Executes the full workspace verification pipeline (`go mod tidy`, formatting, modernizer, `go build`, `go test`, linter, deadcode analysis).
- `test_query`: Executes SQL queries against the local `testquery.db` database to inspect test results and coverage metrics.
- `mutation_test`: Runs Selene mutation testing against Go packages to evaluate unit test effectiveness.
- `list_files`: VCS-aware workspace file mapper ignoring `.git` and build artifacts.
- `add_dependencies`: Installs Go modules, updates `go.mod`/`go.sum`, and returns documentation for installed packages.
- `read_docs`: Fetches Go documentation and function signatures for standard library or third-party packages.

### Non-MCP Fallback Commands (`go run`)
In environments where GoDoctor MCP server tools are not integrated, invoke Selene and TestQuery directly via `go run`:

- **Selene Mutation Testing Fallback**:
  ```bash
  go run github.com/danicat/selene@latest ./...
  ```
- **TestQuery SQL Analyzer Fallback**:
  ```bash
  go run github.com/danicat/testquery@latest -query "SELECT * FROM tests WHERE status = 'FAIL'"
  ```

## TestQuery SQL Analyzer

TestQuery stores test execution logs and statement coverage metrics inside an SQLite database (`testquery.db`), enabling deep SQL analysis of testing quality.

### Database Schema

#### `tests` Table
Stores details of test executions.
| Column | Type | Description |
|---|---|---|
| `id` | INTEGER PRIMARY KEY | Unique identifier for the test run |
| `package` | TEXT | Go package path |
| `name` | TEXT | Test function name |
| `status` | TEXT | Execution status (`PASS`, `FAIL`, `SKIP`) |
| `duration_ms` | REAL | Test duration in milliseconds |
| `output` | TEXT | Console output / log snippet |
| `run_at` | TIMESTAMP | Timestamp when the test ran |

#### `coverage` Table
Stores statement-level coverage data.
| Column | Type | Description |
|---|---|---|
| `id` | INTEGER PRIMARY KEY | Unique identifier |
| `package` | TEXT | Go package path |
| `file` | TEXT | Relative or full file path |
| `start_line` | INTEGER | Starting line of the code block |
| `end_line` | INTEGER | Ending line of the code block |
| `num_stmt` | INTEGER | Number of Go statements in the block |
| `count` | INTEGER | Execution hit count (0 indicates uncovered) |

### Example SQL Queries

#### 1. Finding Failed Tests
```sql
SELECT package, name, output, duration_ms 
FROM tests 
WHERE status = 'FAIL' 
ORDER BY run_at DESC;
```

#### 2. Finding Uncovered Statements and Lines
```sql
SELECT package, file, start_line, end_line, num_stmt 
FROM coverage 
WHERE count = 0 
ORDER BY package, file, start_line;
```

#### 3. Finding Slow Test Cases
```sql
SELECT package, name, duration_ms 
FROM tests 
WHERE duration_ms > 500 
ORDER BY duration_ms DESC;
```

## Selene Mutation Testing Guide

### What is Mutation Testing?
Mutation testing evaluates the quality and thoroughness of unit tests by automatically modifying Abstract Syntax Tree (AST) operators (mutants)—such as swapping `+` to `-`, changing `==` to `!=`, or flipping boolean conditions—and executing the test suite against the mutated code.

- **Killed Mutant**: A test fails when the code is mutated. This is the desired outcome, proving the test suite actively detects code defects.
- **Surviving Mutant**: All tests pass despite code modification. This indicates missing, weak, or incomplete test assertions.
- **Uncovered Mutant**: A mutation occurred in code not reached by any test.

### Running Selene & Interpreting Output

Run mutation testing via MCP or `go run`:
```bash
go run github.com/danicat/selene@latest ./internal/auth
```

#### Sample Output:
```text
Mutation testing results:

Total mutations: 12
Killed:          10
Timeouts:         0
Survived:         2
Uncovered:        0

Mutation Score:  83.33% (killed/total mutations)

Surviving Mutants:
1. ./internal/auth/auth.go:42:15
   Mutated: 'if user.Age >= 18' -> 'if user.Age > 18'
   Status: SURVIVED (Tests passed when condition was mutated)

2. ./internal/auth/auth.go:58:8
   Mutated: 'return token, nil' -> 'return "", nil'
   Status: SURVIVED (Tests passed when token was cleared)
```

### How to Fix Surviving Mutants
To kill a surviving mutant, add targeted test cases with explicit assertions that validate the exact mutated logic:

1. **Fix Boundary Mutations (`>=` to `>`):**
   Add a test case specifically checking boundary values (e.g., testing `Age == 18`).
   ```go
   func TestIsAdult_Boundary(t *testing.T) {
       user := User{Age: 18}
       if !IsAdult(user) {
           t.Errorf("expected age 18 to be adult")
       }
   }
   ```

2. **Fix Return Value Mutations (`token` to `""`):**
   Assert the actual value returned rather than only checking `err == nil`.
   ```go
   func TestGenerateToken_Value(t *testing.T) {
       token, err := GenerateToken("user123")
       if err != nil {
           t.Fatalf("unexpected error: %v", err)
       }
       if token == "" {
           t.Errorf("expected non-empty token")
       }
   }
   ```
