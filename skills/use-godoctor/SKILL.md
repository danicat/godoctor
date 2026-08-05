---
name: use-godoctor
description: >
  MANDATORY skill for all agents working on Go (golang) codebases (.go files, go.mod, Go toolchains). Activate this skill whenever reading, writing, editing, building, testing, refactoring, or auditing Go code. Teaches you how to use GoDoctor's high-density tools (smart_read, smart_edit, smart_build, test_query, etc.) with examples and provides modern Go 1.24+ architectural standards.
---

# GoDoctor: Agentic Development Suite & Go Standards

**GoDoctor** is a specialized suite of high-density MCP tools purpose-built for AI agents working in Go codebases. 

Unlike generic text editors or raw shell commands, GoDoctor provides **compiler-gated safety, atomic transactional multi-file editing, instant type enrichment, and SQL-powered quality auditing**. It ensures that your Go edits are always syntactically correct, formatted, and verified before hitting disk.

---

## 1. Why Use GoDoctor? (Core Capabilities)

* 🛡️ **Compiler-Gated Safety**: Edits are type-checked with `go vet`, formatted with `gofmt` and `goimports`, and verified before saving. If an edit introduces a syntax or type error, GoDoctor automatically rolls back the change and suggests fixes.
* 📦 **Atomic Multi-File Transactions**: Modify multiple files across package boundaries in a single transaction. Either all changes compile cleanly together, or none are applied.
* 🧠 **Type-Enriched Code Reading**: Reading files with `smart_read` automatically appends `<types>` blocks containing struct and interface definitions for referenced symbols, eliminating context guessing.
* ⚡ **Zero-Daemon Speed**: Built natively on Go standard library tools (`go/ast`, `go/parser`, `go/doc`, `godoc`) for sub-millisecond execution with zero external daemon overhead.
* 📊 **SQL-Powered Coverage & Quality**: Interrogate test execution, failures, and uncovered lines using SQL queries against a lightweight SQLite database (`testquery.db`).

---

## 2. Tool Reference & Examples

### ✏️ 1. `smart_edit` — Atomic Compiler-Verified Editor
Performs atomic code edits across one or more files. Automatically formats code with `gofmt` and `goimports`, runs `go vet`, and rolls back changes if errors occur.

* **Always use the `operations` array parameter** for all modification requests.

#### Single-File Edit Request:
```json
{
  "operations": [
    {
      "filename": "/Users/user/projects/app/main.go",
      "old_content": "func main() {\n\tprintln(\"hello\")\n}",
      "new_content": "func main() {\n\tfmt.Println(\"hello world\")\n}",
      "start_line": 1,
      "end_line": 20
    }
  ]
}
```

#### Multi-File Atomic Transaction Request:
```json
{
  "operations": [
    {
      "filename": "/Users/user/projects/app/pkg/user/user.go",
      "old_content": "type User struct {\n\tID string\n}",
      "new_content": "type User struct {\n\tID    string\n\tEmail string\n}"
    },
    {
      "filename": "/Users/user/projects/app/main.go",
      "old_content": "u := &user.User{ID: \"1\"}",
      "new_content": "u := &user.User{ID: \"1\", Email: \"user@example.com\"}"
    }
  ]
}
```

#### Sample Output (Successful Edit):
```text
Successfully edited files: main.go, user.go
```

#### Sample Output (Compiler Rollback & Spelling Suggestion):
```text
go vet verification failed: exit status 1
Output:
# github.com/user/app/internal/config
internal/config/config.go:67:7: c.DisabledToolsss undefined (type *Config has no field or method DisabledToolsss)

Possible spelling suggestions for 'DisabledToolsss':
  - DisabledTools (similarity: 0.81)
```

---

### 🔍 2. `smart_read` — Structure-Aware Reader with Type Enrichment
Reads Go source files and automatically appends a `<types>` block containing definitions of structs and interfaces referenced in the file.

#### Request:
```json
{
  "filenames": ["/Users/user/projects/app/internal/config/config.go"]
}
```

#### Sample Output:
```go
# File: /Users/user/projects/app/internal/config/config.go

   1 | package config
   2 | 
   3 | import (
   4 | 	"flag"
   5 | 	"sync"
   6 | )
   7 | 
   8 | type Config struct {
   9 | 	mu         sync.RWMutex
  10 | 	ListenAddr string
  11 | }

<types>
Config struct {
	mu         sync.RWMutex
	ListenAddr string
}

// sync.RWMutex
type RWMutex struct {
	// contains filtered or unexported fields
}

// flag.NewFlagSet
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet
</types>
```

#### AST Outline Mode Request (`outline: true`):
```json
{
  "filenames": ["/Users/user/projects/app/internal/config/config.go"],
  "outline": true
}
```

#### Sample Output (Outline View):
```go
# File: /Users/user/projects/app/internal/config/config.go (Outline)

Config Struct 13:6-20:2
	ListenAddr Field 15:2-15:19
	Version Field 16:2-16:14
Load Function 23:6-59:2
(*Config).IsToolEnabled Method 62:1-78:2

## Third-Party Imports
- "flag"
- "sync"
```

---

### 🛠️ 3. `smart_build` — Full Workspace Build & Quality Pipeline
Executes GoDoctor's end-to-end quality pipeline in order: `go mod tidy` ➡️ modernization ➡️ `gofmt` ➡️ `go build` ➡️ `go test` ➡️ linter ➡️ deadcode analysis.

#### Request:
```json
{
  "dir": "/Users/user/projects/app",
  "packages": "./..."
}
```

#### Sample Output:
```markdown
# Smart Build Report (`./...`)

### 🔧 Auto-Fix & Modernize:
  - ✅ Go Mod Tidy: SUCCESS
  - ✅ Go Modernizer: SUCCESS (No issues found)
  - ✅ Go Code Formatter: SUCCESS

### 🛠  Build: ✅ PASS

### 🧪 Tests: ✅ PASS

#### 📊 Coverage
✅ **Total Project Coverage**: 58.4%
- **Packages**:
  - `github.com/user/app/cmd/app`: 21.4%
  - `github.com/user/app/internal/config`: 96.2%
  - `github.com/user/app/internal/server`: 50.0%

### 🧹 Lint: ✅ CLEAN
```

---

### 📖 4. `read_docs` — Standard & Workspace Documentation
Retrieves authoritative documentation, type signatures, and usage examples for any standard library package or workspace module symbol by name (`import_path` + `symbol_name`).

#### Request:
```json
{
  "import_path": "net/http",
  "symbol_name": "Server"
}
```

#### Sample Output:
```markdown
# Package net/http — type Server

```go
type Server struct {
	Addr              string
	Handler           Handler
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}
```

A Server defines parameters for running an HTTP server.

Methods:
- `func (srv *Server) ListenAndServe() error`
- `func (srv *Server) Shutdown(ctx context.Context) error`
```

---

### 📊 5. `test_query` — SQL-Powered Test & Coverage Audit
Executes SQL queries against the local `testquery.db` SQLite database to analyze test coverage gaps, historic failures, and untested functions.

#### Request (Find Untested Functions):
```json
{
  "dir": "/Users/user/projects/app",
  "query": "SELECT package, file, function_name, start_line FROM all_coverage WHERE count = 0 LIMIT 5"
}
```

#### Sample Output:
```text
+-----------------------+-----------------------------+---------------+------------+
| PACKAGE               | FILE                        | FUNCTION_NAME | START_LINE |
+-----------------------+-----------------------------+---------------+------------+
| github.com/user/app   | cmd/app/main.go             | main          |         37 |
| github.com/user/app   | internal/server/server.go   | Shutdown      |         85 |
+-----------------------+-----------------------------+---------------+------------+
```

---

### 📦 6. `add_dependency` — Module Manager & Doc Integrator
Installs Go modules, updates `go.mod`/`go.sum`, and immediately returns API documentation for the installed packages.

#### Request:
```json
{
  "dir": "/Users/user/projects/app",
  "packages": ["github.com/go-chi/chi/v5@latest"]
}
```

#### Sample Output:
```markdown
Successfully installed packages: github.com/go-chi/chi/v5@latest
Output:
go: downloading github.com/go-chi/chi/v5 v5.0.12
go: added github.com/go-chi/chi/v5 v5.0.12

---
### Documentation for github.com/go-chi/chi/v5
Package chi is a lightweight, idiomatic and composable router for building Go HTTP services.
```

---

### 📂 7. `list_files` — VCS-Aware Workspace Navigator
Recursively maps workspace files while automatically ignoring `.git` and build artifacts.

#### Request:
```json
{
  "path": "/Users/user/projects/app",
  "depth": 2
}
```

#### Sample Output:
```text
Listing files in /Users/user/projects/app (Depth: 2)

cmd/
cmd/app/main.go
go.mod
go.sum
internal/
internal/config/config.go
internal/server/server.go
README.md

Found 6 files, 3 directories.
```

---

### 🚀 8. `project_init` — Go Project Bootstrapper
Bootstraps a clean, idiomatic Go module with initial dependencies.

#### Request:
```json
{
  "path": "/Users/user/projects/new-service",
  "module_path": "github.com/myorg/new-service",
  "dependencies": ["github.com/go-chi/chi/v5"]
}
```

#### Sample Output:
```text
Successfully initialized Go module 'github.com/myorg/new-service' at /Users/user/projects/new-service
Installed dependencies: github.com/go-chi/chi/v5
```

---

### 🧪 9. `mutation_test` — Selene Mutation Testing
Runs Selene mutation testing on a specific target directory or package. Automatically introduces subtle AST code mutations (swapped operators, inverted logic, boundary tweaks) to test whether existing unit tests catch the introduced bugs.

#### Request (Target Package Directory):
```json
{
  "dir": "/Users/user/projects/app/internal/config"
}
```

#### Sample Output:
```text
✅ Mutation testing results:

Total mutations: 10
Killed:          10
Timeouts:        0
Survived:        0
Uncovered:       0

Total tests:     5
Good tests:      2
Bad tests:       3

Mutation Score:     100.00% (killed/total mutations)
Test Quality Score: 40.00% (good tests/total tests)
```

---

## 3. Official Go Team Development Standards (Go 1.24+)

### Project Layout Conventions
* **Flat Layout (Default for Simple Services & Libraries)**: Keep Go source files in the root directory. This is the official Go team recommendation. Avoid unnecessary abstraction layers and package stuttering.
* **Nested Layout (`cmd/` & `internal/`)**: Use nested layout ONLY when building multiple executable binaries (`cmd/<binary>/main.go`) or protecting private application logic (`internal/`).
* **No `./pkg` Directory**: Avoid root-level `./pkg` folders unless working in Kubernetes ecosystem projects where `./pkg` is the established community convention.

### Idiomatic Go Design
* ❌ **Avoid Architecture Bloat**: Do NOT create `adapters/`, `ports/`, `entities/`, `controllers/`, `repositories/`, `services/`, or `usecases/` folders for Go services. Group by domain/feature or keep flat.
* ❌ **No Premature Interfaces**: Return concrete structs from provider packages. Consumer packages define interfaces where needed (*Accept interfaces, return structs*).
* ❌ **No Package Stuttering**: Avoid repeating package names in exported symbols (`user.UserService` ❌ ➡️ `user.Service` ✅).
* ❌ **No Catch-All Utility Packages**: Avoid `utils`, `helpers`, `common`, or `shared` dumping grounds.
* ❌ **Avoid Global Mutable State**: Avoid package-level mutable variables (`var DB *sql.DB`). Pass dependencies explicitly via constructors.
* ❌ **No Panic for Control Flow**: Use explicit error returns wrapped with context (`fmt.Errorf("context: %w", err)`).

### Modern Go Idioms (Go 1.24+)
* **HTTP Routing**: Use `http.NewServeMux` path/method routing (`mux.HandleFunc("GET /items/{id}", handler)`).
* **Signal Handling**: Use `signal.NotifyContext` for graceful HTTP server shutdown.
* **Structured Logging**: Standardize on `log/slog`.
* **Context Propagation**: Pass `context.Context` as the first parameter in I/O operations and respect cancellation.

---

## 4. Recommended Workflow

1. **Explore**: Map files with `list_files`, inspect AST structure with `smart_read(outline=true)`, or query docs with `read_docs`.
2. **Understand Types**: Use `smart_read` to read relevant source files with automatic `<types>` definition enrichment.
3. **Implement Edits**: Modify code using `smart_edit` with the `edits` array. Enjoy automatic formatting (`gofmt`/`goimports`), `go vet` verification, and rollback safety.
4. **Build & Test**: Run `smart_build` to verify formatting, compilation, test passes, linting, and deadcode detection in one step.
5. **Audit Quality**: Run `test_query` or `mutation_test` to verify coverage strength and uncover edge cases.
