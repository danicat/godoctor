# `godoctor`

`godoctor` gives coding agents and Go engineers safe code edits, build checks, test queries, and mutation tests. You can run it as a Model Context Protocol (MCP) server, call its tools from your shell, or install the bundled `@godoctor` agent skill.

## Quickstart

Download the release build, add the MCP server, and unpack the `@godoctor` skill:

```bash
# Global install (default: ~/.gemini/config)
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash

# Workspace install (.agents/)
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- -w
```

Or build from source with `go install`:

```bash
go install github.com/danicat/godoctor/cmd/godoctor@latest
godoctor install
```

Verify that your Go tools and helper binaries are ready:

```bash
godoctor check
```

## Tools

`godoctor` exposes six tools through both the MCP server and the `godoctor call` command. All file and folder paths (`filename` and `dir`) must be full paths starting from the root directory.

| MCP tool | CLI command | What it does |
| :--- | :--- | :--- |
| `smart_edit` | `godoctor call edit` | Matches code blocks with fuzzy search, formats imports, and runs `go vet ./...`. If the check fails, it rolls back the edit and suggests close symbol names. |
| `smart_build` | `godoctor call build` | Runs `go mod tidy`, `modernize`, `gofmt`, `go build`, unit tests with coverage, `golangci-lint`, and `deadcode`. |
| `smart_test` | `godoctor call test` | Runs Go tests and benchmarks across four levels (`fast`, `basic`, `benchmark`, and `complete`) and logs results to `.godoctor/testquery.db`. |
| `test_query` | `godoctor call tq` | Runs SQL queries against test history and line coverage stored in `.godoctor/testquery.db`. |
| `selene` | `godoctor call selene` | Runs [Selene](https://github.com/danicat/selene) Abstract Syntax Tree (AST) mutation tests to find weak or missing test checks. |
| `read_docs` | `godoctor call docs` | Reads Go package and symbol docs, bundles return type structs, and fetches remote modules in a temporary folder when needed. |

### Test levels (`smart_test`)

Pick a test level based on your loop speed:

- **`fast`**: Runs `go test -v` without coverage tracking or SQLite indexing. Best for quick edits.
- **`basic`** (default): Runs unit tests, measures statement coverage, and saves test data to `.godoctor/testquery.db`.
- **`benchmark`**: Runs Go benchmarks (`-benchmem -run=NONE`) and prints a Markdown table of speed and memory use.
- **`complete`**: Runs `basic` tests and coverage, then runs `selene` mutation tests across your packages.

### Shell usage (`godoctor call`)

When working in a terminal or a shell agent without MCP, pass JSON arguments to `godoctor call`:

```bash
# List all tools
godoctor list

# Edit a file with go vet rollback protection
godoctor call edit '{"filename": "/path/to/main.go", "old_content": "fmt.Println(\"old\")", "new_content": "fmt.Println(\"new\")"}'

# Build, lint, and test a project
godoctor call build '{"dir": "/path/to/project"}'

# Run fast unit tests
godoctor call test '{"dir": "/path/to/project", "level": "fast"}'

# Read package or symbol docs
godoctor call docs '{"import_path": "net/http", "symbol_name": "Client"}'

# Run AST mutation tests
godoctor call selene '{"dir": "/path/to/project"}'

# Query failed tests with SQL
godoctor call tq '{"dir": "/path/to/project", "query": "SELECT package, test, elapsed FROM all_tests WHERE action = '\''fail'\''"}'
```

## What `godoctor install` does

By default (with no flags or `--all`), `godoctor install` runs four steps:

1. **Picks the target folder**: Uses `$GEMINI_CONFIG_DIR` or `~/.gemini/config` for global installs, or `./.agents` when you pass `-w` (`--workspace`). You can override these paths with `--config` (`-c`) and `--skills-dir` (`-s`).
2. **Registers the MCP server**: Adds a `"godoctor"` entry (`godoctor mcp`) under `"mcpServers"` in `mcp_config.json`. If `mcp_config.json` has broken JSON, it saves a backup to `mcp_config.json.bak` before writing a clean file.
3. **Unpacks the `@godoctor` skill**: Writes the embedded skill and reference guides into `<target>/skills/godoctor/`. If you edited any skill files on disk, it skips them unless you pass `--force` (`-f`):
   - **[`skills/godoctor/SKILL.md`](skills/godoctor/SKILL.md)**: Go style rules, flat package design, and tool workflows.
   - **[`skills/godoctor/references/selene.md`](skills/godoctor/references/selene.md)**: Guide to Selene mutation rules, mutant states (`KILLED`, `SURVIVED`, `UNCOVERED`), and fixing test gaps.
   - **[`skills/godoctor/references/testquery.md`](skills/godoctor/references/testquery.md)**: SQLite schema (`all_tests`, `all_coverage`, `test_coverage`, `all_code`) and SQL recipes for `.godoctor/testquery.db`.
4. **Cleans up old files**: On global installs, it deletes old `plugins/godoctor` and `agents/godoctor.md` paths from earlier releases.

You can also run each step on its own or remove the setup:

```bash
godoctor install --mcp       # Register MCP server only
godoctor install --skills    # Unpack @godoctor skill only
godoctor install -w          # Install into current workspace (./.agents/)
godoctor install -f          # Overwrite modified skill files on disk
godoctor uninstall           # Remove global MCP entry and skill files
godoctor uninstall -w        # Remove workspace MCP entry and skill files
```

## Configuration

`godoctor` works out of the box with sensible defaults. To customize timeouts, linter rules, or helper binary paths, create a `.godoctor.yaml` file in your project root:

```bash
# Create a full .godoctor.yaml config
godoctor init

# Create a short .godoctor.yaml config
godoctor init --minimal
```

`godoctor` runs helper tools like `golangci-lint`, `modernize`, `deadcode`, `selene`, and `testquery` straight from your `$PATH`. It never falls back to slow `go run` calls. Use `godoctor check` to inspect tool versions, or pass `--json` for script output.

## Local development

Build the binary and run the test suite from source:

```bash
make build
make test
```

## License

Apache-2.0
