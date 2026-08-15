# GoDoctor MCP Server

GoDoctor exposes 10 Go-compiler-verified development tools via the Model Context Protocol (MCP) over stdio.

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
      "args": []
    }
  }
}
```

## Available Tools

| Tool | Description |
| :--- | :--- |
| `smart_read` | AST-aware Go file reader with automatic `<types>` struct/interface schema annotations. |
| `smart_edit` | Single-file compiler-verified editor (formats with `gofmt`/`goimports` and verifies `go vet` before saving). |
| `smart_multi_edit` | Atomic multi-file editor that validates type safety across all modified files simultaneously. |
| `smart_build` | Full build and hygiene pipeline: `go mod tidy` -> formatting -> `go vet` -> tests -> linter -> dead code. |
| `smart_test` | Test execution engine syncing results and code coverage to SQLite (`testquery.db`). |
| `test_query` | SQL query engine for test results and coverage analytics. |
| `mutation_test` | Selene-powered mutation testing for evaluating unit test assertion quality. |
| `list_files` | Project directory tree explorer. |
| `add_dependencies` | Dependency installer (`go get`) with automated `go.mod` / `go.sum` updates. |
| `read_docs` | Standard library and third-party package documentation viewer (`go doc`). |
