// Package instructions provides dynamic system instructions for the AI agent.
package instructions

import "strings"

type toolDoc struct {
	canonical string
	aliases   []string
	category  string
	entry     string
}

const (
	categoryCodeEditing         = "Code editing"
	categoryBuildAndPkg         = "Build and package management"
	categoryTestingAndAnalytics = "Testing and analytics"
)

var allTools = []toolDoc{
	{
		canonical: "smart_edit",
		aliases:   []string{"edit"},
		category:  categoryCodeEditing,
		entry: "- `smart_edit`: Edits a single Go file (`filename=\"/path/file.go\", old_content=\"...\", " +
			"new_content=\"...\"`). Runs `gofmt` and `goimports` and verifies with compiler diagnostics (`go vet`), " +
			"rolling back on error.\n",
	},
	{
		canonical: "smart_build",
		aliases:   []string{"build"},
		category:  categoryBuildAndPkg,
		entry: "- `smart_build`: Runs module tidying (`go mod tidy`), modernization passes, formatting, `go build`, " +
			"`go test`, linter checks, and dead code analysis (`dir=\"/absolute/path/workspace\"`).\n",
	},
	{
		canonical: "read_docs",
		aliases:   []string{"docs"},
		category:  categoryBuildAndPkg,
		entry: "- `read_docs`: Fetches Go documentation and exported function signatures " +
			"(`import_path=\"net/http\"`).\n",
	},
	{
		canonical: "smart_test",
		aliases:   []string{"test"},
		category:  categoryTestingAndAnalytics,
		entry: "- `smart_test`: Runs tests with selectable levels: `fast` (tests only), " +
			"`basic` (tests, coverage, and testquery.db sync), `benchmark` (benchmarks), " +
			"or `complete` (tests and mutation testing).\n",
	},
	{
		canonical: "selene",
		aliases:   []string{"mutation_test", "mutation"},
		category:  categoryTestingAndAnalytics,
		entry: "- `selene`: Runs Selene mutation testing on target packages to evaluate unit test assertions " +
			"(`dir=\"/absolute/path/workspace\"`).\n",
	},
	{
		canonical: "test_query",
		aliases:   []string{"tq", "testquery"},
		category:  categoryTestingAndAnalytics,
		entry: "- `test_query`: Executes SQL queries against `testquery.db` to inspect test results " +
			"and coverage metrics (`dir=\"/absolute/path/workspace\", query=\"SELECT * FROM all_coverage " +
			"WHERE count = 0\"`).\n",
	},
}

var categories = []string{
	categoryCodeEditing,
	categoryBuildAndPkg,
	categoryTestingAndAnalytics,
}

// Get returns the default agent instructions for the server with all tools.
func Get() string {
	return GetForTools(nil)
}

// GetForTools returns dynamic agent instructions tailored for the specified list of tool names.
// If toolNames is nil or empty, it returns instructions for all available tools.
func GetForTools(toolNames []string) string {
	var filter map[string]bool
	if len(toolNames) > 0 {
		filter = make(map[string]bool, len(toolNames))
		for _, name := range toolNames {
			filter[strings.ToLower(strings.TrimSpace(name))] = true
		}
	}

	var sb strings.Builder
	sb.WriteString("# GoDoctor tooling guide\n\n")
	sb.WriteString("MULTI-ROOT WORKSPACE ENVIRONMENT: Always use absolute paths for all file, directory, " +
		"or path parameters. Relative paths (such as '.', './...', or 'main.go') will be rejected with an error. " +
		"Always pass the absolute path of the target workspace root or target files.\n\n")

	for _, cat := range categories {
		var catTools []toolDoc
		for _, t := range allTools {
			if t.category != cat {
				continue
			}
			if filter == nil || isToolEnabled(t, filter) {
				catTools = append(catTools, t)
			}
		}

		if len(catTools) > 0 {
			sb.WriteString("## ")
			sb.WriteString(cat)
			sb.WriteString("\n")
			for _, t := range catTools {
				sb.WriteString(t.entry)
			}
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func isToolEnabled(t toolDoc, filter map[string]bool) bool {
	if filter[strings.ToLower(t.canonical)] {
		return true
	}
	for _, alias := range t.aliases {
		if filter[strings.ToLower(alias)] {
			return true
		}
	}
	return false
}
