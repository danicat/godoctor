// Package testquery implements the test query tool using tq.
package testquery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_query",
		Title:       "Test Query",
		Description: "Queries Go test results and coverage data using SQL via testquery (tq). Uses a persistent SQLite database (testquery.db) to avoid re-running tests on every query. Set rebuild=true after code changes to refresh the database. Available tables: all_tests (time, action, package, test, elapsed, output), all_coverage (package, file, start_line, start_col, end_line, end_col, stmt_num, count, function_name), test_coverage (test_name, package, file, start_line, start_col, end_line, end_col, stmt_num, count, function_name), all_code (package, file, line_number, content), metadata (key, value).",
	}, toolHandler)
}

// Params defines the input parameters.
type Params struct {
	//nolint:lll
	Dir string `json:"dir,omitempty" jsonschema:"The absolute directory path to analyze. Always pass absolute paths in multi-root workspaces."`
	//nolint:lll
	Query string `json:"query" jsonschema:"SQL query to run against test results (e.g. SELECT * FROM all_tests WHERE action = 'fail')"`
	Pkg   string `json:"pkg,omitempty" jsonschema:"Go package pattern to analyze (default: ./...)"`
	//nolint:lll
	Rebuild bool `json:"rebuild,omitempty" jsonschema:"Force rebuild of the test database before querying. Use after code changes. First call always builds."`
}

const (
	dbFile     = "testquery.db"
	flagOutput = "--output"
	flagDB     = "--db"
	flagPkg    = "--pkg"
	flagFormat = "--format"
	flagTable  = "table"
	cmdBuild   = "build"
	cmdQuery   = "query"
)

func toolHandler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	absDir, err := validateParams(req, args)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	dbPath := filepath.Join(absDir, dbFile)

	if args.Rebuild || !fileExists(dbPath) {
		if errRes := buildDB(ctx, absDir, args, dbPath); errRes != nil {
			return errRes, nil, nil
		}
	}

	return runQuery(ctx, absDir, args.Query)
}

func validateParams(_ *mcp.CallToolRequest, args Params) (string, error) {
	if args.Query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	return filepath.Abs(dir)
}

func buildDB(ctx context.Context, absDir string, args Params, dbPath string) *mcp.CallToolResult {
	pkg := args.Pkg
	if pkg == "" {
		pkg = "./..."
	}

	var tqCmd string
	var tqArgs []string
	if _, err := exec.LookPath("testquery"); err == nil {
		tqCmd = "testquery"
		tqArgs = []string{cmdBuild, flagPkg, pkg, flagOutput, dbFile}
	} else if _, err := exec.LookPath("tq"); err == nil {
		tqCmd = "tq"
		tqArgs = []string{cmdBuild, flagPkg, pkg, flagOutput, dbFile}
	} else {
		tqCmd = "go"
		tqArgs = []string{"run", "github.com/danicat/testquery@latest", cmdBuild, flagPkg, pkg, flagOutput, dbFile}
	}

	buildCmd, err := safeshell.CommandContext(ctx, tqCmd, tqArgs...)
	if err != nil {
		return errorResult(fmt.Sprintf("secure execution validation failed: %v", err))
	}
	buildCmd.Dir = absDir
	out, buildErr := buildCmd.CombinedOutput()
	buildOutput := filterNoise(string(out))

	if buildErr != nil {
		if !fileExists(dbPath) {
			return errorResult(fmt.Sprintf("failed to build test database: %v\n%s", buildErr, buildOutput))
		}
	}
	return nil
}

func runQuery(ctx context.Context, absDir, query string) (*mcp.CallToolResult, any, error) {
	var tqCmd string
	var tqArgs []string
	if _, err := exec.LookPath("testquery"); err == nil {
		tqCmd = "testquery"
		tqArgs = []string{cmdQuery, flagDB, dbFile, flagFormat, flagTable, query}
	} else if _, err := exec.LookPath("tq"); err == nil {
		tqCmd = "tq"
		tqArgs = []string{cmdQuery, flagDB, dbFile, flagFormat, flagTable, query}
	} else {
		tqCmd = "go"
		//nolint:lll
		tqArgs = []string{"run", "github.com/danicat/testquery@latest", cmdQuery, flagDB, dbFile, flagFormat, flagTable, query}
	}

	cmd, err := safeshell.CommandContext(ctx, tqCmd, tqArgs...)
	if err != nil {
		return errorResult(fmt.Sprintf("secure execution validation failed: %v", err)), nil, nil
	}
	cmd.Dir = absDir
	out, runErr := cmd.CombinedOutput()

	output := filterNoise(string(out))

	if runErr != nil && output == "" {
		return errorResult(fmt.Sprintf("test query failed: %v", runErr)), nil, nil
	}

	if runErr != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("⚠️ Query completed with warnings:\n%v\n%s", runErr, output)},
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
