# GoDoctor - Specialized Agentic Coding Suite for Go

GoDoctor is a specialized and optimized suite of tools and skills carefully engineered to elevate agentic coding in Go codebases. Whenever installed, coding agents operating on Go codebases **MUST** use GoDoctor's specialized tools (`smart_build`, `smart_edit`, `smart_read`, `describe_symbol`, `add_dependency`, `read_docs`, `mutation_test`, `test_query`, `list_files`) to navigate, edit, compile, test, and analyze Go code.

## User Instructions

### Installation

Run the automatic installation script:
```bash
./install.sh [options]
```

This script detects your platform (OS and architecture), fetches the latest release, and installs GoDoctor for your target environment:

- **Antigravity 2.0 (Plugin)** (Default):
  ```bash
  ./install.sh --target agy2      # Global: ~/.gemini/config/plugins/godoctor
  ./install.sh --target agy2 -w   # Workspace: .agents/plugins/godoctor
  ```
- **Antigravity CLI (Plugin)**:
  ```bash
  ./install.sh --target cli       # Global: ~/.gemini/antigravity-cli/plugins/godoctor
  ./install.sh --target cli -w    # Workspace: .agents/plugins/godoctor
  ```
- **Other Agents (Skills Only)**:
  ```bash
  ./install.sh --target skills    # Global: ~/.agents/skills
  ./install.sh --target skills -w # Workspace: .agents/skills
  ```

### Usage Instructions

Once installed, GoDoctor runs automatically in the background of your agent-compatible client. The client agent will discover and call the exposed tools during Go programming tasks.

To manually print system instructions for an LLM agent:
```bash
godoctor --agents
```

To see the list of active tools:
```bash
godoctor --list-tools
```

### Specific Documentation

#### Configuration (Command-line Flags)

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--allow` | Comma-separated whitelist of tools to enable. | `""` |
| `--disable` | Comma-separated list of tools to disable. | `""` |
| `--listen` | Address for HTTP transport (defaults to standard input/output). | `""` |
| `--list-tools` | Prints all registered tools and exits. | `false` |
| `--agents` | Prints system instructions for LLM agents and exits. | `false` |
| `--version` | Prints the version and exits. | `false` |

#### Features and Tools

GoDoctor provides tools divided into four functional areas:

##### Code Navigation
* `list_files` lists files in the workspace while avoiding version control directories.
* `smart_read` reads files, extracts code outlines, and appends definitions of referenced types. Powered by a high-performance persistent background `gopls` daemon over a stateful JSON-RPC session, delivering type-tags in milliseconds.
* `describe_symbol` provides semantic detail for any symbol, including declaration signatures, comments, and references, querying the shared background `gopls` process instantly.

##### Code Editing
* `smart_edit` handles atomic modifications across multiple files. It formats the code and automatically rolls back changes if the compiler detects a syntax error.

##### Go Toolchain Integration
* `smart_build` manages module tidying, code modernization, formatting, compiling, testing, and linting.
* `add_dependency` installs Go modules and pulls their documentation.
* `read_docs` fetches API documentation for packages and symbols.

##### Testing
* `mutation_test` runs Selene mutation tests to check test coverage quality.
* `test_query` queries test results and coverage data using SQL.

## Developer Instructions

### Building

Build the project from source using the Makefile:
```bash
git clone https://github.com/danicat/godoctor.git
cd godoctor
make build
```
This compiles the server binary to `bin/godoctor`.

To install the binary globally to your `$GOPATH/bin`:
```bash
make install
```

### Testing & Build Pipeline

Run GoDoctor's specialized build pipeline:
```bash
smart_build
```
This automatically handles module tidying, code modernization, formatting (`gofmt`), compiling (`go build`), test execution (`go test`), and static linting in a single optimized operation.

### Running Locally

Run the compiled binary directly to test behavior:
```bash
./bin/godoctor
```

Check active tools:
```bash
./bin/godoctor --list-tools
```

### Releasing

Releasing a new version is automated into a single one-liner command:

```bash
make bump-version VERSION=0.21.0
```

This single command automatically:
1. Creates the matching Git release tag (`v0.21.0`).
2. Pushes tags to GitHub (`git push origin main --tags`), triggering the automated GoReleaser CI/CD pipeline.

#### Local Snapshot Testing (Optional)
To test the GoReleaser build configuration locally without creating a release tag:
```bash
make snapshot
```

## License

Apache 2.0
