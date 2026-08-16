# GoDoctor Comprehensive Codebase Audit Findings

This document aggregates the deep-dive audit findings across all domains to identify code smells, antipatterns, overengineering, and hardcoded behaviors. These findings directly inform the centralized configuration system (`.godoctor.yaml`), feature flag schema, and [RFC-0001](file:///Users/petruzalek/projects/godoctor/design/rfc/0001-centralized-config-and-version-management.md).

---

## 1. Domain 1: Core CLI & Server Architecture
*Lead: Lead Core & Server Architecture Engineer*
*Status: Completed*

### 1.1 Code Smells & Antipatterns

1. **Critical Usability Breakage in `safeshell.Validate` for Regexes and SQL Queries**:
   - **Locations:** [`internal/safeshell/safeshell.go:48-54`](file:///Users/petruzalek/projects/godoctor/internal/safeshell/safeshell.go#L48-L54)
   - **Issue:** `safeshell.Validate` unconditionally rejects arguments containing `$`, `<`, `>`, `\n`, `\r`, `|`, `&`, `;`, `` ` ``.
   - **Impact:**
     - `smart_test`: Passing exact regex filters (e.g., `-run=^TestAuth$`) is rejected because `$` is treated as a shell operator.
     - `test_query`: SQL comparison queries (e.g., `SELECT * FROM all_coverage WHERE count < 5` or `count > 0`) or multiline SQL queries containing newlines are rejected because `<` and `\n` are blocked.
   - **Root Cause:** Direct process execution via `exec.CommandContext` passes arguments directly into the `argv` array of `execve(2)`. Shell operators are not interpreted by the OS kernel without an explicit subshell (`/bin/sh -c`). The blocklist causes high collateral damage while failing to protect against flag injection (e.g., `-exec=...`).

2. **Insecure CORS Wildcard Reflection in MCP HTTP Transport**:
   - **Locations:** [`internal/server/server.go:68-76`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L68-L76)
   - **Issue:** `strings.HasPrefix(origin, "https://")` reflects any HTTPS origin into `Access-Control-Allow-Origin`. In addition, `strings.HasPrefix(origin, "http://localhost")` matches untrusted origins such as `http://localhost.attacker.com`.
   - **Impact:** Combined with `Access-Control-Allow-Credentials: true`, any malicious website visited in a developer's browser can issue authenticated cross-origin tool calls to the local GoDoctor HTTP daemon (`smart_edit` file modifications, `smart_build` execution).

3. **Goroutine Leaks and Missing Context Synchronization on Server Shutdown**:
   - **Locations:** [`internal/server/server.go:96-103`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L96-L103)
   - **Issue:** If `srv.ListenAndServe()` fails immediately (e.g., port collision `EADDRINUSE`), `ServeHTTP` exits, but the background goroutine remains blocked indefinitely on `<-ctx.Done()`. Furthermore, during shutdown, `ServeHTTP` exits without synchronizing with the shutdown goroutine.

4. **Premature HTTP Write Timeout Killing Long-Running Tool Executions**:
   - **Locations:** [`internal/server/server.go:90-93`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L90-L93)
   - **Issue:** `WriteTimeout` is hardcoded to `15 * time.Second`.
   - **Impact:** Long-running operations such as `smart_build` (mod tidy + modernize + deadcode + build + test + linter), `smart_test` (complete suite), and `selene` (mutation testing) frequently take 30–120+ seconds, causing the HTTP server to terminate the TCP connection mid-execution and corrupt client sessions.

5. **Context Cancellation Discarded in CLI Operations**:
   - **Locations:** [`internal/cli/install.go:46-47`](file:///Users/petruzalek/projects/godoctor/internal/cli/install.go#L46-L47), [`internal/cli/uninstall.go:41-42`](file:///Users/petruzalek/projects/godoctor/internal/cli/uninstall.go#L41-L42)
   - **Issue:** `runInstall` and `runUninstall` explicitly discard `ctx` with `_ = ctx`. User interrupts (SIGINT/SIGTERM) during skill extraction or configuration writing cannot cancel cleanly.

6. **Error Swallowing and Misleading Messages in CLI Argument Parsing**:
   - **Locations:** [`internal/cli/cli.go:276-291`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L276-L291)
   - **Issue:** I/O errors from reading `stdin` are ignored with `if err == nil`. If input is invalid JSON, the function falls through and reports `"missing arguments"` instead of the actual JSON parsing error.

7. **Silently Dropped Non-Text MCP Content in CLI Mode**:
   - **Locations:** [`internal/cli/cli.go:222-230`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L222-L230)
   - **Issue:** `runCall` only checks for `*mcp.TextContent`. Binary, image, or embedded resource contents returned by MCP tools are ignored without warning or fallback.

8. **Loss of Error Cause and Context on Tool Failure**:
   - **Locations:** [`internal/cli/cli.go:232-235`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L232-L235)
   - **Issue:** When a tool returns `res.IsError == true`, `runCall` discards structured error details and returns a flat `errors.New("tool execution returned an error")`.

9. **Non-Idempotent Handler Registration & Deceptive Error Returns**:
   - **Locations:** [`internal/server/server.go:114-122`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L114-L122)
   - **Issue:** `RegisterHandlers()` is called inside both `Run()` and `ServeHTTP()`. Calling it manually beforehand causes duplicate registrations and map collisions. It also hardcodes `return nil` without validating tool registration success.

10. **Hallucinated SQLite Table & Misleading Workflow in System Instructions**:
    - **Locations:** [`internal/instructions/instructions.go:15, 24`](file:///Users/petruzalek/projects/godoctor/internal/instructions/instructions.go#L15)
    - **Issue:**
      - Line 24 recommends `SELECT * FROM coverage WHERE count = 0`. The actual table schema in `testquery.db` is `all_coverage`, causing SQLite query errors.
      - Line 15 claims `smart_edit` runs `go vet` *before* writing to disk; in reality, changes are written and verified with snapshot rollback on error.

11. **Missing Build Info Version Fallback**:
    - **Locations:** [`cmd/godoctor/main.go:28-30`](file:///Users/petruzalek/projects/godoctor/cmd/godoctor/main.go#L28-L30)
    - **Issue:** `version` defaults to `"dev"`. Installing via `go install` leaves the version string as `"dev"` because `runtime/debug.ReadBuildInfo()` is not inspected.

---

### 1.2 Overengineering & Complexity

1. **Duplicated Tool Registries & Metadata Across 3 Subsystems**:
   - Tool names, descriptions, and usages are duplicated across:
     - [`internal/cli/cli.go:33-78`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L33-L78) (`GetTools()`)
     - [`internal/server/server.go:114-122`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L114-L122) (`RegisterHandlers()`)
     - [`internal/instructions/instructions.go:9-27`](file:///Users/petruzalek/projects/godoctor/internal/instructions/instructions.go#L9-L27) (`Get()`)
   - Any tool schema or description change requires manual edits across multiple disconnected files.

2. **Repetitive Tool Invoker Boilerplate**:
   - [`internal/cli/cli.go:293-345`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L293-L345) contains 6 near-identical invoker functions (`invokeSmartEdit`, `invokeSmartBuild`, `invokeSmartTest`, `invokeTestQuery`, `invokeSelene`, `invokeReadDocs`) with copy-pasted parameter unmarshaling.

3. **Fragile Shell Argument JSON Joining Heuristic**:
   - [`internal/cli/cli.go:267-274`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L267-L274) uses `strings.Join(rawArgs, " ")` to reconstruct unquoted shell JSON arguments, which collapses and corrupts whitespace in string literals.

4. **Inflexible Hand-Rolled Flag Routing**:
   - [`internal/cli/cli.go:108-138`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L108-L138) relies on a custom `switch args[0]`. Global flags placed before subcommands (e.g., `godoctor --verbose call test`) are treated as unknown commands.

5. **Redundant String Allocation in Instructions**:
   - [`internal/instructions/instructions.go:9-27`](file:///Users/petruzalek/projects/godoctor/internal/instructions/instructions.go#L9-L27) uses `strings.Builder` with 8 allocations for a completely static string.

---

### 1.3 Hardcoded Behaviors & Magic Values

| Component | Location | Hardcoded Value / Behavior | Impact |
| :--- | :--- | :--- | :--- |
| **CLI** | [`cmd/godoctor/main.go:29`](file:///Users/petruzalek/projects/godoctor/cmd/godoctor/main.go#L29) | `version = "dev"` | No automatic fallback to Go module build info. |
| **CLI** | [`internal/cli/cli.go:143-185`](file:///Users/petruzalek/projects/godoctor/internal/cli/cli.go#L143-L185) | Static help message string | Hardcodes tool names and usage examples; cannot reflect dynamically enabled tools. |
| **CLI** | [`internal/cli/install.go:62,64`](file:///Users/petruzalek/projects/godoctor/internal/cli/install.go#L62-L64) | Flag `-c, --config` for `mcp_config.json` | Name collision with standard root `--config` for `.godoctor.yaml`. |
| **Server** | [`internal/server/server.go:33`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L33) | `Name: "godoctor"` | Static implementation name; cannot be customized for enterprise workspaces. |
| **Server** | [`internal/server/server.go:68-72`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L68-L72) | Hardcoded CORS origin prefixes | Insecure wildcard reflection of any `https://` or `http://localhost*` origin. |
| **Server** | [`internal/server/server.go:90-93`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L90-L93) | `WriteTimeout: 15s`, `ReadTimeout: 15s` | Premature connection termination during long builds or mutation tests. |
| **Server** | [`internal/server/server.go:98`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L98) | `5 * time.Second` | Fixed graceful shutdown timeout. |
| **Server** | [`internal/server/server.go:101,105`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L101-L105) | `log.Printf` to stderr | Unstructured logging without levels, JSON mode, or file target support. |
| **Server** | [`internal/server/server.go:114-122`](file:///Users/petruzalek/projects/godoctor/internal/server/server.go#L114-L122) | 6 hardcoded tool registrations | Inflexible; tools cannot be enabled or disabled via configuration. |
| **SafeShell** | [`internal/safeshell/safeshell.go:48`](file:///Users/petruzalek/projects/godoctor/internal/safeshell/safeshell.go#L48) | `disallowed := []string{"|", "&", ";", "<", ">", "`", "$", "\n", "\r", "\x00"}` | Breaks regex and SQL parameters while failing to prevent flag injection. |
| **Instructions** | [`internal/instructions/instructions.go:24`](file:///Users/petruzalek/projects/godoctor/internal/instructions/instructions.go#L24) | `FROM coverage WHERE count = 0` | Broken SQL table reference in example instruction prompt. |

---

### 1.4 Feature Flag Candidates & Sensible Defaults

#### Feature Flag Candidates
1. **`safeshell.mode`** (`standard` | `strict` | `allowlist` | `disabled`):
   - `standard` (default): Blocks null bytes (`\x00`), validates executables against allowlist, enforces process timeouts, permits valid regex (`$`) and SQL (`<`, `>`, `\n`).
   - `strict`: Legacy metacharacter blocklist.
   - `allowlist`: Enforces strict executable and argument flag validation.
2. **`server.features.tools.<name>.enabled`**: Granular per-tool enable/disable switches.
3. **`server.logging.trace_mcp_payloads`**: Trace raw JSON-RPC request and response payloads for debugging.
4. **`instructions.compact`**: Minified prompt format for token-constrained agent sessions.
5. **`instructions.rules_file`**: Appends repository-specific Markdown rules (e.g., `.godoctor/rules.md`) to the MCP system prompt.

#### Sensible Defaults for `.godoctor.yaml` (Domain 1)
```yaml
version: "1"

# CLI Configuration
cli:
  timeout: "60s"
  output_format: "text"           # text | json | yaml
  color: true
  log_level: "info"               # debug | info | warn | error

# MCP Server Configuration
server:
  name: "godoctor"
  transport: "stdio"              # stdio | http
  http:
    listen: ":8080"
    read_timeout: "30s"
    write_timeout: "5m"           # Allows long-running builds/tests without drops
    idle_timeout: "120s"
    shutdown_timeout: "10s"
    allowed_origins:
      - "http://localhost"
      - "http://localhost:*"
      - "http://127.0.0.1"
      - "http://127.0.0.1:*"
    allow_credentials: true
  logging:
    level: "info"                 # debug | info | warn | error
    format: "text"                # text | json
    trace_mcp_payloads: false
    log_file: ""

# SafeShell Subprocess Safety Configuration
safeshell:
  mode: "standard"                # standard | strict | allowlist | disabled
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

# System Instructions Configuration
instructions:
  enabled: true
  compact: false
  dynamic_tools: true            # Only list instructions for enabled tools
  rules_file: ""                 # e.g., .godoctor/rules.md
  custom_rules: ""

# Tool Registrations & Toggles
tools:
  smart_edit:
    enabled: true
  smart_build:
    enabled: true
  smart_test:
    enabled: true
  read_docs:
    enabled: true
  selene:
    enabled: true
  test_query:
    enabled: true


---

## 2. Domain 2: Build & Edit Engine
*Lead: Lead Build & Edit Engine Engineer*
*Status: Complete*

The Build & Edit Engine domain encompasses the build orchestration pipeline ([`smart_build`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go)), transactional file editing engine ([`smart_edit`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/edit.go)), fuzzy coordinate matching engine ([`match.go`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go)), compiler gates and diagnostics ([`diagnostics.go`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go)), and text/diff/Levenshtein utilities ([`text.go`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go)).

---

### 2.1 Code Smells & Antipatterns

#### 2.1.1 Build Pipeline (`smart_build`)
1. **Fragile Error Inspection via String Matching (`"exit status 3"`)**:
   - *Location*: [`internal/tools/smart_build/build.go:141-146`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L141-L146)
   - *Issue*: `runAutoFix` checks `strings.Contains(err.Error(), "exit status 3")` to detect when `modernize` successfully applied fixes. Error string formats vary across platforms, Go toolchain versions, and test mocks. Furthermore, the tool output detailing which modifications were applied is discarded, leaving users blind to applied fixes.
   - *Remediation*: Use `errors.As(err, &exitErr)` and inspect `exitErr.ExitCode() == 3`. Append the modernized output diff/summary to the report.
2. **Performance Bottleneck: Redundant Test Suite Invocations**:
   - *Location*: [`internal/tools/smart_build/build.go:216-225`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L216-L225), [`build.go:247-263`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L247-L263)
   - *Issue*: `runTestsPhase` executes `go test -v -coverprofile=...` and immediately calls `syncTestQueryDB`, which invokes `testquery build` (or `go run testquery@latest build`). `testquery build` executes `go test -json` on the entire test suite a second time, doubling CI and build execution time.
   - *Remediation*: Stream test JSON output from a single `go test` invocation to both coverage calculation and `testquery`, or decouple database synchronization behind a background task.
3. **Workspace Pollution & Concurrency Hazards with `coverage.out`**:
   - *Location*: [`internal/tools/smart_build/build.go:210-214`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L210-L214)
   - *Issue*: `covFile := filepath.Join(workspaceDir, "coverage.out")` writes the coverage profile directly to the user's project root and runs `defer os.Remove(covFile)`. If the user has an existing `coverage.out`, it is deleted; concurrent builds overwrite each other's coverage profiles; and abnormal terminations leave untracked artifacts.
   - *Remediation*: Generate temporary coverage profiles in `os.TempDir()` via `os.CreateTemp("", "godoctor-coverage-*.out")`.
4. **Single String Argument Passing for Multi-Package Targets**:
   - *Location*: [`internal/tools/smart_build/build.go:134`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L134), [`build.go:194`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L194), [`build.go:216`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L216), [`build.go:364`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L364), [`build.go:382`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L382)
   - *Issue*: In `runBuild`, `runTestsPhase`, `runLinterPhase`, and `modernize`, `pkgs` is passed as a single string argument (`append(buildArgs, pkgs)`), whereas `deadcode` uses `strings.Fields(pkgs)`. If a user passes multiple packages (e.g. `"./cmd/... ./internal/..."`), `go build` fails with `cannot find package`.
   - *Remediation*: Centralize package list parsing with `strings.Fields` or comma separation, spreading slices into command arguments.
5. **Missing Context Cancellation Checks in Handler**:
   - *Location*: [`internal/tools/smart_build/build.go:97-114`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L97-L114)
   - *Issue*: While `runAutoFix` checks `ctx.Err() != nil` between sub-steps, `Handler` does not check context cancellation between major phases (`runBuild`, `runTestsPhase`, `runLinterPhase`), causing cancelled requests to continue invoking heavy subprocesses.
   - *Remediation*: Add explicit `if err := ctx.Err(); err != nil` guards before each phase.
6. **Unbounded Memory Buffering of Subprocess Outputs**:
   - *Location*: [`internal/tools/smart_build/build.go:71`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L71), [`build.go:195`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L195), [`build.go:217`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L217), [`build.go:390`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L390)
   - *Issue*: `RunWithOutput` captures the complete stdout/stderr into memory via `cmd.CombinedOutput()`. On large test suites or high-verbosity linter runs, tens of megabytes are buffered and injected into MCP markdown responses, causing LLM context exhaustion and latency spikes.
   - *Remediation*: Cap captured tool failure output to a configurable window (e.g., last 150 lines or 64KB) with explicit truncation indicators.
7. **Global Mutable Command Runner State**:
   - *Location*: [`internal/tools/smart_build/build.go:80`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L80)
   - *Issue*: `var CommandRunner Runner = &stdRunner{}` is a global variable mutated during unit tests. This prevents `t.Parallel()` and introduces data races under concurrent testing or multi-session tool handling.
   - *Remediation*: Inject `Runner` via a tool struct receiver or context.

#### 2.1.2 Transactional Editing & Diagnostics (`smart_edit`)
1. **Critical Workspace Root Resolution Bug**:
   - *Location*: [`internal/tools/smart_edit/diagnostics.go:31-35`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L31-L35)
   - *Issue*: `workspaceRoot` is derived from an arbitrary map key:
     ```go
     var workspaceRoot string
     for absPath := range currentContents {
         workspaceRoot = filepath.Dir(absPath)
         break
     }
     ```
     For files in subpackages (e.g. `internal/tools/smart_edit/edit.go`), `filepath.Dir` points to `internal/tools/smart_edit`, not the module root containing `go.mod`. In multi-file edits spanning multiple packages, an arbitrary directory is chosen due to map randomization. Subsequent `go vet ./...` runs only inside that subdirectory, missing breaking changes across sibling packages.
   - *Remediation*: Traverse ancestor directories upwards to discover the enclosing `go.mod` / `go.work` root or Common Ancestor root.
2. **Non-Atomic In-Place File Writes & Truncation Hazard**:
   - *Location*: [`internal/tools/smart_edit/transaction.go:171-195`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L171-L195), [`transaction.go:213-222`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L213-L222)
   - *Issue*: `commitWrite` opens live files with `os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)` and writes directly into them. If the write fails midway (e.g. disk full, SIGKILL, OOM), the source file is left truncated/empty. Rollbacks use the same non-atomic truncation method.
   - *Remediation*: Adopt write-to-temp-and-rename semantics (`os.CreateTemp` in same directory $\to$ `f.Sync()` $\to$ `os.Rename`) to guarantee POSIX atomic replacement.
3. **Loss of File Permissions (`os.FileMode`) and Orphaned Directories on Rollback**:
   - *Location*: [`internal/tools/smart_edit/transaction.go:56-68`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L56-L68), [`transaction.go:204-211`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L204-L211), [`transaction.go:228-242`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L228-L242)
   - *Issue*: `backups` stores only raw `[]byte` without preserving original `os.FileMode` (permissions bits like `0755` executable bits are lost). Furthermore, newly created directories created via `os.MkdirAll` during a transaction are orphaned and never cleaned up if the transaction rolls back.
   - *Remediation*: Store a `FileBackup` struct containing content, `os.FileMode`, and existence state; track created directories in reverse order for cleanup on rollback.
4. **Unmemoized AST Reparsing Overhead in Diagnostics Suggestions**:
   - *Location*: [`internal/tools/smart_edit/diagnostics.go:139-147`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L139-L147), [`diagnostics.go:155-196`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L155-L196)
   - *Issue*: `findSuggestions` loops through all compiler errors and calls `extractASTSymbols(filePath)` for every undeclared identifier. If `go vet` reports 10 errors in a file, `parser.ParseDir` reparses every `.go` file in that directory 10 separate times.
   - *Remediation*: Memoize AST symbols by directory path in a local map `map[string][]string`.
5. **Windows Path Incompatibility in Diagnostics Regex**:
   - *Location*: [`internal/tools/smart_edit/diagnostics.go:115`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L115)
   - *Issue*: `fileErrorRegex = regexp.MustCompile("^([^:]+):(\\d+):(\\d+):\\s*(.*)$")` breaks on Windows paths with drive letters (e.g. `C:\repo\main.go:10:5:`), where `^([^:]+)` captures only `C` and line number matching fails.
   - *Remediation*: Support optional drive letters: `^(?:[a-zA-Z]:)?[^:]+:\d+:\d+:\s*.*$`.
6. **Redundant Struct Duplication**:
   - *Location*: [`internal/tools/smart_edit/edit.go:24-38`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/edit.go#L24-L38), [`edit.go:41-55`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/edit.go#L41-L55)
   - *Issue*: `SingleEditParams` is a verbatim 15-line duplicate of `FileEdit`. `Handler` performs a type cast `edit := FileEdit(args)`.
   - *Remediation*: Use type alias `type SingleEditParams = FileEdit` or unify into one struct.
7. **Dead MCP Session Parameter**:
   - *Location*: [`internal/tools/smart_edit/transaction.go:17`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L17), [`diagnostics.go:22`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L22)
   - *Issue*: `session *mcp.ServerSession` is passed through `ExecuteEdits` down to `writeAndVerify` where it is discarded as `_ *mcp.ServerSession`.

#### 2.1.3 Fuzzy Matching Engine & Text Utilities (`match.go` & `text.go`)
1. **Critical UTF-8 Byte Offset Off-by-N Corruption**:
   - *Location*: [`internal/tools/smart_edit/match.go:36, 54`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L36)
   - *Issue*: The matching engine computes the replacement end offset by adding `+ 1` byte to the last matched rune's start offset:
     ```go
     end := mapped[bestEndIdx-1].offset + 1
     ```
     For multi-byte runes (e.g. Japanese kanji, emojis, Cyrillic, Greek, mathematical symbols), this adds only 1 byte instead of the full rune length (`utf8.RuneLen`), slicing through multi-byte UTF-8 sequences and producing corrupt source files.
   - *Remediation*: Compute end byte offset using `mapped[bestEndIdx-1].offset + utf8.RuneLen(mapped[bestEndIdx-1].char)`.
2. **Byte Index vs. Rune Index Desynchronization in Candidate Seeding**:
   - *Location*: [`internal/tools/smart_edit/match.go:80-90`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L80-L90), [`match.go:117`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L117)
   - *Issue*: In `collectCandidates`, `strings.Index` returns a *byte* offset in `normContent`, but `offset` is a *rune* offset in `searchRunes`. Subtracting `realIdx - offset` produces a hybrid value used as an index into `normContentRunes` (a rune slice). Whenever non-ASCII characters precede a match, fuzzy candidate evaluation is shifted out of alignment.
   - *Remediation*: Track rune offsets consistently or convert byte indices via `utf8.RuneCountInString(normContent[:realIdx])`.
3. **Complete Fuzzy Match Failure on Short Search Strings ($< 4$ runes)**:
   - *Location*: [`internal/tools/smart_edit/match.go:69-72, 93`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L69-L72)
   - *Issue*: For search strings shorter than 4 non-whitespace runes (e.g. `i++`, `a=1`, `b&&c`), `searchLen < seedLen` fails to generate seeds because the loop bounds `i <= searchLen - seedLen` evaluates to negative numbers. Fuzzy matching fails with a 100% false negative rate on short expressions.
   - *Remediation*: Scale minimum seed length down to 1 or fall back to linear sliding window for short search strings.
4. **Quadruple Heap Allocations in `Similarity` and `Levenshtein`**:
   - *Location*: [`internal/text/text.go:12-19`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go#L12-L19), [`text.go:125-128`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go#L125-L128)
   - *Issue*: `Similarity` calls `Levenshtein(s1, s2)` (which allocates two `[]rune` slices) and then executes `len([]rune(s1))` and `len([]rune(s2))` (allocating two more `[]rune` slices). Every similarity comparison performs 4 heap allocations.
   - *Remediation*: Use `utf8.RuneCountInString(s)` (zero heap allocations), add an ASCII fast path in `Levenshtein`, and reuse DP rows.
5. **Inefficient Whole-File Memory Splitting in `GetSnippet`**:
   - *Location*: [`internal/text/text.go:81`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go#L81)
   - *Issue*: `GetSnippet` calls `strings.Split(content, "\n")`, allocating a slice containing every line in the entire file just to extract 10 lines around `lineNum`. In 10,000-line files, this creates massive GC pressure.
   - *Remediation*: Use localized byte scanning (`strings.IndexByte`) to isolate only the target line range.
6. **Incomplete Whitespace Normalization (Missing Non-Breaking Spaces)**:
   - *Location*: [`internal/text/text.go:101-106`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go#L101-L106)
   - *Issue*: `IsWhitespace` checks only ASCII `' '`, `'\t'`, `'\n'`, `'\r'`. Non-breaking spaces (`\u00A0` NBSP) and Unicode space variants frequently introduced when copying from browsers or chat tools are rejected as non-whitespace.
   - *Remediation*: Use `unicode.IsSpace(r)`.
7. **Premature Break Bug in `GetLineOffsets`**:
   - *Location*: [`internal/text/text.go:57-60`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go#L57-L60)
   - *Issue*: If `endLine` is passed smaller than `startLine` (e.g. `startLine=10, endLine=2`), the loop breaks at line 3 before finding `startLine`, returning a misleading error: `"start_line 10 is beyond file length (3 lines)"`.
   - *Remediation*: Validate `if endLine > 0 && endLine < startLine` upfront.

---

### 2.2 Overengineering & Complexity

| Issue | Files & Line Numbers | Architectural Complexity | Remediation |
| :--- | :--- | :--- | :--- |
| **Misplaced Deadcode Step in Auto-Fix** | [`build.go:117-185`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L117-L185) | `deadcode` is placed inside `runAutoFix` before compilation, even though it does not fix code and requires a compiling workspace. Violates tool documentation. | Move `deadcode` to a dedicated analysis phase after the linter step. |
| **Broken Config Discovery Traversal** | [`build.go:327-352`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L327-L352) | `findConfigFile` stops ascending when it finds `go.mod`. In monorepos where `.golangci.yml` is at repo root above nested modules, config is missed. | Traverse up to git repository root or Common Ancestor. |
| **Coverage Parsing Duplication** | [`build.go:265-325`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L265-L325) | `parseTotalCoverage`, `parsePackagesCoverage`, and `syncTestQueryDB` are copy-pasted identically across `smart_build` and `smart_test`. | Centralize into a shared `internal/gotest` package. |
| **Redundant Full-Tree Walk in `getAllGoFiles`** | [`diagnostics.go:37-45, 88-109`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L37-L45) | Traverses entire project directory tree via `filepath.Walk` solely to check `if len(goFiles) > 0` before running `go vet`. | Remove `getAllGoFiles`. Check `hasGoFiles(currentContents)` in memory or let `go vet` run directly. |
| **Repeated `[]byte` / `string` Conversions** | [`transaction.go:81, 111, 157, 161`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L81) | `currentContents` stores `[]byte`, converted to `string` in `applyMemoryEdits`, converted to `[]byte` for `imports.Process`, and converted back to `string` for snippet errors. | Standardize in-memory representation to reduce allocations. |
| **Redundant Struct Memory in Matching** | [`match.go:17-31`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L17-L31) | Stores `charMap` slice (16 bytes/char) and duplicate `normContentRunes` slice simultaneously. | Replace with a single rune slice and a parallel `[]int` byte offset map. |
| **Unused Candidate Frequency Map** | [`match.go:74, 87, 116`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L74) | Tallies candidate seed hit counts in `map[int]int`, but `evaluateCandidates` ignores frequencies and scans all keys linearly. | Prune candidates with low seed frequencies to accelerate matching on large files. |

---

### 2.3 Hardcoded Behaviors & Magic Values

| Category | File & Line | Hardcoded Value | Issue & Rationale |
| :--- | :--- | :--- | :--- |
| **Linter Fallback Binary** | [`build.go:354`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L354) | `"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"` | Hardcoded version cannot be updated without recompiling GoDoctor; may fail on newer Go toolchains. |
| **Unpinned Fallback Binaries** | [`build.go:137, 172, 259`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L137) | `modernize@latest`, `deadcode@latest`, `testquery@latest` | Unpinned `@latest` causes slow network checks and non-deterministic behavior. |
| **TestQuery DB Location** | [`build.go:253`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L253) | `"testquery.db"` | Fixed relative path in workspace root; cannot be redirected to `.godoctor/` or cache directory. |
| **Gofmt Format Target** | [`build.go:155`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L155) | `"gofmt", "-w", "."` | Always formats entire workspace root (`.`) even when building a specific package. |
| **Linter Config Names** | [`build.go:328-333`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L328-L333) | `[".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"]` | Misses `.golangci.dist.yml` or custom config file paths. |
| **Fuzzy Matching Threshold** | [`transaction.go:84`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L84) | `threshold = 0.95` | Hardcoded default. If caller passes explicit `0.0`, it is overwritten by `0.95` due to zero-value check. |
| **Diagnostics Verification Command** | [`diagnostics.go:48`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L48) | `"go", "vet", "./..."` | Fixed verification gate; cannot be swapped for `go build`, `golangci-lint`, or targeted packages. |
| **Spelling Distance Bound** | [`diagnostics.go:142`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L142) | `bestDist <= 4` | Absolute distance of 4 is overly loose for short symbols (e.g. `id` matching `item`). |
| **Spelling Sentinel Distance** | [`diagnostics.go:199`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L199) | `999` | Magic integer constant; should use `math.MaxInt`. |
| **Directory Exclusions** | [`diagnostics.go:98`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/diagnostics.go#L98) | `[".git", "skills", "agents", "hooks"]` | Hardcoded list; misses `.godoctor.yaml` exclusions, `vendor`, `node_modules`, or `.gitignore`. |
| **Directory Permissions** | [`transaction.go:204`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/transaction.go#L204) | `0750` | Hardcoded octal permission mode for newly created parent directories. |
| **Error Snippet Radius** | [`text.go:86-87`](file:///Users/petruzalek/projects/godoctor/internal/text/text.go#L86-L87) | $\pm 5$ lines (11 lines total) | Fixed context line window; unconfigurable for compact or expanded displays. |
| **Matching Seed Windows** | [`match.go:62-72`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match.go#L62-L72) | `16/8`, `8/4`, `4/2` | Hardcoded seed sizes; causes complete false negatives for $< 4$ rune strings. |

---

### 2.4 Feature Flag Candidates & Sensible Defaults

#### 2.4.1 Feature Flags Matrix (Domain 2)

| Feature Flag | Environment Variable | Type | Default | Scope | Description & Rationale |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `autofix.enabled` | `GODOCTOR_AUTOFIX_ENABLED` | `bool` | `true` | `smart_build` | Master switch for auto-fix phase. Disable for read-only CI or strict validation modes. |
| `autofix.dry_run` | `GODOCTOR_AUTOFIX_DRY_RUN` | `bool` | `false` | `smart_build` | Emits proposed modernize/format changes as diffs without modifying files on disk. |
| `autofix.tidy` | `GODOCTOR_AUTOFIX_TIDY` | `bool` | `true` | `smart_build` | Runs `go mod tidy` during auto-fix. |
| `autofix.modernize` | `GODOCTOR_AUTOFIX_MODERNIZE` | `bool` | `true` | `smart_build` | Runs `modernize -fix` during auto-fix. |
| `autofix.gofmt` | `GODOCTOR_AUTOFIX_GOFMT` | `bool` | `true` | `smart_build` | Runs code formatter (`gofmt` / `goimports`) during auto-fix. |
| `autofix.order` | `GODOCTOR_AUTOFIX_ORDER` | `[]string` | `["tidy", "modernize", "gofmt"]` | `smart_build` | Custom execution sequence for auto-fix pipeline. |
| `build.race` | `GODOCTOR_BUILD_RACE` | `bool` | `false` | `smart_build` | Passes `-race` detector flag to `go build` and `go test`. |
| `build.tags` | `GODOCTOR_BUILD_TAGS` | `[]string` | `[]` | `smart_build` | Build tags forwarded to `go build`, `go test`, and linters. |
| `build.trimpath` | `GODOCTOR_BUILD_TRIMPATH` | `bool` | `false` | `smart_build` | Passes `-trimpath` to remove host file paths from binaries. |
| `edit.atomic_write` | `GODOCTOR_EDIT_ATOMIC_WRITE` | `bool` | `true` | `smart_edit` | Writes edits to temporary sibling files before atomic rename. |
| `edit.preserve_permissions` | `GODOCTOR_EDIT_PRESERVE_PERMISSIONS` | `bool` | `true` | `smart_edit` | Preserves original `os.FileMode` bits across edits and rollbacks. |
| `edit.backup_strategy` | `GODOCTOR_EDIT_BACKUP_STRATEGY` | `string` | `"memory"` | `smart_edit` | Rollback storage strategy: `"memory"`, `"temp_file"`, or `"git"`. |
| `edit.format_on_save` | `GODOCTOR_EDIT_FORMAT_ON_SAVE` | `string` | `"goimports"` | `smart_edit` | Formatter applied post-edit: `"goimports"`, `"gofmt"`, `"none"`. |
| `matching.fuzzy_fallback` | `GODOCTOR_MATCHING_FUZZY_FALLBACK` | `bool` | `true` | `smart_edit` | Fallback to fuzzy Levenshtein matching when exact normalized match fails. |
| `matching.normalize_unicode` | `GODOCTOR_MATCHING_NORMALIZE_UNICODE`| `bool` | `true` | `smart_edit` | Strips non-breaking spaces (`\u00A0`) and Unicode whitespace variants. |
| `diagnostics.collect_on_edit`| `GODOCTOR_DIAGNOSTICS_COLLECT_ON_EDIT`| `bool`| `true` | `smart_edit` | Executes compiler diagnostics verification gate after applying edits. |
| `diagnostics.enable_suggestions`| `GODOCTOR_DIAGNOSTICS_ENABLE_SUGGESTIONS`| `bool`| `true` | `smart_edit` | Returns AST Levenshtein spelling suggestions on compiler errors. |
| `diagnostics.strict` | `GODOCTOR_DIAGNOSTICS_STRICT` | `bool` | `false` | `smart_edit` | Treats compiler warnings as fatal transaction errors. |

#### 2.4.2 Recommended `.godoctor.yaml` Configuration Schema & Defaults

```yaml
version: "1"

build:
  packages: "./..."
  output: ""                  # Target binary path (passed to go build -o)
  tags: []                    # Build tags e.g. ["integration"]
  race: false                 # Enable data race detector (-race)
  trimpath: false             # Strip file system paths from binary
  timeout: "5m"               # Timeout duration for build phase
  flags: []                   # Additional compiler flags

autofix:
  enabled: true               # Master switch for smart_build auto-fix phase
  dry_run: false              # Output diffs without modifying files
  order:                      # Execution sequence
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
    tool: "gofmt"             # Options: "gofmt", "goimports", "gofumpt"
    path: "."

edit:
  backup_strategy: "memory"   # Options: "memory", "temp_file", "git"
  atomic_write: true          # Use write-to-temp and atomic os.Rename
  preserve_permissions: true  # Retain original os.FileMode
  default_threshold: 0.95     # Default fuzzy matching confidence (0.0 - 1.0)
  format_on_save: "goimports" # Options: "goimports", "gofmt", "none"
  exclude_paths:
    - ".git"
    - "vendor"
    - "node_modules"
    - "skills"
    - "agents"
    - "hooks"

matching:
  fuzzy_fallback: true        # Enable Levenshtein fuzzy matching fallback
  similarity_threshold: 0.95  # Minimum similarity score required
  normalize_unicode: true     # Strip non-breaking spaces (\u00A0) and Unicode whitespace
  min_seed_length: 3          # Minimum seed length for fuzzy candidate collection
  window_expansion_delta: 4   # +/- rune expansion window around candidates

diagnostics:
  collect_on_edit: true       # Run compiler gate verification post-edit
  verification_scope: "module"# Scope: "module", "package", "edited_files"
  check_command:              # Verification command
    - "go"
    - "vet"
    - "./..."
  timeout: "30s"              # Verification execution timeout
  enable_suggestions: true    # Enable Levenshtein symbol typo suggestions
  max_levenshtein_distance: 3 # Maximum edit distance for typo suggestions
  max_suggestions: 5          # Maximum typo suggestions returned
  snippet_context_lines: 5    # Lines of context before/after error lines in snippet

analysis:
  deadcode:
    enabled: true             # Run deadcode analysis in smart_build
    strict: false             # Treat unreachable functions as fatal build errors
```

---

### 2.5 Test Suite Quality & Gap Analysis

1. **Dead Test Assertion in `TestEdit_Broken`**:
   - *Location*: [`internal/tools/smart_edit/edit_test.go:158-168`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/edit_test.go#L158-L168)
   - *Issue*: `res2, _, _ := Handler(context.TODO(), nil, SingleEditParams{...}); output := res2.Content[0].(*mcp.TextContent).Text; _ = output` discards the result with zero assertions, leaving compiler gate verification untested.
2. **Missing UTF-8 Multi-Byte Fuzzy Tests**:
   - *Location*: [`internal/tools/smart_edit/match_test.go:73-78`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/match_test.go#L73-L78)
   - *Issue*: Existing Unicode test covers only exact matches; there are no tests for fuzzy matches or typos on multi-byte runes, which would have caught the `offset + 1` truncation bug.
3. **Missing Multi-Package Argument Tests**:
   - *Location*: [`internal/tools/smart_build/build_test.go`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build_test.go)
   - *Issue*: No tests verify multi-package argument inputs like `"./cmd/... ./internal/..."` or comma-separated package lists.
4. **Missing Invariants in Fuzz Testing**:
   - *Location*: [`internal/tools/smart_edit/fuzz_test.go:15-39`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_edit/fuzz_test.go#L15-39)
   - *Issue*: `FuzzFindBestMatch` lacks the `utf8.ValidString(content[start:end])` invariant and does not test random 1-rune mutation retrieval.
5. **Missing Benchmark Suite**:
   - *Issue*: There are no benchmarks (`Benchmark*`) in `internal/text` or `internal/tools/smart_edit` to measure Levenshtein, Similarity, `findBestMatch`, or `GetSnippet` performance.


---

## 3. Domain 3: Test & Analytics Engine
*Lead: Lead Test & Analytics Engine Engineer*
*Status: Complete*

The Test & Analytics Engine encompasses Go test execution, coverage analysis, benchmark parsing, AST mutation testing, SQLite test history analytics, and Go documentation extraction across `smart_test`, `selene`, `test_query`, `godoc`, and `read_docs`.

### 3.1 Code Smells & Antipatterns

#### 3.1.1 Critical: `safeshell.Validate` Blocks Standard SQL Operators (`>`, `<`, `;`, `\n`, `|`, `` ` ``)
- **Location:** [`internal/tools/test_query/testquery.go:47, 56`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L47-L56), [`internal/safeshell/safeshell.go:44-55`](file:///Users/petruzalek/projects/godoctor/internal/safeshell/safeshell.go#L44-L55)
- **Description:** `stdRunner.Run` and `stdRunner.RunWithOutput` invoke `safeshell.CommandContext(ctx, name, args...)`. `safeshell.Validate` checks each argument and rejects strings containing `|`, `&`, `;`, `<`, `>`, `` ` ``, `$`, `\n`, `\r`, `\x00`. When `testquery` passes raw SQL query strings, valid SQL statements fail validation before execution:
  - `SELECT * FROM all_tests WHERE elapsed > 0.25` (fails on `>`)
  - `SELECT * FROM all_coverage WHERE count < 1` (fails on `<`)
  - `SELECT * FROM all_tests;` (fails on `;`)
  - Multi-line queries (fails on `\n`)
  - Identifier escapes `` `all_tests` `` (fails on `` ` ``)
  - String concatenation `col1 || col2` (fails on `|`)
  - **All high-value recipes** documented in [`skills/testquery/SKILL.md`](file:///Users/petruzalek/projects/godoctor/skills/testquery/SKILL.md) fail in production with `invalid argument: value contains disallowed shell operator character`.
- **Root Cause of Blind Spot:** `testquery_test.go` uses `mockRunner` which matches command strings directly and completely bypasses `safeshell.CommandContext`.
- **Recommendation:** Do not apply shell-injection filtering to payload arguments where `exec.CommandContext` executes processes directly via `argv` (without subshell invocation). Provide a sanitized command launcher for data arguments or validate only the executable name.

#### 3.1.2 Silent Error Masking & Stale Data Return on Rebuild Failure
- **Location:** [`internal/tools/test_query/testquery.go:140-146`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L140-L146)
- **Description:** In `buildDB`:
  ```go
  if buildErr != nil {
      if !fileExists(dbPath) {
          hint := "**HINT:** Ensure Go tests compile cleanly..."
          return errorResult(fmt.Sprintf("failed to build test database: %v\n%s\n\n%s", buildErr, buildOutput, hint))
      }
  }
  return nil
  ```
  If `args.Rebuild == true` is requested or an automatic rebuild is triggered, but the test suite fails to compile or `tq build` fails, the error is completely suppressed if an old `testquery.db` already exists on disk.
- **Impact:** The tool silently returns results from stale test/coverage data recorded hours or days ago, misleading developers and LLM agents.
- **Recommendation:** Always propagate build errors when rebuilding fails, or return a clear diagnostic warning header indicating that the database rebuild failed.

#### 3.1.3 SQLite Database Locking & Concurrency Hazards on `testquery.db`
- **Location:** [`internal/tools/test_query/testquery.go:92-98`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L92-L98), [`internal/tools/smart_test/test.go:328-344`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L328-L344), [`internal/tools/smart_build/build.go:247-263`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L247-L263)
- **Description:** `testquery.db` is written to directly by `smart_test` and `smart_build` test runs without SQLite WAL (Write-Ahead Logging) mode or busy timeout configuration.
- **Impact:** When a background test run synchronizes `testquery.db` while `test_query` executes a read query, SQLite throws `database is locked` (`busy`) errors.
- **Recommendation:** Automatically enable WAL mode (`PRAGMA journal_mode=WAL;`) and a busy timeout (`PRAGMA busy_timeout=5000;`) on `testquery.db`, and serialize concurrent writes in GoDoctor.

#### 3.1.4 Workspace Pollution & Unsafe `coverage.out` Temp File Handling
- **Location:** [`internal/tools/smart_test/test.go:169-173`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L169-L173)
- **Description:** `covFile := filepath.Join(workspaceDir, "coverage.out")` writes the coverage profile directly into the repository root and removes it via `defer os.Remove(covFile)`.
- **Impact:**
  1. Overwrites and permanently deletes any developer-owned `coverage.out` file in the workspace.
  2. Creates race conditions if multiple test runs execute concurrently in the same workspace.
  3. Fails in read-only or sandboxed workspaces.
- **Recommendation:** Use `os.CreateTemp("", "godoctor-cov-*.out")` with guaranteed cleanup in `defer`.

#### 3.1.5 Redundant Test Suite Re-Execution on Failure during `syncTestQueryDB`
- **Location:** [`internal/tools/smart_test/test.go:184-189, 328-344`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L184-L189)
- **Description:** When tests fail in `runBasicLevel`, `smart_test` immediately calls `syncTestQueryDB`. `testquery build` re-runs `go test -json` from scratch across the packages.
- **Impact:** Doubles the execution time on test failures. In `syncTestQueryDB`, errors are completely discarded (`_, _ = CommandRunner.RunWithOutput(...)`), masking failures when indexing crashes.
- **Recommendation:** Make indexing configurable via `features.testquery_sync`, feed the existing test output to `testquery` if supported, and log warnings on failure.

#### 3.1.6 Inconsistent Error Handling & Mutant Failure Masking between `selene` and `smart_test`
- **Location:** [`internal/tools/smart_test/test.go:116-120, 238-263`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L116-L120) vs [`internal/tools/selene/selene.go:103-110`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L103-L110)
- **Description:** When `selene` runs as a standalone MCP tool, surviving mutants result in exit code 1 and `IsError: true`. When `smart_test` runs with `level: complete`, `runMutationLevel` discards the error code and `Handler` returns `IsError: false` (`err` is `nil`).
- **Impact:** Inconsistent MCP contract: mutation failures are flagged as errors in `selene` but marked as successful tool execution in `smart_test`.
- **Recommendation:** Standardize error semantics and tool result status across both tools.

#### 3.1.7 Fragile State-Machine Failure Output Parsing in `smart_test`
- **Location:** [`internal/tools/smart_test/test.go:346-367`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L346-L367)
- **Description:** `formatFailures` identifies failing test blocks by scanning for `--- FAIL:` and terminates the block at the first blank line (`strings.TrimSpace(line) == ""`).
- **Impact:** If a failing test logs multiline JSON diffs, formatted error strings, or stack traces containing blank lines, the parser prematurely cuts off output, dropping critical failure diagnostics.
- **Recommendation:** Parse failure blocks until the next test runner marker (`=== RUN`, `--- PASS:`, `ok`, `FAIL\tpkg`) rather than breaking on blank lines.

#### 3.1.8 Over-Aggressive Log Stripping in `filterNoise` Swallowing Test Assertions
- **Location:** [`internal/tools/smart_test/test.go:428-438`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L428-L438), [`internal/tools/selene/selene.go:119-129`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L119-L129), [`internal/tools/test_query/testquery.go:213-223`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L213-L223)
- **Description:** `filterNoise` deletes any line containing `"exit status"`.
- **Impact:** Strips valid test failure output, assertion logs (e.g. `expected exit status 0, got exit status 2`), and SQL query logs from test results.
- **Recommendation:** Filter only exact prefix lines (`^exit status \d+$`) or separate stdout from stderr.

#### 3.1.9 Package Coverage Parser Hiding 0.0% Coverage Packages
- **Location:** [`internal/tools/smart_test/test.go:307-309`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L307-L309)
- **Description:** `parsePackagesCoverage` explicitly filters out lines where `covStr == "0.0%"` or `covStr == "[no"`.
- **Impact:** Packages with 0.0% coverage are omitted from the report, giving a false sense of comprehensive coverage.
- **Recommendation:** Surface 0.0% coverage packages under a dedicated "⚠️ Uncovered Packages" section.

#### 3.1.10 Deprecated AST Parsing APIs (`parser.ParseDir` & `ast.Package` SA1019)
- **Location:** [`internal/godoc/godoc.go:162-164, 193-207`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L162-L164)
- **Description:** Uses deprecated `parser.ParseDir` and `map[string]*ast.Package` with staticcheck lint suppressions (`//nolint:staticcheck // SA1019`).
- **Impact:** Technical debt and reliance on deprecated standard library AST wrappers.
- **Recommendation:** Refactor `parsePackageDocs` to read directory files using `os.ReadDir`, parse `.go` source files with `parser.ParseFile`, and pass `[]*ast.File` directly to `doc.NewFromFiles`.

#### 3.1.11 Zero In-Memory Caching & Excessive Subprocess Spawning on Hot Paths in `godoc`
- **Location:** [`internal/godoc/godoc.go:50, 147-158, 749-766`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L50)
- **Description:** Every `Load` or `LoadWithFallback` call synchronously executes external processes (`go list -f {{.Dir}} <pkgPath>` and `go list -f {{.ImportPath}} ./...`). There is no in-memory cache or LRU cache.
- **Impact:** 30–150ms latency penalty on every query, even for standard library packages (`fmt`, `os`, `io`).
- **Recommendation:** Introduce a thread-safe in-memory cache (`sync.RWMutex` + `map[string]*Doc` or LRU with configurable TTL) storing resolved directory paths and parsed doc models.

#### 3.1.12 AST Parsing Memory Bloat & Unfiltered Test Files in `godoc`
- **Location:** [`internal/godoc/godoc.go:163`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L163)
- **Description:** `parser.ParseDir(fset, pkgDir, nil, parser.ParseComments)` parses full ASTs (including all function bodies and expressions) for every `_test.go` file before `doc.NewFromFiles` discards them.
- **Impact:** High memory consumption and wasted CPU cycles on packages with large test suites.
- **Recommendation:** Filter out `*_test.go` files prior to parsing (except `example_*_test.go`).

#### 3.1.13 Catastrophic Latency on Missing Packages (`suggestPackages` running `go list all`)
- **Location:** [`internal/godoc/godoc.go:689-693, 707-747`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L689-L693)
- **Description:** When a package lookup fails, `suggestPackages` sequentially executes `go list std`, `go list all`, and `go list <parent>/...`.
- **Impact:** `go list all` scans the entire transitive dependency graph, blocking the MCP request for 5–30 seconds just to report a typo.
- **Recommendation:** Restrict fuzzy package suggestions to `go list std` and cached workspace modules with a strict timeout context (e.g. 300ms).

#### 3.1.14 Ephemeral Temp Module Creation for External Package Downloads
- **Location:** [`internal/godoc/godoc.go:678-685, 768-785, 789-847`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L678-L685)
- **Description:** `fetchAndRetryStructured` creates an ephemeral temporary directory (`os.MkdirTemp`), executes `go mod init temp_docs_fetcher`, runs `go get <pkgPath>`, extracts docs, and destroys the directory on return.
- **Impact:** Subsequent queries for the same external package re-download and re-initialize from scratch.
- **Recommendation:** Use a persistent, managed cache directory (`~/.cache/godoctor/fetch_module` or `.godoctor/cache`) with a long-lived `go.mod`.

#### 3.1.15 Fragile Regex for Vanity Import Resolution
- **Location:** [`internal/godoc/godoc.go:787, 801-806`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L787)
- **Description:** Vanity import detection matches `module declares its path as:\s+([^\s]+)` against stderr output of `go get`.
- **Impact:** Breaks if Go compiler error wording changes.
- **Recommendation:** Query standard HTTP `<meta name="go-import" content="...">` tags for vanity domains (`google.golang.org`, `gopkg.in`, `go.uber.org`) before executing `go get`.

#### 3.1.16 Unsynchronized Global Mutable State (`CommandRunner`)
- **Location:** [`internal/tools/smart_test/test.go:87`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L87), [`internal/tools/selene/selene.go:66`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L66), [`internal/tools/test_query/testquery.go:70`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L70)
- **Description:** `var CommandRunner Runner = &stdRunner{}` is a global package variable modified directly by unit tests without mutex synchronization.
- **Impact:** Prevents `t.Parallel()` testing and causes race conditions if multiple server handlers execute concurrently.
- **Recommendation:** Inject runners via tool instances or protect with synchronization.

---

### 3.2 Overengineering & Complexity

#### 3.2.1 Quad-Level Test Hierarchy (`fast`, `basic`, `benchmark`, `complete`)
- **Location:** [`internal/tools/smart_test/test.go:111-126`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L111-L126)
- **Complexity Analysis:**
  - `fast`: `go test -v`
  - `basic`: `go test -v -coverprofile=...` + `testquery.db` sync
  - `benchmark`: `go test -bench=... -benchmem -run=NONE`
  - `complete`: `basic` + `selene` mutation testing
- **Issues:** Arbitrarily bundles disparate concerns (unit tests, benchmarks, mutation testing, SQLite indexing). Contradictory parameter mappings (`run` parameter maps to `-run` in `fast`/`basic`, but `-bench` with `-run=NONE` in `benchmark`). Standalone `selene` and `test_query` tools already exist.
- **Recommendation:** Simplify into orthogonal execution toggles (`coverage: bool`, `benchmark: bool`, `mutation: bool`) with sensible defaults driven by `.godoctor.yaml`.

#### 3.2.2 Triple Duplication of `Runner` Interface, `stdRunner`, and `filterNoise`
- **Location:**
  - `Runner` & `stdRunner`: [`smart_test/test.go:40-69`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L40-L69), [`selene/selene.go:34-63`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L34-L63), [`testquery/testquery.go:38-67`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L38-L67)
  - `filterNoise`: [`smart_test/test.go:428-438`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L428-L438), [`selene/selene.go:119-129`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L119-L129), [`testquery/testquery.go:214-223`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L214-L223)
  - `syncTestQueryDB` / `resolveTQCommand`: [`smart_test/test.go:328-344`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L328-L344), [`testquery/testquery.go:123-135`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L123-L135), [`smart_build/build.go:247-263`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_build/build.go#L247-L263)
- **Issue:** 100% duplicated boilerplate across packages in Domain 3.
- **Recommendation:** Consolidate into `internal/runner` and `internal/text` packages.

#### 3.2.3 Subprocess Overhead vs Direct Access
- **Location:** [`internal/tools/test_query/testquery.go:150-167`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L150-L167)
- **Description:** `testquery` spawns an external process for every SQL query. When falling back to `go run`, query latency is 1–3 seconds.
- **Recommendation:** Direct embedded SQLite querying (`modernc.org/sqlite` or `database/sql`) in future iterations will reduce query latency to microseconds.

#### 3.2.4 Duplicated Parent Path Walking Algorithms in `godoc`
- **Location:** [`internal/godoc/godoc.go:58-73`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L58-L73) vs [`internal/godoc/godoc.go:94-110`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L94-L110)
- **Description:** Parent path walking is duplicated in `loadInternal` and `GetDocumentationWithFallback`.
- **Recommendation:** Consolidate `GetDocumentationWithFallback` to call `LoadWithFallback` and format the resulting `*Doc`.

#### 3.2.5 Formatting Logic Tightly Coupled with Data Model in `godoc`
- **Location:** [`internal/godoc/godoc.go:248, 428-430, 532-651`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L248)
- **Description:** AST extraction embeds markdown syntax, blockquotes (`> ℹ️ **Note:**`), and code fences directly into model fields like `Definition` and `Description`.
- **Recommendation:** Keep `*Doc` clean and delegate all markdown/JSON styling to dedicated renderers.

---

### 3.3 Hardcoded Behaviors & Magic Values

| Category | File & Line | Hardcoded Value | Impact & Proposed Config |
| :--- | :--- | :--- | :--- |
| **Tool Fallback Package** | [`selene.go:83`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L83), [`smart_test/test.go:248`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L248) | `github.com/danicat/selene/cmd/selene@latest` | Hardcoded remote package path. Expose via `tools.selene.pkg`. |
| **Tool Fallback Package** | [`testquery.go:132`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L132), [`smart_test/test.go:340`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L340) | `github.com/danicat/testquery@latest` | Hardcoded remote package path. Expose via `tools.testquery.pkg`. |
| **Database File Path** | [`testquery.go:75`](file:///Users/petruzalek/projects/godoctor/internal/tools/test_query/testquery.go#L75), [`smart_test/test.go:83`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L83) | `testquery.db` | Hardcoded database filename in workspace root. Expose via `tools.testquery.db_path`. |
| **Test Flags** | [`smart_test/test.go:148, 176`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L148) | `-v` | Hardcoded verbose flag; produces massive log payloads. Expose via `test.verbose`. |
| **Benchmark Flags** | [`smart_test/test.go:218`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L218) | `-benchmem`, `-run=NONE` | Hardcoded benchmark flags. Expose via `test.benchmark_flags`. |
| **Execution Timeouts** | [`smart_test/test.go:90`](file:///Users/petruzalek/projects/godoctor/internal/tools/smart_test/test.go#L90), [`selene.go:69`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L69) | *None* (naked `ctx`) | No default timeout (e.g. 60s/120s); hanging tests block indefinitely. Expose via `test.timeout`, `tools.selene.timeout`. |
| **Mutation Target Scope** | [`selene.go:80, 83`](file:///Users/petruzalek/projects/godoctor/internal/tools/selene/selene.go#L80) | `./...` | `selene` standalone tool forces whole-repo `./...` runs. Add `packages` param & config. |
| **Documentation Remote URL** | [`godoc.go:212, 273, 587`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L212) | `"https://pkg.go.dev/%s"` | Generates invalid links for private/enterprise repos. Expose via `docs.pkg_go_dev_url`. |
| **Domain Depth Heuristic** | [`godoc.go:61-64, 97-100`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L61-L64) | `minParts = 3` if domain has `.` | Fails on 2-part domains (e.g. `gopkg.in/yaml.v3`). Use `go list -m` module boundaries. |
| **Fuzzy Match Limits** | [`godoc.go:666, 671`](file:///Users/petruzalek/projects/godoctor/internal/godoc/godoc.go#L666) | `dist <= 2`, `max = 5` | Hardcoded distance limit and suggestion cap. Expose via `docs.fuzzy_distance`, `docs.max_suggestions`. |
| **Default Docs Format** | [`read_docs/docs.go:43`](file:///Users/petruzalek/projects/godoctor/internal/tools/read_docs/docs.go#L43) | `"markdown"` | Hardcoded markdown default. Expose via `docs.default_format`. |

---

### 3.4 Feature Flag Candidates & Sensible Defaults

Consolidated Domain 3 configuration schema for `.godoctor.yaml`:

```yaml
version: "1"

# Domain 3 External Utilities & Version Management (RFC-0001)
tools:
  selene:
    recommended_version: "latest"
    pkg: "github.com/danicat/selene/cmd/selene@latest"
    timeout: "120s"
  testquery:
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: ".godoctor/testquery.db" # Avoid workspace root pollution
    format: "table"                  # "table" | "json" | "csv"
    timeout: "30s"

# Domain 3 Test & Benchmark Engine
test:
  default_level: "basic"             # "fast" | "basic" | "benchmark" | "complete"
  timeout: "60s"
  verbose: true
  race_detector: false
  coverage_threshold: 80.0
  coverage_output_dir: ""            # Empty string uses os.TempDir()

# Domain 3 Documentation Engine
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
  default_format: "markdown"         # "markdown" | "json"
  temp_dir: ""                       # Managed long-lived fetch module cache

# Global Feature Flags Matrix
features:
  testquery_sync: true               # Auto-index test run into testquery.db
  wal_mode: true                     # Configure SQLite WAL mode & busy timeout
  mutation_testing: true             # Enable Selene in 'complete' test level
  docs_cache: true                   # In-memory LRU/sync cache for godoc
  docs_external_fetch: true          # Allow network downloading of third-party docs
  docs_vanity_resolver: true         # HTTP meta tag & regex vanity import resolution
  docs_fuzzy_suggestions: true       # Levenshtein distance typo suggestions
  version_check_hints: true          # Non-blocking upgrade hints on outdated tools
```

---

---

## 4. Domain 4: Configuration System & RFC-0001 Synthesis
*Lead: Lead Configuration Architect*
*Status: Complete*

### 4.1 Master `.godoctor.yaml` Schema

The master schema synthesizes all technical domains into a type-safe, validated YAML specification supported by Viper and Cobra:

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

### 4.2 Feature Flags Matrix

| Key (`features.<key>`) | Default | Affected Tools / Scope | Description & Rationale |
| :--- | :--- | :--- | :--- |
| `autofix` | `true` | `smart_build` | Master switch for auto-fix phase (`tidy`, `modernize`, `gofmt`, `deadcode`). |
| `mod_tidy` | `true` | `smart_build` | Executes `go mod tidy` in auto-fix phase. |
| `modernize_check` | `true` | `smart_build` | Runs modernizer AST syntax upgrade pass. |
| `deadcode_check` | `true` | `smart_build` | Scans workspace for unreachable functions using `deadcode`. |
| `format_on_build` | `true` | `smart_build` | Formats code during build pipeline. |
| `format_on_edit` | `true` | `smart_edit` | Formats modified files using `goimports` during edit transactions. |
| `vet_gate` | `true` | `smart_edit` | Executes compiler diagnostics (`go vet ./...`) before committing file edits. |
| `auto_rollback` | `true` | `smart_edit` | Reverts file modifications atomically if compiler or formatting fails. |
| `testquery_sync` | `true` | `smart_build`, `smart_test` | Syncs test results and statement coverage into `testquery.db`. |
| `version_check_hints` | `true` | `versioncheck`, CLI `check` | Emits non-blocking actionable tool upgrade recommendations. |
| `coverage_gate` | `false` | `smart_build`, `smart_test` | Fails build/test if total statement coverage < `test.coverage_threshold`. |
| `race_detector` | `false` | `smart_build`, `smart_test` | Appends `-race` detector flag to `go test` runs. |
| `strict_mode` | `false` | `smart_build`, `smart_edit` | Treats linter warnings and deadcode detections as fatal errors. |
| `remote_doc_fetch` | `true` | `read_docs`, `godoc` | Allows downloading remote packages into cache for doc inspection. |
| `vanity_resolution` | `true` | `godoc` | Auto-resolves vanity module imports via HTTP `<meta>` tags. |
| `docs_cache` | `true` | `godoc` | Enables in-memory thread-safe caching of AST package documentation. |

---

### 4.3 Version Checking & Recommendation Matrix

| Tool | Detection Command / Inspection | Recommended Version | Minimum Version | Upgrade Installation Command |
| :--- | :--- | :--- | :--- | :--- |
| **Go Toolchain** | `go version` | `>=1.24.0` | `1.22.0` | [Download Go Release](https://go.dev/dl/) |
| **`golangci-lint`** | `golangci-lint --version` | `v2.12.2` | `v1.60.0` | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` |
| **`modernize`** | `debug.ReadBuildInfo` / `modernize -h` | `latest` | `latest` | `go install golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest` |
| **`deadcode`** | `debug.ReadBuildInfo` / `deadcode -h` | `latest` | `latest` | `go install golang.org/x/tools/cmd/deadcode@latest` |
| **`selene`** | `selene --version` / `debug.ReadBuildInfo` | `latest` | `latest` | `go install github.com/danicat/selene/cmd/selene@latest` |
| **`testquery` (`tq`)** | `tq --version` / `debug.ReadBuildInfo` | `latest` | `latest` | `go install github.com/danicat/testquery@latest` |

---

### 4.4 Viper + Cobra Configuration Architecture & Precedence

1. **Precedence Hierarchy (3-Tier)**:
   $$\text{Per-Call Payload (JSON)} \succ \text{Config File } (\texttt{.godoctor.yaml}) \succ \text{Built-in Go Defaults } (\texttt{NewDefaultConfig()})$$
   - *Note*: Per ADR-0001, environment variables (`GODOCTOR_*`) are intentionally not bound to ensure deterministic, reproducible tool execution in automated multi-agent environments.

2. **Hierarchical Discovery**:
   - Current working directory $\to$ Parent directory chain up to `.git` or `go.mod` $\to$ User home directory (`$HOME/.godoctor.yaml` or `$HOME/.config/godoctor/config.yaml`).

3. **Zero-Config Guarantee**:
   - If no `.godoctor.yaml` is present, `config.NewDefaultConfig()` is returned immediately with all production defaults intact.

---

### 4.5 Zero-Breaking-Change Migration Plan

1. **Phase 1: `internal/config`**:
   - Implement typed Go structs with YAML, Mapstructure, and JSON tags.
   - Wire Viper loader, environment variable binding, and schema validation.
2. **Phase 2: `internal/versioncheck`**:
   - Implement binary detection, semver parsing/comparison, and in-memory TTL cache.
   - Implement `godoctor check` command.
3. **Phase 3: Subsystem Integration**:
   - Inject config into `smart_build`, `smart_test`, `smart_edit`, `selene`, `test_query`, and `godoc`.
   - Update error parsing, SQL validation in `safeshell`, and atomic write mechanics.
4. **Phase 4: CLI Modernization (Cobra)**:
   - Migrate CLI dispatcher to Cobra subcommands (`call`, `mcp`, `init`, `check`, `install`, `uninstall`).
   - Preserve strict JSON input for `godoctor call <tool> '<json>'`.


