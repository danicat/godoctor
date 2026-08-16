# ADR-0001: Centralized YAML Configuration & Tool Version Management

- **Status**: Accepted
- **Date**: 2026-08-16
- **Author(s)**: Lead Configuration Architect, Daniela Petruzalek
- **Deciders**: Daniela Petruzalek
- **Consulted**: Lead Core & Server Architecture Engineer, Lead Build & Edit Engine Engineer, Lead Test & Analytics Engine Engineer
- **Informed**: GoDoctor Development Team

---

## 1. Context and Problem Statement

GoDoctor's configuration was previously fragmented across hardcoded constants in individual tool packages (`smart_build`, `smart_test`, `smart_edit`, `selene`, `test_query`, `read_docs`, `server`, and `safeshell`). This caused multiple operational issues:
1. **Silent Version Drift & Remote Fallback Penalties**: Outdated or missing external tools (`golangci-lint`, `modernize`, `deadcode`, `selene`, `testquery`) triggered slow `go run <pkg>@latest` fallback compilations (adding 1.5s–4.5s latency per run) without providing actionable upgrade guidance.
2. **Inflexible Execution Parameters**: Server timeouts (hardcoded 15s write timeout), test levels, coverage gates, fuzzy matching thresholds, and SQLite database paths could not be customized per-repository or per-organization.
3. **Usability Hazards**: `safeshell.Validate` blocked valid SQL and regex characters (`<`, `>`, `\n`, `$`), breaking SQL test queries and filtered test runs.

We needed a centralized, type-safe configuration system and an integrated tool health/version recommendation mechanism.

---

## 2. Decision Drivers

* **Zero Breaking Changes**: Out-of-the-box operation must work seamlessly without requiring an explicit configuration file.
* **Declarative Configuration**: All workspace preferences must be declaratively defined in a human-readable YAML file (`.godoctor.yaml`).
* **Strict Configuration Source Policy**: To ensure deterministic behavior in AI agent environments, configuration is strictly driven by `.godoctor.yaml`, built-in Go defaults, and explicit per-call CLI/MCP payloads. **No environment variable overrides (`GODOCTOR_*`) are supported.**
* **Active Version Guidance**: Provide non-blocking diagnostic hints when external tools lag behind recommended releases.
* **Test Level Continuity**: Preserve established test execution levels (`fast`, `basic`, `benchmark`, `complete`).

---

## 3. Considered Options

* **Option 1: Ad-hoc CLI flags and per-package constants (Status Quo)**
* **Option 2: Environment-variable-heavy configuration (`GODOCTOR_*`)**
* **Option 3: Centralized `.godoctor.yaml` with Viper + Cobra, in-memory defaults, and zero environment variables** (Chosen)

---

## 4. Decision Outcome

**Chosen Option**: **Option 3 — Centralized `.godoctor.yaml` powered by Viper and Cobra**.

### 4.1 Key Architecture Decisions

1. **Precedence Hierarchy (3-Tier)**:
   $$\text{Per-Call Payload (CLI / MCP JSON)} \succ \text{Config File } (\texttt{.godoctor.yaml}) \succ \text{Built-in Go Defaults } (\texttt{NewDefaultConfig()})$$
   - Environment variables are deliberately excluded to avoid non-deterministic behavior and silent environment pollution in AI swarm workflows.

2. **Hierarchical Discovery Algorithm**:
   - Step 1: Scan current working directory for `.godoctor.yaml` or `.godoctor.yml`.
   - Step 2: Ascend parent directories up to git root (`.git`) or Go module root (`go.mod`).
   - Step 3: Fall back to `$HOME/.godoctor.yaml` or `$HOME/.config/godoctor/config.yaml`.
   - Step 4: If no file is present, load built-in default values (`NewDefaultConfig()`) with zero errors.

3. **Package Architecture**:
   - `internal/config`: Typed Go structs, Viper loader, upward traversal, defaults, and validation.
   - `internal/versioncheck`: Discovery (`LookPath`), safe CLI version probing, `debug.ReadBuildInfo` extraction, semver comparison, 5-minute thread-safe TTL cache, and non-blocking diagnostic formatting.
   - `internal/cli`: Cobra subcommands (`call`, `mcp`, `check`, `init`, `install`, `uninstall`).

4. **Preservation of Test Levels**:
   - The 4-tier test execution hierarchy (`fast`, `basic`, `benchmark`, `complete`) is retained under `test.default_level`.

5. **Safe Execution and Concurrency**:
   - `safeshell.mode: standard` allows safe execution of SQL and regex arguments via direct process invocation (`exec.CommandContext`).
   - `testquery.wal_mode: true` enables SQLite WAL mode to eliminate `database is locked` concurrency errors.

---

## 5. Pros and Cons of the Chosen Option

### Pros
* **Deterministic Configuration**: Absence of environment variables guarantees reproducible behavior across CI and agent environments.
* **Instant Diagnostic Guidance**: Non-blocking tool upgrade suggestions eliminate remote fallback latency.
* **Type-Safe Extensibility**: Strong Go structs prevent typos and unmarshaling errors.
* **Zero Disruption**: Existing MCP sessions and CLI commands (`godoctor call <tool> '<json>'`) work without modification.

### Cons
* **Viper Dependency**: Introduces `github.com/spf13/viper` (`v1.21.0`) and `github.com/spf13/cobra` (`v1.10.2`) to `go.mod`.
* **Subprocess Probing Overhead**: Initial version check executes external binaries (mitigated by 5-minute in-memory caching).

---

## 6. Implementation Roadmap

1. **Phase 1**: `internal/config` (Typed structs, Viper loader, `NewDefaultConfig()`, tests).
2. **Phase 2**: `internal/versioncheck` (Tool discovery, semver comparison, cache, `godoctor check`).
3. **Phase 3**: Subsystem Wiring (`smart_build`, `smart_test`, `smart_edit`, `selene`, `test_query`, `read_docs`, `safeshell`, `server`).
4. **Phase 4**: Cobra CLI Migration (`call`, `mcp`, `init`, `check`, `install`, `uninstall`).
