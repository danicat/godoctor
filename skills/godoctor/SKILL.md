---
name: godoctor
description: >
  MANDATORY skill for all agents working on Go codebases (.go files, go.mod, Go build/test workflows). Activate this skill whenever reading, writing, editing, building, testing, refactoring, or auditing Go code. Enforces official Go Team best practices.
---

# GoDoctor Agentic Suite & Go Development Standards

GoDoctor is a specialized and optimized suite of tools carefully engineered to elevate agentic coding in Go codebases. Whenever working on Go code, agents **MUST** strictly adhere to the mandatory tooling enforcement and Go team architectural standards detailed below.

---

## 1. Mandatory Tooling Enforcement & Prohibitions

Whenever operating on Go codebases (`.go` files, `go.mod`, Go toolchains), coding agents **MUST** use GoDoctor's specialized MCP tools instead of lower-level shell commands or raw file tools.

### 🚫 Explicit Prohibitions (DO NOT DO)
- ❌ **No Lower-Level Tool Calls for Files**: DO NOT call `view_file`, `replace_file_content`, `multi_replace_file_content`, or `write_to_file` on Go files (`.go`). Use `smart_read` or `smart_edit`.
- ❌ **No Shell Execution for Build/Test/Linting**: DO NOT execute `go build`, `go test`, `go vet`, or `golangci-lint` via shell (`run_command`). Use `smart_build`.
- ❌ **No Shell Execution for Dependencies**: DO NOT execute `go get` via shell (`run_command`). Use `add_dependency`.
- ❌ **No Shell File Editing/Reading**: DO NOT use shell tools like `cat`, `sed`, `echo >`, or `tee` to read or modify Go files.
- ❌ **No Ad-Hoc Scripts**: DO NOT write temporary bash, python, or go scripts to perform build, edit, read, test, or coverage operations when a GoDoctor tool is available.

---

## 2. GoDoctor MCP Tool Reference & Usage Benefits

### ✏️ Editing (`smart_edit`)
- **Usage**: Atomic, compiler-gated multi-file editor for creating or modifying Go files.
- **Benefits**:
  - Automatically runs `gofmt`, `goimports`, and type-checks with `gopls check ./...` *before* committing edits to disk.
  - Safely rolls back changes on syntax or type errors to prevent workspace corruption and provides Levenshtein spelling suggestions.

### 🔍 Reading & Type Exploration (`smart_read` & `describe_symbol`)
- **`smart_read`**: Structure-aware source code reader.
  - **Type Enrichment**: Appends exact struct and interface definitions of referenced types in `<types>` blocks.
  - **Snippet & Outline Modes**: Read targeted line ranges or retrieve structural AST outlines (`outline=true`).
- **`describe_symbol`**: Queries `gopls` for exact declaration signatures, line coordinates, package comments, and workspace call-sites for any symbol.

### 🛠️ Build & Package Management (`smart_build` & `add_dependency`)
- **`smart_build`**: GoDoctor's automated build pipeline (`go mod tidy` -> modernization -> `gofmt` -> `go build` -> `go test` -> linter -> deadcode).
- **`add_dependency`**: Installs Go modules and fetches API documentation automatically.
- **`read_docs`**: Directly fetches package and symbol documentation from Go doc servers.

### 🧪 Quality Engineering & Testing (`test_query` & `mutation_test`)

#### SQL Coverage Analysis (`test_query`)
Execute SQL queries against `testquery.db` to isolate uncovered code paths (`SELECT * FROM all_coverage WHERE count = 0`), panics, or historic test failures.
> [!NOTE]
> For complete database schemas (`all_tests`, `all_coverage`, `test_coverage`, `all_code`, `metadata`) and SQL examples, inspect [`references/testquery-schema.md`](file:///Users/petruzalek/projects/godoctor/skills/godoctor/references/testquery-schema.md).

#### Selene Mutation Testing (`mutation_test`)
Execute `mutation_test` using Selene to introduce subtle AST code mutations (swapped operators, inverted conditionals, boundary tweaks) into Go source code.
> [!NOTE]
> For mutation operator types, mutation scores, and mutant survival workflows, inspect [`references/selene-mutation-testing.md`](file:///Users/petruzalek/projects/godoctor/skills/godoctor/references/selene-mutation-testing.md).

---

## 3. Official Go Team Development Standards (Go 1.24+)

### Project Layout Conventions
- **Flat Layout (Default for Simple Services & Libraries)**: Keep Go source files in the root directory. This is the official Go team recommendation. Avoid unnecessary abstraction layers and package stuttering.
- **Nested Layout (`cmd/` & `internal/`)**: Use nested layout ONLY when building multiple executable binaries (`cmd/<binary>/main.go`) or protecting private application logic (`internal/`).
- 🚫 **No `./pkg` Directory (Strict Rule)**:
  - **Antipattern**: Creating a root-level `./pkg` folder (e.g. `my-project/pkg/user`). It adds redundant path depth without access control.
  - **ONLY EXCEPTION**: `./pkg` is permitted **ONLY** in Kubernetes ecosystem projects (e.g. Kubernetes controllers, operators, or k8s.io ecosystem plugins) where `./pkg` is the established community convention.

### Explicit Antipattern Rejections
- ❌ **Enterprise Package Bloat**: Do NOT create `adapters/`, `ports/`, `entities/`, `controllers/`, `repositories/`, `services/`, or `usecases/` folders for Go services. Group by domain/feature or keep flat.
- ❌ **Premature Interfaces**: Provider packages must return concrete structs. Interfaces MUST be defined by consumer packages (*Accept interfaces, return structs*).
- ❌ **Package Stuttering**: Avoid repeating package names in exported symbols (`user.UserService` ❌ -> `user.Service` ✅).
- ❌ **Catch-All Utility Packages**: Do NOT create `utils`, `helpers`, `common`, or `shared` dumping grounds.
- ❌ **Global Mutable State**: Avoid package-level mutable variables (`var DB *sql.DB`). Pass dependencies explicitly via constructors.
- ❌ **Panic for Control Flow & Silent Errors**: Never use `panic()` for flow control or swallow errors (`_`). Propagate errors with context (`fmt.Errorf("context: %w", err)`) or fail fast and loudly on startup failures.

---

## 4. Modern Go Idioms (Go 1.24+)

- **HTTP Server**: Use `http.NewServeMux` path/method routing (`mux.HandleFunc("GET /items/{id}", handler)`).
- **Process Signals**: Use `signal.NotifyContext` for graceful HTTP server shutdown.
- **Structured Logging**: Standardize on `log/slog`.
- **Context Propagation**: Pass `context.Context` as the first parameter in I/O operations and respect cancellation.

---

## 5. Workflow Checklist

1. **Explore**: Use `list_files`, `smart_read` (Outline mode), or `describe_symbol` to inspect type signatures and project structure.
2. **Implement/Edit**: Use `smart_edit` to create or modify Go source files.
3. **Build & Test**: Execute `smart_build` to run full compilation, tests, formatting, linting, and deadcode analysis.
4. **Audit Quality**: Use `test_query` or `mutation_test` to verify coverage gaps and test assertion strength.
