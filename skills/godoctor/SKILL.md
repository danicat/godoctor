---
name: godoctor
description: Activate this skill whenever developing, building, editing, testing, documenting, or verifying Go (golang) code, or managing GoDoctor CLI and MCP surfaces. Enforces strict Google Go Style, flat package architecture, zero-fallback execution, multi-tier testing, AST-aware edits with compiler rollback gates, Selene mutation testing, and TestQuery SQL analytics.
metadata:
  author: daniela@danicat.dev
  version: 0.34.0
---

# Go Quality & Tooling Guide (GoDoctor)

GoDoctor provides AST-aware Go developer tooling, code quality enforcement, and testing analytics available both as a command-line interface (CLI) and as a Model Context Protocol (MCP) server.

---

## 1. Go Coding & Architectural Standards

### Google Go Style & Idiomatic Practices
- **Standard Toolchain Enforcement**: All code must be strictly formatted with `gofmt`, organized with `goimports`, checked with `go vet`, and linted with `golangci-lint`.
- **Naming Conventions**:
  - Avoid repeating package names in exported types or functions (*no stuttering*). Use `user.Service` instead of `user.UserService`, `http.Server` instead of `http.HttpServer`, and `config.Load` instead of `config.LoadConfig`.
  - Use camelCase for unexported identifiers and PascalCase for exported identifiers. Acronyms must remain uniform in case (e.g., `JSONURL`, `dbID`, `xmlHTTP`).
- **Error Handling**:
  - Return errors as the last return value.
  - Wrap errors with contextual information using `fmt.Errorf("action description: %w", err)`.
  - Do not panic in libraries or standard business logic; return explicit errors.

### Package Architecture & Layout
- **Flat Package Structure**: Prefer flat package layouts over deep enterprise layered modeling (such as `adapters/`, `ports/`, `entities/`, `controllers/`, `repositories/`, `services/`, `usecases/`). Keep code flat in the root or logically grouped by feature/domain.
- **Private vs. Public API**: Use `internal/` for private packages that should not be imported by external modules. Do not create a `pkg/` directory unless developing a cloud-native project in the Kubernetes ecosystem.
- **Test Fixtures & Golden Files**: Store test fixtures, golden files, mock datasets, and external test inputs in `testdata/` directories. The Go toolchain ignores `testdata/` folders during normal package compilation.
- **Avoid Monolithic Files**: Split package logic into clear, focused files named after their primary responsibility (e.g., `server.go`, `handler.go`, `config.go`, `types.go`).
- **Prohibition of Generic Catch-All Packages**: NEVER create generic `util`, `shared`, `common`, or `helpers` packages. These act as catch-all dumping grounds that destroy dependency boundaries. Place functionality in specific, domain-named packages or close to its site of use.

### API Design & HTTP Architecture
- **Interface Segregation**: Keep interfaces small and consumer-defined (*accept interfaces, return structs*). Do not create premature interfaces with single implementations. Expose concrete types from producer packages.
- **HTTP Service Design**: Follow modern Go HTTP service design patterns:
  - Constructor-based dependency injection (e.g., `NewServer(cfg, logger)`).
  - Group HTTP routes and handlers on a single server struct.
  - Write explicit HTTP middleware for cross-cutting concerns (logging, authentication, tracing).

---

## 2. Tool Selection Matrix

| Task / Goal | CLI Command (`godoctor call`) | MCP Tool Name | Behavior / Safeguards |
| :--- | :--- | :--- | :--- |
| **AST-Aware Code Edits** | `godoctor call edit` | `smart_edit` | Coordinate matching + AST formatting + atomic write + compiler rollback gate (`go vet`). |
| **Build & Quality Pipeline** | `godoctor call build` | `smart_build` | Builds Go binaries and packages with integrated compilation, testing, coverage analysis, linting, and quality verification. |
| **Test & Benchmark Runner** | `godoctor call test` | `smart_test` | Multi-tier runner (`fast`, `basic`/`standard`, `benchmark`, `complete`) + auto-indexes into `testquery.db`. |
| **AST Documentation** | `godoctor call docs` | `read_docs` | Fetches package docs, exported symbols, types, and function signatures with 3-tier fallback caching. |
| **Mutation Testing** | `godoctor call selene` | `selene` | Evaluates test suite quality by mutating AST operators and checking for test assertion kills. See [references/selene.md](references/selene.md). |
| **SQL Test Analytics** | `godoctor call tq` | `test_query` | Executes SQLite queries against test history and statement coverage in `testquery.db`. See [references/testquery.md](references/testquery.md). |

### Test Runner Tiers (`smart_test` / `godoctor call test`)
- **`level: "fast"`**: Sub-second inner loop test execution. Runs package unit tests directly; skips coverage profiling, benchmarks, and mutation analysis. Ideal for rapid iterative development.
- **`level: "basic"` / `"standard"`**: Standard testing tier. Runs unit tests with statement coverage profiling and auto-indexes execution metrics into `.godoctor/testquery.db`.
- **`level: "benchmark"`**: Runs unit tests, coverage profiling, and Go benchmark suites (`go test -bench=.`).
- **`level: "complete"`**: Comprehensive quality gate. Runs unit tests, coverage profiling, benchmarks, and full multi-worker Selene AST mutation testing across all packages. Ideal for pre-commit, CI verification, and release audits.

---

## 3. Core Principles & Safeguards

- **Zero-Fallback Policy**: External binaries (`golangci-lint`, `modernize`, `deadcode`, `selene`, `testquery`) must be pre-installed in `$PATH` or defined in `.godoctor.yaml`. Dynamic `go run` compilation fallbacks are banned to eliminate 1.5s–4.5s latency delays and ensure reproducible execution.
- **Tool Version Tracking**: GoDoctor actively verifies installed tool versions against recommended baselines, reporting non-blocking upgrade recommendations and providing `godoctor check`.
- **Absolute Paths Required**: All directory (`dir`) and file (`filename`) parameters must be absolute paths (e.g. `/Users/.../project`).
- **Atomic Edit Transactions & Compiler Gate**: `edit` / `smart_edit` writes changes to temporary files before atomic replacement, preserving file permissions. Edits are verified via `go vet ./...` and automatically rolled back if errors are introduced.
- **Concurrency & Resource Management**: Heavy operations like `level: "complete"` (Selene AST mutation testing) utilize all CPU cores; avoid spawning concurrent test/build tasks while complete runs are in flight to prevent CPU exhaustion and SQLite WAL contention.
- **Configuration-Driven**: Subsystems read settings from `.godoctor.yaml` following a strict 3-tier precedence hierarchy:
  $$\text{Per-Call Payload (JSON)} \succ \text{Config File } (\texttt{.godoctor.yaml}) \succ \text{Built-in Defaults}$$

---

## 4. Environment Diagnostics (`godoctor check`)

Inspect installed external tools, versions, and health status:

```bash
# Formatted ASCII diagnostic table
godoctor check

# Machine-readable JSON output
godoctor check --json
```

---

## 5. Centralized Configuration (`.godoctor.yaml`)

Initialize a configuration file in your repository:

```bash
godoctor init
```

Key configuration sections in `.godoctor.yaml`:

```yaml
version: "1"

# CLI & Runtime Settings
cli:
  default_output: "text"
  color: true

# Server Execution Settings
server:
  write_timeout: "5m"
  allowed_origins:
    - "http://localhost"
    - "http://localhost:*"
    - "http://127.0.0.1"
    - "http://127.0.0.1:*"

# External Tools & Version Management
tools:
  golangci_lint:
    recommended_version: "v2.12.2"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
  modernize:
    recommended_version: "latest"
    pkg: "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest"
  deadcode:
    recommended_version: "latest"
    pkg: "golang.org/x/tools/cmd/deadcode@latest"
  selene:
    recommended_version: "latest"
    pkg: "github.com/danicat/selene/cmd/selene@latest"
    workers: 0 # 0 defaults to runtime.GOMAXPROCS
    testquery_compat: true
  testquery:
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: ".godoctor/testquery.db"

# Subsystem Flags & Behavior
features:
  autofix: true
  deadcode_check: true
  testquery_sync: true
  version_check_hints: true
  auto_rollback: true
```

---

## 6. Installation & Surface Management

### Installing GoDoctor CLI
```bash
go install github.com/danicat/godoctor/cmd/godoctor@latest
```

### Managing Surfaces (`godoctor install` & `uninstall`)
Configure MCP server registration in `mcp_config.json` and unpack agent skills:

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

## 7. Direct CLI Invocation Examples (`godoctor call`)

### 1. `edit` (AST-Verified Coordinate Edits with Atomic Rollback)
```bash
godoctor call edit '{"filename": "/absolute/path/to/main.go", "old_content": "fmt.Println(\"old\")", "new_content": "fmt.Println(\"new\")"}'
```

### 2. `build` (Build, Test, and Lint Pipeline)
```bash
# Standard workspace build and test
godoctor call build '{"dir": "/absolute/path/to/project"}'

# Build with specific output binary target
godoctor call build '{"dir": "/absolute/path/to/project", "packages": "./cmd/godoctor", "output": "bin/godoctor"}'
```

### 3. `test` (Multi-Tier Test Runner)
```bash
# Available levels: fast, basic, benchmark, complete
godoctor call test '{"dir": "/absolute/path/to/project", "level": "basic"}'
```

### 4. `docs` (AST Symbol & Type Documentation)
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

## 8. Detailed References

For specialized workflows, refer to the companion references:
- **TestQuery SQL Analytics & Schema**: [references/testquery.md](references/testquery.md) — Comprehensive database schema (`all_tests`, `all_coverage`, `test_coverage`, `all_code`), SQLite query recipes, and statement coverage metrics.
- **Selene Mutation Testing Guide**: [references/selene.md](references/selene.md) — AST mutation operators, mutant statuses (`KILLED`, `SURVIVED`, `UNCOVERED`), targeted mode execution, and surviving mutant remediation strategies.
