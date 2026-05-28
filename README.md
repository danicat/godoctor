# GoDoctor

GoDoctor is a Model Context Protocol (MCP) server and CLI extension for Go development. It provides structured tools to help coding agents navigate, edit, build, and test Go codebases safely.

## User Instructions

### Installation

#### Antigravity and Agy CLI
Install the plugin directly from GitHub:
```bash
agy plugin install https://github.com/danicat/godoctor
```

#### Gemini CLI
Install the extension from GitHub:
```bash
gemini extensions install https://github.com/danicat/godoctor
```

#### Claude Code
1. Install the binary globally:
   ```bash
   go install github.com/danicat/godoctor/cmd/godoctor@latest
   ```
2. Register the MCP server:
   ```bash
   claude mcp add --transport stdio --scope user godoctor -- godoctor
   ```
3. Append agent instructions to your project:
   ```bash
   godoctor --agents >> CLAUDE.md
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

#### Command Interception (Hooks)
When running inside the Antigravity or Gemini CLI, GoDoctor intercepts standard terminal commands (such as `go build`, `cat`, or `sed`) and raw file tools. It redirects the agent to GoDoctor's specialized tools (`smart_build`, `smart_read`, and `smart_edit`). This prevents syntax errors and conserves context window tokens.

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
* `smart_read` reads files, extracts code outlines, and appends definitions of referenced types.
* `describe_symbol` provides semantic detail for any symbol, including declaration signatures, comments, and references.

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

### Testing

Run the test suite:
```bash
make test
```

To run tests and generate a coverage report:
```bash
make test-cov
```

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

GoDoctor relies on Git tags for versioning. Build versions are dynamically injected at compile time using `git describe`.

To release a new version:

1. Update the version string in `gemini-extension.json`:
   ```bash
   make bump-version VERSION=0.16.2
   ```

2. Commit the manifest changes:
   ```bash
   git add gemini-extension.json
   git commit -m "chore: bump version to 0.16.2"
   ```

3. Create and push a new Git tag:
   ```bash
   git tag v0.16.2
   git push origin v0.16.2
   ```

The release pipeline will automatically run GoReleaser when a new tag is pushed.

To test the GoReleaser configuration locally, generate a snapshot release:
```bash
make snapshot
```

## License

Apache 2.0
