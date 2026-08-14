# GoDoctor - Specialized Agentic Coding Suite for Go

GoDoctor is a specialized engineering plugin and custom agent engineered for Go codebases. It is built in full compliance with the **Agent Plugins Specification 1.0.0**, combining a dedicated custom named agent (`godoctor`), high-performance MCP tools, procedural skills (`selene`, `testquery`), and automated lifecycle hook safety guards.

---

## 1. Overview & Architecture

GoDoctor delivers an end-to-end Go engineering experience across Antigravity 2.0 and the Antigravity CLI:

```text
godoctor/
├── plugin.json                 # Agent Plugins 1.0.0 Manifest
├── mcp.json                    # MCP stdio configuration (./bin/godoctor)
├── agents/
│   └── godoctor.md             # Custom named agent (Main & Subagent symmetry)
├── hooks/
│   └── godoctor-hook.py        # PreToolUse safety interceptor
├── skills/
│   ├── selene/                 # Mutation testing procedural workflows
│   │   └── SKILL.md
│   └── testquery/              # SQLite test intelligence & analytics
│       └── SKILL.md
└── bin/
    └── godoctor                # Pre-compiled cross-platform server binary
```

---

## 2. Installation

Install GoDoctor using the one-line installation command:

```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh
```

### Installation Options

You can customize the installation target runtime and scope:

- **Antigravity 2.0 (Global)** (Default):
  ```bash
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target agy2
  ```
- **Workspace-Only Installation**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target agy2 -w
  ```
- **Antigravity CLI**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target cli
  ```

---

## 3. Running GoDoctor

GoDoctor supports **true execution symmetry**:

### As a Main Agent
Launch an interactive GoDoctor session from the CLI:
```bash
agy --agent godoctor
```
Or select **GoDoctor** directly from the agent dropdown selector in the Antigravity 2.0 Desktop UI.

### As an Autonomous Subagent
Coordinator agents can dynamically delegate Go refactoring, testing, and diagnostic tasks directly to `godoctor` via `invoke_subagent`.

---

## 4. Features & Tools

### GoDoctor MCP Tool Suite (`mcp.json`)
1. `smart_read`: Reads Go source files with AST-level type metadata and structure awareness.
2. `smart_edit`: Performs single-file code modifications verified by `gofmt`, `goimports`, and `go vet` before committing to disk.
3. `smart_multi_edit`: Performs atomic, multi-file batch code refactoring verified by compiler rules.
4. `smart_build`: Executes the workspace verification pipeline (`go mod tidy`, modernizers, formatting, `go build`, `go test`, linter, deadcode).
5. `smart_test`: Executes Go tests with coverage gap analysis and synchronizes metrics to `testquery.db`.
6. `test_query`: Queries test results and code coverage metrics using SQL.
7. `mutation_test`: Runs Selene mutation testing against Go packages to evaluate unit test effectiveness.
8. `list_files`: VCS-aware workspace file mapper.
9. `add_dependencies`: Installs Go modules, updates `go.mod`/`go.sum`, and returns documentation.
10. `read_docs`: Fetches Go documentation and function signatures for standard library or third-party packages.

### Specialized Procedural Skills
- **`selene`**: Detailed guide on mutation testing, interpreting killed vs surviving mutants, and fixing assertion gaps.
- **`testquery`**: Complete SQLite schema documentation and analytical SQL recipes for test performance and coverage gap discovery.

---

## 5. Developer Instructions

### Local Development
To build the GoDoctor binary locally:
```bash
make build
```

Run test suite:
```bash
make test
```

### Snapshot Testing
To test the cross-platform GoReleaser build locally without publishing a release tag:
```bash
make snapshot
```

### Releasing
Releasing a new version is automated via GoReleaser:
```bash
make bump-version VERSION=0.2.0
```

---

## 6. License

Apache-2.0
