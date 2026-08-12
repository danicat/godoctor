// Package instructions provides dynamic system instructions for the AI agent.
package instructions

import "strings"

// Get returns the agent instructions for the server.
//
//nolint:funlen,lll
func Get() string {
	var sb strings.Builder
	sb.WriteString("# Go Smart Tooling Guide\n\n")
	sb.WriteString("⚠️ **CRITICAL: MULTI-ROOT WORKSPACE ENVIRONMENT**\n")
	sb.WriteString("This environment has multiple project roots registered. To ensure that your requests are performed on the correct target project (and do not fallback to the GoDoctor project folder), **YOU MUST ALWAYS USE ABSOLUTE PATHS** for all file, directory, or path parameters. Never pass relative paths (e.g., '.', '', or relative paths like 'pkg/main.go'). Always pass the absolute path of the target workspace root or files.\n\n")

	sb.WriteString("### 🔍 Navigation: Save Tokens & Context\n")
	sb.WriteString("*   **`smart_read`**: Inspect file contents with automated type signature annotations.\n")
	sb.WriteString("    *   **Multi-File Read:** `smart_read(filenames=[\"/absolute/path/A.go\", \"/absolute/path/B.go\"])` (Batch read multiple files in a single turn).\n")
	sb.WriteString("    *   **Read All:** `smart_read(filenames=[\"/absolute/path/to/target/pkg/utils.go\"])\n")
	sb.WriteString("    *   **Snippet:** `smart_read(filenames=[\"/absolute/path/to/target/pkg/utils.go\"], start_line=10, end_line=50)` (Targeted range reading).\n")
	sb.WriteString("    *   **Outline:** `smart_read(filenames=[\"/absolute/path/to/target/pkg/utils.go\"], outline=true)` (Retrieve outline via native Go AST).\n")
	sb.WriteString("    *   **Type-Enriched:** Append `<types>` blocks showing referenced type definitions to avoid guessing.\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST use absolute file paths in `filenames` to ensure the correct project files are read.\n")
	sb.WriteString("*   **`list_files`**: Explore the project structure.\n")
	sb.WriteString("    *   **Usage:** `list_files(path=\"/absolute/path/to/target-workspace\")`\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the target workspace root to `path`.\n\n")

	sb.WriteString("### ✏️ Editing: Ensure Safety\n")
	sb.WriteString("*   **`smart_edit`**: The primary tool for single file modifications.\n")
	sb.WriteString("    *   **Single-File Example:** `smart_edit(filename=\"/path/file.go\", old_content=\"old\", new_content=\"new\")`\n")
	sb.WriteString("    *   **Capabilities:** Validates syntax and type safety (gofmt/goimports/go vet) *before* committing to disk.\n")
	sb.WriteString("    *   **Rollback Safety:** On compilation error, the edit rolls back atomically and returns Levenshtein 'Did you mean?' suggestions.\n")
	sb.WriteString("*   **`smart_multi_edit`**: The tool for atomic multi-file batch edits.\n")
	sb.WriteString("    *   **Batch Example:** `smart_multi_edit(operations=[{\"filename\": \"/A.go\", \"new_content\": \"...\"}, {\"filename\": \"/B.go\", \"new_content\": \"...\"}])`\n")
	sb.WriteString("    *   **Capabilities:** Validates type safety across all modified files simultaneously.\n\n")

	sb.WriteString("### 🛠️ Utilities\n")
	sb.WriteString("*   **`smart_build`**: GoDoctor's specialized build pipeline.\n")
	sb.WriteString("    *   **Usage:** `smart_build(dir=\"/absolute/path/to/target-workspace\", packages=\"./...\")`\n")
	sb.WriteString("    *   **Pipeline:** Automatically runs `go mod tidy` -> modernization -> `gofmt` -> `go build` -> `go test` -> linter -> deadcode.\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the target workspace root to `dir`.\n")
	sb.WriteString("*   **`read_docs`**: Access API documentation.\n")
	sb.WriteString("    *   **Usage:** `read_docs(import_path=\"net/http\")`\n")
	sb.WriteString("    *   **Outcome:** API reference and usage guidance.\n")
	sb.WriteString("*   **`add_dependencies`**: Install dependencies and fetch documentation.\n")
	sb.WriteString("    *   **Usage:** `add_dependencies(dir=\"/absolute/path/to/target-workspace\", packages=[\"github.com/go-chi/chi/v5@latest\"])\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the target workspace root to `dir`.\n\n")

	sb.WriteString("### 🧪 Testing\n")
	sb.WriteString("*   **`smart_test`**: GoDoctor's specialized test runner.\n")
	sb.WriteString("    *   **Usage:** `smart_test(dir=\"/absolute/path/to/target-workspace\", packages=\"./...\", level=\"basic\")`\n")
	sb.WriteString("    *   **Modes:** `level=\"fast\"` (tests only), `level=\"basic\"` (tests + coverage + testquery.db sync), `level=\"benchmark\"` (benchmarks), `level=\"complete\"` (tests + mutation testing).\n")
	sb.WriteString("    *   **Filter:** `run=\"TestValidateToken\"` (filter specific test or benchmark functions).\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the target workspace root to `dir`.\n")
	sb.WriteString("*   **`mutation_test`**: Verify test quality with mutation testing.\n")
	sb.WriteString("    *   **Usage:** `mutation_test(dir=\"/absolute/path/to/target-workspace\")`\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the target workspace root to `dir`.\n")
	sb.WriteString("*   **`test_query`**: Query test results with SQL.\n")
	sb.WriteString("    *   **Usage:** `test_query(dir=\"/absolute/path/to/target-workspace\", query=\"SELECT * FROM all_coverage WHERE count = 0\")`\n")
	sb.WriteString("    *   **Caching:** Uses a persistent `testquery.db` file. First call builds it automatically. Set `rebuild=true` after code changes.\n")
	sb.WriteString("    *   **CRITICAL:** In multi-root workspaces, you MUST pass the absolute path of the target workspace root to `dir`.\n")

	return sb.String()
}
