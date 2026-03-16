// Package testquery implements the test query tool using tq.
package testquery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["test_query"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, toolHandler)
}

// Params defines the input parameters.
type Params struct {
	Dir     string `json:"dir,omitempty" jsonschema:"Directory to analyze (default: current)"`
	Query   string `json:"query" jsonschema:"SQL query to run against test results (e.g. SELECT * FROM all_tests WHERE action = 'fail')"`
	Pkg     string `json:"pkg,omitempty" jsonschema:"Go package pattern to analyze (default: ./...)"`
	Rebuild bool   `json:"rebuild,omitempty" jsonschema:"Force rebuild of the test database before querying. Use after code changes. First call always builds."`
}

const dbFile = "testquery.db"

func toolHandler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return errorResult("query cannot be empty"), nil, nil
	}

	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	absDir, err := roots.Global.Validate(dir)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	pkg := args.Pkg
	if pkg == "" {
		pkg = "./..."
	}

	dbPath := filepath.Join(absDir, dbFile)

	// Build the DB if it doesn't exist or if rebuild is requested
	if args.Rebuild || !fileExists(dbPath) {
		buildCmd := exec.CommandContext(ctx, "go", "run", "github.com/danicat/testquery@latest",
			"build", "--pkg", pkg, "--output", dbFile)
		buildCmd.Dir = absDir
		out, buildErr := buildCmd.CombinedOutput()
		buildOutput := filterNoise(string(out))

		if buildErr != nil {
			// Build may fail if tests fail, but the DB might still be usable
			if !fileExists(dbPath) {
				return errorResult(fmt.Sprintf("failed to build test database: %v\n%s", buildErr, buildOutput)), nil, nil
			}
			// DB exists despite test failures — continue with query but warn
		}
	}

	// Query the persistent DB (offline mode)
	cmd := exec.CommandContext(ctx, "go", "run", "github.com/danicat/testquery@latest",
		"query", "--db", dbFile, "--format", "table", args.Query)
	cmd.Dir = absDir
	out, runErr := cmd.CombinedOutput()

	output := filterNoise(string(out))

	if runErr != nil && output == "" {
		return errorResult(fmt.Sprintf("test query failed: %v", runErr)), nil, nil
	}

	if runErr != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("⚠️ Query completed with warnings:\n\n%s", output)},
			},
		}, nil, nil
	}

	if output == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Query returned no results."},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func filterNoise(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var filtered []string
	for _, line := range lines {
		if strings.HasPrefix(line, "go: downloading ") || strings.Contains(line, "exit status") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
