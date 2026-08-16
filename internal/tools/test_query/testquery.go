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
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	//nolint:lll
	Dir string `json:"dir" jsonschema:"The absolute directory path to analyze. Required. Relative paths are rejected."`
	//nolint:lll
	Query string `json:"query" jsonschema:"SQL query to run against test results (e.g. SELECT * FROM all_tests WHERE action = 'fail')"`
	Pkg   string `json:"pkg,omitempty" jsonschema:"Go package pattern to analyze (default: ./...)"`
	//nolint:lll
	Rebuild bool `json:"rebuild,omitempty" jsonschema:"Force rebuild of the test database before querying. Use after code changes. First call always builds."`
}

// Runner defines the interface for running commands.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) error
	RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
	LookPath(file string) (string, error)
}

type stdRunner struct{}

func (r *stdRunner) Run(ctx context.Context, dir, name string, args ...string) error {
	cmd, err := safeshell.CommandContext(ctx, name, args...)
	if err != nil {
		return err
	}
	cmd.Dir = dir
	return cmd.Run()
}

func (r *stdRunner) RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd, err := safeshell.CommandContext(ctx, name, args...)
	if err != nil {
		return "", err
	}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *stdRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// CommandRunner is used to execute CLI commands.
var CommandRunner Runner = &stdRunner{}

const (
	cmdTestQuery = "testquery"
	cmdTQ        = "tq"
	dbFile       = "testquery.db"
	flagOutput   = "--output"
	flagDB       = "--db"
	flagPkg      = "--pkg"
	flagFormat   = "--format"
	flagTable    = "table"
	cmdBuild     = "build"
	cmdQuery     = "query"
)

// Handler handles the test_query tool execution.
func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
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

	if strings.TrimSpace(args.Dir) == "" || !filepath.IsAbs(args.Dir) {
		return "", fmt.Errorf("dir is required and must be an absolute path")
	}

	return filepath.Clean(args.Dir), nil
}

func buildDB(ctx context.Context, absDir string, args Params, dbPath string) *mcp.CallToolResult {
	pkg := args.Pkg
	if pkg == "" {
		pkg = "./..."
	}

	var tqCmd string
	var tqArgs []string
	if _, err := CommandRunner.LookPath(cmdTestQuery); err == nil {
		tqCmd = cmdTestQuery
		tqArgs = []string{cmdBuild, flagPkg, pkg, flagOutput, dbFile}
	} else if _, err := CommandRunner.LookPath(cmdTQ); err == nil {
		tqCmd = cmdTQ
		tqArgs = []string{cmdBuild, flagPkg, pkg, flagOutput, dbFile}
	} else {
		tqCmd = "go"
		tqArgs = []string{
			"run", "github.com/danicat/testquery@latest",
			cmdBuild, flagPkg, pkg, flagOutput, dbFile,
		}
	}

	out, buildErr := CommandRunner.RunWithOutput(ctx, absDir, tqCmd, tqArgs...)
	buildOutput := filterNoise(out)

	if buildErr != nil {
		if !fileExists(dbPath) {
			hint := "**HINT:** Ensure Go tests compile cleanly. " +
				"Run `smart_build` or `smart_test` first to identify any compilation or syntax errors."
			return errorResult(fmt.Sprintf("failed to build test database: %v\n%s\n\n%s", buildErr, buildOutput, hint))
		}
	}
	return nil
}

func runQuery(ctx context.Context, absDir, query string) (*mcp.CallToolResult, any, error) {
	var tqCmd string
	var tqArgs []string
	if _, err := CommandRunner.LookPath(cmdTestQuery); err == nil {
		tqCmd = cmdTestQuery
		tqArgs = []string{cmdQuery, flagDB, dbFile, flagFormat, flagTable, query}
	} else if _, err := CommandRunner.LookPath(cmdTQ); err == nil {
		tqCmd = cmdTQ
		tqArgs = []string{cmdQuery, flagDB, dbFile, flagFormat, flagTable, query}
	} else {
		tqCmd = "go"
		tqArgs = []string{
			"run", "github.com/danicat/testquery@latest",
			cmdQuery, flagDB, dbFile, flagFormat, flagTable, query,
		}
	}

	out, runErr := CommandRunner.RunWithOutput(ctx, absDir, tqCmd, tqArgs...)
	output := filterNoise(out)

	if runErr != nil && output == "" {
		hint := "**HINT:** Check SQL query syntax and available tables:\n" +
			"- `all_tests` (time, action, package, test, elapsed, output)\n" +
			"- `all_coverage` (package, file, start_line, start_col, end_line, end_col, stmt_num, count, function_name)\n" +
			"- `test_coverage` (test_name, package, file, start_line, start_col,\n" +
			"  end_line, end_col, stmt_num, count, function_name)\n" +
			"- `all_code` (package, file, line_number, content)\n" +
			"- `metadata` (key, value)"
		msg := fmt.Sprintf("test query failed: %v\n\n%s", runErr, hint)
		return errorResult(msg), nil, nil
	}

	if runErr != nil {
		hint := "**HINT:** Available tables: `all_tests`, `all_coverage`, `test_coverage`, `all_code`, `metadata`."
		msg := fmt.Sprintf("⚠️ Query completed with warnings:\n%v\n%s\n\n%s", runErr, output, hint)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
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
