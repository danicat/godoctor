---
name: godoctor
description: Run GoDoctor intelligence tools in CLI mode (edit, build, test, docs, selene, tq) or configure the godoctor MCP server.
---

# GoDoctor CLI & MCP operational guide

GoDoctor provides AST-aware Go developer tooling available both as a standalone Command Line Interface (CLI) and as a Model Context Protocol (MCP) server.

## Tool Names by Mode

| Tool Category | CLI Tool Name (`godoctor call`) | MCP Tool Name |
| :--- | :--- | :--- |
| Code Editor | `edit` | `smart_edit` |
| Build & Lint Pipeline | `build` | `smart_build` |
| Test Runner | `test` | `smart_test` |
| Documentation | `docs` | `read_docs` |
| Mutation Testing | `selene` | `selene` |
| SQL Analytics | `tq` | `test_query` |

---

## CLI Subcommands

### 1. Main Help
Running `godoctor` without arguments or with `--help` prints the full help summary:
```bash
godoctor
# or
godoctor help
```

### 2. Surface Management (`godoctor install` & `godoctor uninstall`)
Registers/unregisters the MCP server in `mcp_config.json` and unpacks/removes embedded agent skills (`@godoctor`, `@selene`, `@testquery`):
```bash
# Configure MCP and skills globally (default: ~/.gemini/config)
godoctor install

# Configure in workspace scope (.agents/)
godoctor install -w

# Modular configuration
godoctor install --mcp        # MCP server registration only
godoctor install --skills     # Skills unpacking only

# Remove configuration
godoctor uninstall
godoctor uninstall -w
```

### 3. Listing Available Tools
List all available intelligence tools, descriptions, parameter usages, and aliases:
```bash
godoctor list
```

### 4. Invoking Tools directly (`godoctor call`)

Tools can be invoked from the command line using `godoctor call <tool-name> [arguments]`. 

> [!IMPORTANT]
> All directory and file path arguments MUST be absolute paths. Relative paths (such as `.`, `./...`, or `main.go`) are strictly rejected.

#### Input Argument Formats:
The `call` subcommand supports three flexible argument formats:
1. **CLI Flags / Key-Value Pairs**:
   ```bash
   godoctor call selene --dir=/absolute/path/to/project
   godoctor call tq --dir=/absolute/path/to/project --query="SELECT * FROM all_tests WHERE action='fail'"
   godoctor call docs --import_path=net/http --symbol_name=Get
   godoctor call build --dir=/absolute/path/to/project
   godoctor call test --dir=/absolute/path/to/project --level=fast
   ```
2. **JSON Argument String**:
   ```bash
   godoctor call selene '{"dir": "/absolute/path/to/project"}'
   godoctor call edit '{"filename": "/absolute/path/to/main.go", "old_content": "fmt.Println(\"old\")", "new_content": "fmt.Println(\"new\")"}'
   ```
3. **Piped JSON via Stdin**:
   ```bash
   echo '{"dir": "/absolute/path/to/project", "level": "fast"}' | godoctor call test
   ```

---

## CLI Tool Reference (Standard JSON Format)

All `godoctor call` tools take a JSON arguments object as their input.

### `edit` (MCP: `smart_edit`)
Performs coordinate edits with automatic compiler gate (`go vet`), code formatting (`gofmt`/`goimports`), and automatic rollback on failure.
```bash
godoctor call edit '{"filename": "/absolute/path/main.go", "old_content": "fmt.Println(\"old\")", "new_content": "fmt.Println(\"new\")"}'
```

### `build` (MCP: `smart_build`)
Runs the 4-phase hygiene and verification pipeline: `go mod tidy` $\to$ modernize $\to$ `gofmt` $\to$ deadcode $\to$ `go build` $\to$ `go test` + coverage $\to$ linter.
```bash
godoctor call build '{"dir": "/absolute/path/to/project"}'
```

### `test` (MCP: `smart_test`)
Runs the multi-tier test suite (`fast`, `basic`, `benchmark`, `complete`) and indexes run results and statement coverage into `testquery.db`.
```bash
godoctor call test '{"dir": "/absolute/path/to/project", "level": "basic"}'
```

### `docs` (MCP: `read_docs`)
Fetches package-level documentation, function signatures, and types directly from Go source.
```bash
godoctor call docs '{"import_path": "net/http", "symbol_name": "Client"}'
```

### `selene` (MCP: `selene`)
Executes Selene AST mutation testing to measure unit test assertion strength.
```bash
godoctor call selene '{"dir": "/absolute/path/to/project"}'
```

### `tq` (MCP: `test_query`)
Runs SQL analytics queries against `testquery.db`.
```bash
godoctor call tq '{"dir": "/absolute/path/to/project", "query": "SELECT test, elapsed FROM all_tests WHERE action='\''fail'\'' ORDER BY elapsed DESC"}'
```

---

## MCP Server Mode

To run GoDoctor as a Model Context Protocol server:

### Standard I/O Mode (Default for MCP Clients)
```bash
godoctor mcp
```

### Streamable HTTP Mode
```bash
godoctor mcp -listen=:8080
```
