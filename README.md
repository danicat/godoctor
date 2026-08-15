# GoDoctor

GoDoctor is a specialized agentic coding suite for Go developers, providing AST-aware code navigation, compiler-verified editing, automated test and coverage analytics, and mutation testing.

GoDoctor is built as three independent, modular components that can be installed together or individually:

1. **GoDoctor MCP Server**: Exposes 10 compiler-verified tools over Model Context Protocol (stdio).
2. **GoDoctor Agent**: A specialized named agent (`@godoctor`) for Antigravity supporting both interactive main-agent sessions and autonomous subagent delegation.
3. **GoDoctor Skills**: Procedural Agent Skills (`@selene`, `@testquery`) discoverable by any agent in the workspace.

---

## Installation

### Automated Installation (`install.sh`)

Install all components globally (Default: `~/.gemini/config`):

```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash
```

Or install directly to the current workspace (`.agents/`):

```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- -w
```

#### Modular Installation

You can install any combination of components using flags:

```bash
# Install MCP server only
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- --mcp

# Install Agent definition only
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- --agent

# Install Skills only
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- --skills

# Install Agent and Skills to workspace scope (.agents/)
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | bash -s -- --agent --skills -w
```

---

### Manual / Independent Component Installation

Each component uses standard ecosystem tooling and can be installed independently without running `install.sh`:

#### 1. GoDoctor MCP Server (`go install`)

Install the server binary via the Go toolchain:

```bash
go install github.com/danicat/godoctor/cmd/godoctor@latest
```

Ensure `$(go env GOPATH)/bin` is in your `$PATH`.

Register the server in your MCP client configuration (`~/.gemini/config/mcp_config.json`, `~/.claude.json`, or `.cursor/mcp.json`):

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

See [mcp/README.md](mcp/README.md) for full MCP server details.

#### 2. GoDoctor Named Agent

Download [`agent/godoctor.md`](agent/godoctor.md) into your Antigravity agents directory:

- **Global**: `~/.gemini/config/agents/godoctor.md`
- **Workspace**: `.agents/agents/godoctor.md`

```bash
# Global
mkdir -p ~/.gemini/config/agents
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/agent/godoctor.md -o ~/.gemini/config/agents/godoctor.md

# Workspace
mkdir -p .agents/agents
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/agent/godoctor.md -o .agents/agents/godoctor.md
```

#### 3. GoDoctor Skills (`npx skills`)

Install GoDoctor skills using the standard `skills` CLI:

```bash
# Global
npx skills add danicat/godoctor -g -y

# Workspace
npx skills add danicat/godoctor -y
```

---

## Uninstallation

### Automated Uninstallation (`uninstall.sh`)

Uninstall all components globally:

```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/uninstall.sh | bash
```

Or uninstall from the current workspace (`.agents/`):

```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/uninstall.sh | bash -s -- -w
```

Or from local clone:

```bash
./uninstall.sh
```

#### Modular Uninstallation

Uninstall specific components individually using component flags:

```bash
# Uninstall MCP server only (removes binary and unregisters from mcp_config.json)
./uninstall.sh --mcp

# Uninstall Agent definition only (@godoctor)
./uninstall.sh --agent

# Uninstall Skills only (@selene, @testquery)
./uninstall.sh --skills

# Uninstall Agent and Skills from workspace scope (.agents/)
./uninstall.sh --agent --skills -w
```

---

## Available MCP Tools

| Tool | Description |
| :--- | :--- |
| `smart_read` | AST-aware Go file reader with automatic `<types>` struct/interface schema annotations. |
| `smart_edit` | Single-file compiler-verified editor (formats with `gofmt`/`goimports` and verifies `go vet` before saving). |
| `smart_multi_edit` | Atomic multi-file editor that validates type safety across all modified files simultaneously. |
| `smart_build` | Full build and hygiene pipeline: `go mod tidy` -> formatting -> `go vet` -> tests -> linter -> dead code. |
| `smart_test` | Test execution engine syncing results and code coverage to SQLite (`testquery.db`). |
| `test_query` | SQL query engine for test results and coverage analytics. |
| `mutation_test` | Selene-powered mutation testing for evaluating unit test assertion quality. |
| `list_files` | VCS-aware workspace file explorer. |
| `add_dependencies` | Dependency installer (`go get`) with automated `go.mod` / `go.sum` updates. |
| `read_docs` | Standard library and third-party package documentation viewer (`go doc`). |

---

## Specialized Agent Skills

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

