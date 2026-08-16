// Package instructions provides dynamic system instructions for the AI agent.
package instructions

import "strings"

// Get returns the agent instructions for the server.
//
//nolint:funlen,lll
func Get() string {
	var sb strings.Builder
	sb.WriteString("# GoDoctor tooling guide\n\n")
	sb.WriteString("MULTI-ROOT WORKSPACE ENVIRONMENT: Always use absolute paths for all file, directory, or path parameters. Relative paths (such as '.', './...', or 'main.go') will be rejected with an error. Always pass the absolute path of the target workspace root or target files.\n\n")

	sb.WriteString("## Code editing\n")
	sb.WriteString("- `smart_edit`: Edits a single Go file (`filename=\"/path/file.go\", old_content=\"...\", new_content=\"...\"`). Runs `gofmt`, `goimports`, and `go vet` before writing to disk, rolling back changes if a compiler error occurs.\n\n")

	sb.WriteString("## Build and package management\n")
	sb.WriteString("- `smart_build`: Runs module tidying (`go mod tidy`), modernization passes, formatting, `go build`, `go test`, linter checks, and dead code analysis (`dir=\"/absolute/path/workspace\"`).\n")
	sb.WriteString("- `read_docs`: Fetches Go documentation and exported function signatures (`import_path=\"net/http\"`).\n\n")

	sb.WriteString("## Testing and analytics\n")
	sb.WriteString("- `smart_test`: Runs tests with selectable levels: `fast` (tests only), `basic` (tests, coverage, and testquery.db sync), `benchmark` (benchmarks), or `complete` (tests and mutation testing).\n")
	sb.WriteString("- `selene`: Runs Selene mutation testing on target packages to evaluate unit test assertions (`dir=\"/absolute/path/workspace\"`).\n")
	sb.WriteString("- `test_query`: Executes SQL queries against `testquery.db` to inspect test results and coverage metrics (`dir=\"/absolute/path/workspace\", query=\"SELECT * FROM coverage WHERE count = 0\"`).\n")

	return sb.String()
}
