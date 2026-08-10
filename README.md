# GoDoctor - Specialized Agentic Coding Suite for Go

GoDoctor is a specialized and optimized suite of tools carefully engineered to elevate agentic coding in Go codebases.

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

## Features and Tools

GoDoctor provides the following tools:

* `list_files` lists files in the workspace up to a given depth.
* `smart_read` reads files including contextual type information.
* `smart_edit` handles atomic modifications across multiple files. It formats the code and automatically rolls back changes if the compiler detects a syntax error.
* `smart_build` manages module tidying, code modernization, formatting, compiling, testing, linting, and deadcode analysis.
* `project_init` initialises a module and pulls dependencies
* `add_dependency` installs Go modules and pulls their documentation.
* `read_docs` fetches documentation for packages and symbols.
* `mutation_test` runs Selene mutation tests to check test coverage quality.
* `test_query` queries test results and coverage data using SQL.

## Developer Instructions

### Local development

Whenever possible, use the current version of GoDoctor to build the next one (using `smart_build`). This will enable continuous improvement as the smart build pipeline ensures build, test, lint and modernizers are always up to date.

### Releasing

Releasing a new version is automated into a single one-liner command:

```bash
make bump-version VERSION=0.21.0
```

This single command automatically:
1. Creates the matching Git release tag (`v0.21.0`).
2. Pushes tags to GitHub (`git push origin main --tags`), triggering the automated GoReleaser CI/CD pipeline.

### Local Snapshot Testing
To test the GoReleaser build configuration locally without creating a release tag:
```bash
make snapshot
```

## License

Apache 2.0
