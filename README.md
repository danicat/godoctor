# GoDoctor - Specialized Agentic Coding Suite for Go

GoDoctor is a specialized and optimized suite of tools carefully engineered to elevate agentic coding in Go codebases.

## User Instructions

### Installation

Install GoDoctor using the one-line installation command:

```bash
curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh
```

You can pass options to customize the installation target or scope:

- **Antigravity 2.0 (Plugin)** (Default):
  ```bash
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target agy2      # Global: ~/.gemini/config/plugins/godoctor
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target agy2 -w   # Workspace: .agents/plugins/godoctor
  ```
- **Antigravity CLI (Plugin)**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target cli       # Global: ~/.gemini/antigravity-cli/plugins/godoctor
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target cli -w    # Workspace: .agents/plugins/godoctor
  ```
- **Other Agents (Skills Only)**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target skills    # Global: ~/.agents/skills
  curl -fsSL https://raw.githubusercontent.com/danicat/godoctor/main/install.sh | sh -s -- --target skills -w # Workspace: .agents/skills
  ```

Alternatively, if you have cloned the repository locally:

```bash
./install.sh [options]
```

## Features and Tools

GoDoctor provides the following tools:

1. `list_files`: lists files in the workspace up to a given depth.
2. `smart_read`: reads files including contextual type information.
3. `smart_edit`: handles single-file modifications with compiler verification.
4. `smart_multi_edit`: handles atomic modifications across multiple files in batch with compiler verification.
5. `smart_build`: manages module tidying, code modernization, formatting, compiling, testing, linting, and deadcode analysis.
6. `add_dependencies`: installs Go modules and pulls their documentation.
7. `read_docs`: fetches documentation for packages and symbols.
8. `mutation_test`: runs Selene mutation tests to check test coverage quality.
9. `test_query`: queries test results and coverage data using SQL.

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
