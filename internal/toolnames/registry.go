// Package toolnames defines the registry of available tools for the godoctor server.
// It serves as a centralized catalog containing metadata (Name, Title, Description, Instructions)
// for each tool, which is used to advertise capabilities to the MCP client and guide the LLM.
package toolnames

// ToolDef defines the textual representation of a tool.
type ToolDef struct {
	Name        string // The canonical name (e.g. "file_create")
	Title       string // Human-readable title
	Description string // Description passed to the LLM via MCP
	Instruction string // Guidance for the system prompt
}

// Registry holds all tool definitions, keyed by Name.
var Registry = map[string]ToolDef{
	// --- FILE OPERATIONS ---
	"smart_edit": {
		Name:  "smart_edit",
		Title: "Smart Edit",
		Description: "Atomic, multi-file coordinate editing transaction. Automatically applies edits, " +
			"formats using gofmt/goimports, and runs type verification (go vet) across the entire " +
			"workspace. If the compiler check fails, all edits are completely rolled back to backup state, " +
			"and Levenshtein-based spelling suggestions are returned for misspelled symbols.",
		Instruction: "*   **`smart_edit`**: The primary tool for modifying Go files.\n" +
			"    *   **Unified Schema:** Always pass edit operations inside the `operations` array parameter.\n" +
			"    *   **Single-File Example:** `smart_edit(operations=[{\"filename\": \"/path/file.go\", " +
			"\"old_content\": \"old\", \"new_content\": \"new\"}])`\n" +
			"    *   **Multi-File Example:** `smart_edit(operations=[{\"filename\": \"/A.go\", \"new_content\": \"...\"}, " +
			"{\"filename\": \"/B.go\", \"new_content\": \"...\"}])`\n" +
			"    *   **Capabilities:** Validates syntax and type safety (gofmt/goimports/go vet) " +
			"*before* committing to disk.\n" +
			"    *   **Rollback Safety:** On compilation error, all edits roll back atomically " +
			"and return Levenshtein 'Did you mean?' suggestions.",
	},
	"smart_read": {
		Name:  "smart_read",
		Title: "Read File",
		Description: "High-density multi-file code reader with unconditional type-tag enrichment. " +
			"Automatically uses native Go AST parsing and godoc to extract and " +
			"append Go struct/interface schemas in a custom <types> block.",
		Instruction: "*   **`smart_read`**: Inspect file contents with automated type signature annotations.\n" +
			"    *   **Read All:** `smart_read(filenames=[\"/absolute/path/to/target/pkg/utils.go\"])`\n" +
			"    *   **Snippet:** `smart_read(filenames=[\"/absolute/path/to/target/pkg/utils.go\"], start_line=10, " +
			"end_line=50)` (Targeted range reading).\n" +
			"    *   **Outline:** `smart_read(filenames=[\"/absolute/path/to/target/pkg/utils.go\"], outline=true)` " +
			"(Retrieve outline via native Go AST).\n" +
			"    *   **Type-Enriched:** Append `<types>` blocks showing referenced type definitions to avoid guessing.\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST use absolute file paths in `filenames` " +
			"to ensure the correct project files are read.",
	},
	"list_files": {
		Name:  "list_files",
		Title: "List Files",
		Description: "Recursively lists files and directories in the workspace, excluding only standard " +
			"VCS directories (.git) to prevent infinite recursion, and presenting an unfiltered map of active workspace files.",
		Instruction: "*   **`list_files`**: Explore the project structure.\n" +
			"    *   **Usage:** `list_files(path=\"/absolute/path/to/target-workspace\")`\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path " +
			"of the target workspace root to `path`.",
	},

	// --- DOCS ---
	"read_docs": {
		Name:  "read_docs",
		Title: "Get Documentation",
		Description: "Retrieves authoritative Go documentation for any package or symbol. Streamlines " +
			"development by providing API signatures and usage examples directly within the workflow.",
		Instruction: "*   **`read_docs`**: Access API documentation.\n" +
			"    *   **Usage:** `read_docs(import_path=\"net/http\")`\n" +
			"    *   **Outcome:** API reference and usage guidance.",
	},

	// --- GO TOOLCHAIN ---
	"smart_build": {
		Name:  "smart_build",
		Title: "Smart Build",
		Description: "GoDoctor's specialized build pipeline: Tidy -> Modernize -> Format -> Build -> " +
			"Test -> Lint -> Deadcode. Runs `go mod tidy` -> modernization -> `gofmt` -> `go build` -> `go test` -> " +
			"linter -> deadcode to verify workspace health.",
		Instruction: "*   **`smart_build`**: GoDoctor's specialized build pipeline.\n" +
			"    *   **Usage:** `smart_build(dir=\"/absolute/path/to/target-workspace\", packages=\"./...\")`\n" +
			"    *   **Pipeline:** Automatically runs `go mod tidy` -> modernization -> `gofmt` -> " +
			"`go build` -> `go test` -> linter -> deadcode.\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the " +
			"target workspace root to `dir`.",
	},
	"add_dependency": {
		Name:  "add_dependency",
		Title: "Add Dependency",
		Description: "Manages Go module installation and manifest updates. Consolidates the workflow " +
			"by immediately returning the public API documentation for the installed packages.",
		Instruction: "*   **`add_dependency`**: Install dependencies and fetch documentation.\n" +
			"    *   **Usage:** `add_dependency(dir=\"/absolute/path/to/target-workspace\", " +
			"packages=[\"github.com/go-chi/chi/v5@latest\"])`\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path " +
			"of the target workspace root to `dir`.",
	},
	"project_init": {
		Name:  "project_init",
		Title: "Initialize Project",
		Description: "Bootstraps a new Go project by creating the directory, initializing the " +
			"Go module, and installing essential dependencies. Layout-agnostic and does not run compilation.",
		Instruction: "*   **`project_init`**: Bootstrap a new Go project.\n" +
			"    *   **Usage:** `project_init(path=\"/absolute/path/to/new-app\", " +
			"module_path=\"github.com/user/new-app\", dependencies=[\"github.com/go-chi/chi/v5\"])`\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path " +
			"of the target directory to `path`.",
	},

	// --- TESTING ---
	"mutation_test": {
		Name:  "mutation_test",
		Title: "Mutation Test",
		Description: "Runs mutation testing using Selene. Introduces small code mutations " +
			"(flipped conditions, swapped operators) and checks if existing tests catch them, " +
			"objectively measuring test suite quality.",
		Instruction: "*   **`mutation_test`**: Verify test quality with mutation testing.\n" +
			"    *   **Usage:** `mutation_test(dir=\"/absolute/path/to/target-workspace\")`\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path " +
			"of the target workspace root to `dir`.",
	},
	"test_query": {
		Name:  "test_query",
		Title: "Test Query",
		Description: "Queries Go test results and coverage data using SQL via testquery (tq). " +
			"Uses a persistent SQLite database (testquery.db) to avoid re-running tests on every query. " +
			"Set rebuild=true after code changes to refresh the database. Available tables: all_tests " +
			"(time, action, package, test, elapsed, output), all_coverage (package, file, start_line, " +
			"start_col, end_line, end_col, stmt_num, count, function_name), test_coverage (test_name, " +
			"package, file, start_line, start_col, end_line, end_col, stmt_num, count, function_name), " +
			"all_code (package, file, line_number, content), metadata (key, value).",
		Instruction: "*   **`test_query`**: Query test results with SQL.\n" +
			"    *   **Usage:** `test_query(dir=\"/absolute/path/to/target-workspace\", " +
			"query=\"SELECT * FROM all_coverage WHERE count = 0\")`\n" +
			"    *   **Caching:** Uses a persistent `testquery.db` file. First call builds it automatically. " +
			"Set `rebuild=true` after code changes.\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the " +
			"target workspace root to `dir`.",
	},

	// --- NAVIGATION ---
	"describe_symbol": {
		Name:  "describe_symbol",
		Title: "Describe Symbol",
		Description: "Returns complete native Go AST and godoc-backed symbol information including exact " +
			"coordinates, declaration signature, package comments, and workspace references.",
		Instruction: "*   **`describe_symbol`**: Track declaration and usage reference coordinates " +
			"of a symbol.\n" +
			"    *   **Usage:** `describe_symbol(filename=\"/absolute/path/to/target/file.go\", " +
			"line=25, col=10)`\n" +
			"    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the " +
			"target file to `filename`.",
	},
}
