# GoDoctor

GoDoctor is a specialized agentic coding suite for Go developers, providing AST-aware code navigation, compiler-verified editing, automated test and coverage analytics, and mutation testing.

GoDoctor is delivered across two primary surfaces powered by embedded Agent Skills:

1. **MCP Server**: Exposes 6 compiler-verified tools (`smart_build`, `smart_test`, `smart_edit`, `read_docs`, `selene`, `test_query`) over Model Context Protocol (stdio & streamable HTTP).
2. **Headless CLI**: Provides direct subshell invocation (`edit`, `build`, `test`, `docs`, `selene`, `tq`) via `godoctor call`.
3. **Embedded Agent Skills**: Bundled operational guides (`@godoctor`, `@selene`, `@testquery`) unpacked directly into agent workspaces.

---

## Installation

### Option A: One-Line Installer (`install.sh`)

Downloads the latest prebuilt release binary (via GoReleaser) and initializes MCP & Skills:

```bash
# Global install (Default: ~/.gemini/config)
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash

# Workspace-scoped install (.agents/)
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- -w
```

### Option B: Go Toolchain (`go install` + `godoctor install`)

```bash
# 1. Install binary
go install github.com/danicat/godoctor/cmd/godoctor@latest

# 2. Configure surfaces (registers MCP server and unpacks embedded skills)
godoctor install
```

#### Granular Surface Management Flags:

```bash
# Configure MCP server only
godoctor install --mcp

# Unpack embedded Agent Skills only
godoctor install --skills

# Configure in current workspace scope (.agents/)
godoctor install -w

# Remove configuration (Global or Workspace)
godoctor uninstall
godoctor uninstall -w
```

#### Headless Agent Tool Invocation (`godoctor call`)

For agents operating in subshells or environments without native MCP integration, all tools can be invoked via JSON payloads using the `call` subcommand:

```bash
# Print help
godoctor

# List all intelligence tools
godoctor list

# Invoke tools via CLI (edit, build, test, docs, selene, tq)
godoctor call edit '{"filename": "/path/to/file.go", "old_content": "...", "new_content": "..."}'
godoctor call build '{"dir": "/path/to/project"}'
godoctor call test '{"dir": "/path/to/project", "level": "fast"}'
godoctor call docs '{"import_path": "net/http", "symbol_name": "Get"}'
godoctor call selene '{"dir": "/path/to/project"}'
godoctor call tq '{"dir": "/path/to/project", "query": "SELECT test, elapsed FROM all_tests WHERE action='\''fail'\''"}'
```

---

## Available MCP Tools

| Tool | Summary |
| :--- | :--- |
| [`smart_edit`](#smart_edit) | Single-file compiler-verified coordinate editor with formatting, `go vet` verification, rollback protection, and Levenshtein typo suggestions. |
| [`smart_build`](#smart_build) | 4-phase hygiene and build pipeline: `go mod tidy` -> `modernize` -> `gofmt` -> `deadcode` -> `go build` -> `go test` + coverage -> linter. |
| [`smart_test`](#smart_test) | Multi-tier test and benchmark engine (`fast`, `basic`, `benchmark`, `complete`) with SQLite (`testquery.db`) sync. |
| [`test_query`](#test_query) | SQL analytics engine executing queries against statement coverage and test run history in `testquery.db`. |
| [`selene`](#selene) | Selene-powered AST mutation testing evaluating unit test quality by introducing code mutations. |
| [`read_docs`](#read_docs) | AST documentation reader with ephemeral remote package downloading, fuzzy matching, parent fallback, and return type inspection. |

### Tool Breakdown & Real Behaviors

#### `smart_edit`
- **Parameters**: `filename` (string, required absolute path — relative paths are rejected), `new_content` (string, required), `old_content` (string, optional), `start_line` (int, optional), `end_line` (int, optional), `threshold` (float64, optional, default `0.95`), `append` (bool, optional).
- **Behavior**:
  - Locates code blocks using 4-gram anchor seeding and Levenshtein similarity within the specified line window.
  - Aborts before disk writes if match confidence is below `threshold` (default `0.95`), returning the best match candidate with line coordinates.
  - Formats with `imports.Process` (validates syntax and cleans up `import` declarations).
  - Commits atomically to disk and executes `go vet ./...` across the workspace.
  - If `go vet` fails, immediately rolls back disk changes (restoring previous state or deleting newly created files), parses compiler errors, extracts AST package symbols, and returns Levenshtein-based suggestions for misspelled identifiers.

#### `smart_build`
- **Parameters**: `dir` (string, required absolute path — relative paths are rejected), `packages` (string, optional, default `./...`), `output` (string, optional binary target path passed as `-o`).
- **Behavior**:
  - **Phase 1 (Auto-Fix & Modernize)**: Runs `go mod tidy`, `modernize -fix` (handling exit status 3 as auto-fix success), `gofmt -w .`, and `deadcode` (reporting unreachable functions as warnings).
  - **Phase 2 (Build)**: Runs `go build` (with `-o <output>` if specified). If compilation fails, inspects missing imports or undefined symbols to suggest `read_docs` lookups.
  - **Phase 3 (Test & Coverage)**: Runs `go test -v -coverprofile=coverage.out`, parses total statement coverage via `go tool cover -func`, and outputs package breakdown.
  - **Phase 4 (Linter)**: Detects `.golangci.yml/yaml/toml/json` and invokes `golangci-lint` (matching config version v1 or v2), or falls back to `go vet`.

#### `smart_test`
- **Parameters**: `dir` (string, required absolute path — relative paths are rejected), `packages` (string, optional, default `./...`), `level` (string, optional: `fast`, `basic` [default], `benchmark`, `complete`), `run` (string, optional regex filter).
- **Behavior**:
  - **`fast`**: Executes unit tests without coverage profiling or SQLite syncing.
  - **`basic` (Default)**: Runs tests with coverage profiling (`-coverprofile`), formats failure traces, computes total/package coverage, and builds/updates SQLite `testquery.db` via `testquery build`.
  - **`benchmark`**: Runs `go test -bench=<run|'.'> -benchmem -run=NONE` and formats outputs into a markdown table (`Benchmark`, `Iterations`, `Time / Op`, `Memory / Op`, `Allocs / Op`).
  - **`complete`**: Runs `basic` tests and coverage; if successful, immediately executes Selene AST mutation testing to check for surviving mutants.

#### `test_query`
- **Parameters**: `dir` (string, required absolute path — relative paths are rejected), `query` (string, required), `pkg` (string, optional, default `./...`), `rebuild` (bool, optional).
- **Behavior**:
  - Auto-initializes `testquery.db` if missing or if `rebuild=true`.
  - Executes SQL queries against:
    - `all_tests` (`time`, `action`, `package`, `test`, `elapsed`, `output`)
    - `all_coverage` (`package`, `file`, `start_line`, `start_col`, `end_line`, `end_col`, `stmt_num`, `count`, `function_name`)
    - `test_coverage` (`test_name`, `package`, `file`, `start_line`, `start_col`, `end_line`, `end_col`, `stmt_num`, `count`, `function_name`)
    - `all_code` (`package`, `file`, `line_number`, `content`)
    - `metadata` (`key`, `value`)
  - Returns structured ASCII / markdown SQL table output.

#### `selene`
- **Parameters**: `dir` (string, required absolute path — relative paths are rejected).
- **Behavior**:
  - Runs Selene AST mutation testing (`selene ./...` or `go run github.com/danicat/selene/cmd/selene@latest ./...`) in target directory.
  - Tests whether the unit test suite catches comparison shifts, boolean inversions, arithmetic swaps, and modified return values.
  - Returns `IsError: true` with surviving mutant source coordinates when defects slip past assertions.

#### `read_docs`
- **Parameters**: `import_path` (string, required), `symbol_name` (string, optional), `format` (string, optional: `markdown` [default] or `json`).
- **Behavior**:
  - Resolves local packages via `go list`. If not found, spins up an ephemeral temp module, downloads the package via `go get`, extracts AST documentation, and cleans up.
  - If download fails, provides fuzzy suggestions (`Did you mean: ...?`) from `std` and local packages.
  - Walks parent directories if subpackages are missing, returning parent module docs.
  - When inspecting functions, automatically bundles the return type's struct/interface definition.
  - Extracts runnable examples with output and performs fuzzy symbol matching on misspelled names.

---

## Specialized Agent Skills

- **`@godoctor`**: Full guide for GoDoctor CLI subcommands (`call`, `list`, `mcp`) and operational tips ([`skills/godoctor/SKILL.md`](skills/godoctor/SKILL.md)).
- **`@selene`**: Guide for interpreting mutation scores, boundary mutants, and assertion gaps ([`skills/selene/SKILL.md`](skills/selene/SKILL.md)).
- **`@testquery`**: SQLite schema reference and analytical SQL recipes for `testquery.db` ([`skills/testquery/SKILL.md`](skills/testquery/SKILL.md)).

---

## Developer Instructions

### Local Development

Build binary locally:
```bash
make build
```

Run test suite:
```bash
make test
```

### Releasing

Releasing a new version is automated via GoReleaser:
```bash
make bump-version VERSION=0.2.0
```

---

## License

Apache-2.0

