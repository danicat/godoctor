# RFC-0001: Centralized Configuration (.godoctor.yaml) & Tool Version Management

- Status: Draft / Ready for Review
- Date: 2026-08-16
- Author(s): Daniela Petruzalek, Antigravity AI
- Deciders/Reviewers: Daniela Petruzalek
- ADR Reference: TBD

---

## 1. Executive Summary

This proposal introduces a centralized configuration file (`.godoctor.yaml`) powered by **Cobra** and **Viper** to govern GoDoctor workspace settings, utility versions, execution paths, and feature flags. It establishes an active **Version Checking & Upgrade Recommendation Engine** across all workspace utilities (`golangci-lint`, `modernize`, `deadcode`, `selene`, `testquery`), providing non-blocking, actionable upgrade recommendations when local tools lag behind recommended releases.

---

## 2. Context and Problem Statement

Currently, GoDoctor manages its tool parameters and fallback versions via hardcoded constants spread across individual packages (`smart_build`, `smart_test`, `selene`, `test_query`). While effective, this creates several challenges:
1. **Fragmented Tool Configuration**: Users cannot customize utility paths, recommended versions, linter config files, or feature flags across teams.
2. **Silent Version Drift**: When developers or CI environments run outdated versions of tools (e.g. `golangci-lint v1.64` instead of `v2.12.2`), GoDoctor cannot currently warn them or provide quick upgrade commands.
3. **CLI Extensibility**: The custom hand-rolled CLI dispatcher in `internal/cli` is functional, but lacks standard shell completions, environment variable binding (`GODOCTOR_*`), and unified configuration binding that Cobra + Viper provide out of the box.

---

## 3. Proposed Solution

### 3.1 Configuration Schema (`.godoctor.yaml`)

A standardized YAML configuration placed at the root of a Go repository:

```yaml
version: "1"

# External utility version tracking and execution paths
tools:
  golangci_lint:
    recommended_version: "v2.12.2"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
    config: ".golangci.yml"
  modernize:
    recommended_version: "latest"
    pkg: "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest"
  deadcode:
    recommended_version: "latest"
    pkg: "golang.org/x/tools/cmd/deadcode@latest"
  selene:
    recommended_version: "latest"
    pkg: "github.com/danicat/selene@latest"
    timeout: "60s"
  testquery:
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: "testquery.db"

# Global feature flags
features:
  autofix: true
  deadcode_check: true
  testquery_sync: true
  version_check_hints: true
```

### 3.2 Architectural Packages

```text
internal/
  ├── config/             # Typed Go structs, Viper loader, defaults, .godoctor.yaml finder
  │     ├── config.go
  │     └── config_test.go
  ├── versioncheck/       # Tool version extraction, semver comparison, upgrade hints
  │     ├── checker.go
  │     └── checker_test.go
  ├── cli/                # Cobra root command and subcommands (call, mcp, install, init, check)
  │     ├── root.go
  │     ├── call.go
  │     ├── check.go
  │     └── init.go
```

### 3.3 Version Checking & Upgrade Recommendation Engine

When `smart_build`, `smart_test`, or `godoctor check` runs:
1. **Detection**:
   - Check if tool is installed locally (`LookPath`).
   - Execute `<tool> --version` (or query binary build metadata via `debug.ReadBuildInfo`).
   - Extract semantic version string.
2. **Comparison**:
   - Compare installed version against `tools.<tool>.recommended_version` from `.godoctor.yaml` (or built-in defaults).
3. **Actionable Hint Output**:
   - If installed version is older than recommended, emit a non-blocking diagnostic hint:
     ```text
     💡 Upgrade Hint: Installed golangci-lint is v1.64.0 (recommended: v2.12.2).
        Run 'go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2' to upgrade.
     ```
   - Tool execution continues smoothly without halting the build or test pipeline.

### 3.4 CLI Subcommands with Cobra

- `godoctor call <tool> [json-payload]` (preserves strict JSON input, rejects ad-hoc flags).
- `godoctor mcp` (starts MCP stdio server).
- `godoctor init` (generates a documented `.godoctor.yaml` in the current repo).
- `godoctor check` (inspects health and versions of all 5 external tools, outputting status table and upgrade commands).
- `godoctor install` / `godoctor uninstall` (manages Antigravity / Claude / Cursor configurations).

---

## 4. Dependencies & Version Constraints

Following the `latest-version` verification protocol:
- `github.com/spf13/cobra` $\rightarrow$ `v1.10.2`
- `github.com/spf13/viper` $\rightarrow$ `v1.21.0`

---

## 5. Migration Strategy

1. **Zero Breaking Changes**: Projects without a `.godoctor.yaml` continue to work seamlessly using built-in defaults.
2. **CLI Parity**: `godoctor call <tool> '<json>'` arguments and MCP schemas remain 100% backward compatible.
3. **Incremental Rollout**:
   - Phase 1: Implement `internal/config` with Viper and typed schema.
   - Phase 2: Implement `internal/versioncheck` with tests for version parsing and comparison.
   - Phase 3: Wire version checking into `smart_build` and `smart_test`.
   - Phase 4: Refactor `internal/cli` to Cobra, adding `init` and `check` commands.
