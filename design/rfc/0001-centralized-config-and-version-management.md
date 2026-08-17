# RFC-0001: Centralized Configuration (.godoctor.yaml) & Tool Version Management

- **Status**: Accepted / Implemented
- **Date**: 2026-08-16
- **Author(s)**: Lead Configuration Architect, Daniela Petruzalek
- **Deciders/Reviewers**: Daniela Petruzalek
- **Finalized Specification**: [RFC-0001: Centralized Configuration System](file:///Users/petruzalek/projects/godoctor/design/RFC-0001-configuration-system.md)
- **ADR Reference**: [ADR-0001: Centralized YAML Configuration & Tool Version Management](file:///Users/petruzalek/projects/godoctor/design/adr/0001-centralized-yaml-configuration-and-version-management.md)

---

## 1. Executive Summary

This RFC establishes a unified, centralized configuration system (`.godoctor.yaml`) and an intelligent **Tool Version Checking & Upgrade Recommendation Engine** for GoDoctor. Powered by **Cobra** (`v1.10.2`) and **Viper** (`v1.21.0`), this architecture harmonizes configuration across all sub-systems (`smart_build`, `smart_test`, `smart_edit`, `selene`, `test_query`, `read_docs`, `safeshell`, and `server`), replacing fragmented constants, magic numbers, and hardcoded tool fallbacks with a type-safe, extensible foundation while guaranteeing **zero breaking changes** and seamless backward compatibility.

In accordance with architectural decisions, configuration is strictly driven by `.godoctor.yaml` and built-in defaults without environment variable bindings (`GODOCTOR_*`), ensuring deterministic behavior across AI agent environments.

---

## 2. Context & Problem Statement

A comprehensive audit across all technical domains ([`design/AUDIT_FINDINGS.md`](file:///Users/petruzalek/projects/godoctor/design/AUDIT_FINDINGS.md)) identified several critical architectural challenges:

1. **Fragmented Magic Values & Inflexible Defaults**:
   - Timeouts, package targets, formatters, and buffer sizes were hardcoded across disjoint files (e.g. `15s` HTTP write timeouts killing long builds, `0.95` fuzzy thresholds, `30s` test timeouts, fixed `testquery.db` paths).
2. **Ephemeral Fallback Latency & Silent Version Drift**:
   - When tools (`golangci-lint`, `modernize`, `deadcode`, `selene`, `testquery`) are missing from `$PATH`, GoDoctor falls back to `go run <pkg>@latest`, causing a **1.5s–4.5s latency penalty** per invocation.
   - When tools are installed, developers frequently run outdated versions without diagnostic warnings, missing modern Go 1.24+ AST rules and optimizations.
3. **Usability & Security Hazards**:
   - `safeshell.Validate` rejected valid SQL operators (`<`, `>`, `\n`, `;`) and regex patterns (`$`), breaking [`skills/testquery/SKILL.md`](file:///Users/petruzalek/projects/godoctor/skills/testquery/SKILL.md) recipes and targeted test filtering.
   - CORS origin reflection allowed untrusted localhost origins (`http://localhost.attacker.com`).
3. **CLI Dispatcher Limitations**:
   - The hand-rolled CLI dispatcher in `internal/cli` lacked structured flag binding and shell auto-completion.

---

## 3. Proposed Architecture & System Design

```text
                               ┌─────────────────────────────┐
                               │       CLI / MCP Call        │
                               └──────────────┬──────────────┘
                                              │
                                              ▼
┌───────────────────────────────────────────────────────────────────────────────────────────┐
│                                 internal/config (Viper)                                   │
│            Precedence: Explicit Payload > CLI Flags > .godoctor.yaml > Go Defaults        │
└──────────────────────┬─────────────────────────────────────────────┬──────────────────────┘
                       │                                             │
                       ▼                                             ▼
┌──────────────────────────────────────────────┐ ┌──────────────────────────────────────────┐
│          internal/versioncheck               │ │             Subsystem Execution          │
│  - Binary Discovery (LookPath)               │ │  - smart_build (Autofix, Linter, Tags)   │
│  - Version Parsing (Regex & BuildInfo)       │ │  - smart_edit (Atomic write, Vet gate)   │
│  - Semver Comparison (>=, latest)            │ │  - smart_test (Timeouts, Coverage gate)  │
│  - Non-blocking Diagnostic Hints             │ │  - test_query (WAL mode, SQLite path)    │
│  - In-Memory TTL Cache (5m)                  │ │  - read_docs  (AST LRU cache, Vanity)    │
└──────────────────────────────────────────────┘ └──────────────────────────────────────────┘
```

---

### 3.1 Master `.godoctor.yaml` Specification

```yaml
# ==============================================================================
# GoDoctor Master Configuration (.godoctor.yaml)
# ==============================================================================
version: "1"

# CLI Global Execution Settings
cli:
  timeout: "60s"
  output_format: "text"           # "text" | "json" | "yaml"
  color: true
  log_level: "info"               # "debug" | "info" | "warn" | "error"

# MCP Server Settings
server:
  name: "godoctor"
  transport: "stdio"              # "stdio" | "http"
  http:
    listen: ":8080"
    read_timeout: "30s"
    write_timeout: "5m"           # Extended to accommodate long-running builds/tests
    idle_timeout: "120s"
    shutdown_timeout: "10s"
    allowed_origins:
      - "http://localhost"
      - "http://localhost:*"
      - "http://127.0.0.1"
      - "http://127.0.0.1:*"
    allow_credentials: true
  logging:
    level: "info"                 # "debug" | "info" | "warn" | "error"
    format: "text"                # "text" | "json"
    trace_mcp_payloads: false
    log_file: ""

# SafeShell Subprocess Safety & Allowed Executables
safeshell:
  mode: "standard"                # "standard" | "strict" | "allowlist" | "disabled"
  command_timeout: "120s"
  allowed_binaries:
    - "go"
    - "gofmt"
    - "golangci-lint"
    - "selene"
    - "testquery"
    - "tq"
    - "deadcode"
    - "modernize"

# System Instructions Prompt Configuration
instructions:
  enabled: true
  compact: false
  dynamic_tools: true            # Filter instructions to only active tools
  rules_file: ""                 # Optional repo-specific markdown rules (e.g. .godoctor/rules.md)
  custom_rules: ""

# External Utility Definitions & RFC-0001 Version Tracking
tools:
  golangci_lint:
    binary_name: "golangci-lint"
    recommended_version: "v2.12.2"
    min_version: "v1.60.0"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
    config: ".golangci.yml"
    timeout: "5m"
    args: ["run"]
    disabled: false

  modernize:
    binary_name: "modernize"
    recommended_version: "latest"
    pkg: "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest"
    timeout: "2m"
    args: ["-fix"]
    disabled: false

  deadcode:
    binary_name: "deadcode"
    recommended_version: "latest"
    pkg: "golang.org/x/tools/cmd/deadcode@latest"
    timeout: "2m"
    disabled: false

  selene:
    binary_name: "selene"
    recommended_version: "latest"
    pkg: "github.com/danicat/selene/cmd/selene@latest"
    packages: "./..."
    timeout: "3m"
    disabled: false

  testquery:
    binary_name: "testquery"
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: ".godoctor/testquery.db"
    format: "table"
    timeout: "2m"
    disabled: false

# Build Pipeline Configuration (smart_build)
build:
  default_packages: "./..."
  output: ""
  tags: []
  race: false
  trimpath: false
  timeout: "5m"
  flags: []

# Auto-Fix Pipeline Configuration (smart_build)
autofix:
  enabled: true
  dry_run: false
  order:
    - tidy
    - modernize
    - gofmt
  tidy:
    enabled: true
  modernize:
    enabled: true
    flags: ["-fix"]
  gofmt:
    enabled: true
    tool: "gofmt"                 # "gofmt" | "goimports" | "gofumpt"
    path: "."

# Transactional Edit Engine (smart_edit)
edit:
  backup_strategy: "memory"       # "memory" | "temp_file" | "git"
  atomic_write: true              # Write-to-temp + atomic os.Rename
  preserve_permissions: true      # Retain original os.FileMode
  default_threshold: 0.95
  format_on_save: "goimports"     # "goimports" | "gofmt" | "none"
  exclude_paths:
    - ".git"
    - "vendor"
    - "node_modules"
    - "skills"
    - "agents"
    - "hooks"

# Fuzzy Coordinate Matching Engine (match.go)
matching:
  fuzzy_fallback: true
  similarity_threshold: 0.95
  normalize_unicode: true         # Strip non-breaking spaces (\u00A0) and Unicode whitespace
  min_seed_length: 3
  window_expansion_delta: 4

# Compiler Diagnostics & Verification Gate (diagnostics.go)
diagnostics:
  collect_on_edit: true
  verification_scope: "module"    # "module" | "package" | "edited_files"
  check_command:
    - "go"
    - "vet"
    - "./..."
  timeout: "30s"
  enable_suggestions: true
  max_levenshtein_distance: 3
  max_suggestions: 5
  snippet_context_lines: 5

# Test & Benchmark Runner (smart_test)
test:
  default_level: "basic"           # "fast" | "basic" | "benchmark" | "complete"
  default_packages: "./..."
  timeout: "60s"
  verbose: true
  race_detector: false
  coverage_threshold: 80.0
  coverage_profile: "coverage.out"
  coverage_output_dir: ""          # Empty string uses os.TempDir()
  benchmark_pattern: "."
  benchmark_flags:
    - "-benchmem"
    - "-run=NONE"

# SQLite Test Analytics (test_query)
testquery:
  db_path: ".godoctor/testquery.db"
  wal_mode: true
  busy_timeout: "5s"
  format: "table"                  # "table" | "json" | "csv"

# AST Documentation Engine (read_docs, godoc)
docs:
  cache_enabled: true
  cache_ttl: "15m"
  cache_max_entries: 500
  external_fetch: true
  offline_mode: false
  pkg_go_dev_url: "https://pkg.go.dev"
  max_symbols_rendered: 100
  fuzzy_suggestions: true
  max_fuzzy_suggestions: 5
  fuzzy_distance_threshold: 2
  default_format: "markdown"       # "markdown" | "json"
  temp_dir: ""

# Global Feature Flags
features:
  autofix: true
  mod_tidy: true
  modernize_check: true
  deadcode_check: true
  format_on_build: true
  format_on_edit: true
  vet_gate: true
  auto_rollback: true
  testquery_sync: true
  version_check_hints: true
  coverage_gate: false
  race_detector: false
  strict_mode: false
  remote_doc_fetch: true
  vanity_resolution: true
  docs_cache: true
```

---

### 3.2 Go Struct Definitions (`internal/config`)

```go
package config

import "time"

type Config struct {
	Version      string             `mapstructure:"version" yaml:"version" json:"version"`
	CLI          CLIConfig          `mapstructure:"cli" yaml:"cli" json:"cli"`
	Server       ServerConfig       `mapstructure:"server" yaml:"server" json:"server"`
	SafeShell    SafeShellConfig    `mapstructure:"safeshell" yaml:"safeshell" json:"safeshell"`
	Instructions InstructionsConfig `mapstructure:"instructions" yaml:"instructions" json:"instructions"`
	Tools        ToolsConfig        `mapstructure:"tools" yaml:"tools" json:"tools"`
	Build        BuildConfig        `mapstructure:"build" yaml:"build" json:"build"`
	Autofix      AutofixConfig      `mapstructure:"autofix" yaml:"autofix" json:"autofix"`
	Edit         EditConfig         `mapstructure:"edit" yaml:"edit" json:"edit"`
	Matching     MatchingConfig     `mapstructure:"matching" yaml:"matching" json:"matching"`
	Diagnostics  DiagnosticsConfig  `mapstructure:"diagnostics" yaml:"diagnostics" json:"diagnostics"`
	Test         TestConfig         `mapstructure:"test" yaml:"test" json:"test"`
	TestQuery    TestQueryConfig    `mapstructure:"testquery" yaml:"testquery" json:"testquery"`
	Docs         DocsConfig         `mapstructure:"docs" yaml:"docs" json:"docs"`
	Features     FeaturesConfig     `mapstructure:"features" yaml:"features" json:"features"`
}

type ToolSpec struct {
	BinaryName         string        `mapstructure:"binary_name" yaml:"binary_name" json:"binary_name"`
	RecommendedVersion string        `mapstructure:"recommended_version" yaml:"recommended_version" json:"recommended_version"`
	MinVersion         string        `mapstructure:"min_version" yaml:"min_version" json:"min_version"`
	Pkg                string        `mapstructure:"pkg" yaml:"pkg" json:"pkg"`
	Config             string        `mapstructure:"config,omitempty" yaml:"config,omitempty" json:"config,omitempty"`
	Packages           string        `mapstructure:"packages,omitempty" yaml:"packages,omitempty" json:"packages,omitempty"`
	DbPath             string        `mapstructure:"db_path,omitempty" yaml:"db_path,omitempty" json:"db_path,omitempty"`
	Format             string        `mapstructure:"format,omitempty" yaml:"format,omitempty" json:"format,omitempty"`
	Timeout            time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Args               []string      `mapstructure:"args" yaml:"args" json:"args"`
	Disabled           bool          `mapstructure:"disabled" yaml:"disabled" json:"disabled"`
}

type ToolsConfig struct {
	GolangciLint ToolSpec `mapstructure:"golangci_lint" yaml:"golangci_lint" json:"golangci_lint"`
	Modernize    ToolSpec `mapstructure:"modernize" yaml:"modernize" json:"modernize"`
	Deadcode     ToolSpec `mapstructure:"deadcode" yaml:"deadcode" json:"deadcode"`
	Selene       ToolSpec `mapstructure:"selene" yaml:"selene" json:"selene"`
	TestQuery    ToolSpec `mapstructure:"testquery" yaml:"testquery" json:"testquery"`
}

type FeaturesConfig struct {
	Autofix           bool `mapstructure:"autofix" yaml:"autofix" json:"autofix"`
	ModTidy           bool `mapstructure:"mod_tidy" yaml:"mod_tidy" json:"mod_tidy"`
	ModernizeCheck    bool `mapstructure:"modernize_check" yaml:"modernize_check" json:"modernize_check"`
	DeadcodeCheck     bool `mapstructure:"deadcode_check" yaml:"deadcode_check" json:"deadcode_check"`
	FormatOnBuild     bool `mapstructure:"format_on_build" yaml:"format_on_build" json:"format_on_build"`
	FormatOnEdit      bool `mapstructure:"format_on_edit" yaml:"format_on_edit" json:"format_on_edit"`
	VetGate           bool `mapstructure:"vet_gate" yaml:"vet_gate" json:"vet_gate"`
	AutoRollback      bool `mapstructure:"auto_rollback" yaml:"auto_rollback" json:"auto_rollback"`
	TestquerySync     bool `mapstructure:"testquery_sync" yaml:"testquery_sync" json:"testquery_sync"`
	VersionCheckHints bool `mapstructure:"version_check_hints" yaml:"version_check_hints" json:"version_check_hints"`
	CoverageGate      bool `mapstructure:"coverage_gate" yaml:"coverage_gate" json:"coverage_gate"`
	RaceDetector      bool `mapstructure:"race_detector" yaml:"race_detector" json:"race_detector"`
	StrictMode        bool `mapstructure:"strict_mode" yaml:"strict_mode" json:"strict_mode"`
	RemoteDocFetch    bool `mapstructure:"remote_doc_fetch" yaml:"remote_doc_fetch" json:"remote_doc_fetch"`
	VanityResolution  bool `mapstructure:"vanity_resolution" yaml:"vanity_resolution" json:"vanity_resolution"`
	DocsCache         bool `mapstructure:"docs_cache" yaml:"docs_cache" json:"docs_cache"`
}
```

---

### 3.3 Viper + Cobra Integration & 3-Tier Precedence

1. **Precedence Hierarchy (3-Tier)**:
   $$\text{Per-Call Payload (JSON)} \succ \text{Config File } (\texttt{.godoctor.yaml}) \succ \text{Built-in Go Defaults } (\texttt{NewDefaultConfig()})$$
   - *Note*: Per ADR-0001, environment variables (`GODOCTOR_*`) are intentionally not bound to ensure deterministic, reproducible tool execution in automated multi-agent environments.

2. **Hierarchical Discovery Algorithm**:
   - Step 1: Scan current working directory for `.godoctor.yaml` or `.godoctor.yml`.
   - Step 2: Ascend directory hierarchy until `.git`, `go.mod`, or filesystem root is reached.
   - Step 3: Check `$HOME/.godoctor.yaml` and `$HOME/.config/godoctor/config.yaml`.
   - Step 4: If no file is found, return `NewDefaultConfig()` without error.

---

### 3.4 Tool Version Checking & Upgrade Recommendation Engine (`internal/versioncheck`)

The version checking engine executes lightweight, non-blocking diagnostics across workspace tools:

1. **Detection & Extraction Pipeline**:
   - Fast `exec.LookPath(tool)` discovery.
   - CLI version probe with timeout (`--version`, `-version`, `go version`).
   - Deep fallback inspection via `runtime/debug.ReadBuildInfo` on compiled binaries.
2. **Semver Comparison Engine**:
   - Robust semver parsing handling `v` prefixes, prereleases, commit hashes, and `"latest"`.
   - Evaluates constraints such as `>=1.24.0` or `v2.12.2`.
3. **In-Memory TTL Caching**:
   - Thread-safe `sync.RWMutex` cache with a 5-minute TTL.
   - Validates file `mtime` via `os.Stat` to detect tool upgrades/uninstalls instantly.
4. **Actionable Non-Blocking Hints**:
   - When tools are missing or outdated, non-blocking diagnostic hints are appended to terminal output and MCP responses without interrupting execution:
     ```markdown
     > [!TIP]
     > **GoDoctor Tool Upgrade Recommendation**
     > - ⚠️ `golangci-lint`: Installed `v1.64.0` is older than recommended `v2.12.2`.
     >   Run: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`
     ```

---

### 3.5 CLI Commands Modernization

Migrating `internal/cli` to Cobra adds structured commands:

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `godoctor call` | `<tool> [json-payload]` | Executes a specific GoDoctor tool via CLI (strict JSON input preserved). |
| `godoctor mcp` | `[--transport stdio\|http]` | Starts the GoDoctor MCP server daemon. |
| `godoctor check` | `[--json] [--strict] [--no-cache]` | Inspects tool health, versions, and emits upgrade recipes table. |
| `godoctor init` | `[--force] [--minimal]` | Generates a fully annotated `.godoctor.yaml` configuration file. |
| `godoctor install` | `[--all] [--claude] [--cursor] [--antigravity]` | Installs GoDoctor MCP server definitions into agent environments. |
| `godoctor uninstall`| `[--all]` | Removes GoDoctor MCP server definitions. |

---

## 4. Dependencies & Version Verification

Following the `latest-version` verification protocol:
- `github.com/spf13/cobra` $\rightarrow$ `v1.10.2`
- `github.com/spf13/viper` $\rightarrow$ `v1.21.0`
- `gopkg.in/yaml.v3` $\rightarrow$ `v3.0.1`

---

## 5. Backward Compatibility & Migration Strategy

1. **Zero-Config Compatibility**: GoDoctor operates 100% out-of-the-box in repositories without `.godoctor.yaml`.
2. **CLI Parity**: `godoctor call <tool> '<json>'` signatures remain unchanged for MCP consumers and agent integrations.
3. **Rollout Roadmap**:
   - **Phase 1**: Implement `internal/config` (structs, Viper loader, defaults, tests).
   - **Phase 2**: Implement `internal/versioncheck` (detection, semver comparison, cache, tests).
   - **Phase 3**: Wire configuration and version check hooks into `smart_build`, `smart_test`, `smart_edit`, `selene`, `test_query`, and `godoc`.
   - **Phase 4**: Modernize `internal/cli` with Cobra subcommands (`check`, `init`, `call`, `mcp`, `install`, `uninstall`).
