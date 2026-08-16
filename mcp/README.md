# GoDoctor MCP Server

GoDoctor exposes 6 Go development tools via the Model Context Protocol (MCP) over stdio.

## Installation

```bash
go install github.com/danicat/godoctor/cmd/godoctor@latest
```

Ensure `$(go env GOPATH)/bin` is in your `$PATH`.

## Client Configuration

Add GoDoctor to your client configuration file (`~/.gemini/config/mcp_config.json`, `~/.claude.json`, or Cursor `mcp.json`):

```json
{
  "mcpServers": {
    "godoctor": {
      "command": "godoctor",
      "args": ["mcp"]
    }
  }
}
```

## Available Tools

| Tool | Summary |
| :--- | :--- |
| [`smart_edit`](#smart_edit) | Single-file compiler-verified coordinate editor with automatic formatting, `go vet` verification, rollback protection, and Levenshtein typo suggestions. |
| [`smart_build`](#smart_build) | 4-phase hygiene and build pipeline: `go mod tidy` -> `modernize` -> `gofmt` -> `deadcode` -> `go build` -> `go test` + coverage -> linter. |
| [`smart_test`](#smart_test) | Multi-tier test and benchmark engine (`fast`, `basic`, `benchmark`, `complete`) with SQLite (`testquery.db`) sync. |
| [`test_query`](#test_query) | SQL analytics engine executing queries against statement coverage and test run history in `testquery.db`. |
| [`selene`](#selene) | Selene-powered AST mutation testing evaluating unit test quality by introducing code mutations. |
| [`read_docs`](#read_docs) | AST documentation reader with ephemeral remote package downloading, fuzzy matching, parent fallback, and return type inspection. |

---

### `smart_edit`

Coordinates single-file code edits protected by compiler verification and rollback guarantees.

#### Parameters
- `filename` (*string, required*): Absolute path to the Go file to edit (relative paths are rejected).
- `old_content` (*string, optional*): The exact block of code to find. Whitespace is normalized during matching. If empty or `append=true`, appends `new_content`.
- `new_content` (*string, required*): Replacement or new code.
- `start_line` (*integer, optional*): Restricts fuzzy search window to line numbers $\ge$ `start_line`.
- `end_line` (*integer, optional*): Restricts fuzzy search window to line numbers $\le$ `end_line`.
- `threshold` (*number, optional, default: `0.95`*): Fuzzy match similarity threshold between `0.0` and `1.0`.
- `append` (*boolean, optional*): When `true`, appends `new_content` to the end of the file.

#### Real Runtime Behavior
1. **In-Memory Backup**: Reads the target file into memory. If the file does not exist, marks it for creation.
2. **Fuzzy Search & Anchoring**: Uses 4-gram anchor seeding for long blocks and character-level Levenshtein matching for short blocks within the line range window.
3. **Threshold Gate**: If similarity falls below `threshold` (default `0.95`), the edit aborts without modifying the disk, returning the closest match found with line numbers and score.
4. **Syntax & Import Processing**: Executes `golang.org/x/tools/imports.Process`. If syntax is invalid, captures the error line snippet and aborts before disk writes.
5. **Disk Commit**: Atomically writes changes to disk.
6. **Compiler Gate (`go vet ./...`)**: Runs `go vet ./...` from the workspace root across all Go files (ignoring `.git`, `skills`, `agents`, `hooks`).
7. **Rollback & Diagnostic Suggestions**: If `go vet` fails, restores the original file content (or deletes newly created files), parses compiler errors, inspects package AST symbols, and generates `💡 Suggestions: Did you mean 'X' instead of 'Y'?` for symbols within Levenshtein distance $\le 4$.

---

### `smart_build`

Comprehensive 4-phase workspace health, modernization, test, and linting pipeline.

#### Parameters
- `dir` (*string, required*): Workspace absolute directory path (relative paths are rejected).
- `packages` (*string, optional, default: `./...`*): Package pattern to analyze and build.

#### Real Runtime Pipeline
1. **Phase 1: Auto-Fix & Modernize**:
   - Runs `go mod tidy` (reports `SUCCESS` or `FAILED`).
   - Runs Go Modernizer (`modernize -fix` or `go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix`). Handles exit code 3 as successful auto-fixes.
   - Runs `gofmt -w .` across the workspace.
   - Runs Deadcode Analysis (`deadcode <pkgs>` or `go run golang.org/x/tools/cmd/deadcode@latest`). Unreachable functions are printed as warnings without halting the build.
2. **Phase 2: Compilation**:
   - Runs `go build <pkgs>`.
   - On build failure, halts pipeline, inspects compiler errors for missing packages or undefined identifiers, and suggests `read_docs` lookups.
3. **Phase 3: Tests & Coverage**:
   - Runs `go test -v -coverprofile=coverage.out <pkgs>`.
   - On failure, halts pipeline and outputs formatted failure blocks.
   - On success, parses total project statement coverage via `go tool cover -func` and per-package coverage from test output.
4. **Phase 4: Linter**:
   - Detects configuration files (`.golangci.yml`, `.golangci.yaml`, `.golangci.toml`, `.golangci.json`).
   - If no config exists, runs `go vet <pkgs>`.
   - If config exists, runs local `golangci-lint` or downloads matching major version (`v1.64.5` or `v2.12.2`) via `go run`.

---

### `smart_test`

Multi-depth test and benchmark runner with automated SQLite test history indexing.

#### Parameters
- `dir` (*string, required*): Workspace absolute directory path (relative paths are rejected).
- `packages` (*string, optional, default: `./...`*): Target package pattern.
- `level` (*string, optional, default: `"basic"`*): Execution mode:
  - `"fast"`: Unit tests only (`go test -v`). No coverage profiling and no SQLite sync.
  - `"basic"`: Unit tests with coverage (`go test -v -coverprofile`) and automatic `testquery.db` SQLite indexing.
  - `"benchmark"`: Benchmark execution (`go test -bench -benchmem -run=NONE`) parsed into markdown performance tables (`| Benchmark | Iterations | Time / Op | Memory / Op | Allocs / Op |`).
  - `"complete"`: Runs `"basic"` tests + coverage, followed by Selene AST mutation testing if tests pass.
- `run` (*string, optional*): Regex filter pattern passed to `-run` (tests) or `-bench` (benchmarks).

---

### `test_query`

SQL analytics engine querying statement-level coverage and test results stored in `testquery.db`.

#### Parameters
- `dir` (*string, required*): Absolute directory path containing `testquery.db` (relative paths are rejected).
- `query` (*string, required*): SQL query string.
- `pkg` (*string, optional, default: `./...`*): Package pattern to analyze during database build.
- `rebuild` (*boolean, optional, default: `false`*): Forces rebuilding `testquery.db` before running query.

#### Database Schema
- `all_tests`: `time`, `action` (`pass`/`fail`/`skip`/`output`), `package`, `test`, `elapsed`, `output`
- `all_coverage`: `package`, `file`, `start_line`, `start_col`, `end_line`, `end_col`, `stmt_num`, `count`, `function_name`
- `test_coverage`: `test_name`, `package`, `file`, `start_line`, `start_col`, `end_line`, `end_col`, `stmt_num`, `count`, `function_name`
- `all_code`: `package`, `file`, `line_number`, `content`
- `metadata`: `key`, `value`

---

### `selene`

AST mutation test runner measuring test suite assertion strength via Selene.

#### Parameters
- `dir` (*string, required*): Workspace absolute directory path (relative paths are rejected).

#### Real Runtime Behavior
- Runs `selene ./...` (or `go run github.com/danicat/selene/cmd/selene@latest ./...`) in `dir`.
- Mutates AST nodes (swapping comparison boundaries `>=` to `>`, inverting conditionals, changing return values, swapping arithmetic operators).
- Returns `✅ All mutations were caught by tests.` if all mutations fail tests.
- If mutants survive, returns `IsError: true` with a detailed list of surviving mutants, file coordinates, and mutated syntax.

---

### `read_docs`

Authoritative documentation viewer and symbol extractor for standard library and third-party Go packages.

#### Parameters
- `import_path` (*string, required*): Package import path (e.g. `"net/http"`, `"github.com/google/uuid"`).
- `symbol_name` (*string, optional*): Specific function, type, method, variable, or constant to inspect.
- `format` (*string, optional, default: `"markdown"`*): Output format: `"markdown"` or `"json"`.

#### Real Runtime Behavior
- **Local Resolution**: Resolves package directory via `go list -f '{{.Dir}}'`.
- **Ephemeral Remote Fetch**: If not available locally, creates an ephemeral module in `os.MkdirTemp`, runs `go get <import_path>`, parses AST documentation, and cleans up temp files.
- **Fuzzy Suggestion Fallback**: If package download fails, queries `go list std`, `go list all`, and parent subpackages to suggest closest package names.
- **Parent Module Walk**: If a subpackage is not found, walks up path segments to display parent module documentation with a banner notice.
- **Return Type Declaration Bundling**: When inspecting a function, automatically resolves and attaches the definition of its returned struct or interface.
- **Executable Examples**: Extracts runnable code examples and expected test outputs.
- **Fuzzy Symbol Matching**: If a symbol name has a typo, runs Levenshtein distance checks across all package exports and suggests closest matches.

