// Package cli implements the command-line interface for godoctor.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/danicat/godoctor/internal/server"
	readdocs "github.com/danicat/godoctor/internal/tools/read_docs"
	"github.com/danicat/godoctor/internal/tools/selene"
	smartbuild "github.com/danicat/godoctor/internal/tools/smart_build"
	smartedit "github.com/danicat/godoctor/internal/tools/smart_edit"
	smarttest "github.com/danicat/godoctor/internal/tools/smart_test"
	testquery "github.com/danicat/godoctor/internal/tools/test_query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolDef represents metadata and invoker for a tool.
type ToolDef struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Invoke      func(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error)
}

// GetTools returns the list of all registered tools.
func GetTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "edit",
			Aliases:     []string{"smart_edit"},
			Description: "Single-file coordinate editing transaction with compiler verification and rollback.",
			Usage:       `godoctor call edit '{"filename": "/abs/path/file.go", "old_content": "...", "new_content": "..."}'`,
			Invoke:      invokeSmartEdit,
		},
		{
			Name:        "build",
			Aliases:     []string{"smart_build"},
			Description: "4-phase pipeline: mod tidy -> modernize -> gofmt -> deadcode -> build -> test -> linter.",
			Usage:       `godoctor call build '{"dir": "/abs/path"}'`,
			Invoke:      invokeSmartBuild,
		},
		{
			Name:        "test",
			Aliases:     []string{"smart_test"},
			Description: "Multi-tier test and benchmark runner (fast, basic, benchmark, complete) with testquery.db indexing.",
			Usage:       `godoctor call test '{"dir": "/abs/path", "level": "basic"}'`,
			Invoke:      invokeSmartTest,
		},
		{
			Name:        "docs",
			Aliases:     []string{"read_docs"},
			Description: "AST documentation reader with ephemeral remote package downloading and symbol extraction.",
			Usage:       `godoctor call docs '{"import_path": "net/http", "symbol_name": "Get"}'`,
			Invoke:      invokeReadDocs,
		},
		{
			Name:        "selene",
			Aliases:     []string{"mutation_test", "mutation"},
			Description: "Selene-powered AST mutation testing evaluating unit test quality by introducing code defects.",
			Usage:       `godoctor call selene '{"dir": "/abs/path"}'`,
			Invoke:      invokeSelene,
		},
		{
			Name:        "tq",
			Aliases:     []string{"test_query", "testquery"},
			Description: "SQL analytics engine executing queries against coverage and test execution history in testquery.db.",
			Usage:       `godoctor call tq '{"dir": "/abs/path", "query": "SELECT ... "}'`,
			Invoke:      invokeTestQuery,
		},
	}
}

// FindTool looks up a tool by name or alias.
func FindTool(name string) *ToolDef {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, tool := range GetTools() {
		if strings.ToLower(tool.Name) == name {
			t := tool
			return &t
		}
		for _, alias := range tool.Aliases {
			if strings.ToLower(alias) == name {
				t := tool
				return &t
			}
		}
	}
	return nil
}

// Run executes the CLI with the given arguments.
func Run(ctx context.Context, version string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		PrintHelp(stdout, version)
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "-h", "--help", "-help":
		PrintHelp(stdout, version)
		return nil

	case "version", "-v", "--version", "-version":
		fmt.Fprintln(stdout, version)
		return nil

	case "list":
		return runList(stdout)

	case "call":
		return runCall(ctx, cmdArgs, stdin, stdout, stderr)

	case "install", "init":
		return runInstall(ctx, cmdArgs, stdout, stderr)

	case "uninstall":
		return runUninstall(ctx, cmdArgs, stdout, stderr)

	case "mcp":
		return runMCP(ctx, version, cmdArgs)

	default:
		// If unknown subcommand or flag
		if strings.HasPrefix(cmd, "-") {
			return fmt.Errorf("unknown flag: %s\nRun 'godoctor help' for usage", cmd)
		}
		return fmt.Errorf("unknown command: %s\nRun 'godoctor help' for usage", cmd)
	}
}

// PrintHelp writes the main help text to w.
func PrintHelp(w io.Writer, version string) {
	fmt.Fprintf(w, `godoctor %s - Go Developer Intelligence and MCP Server

Usage:
  godoctor [command]

Available Commands:
  install        Configure MCP server registration and unpack agent skills
  uninstall      Remove MCP server registration and agent skills
  mcp            Run in Model Context Protocol (MCP) server mode
  list           List all available intelligence tools
  call           Invoke a tool directly from the CLI
  version        Print the godoctor version
  help           Print this help message

Surface Management:
  godoctor install              Configure MCP server and install skills (Global)
  godoctor install -w           Configure MCP server and install skills (Workspace)
  godoctor install --mcp        Configure MCP server only
  godoctor install --skills     Install skills only
  godoctor uninstall            Remove MCP server and skills (Global)
  godoctor uninstall -w         Remove MCP server and skills (Workspace)

MCP Server Mode:
  godoctor mcp                  Run MCP server using standard I/O (default for MCP clients)
  godoctor mcp -listen=:8080    Run MCP server as Streamable HTTP service on specified address

Tool Invocation:
  godoctor call <tool-name> '<json-arguments>'

Tools:
  edit, build, test, docs, selene, tq

Examples:
  godoctor init
  godoctor list
  godoctor call edit '{"filename": "/path/to/main.go", "old_content": "...", "new_content": "..."}'
  godoctor call build '{"dir": "/path/to/project"}'
  godoctor call test '{"dir": "/path/to/project", "level": "fast"}'
  godoctor call docs '{"import_path": "net/http", "symbol_name": "Get"}'
  godoctor call selene '{"dir": "/path/to/project"}'
  godoctor call tq '{"dir": "/path/to/project", "query": "SELECT * FROM all_tests"}'
`, version)
}

func runList(w io.Writer) error {
	fmt.Fprintln(w, "Available godoctor tools:")
	fmt.Fprintln(w)
	for _, tool := range GetTools() {
		aliasStr := ""
		if len(tool.Aliases) > 0 {
			aliasStr = fmt.Sprintf(" (aliases: %s)", strings.Join(tool.Aliases, ", "))
		}
		fmt.Fprintf(w, "• %s%s\n", tool.Name, aliasStr)
		fmt.Fprintf(w, "  %s\n", tool.Description)
		fmt.Fprintf(w, "  Usage: %s\n\n", tool.Usage)
	}
	return nil
}

func runCall(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing tool name\nUsage: godoctor call <tool-name> '<json-arguments>'")
	}

	toolName := args[0]
	tool := FindTool(toolName)
	if tool == nil {
		return fmt.Errorf("unknown tool: %q\nRun 'godoctor list' to see available tools", toolName)
	}

	res, err := tool.Invoke(ctx, args[1:], stdin)
	if err != nil {
		return err
	}

	if res == nil {
		return nil
	}

	for _, content := range res.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			if res.IsError {
				fmt.Fprintln(stderr, tc.Text)
			} else {
				fmt.Fprintln(stdout, tc.Text)
			}
		}
	}

	if res.IsError {
		return errors.New("tool execution returned an error")
	}

	return nil
}

func runMCP(ctx context.Context, version string, args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	var listenAddr string
	fs.StringVar(&listenAddr, "listen", "", "HTTP listen address (e.g. :8080)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	srv := server.New(version)

	if listenAddr != "" {
		return srv.ServeHTTP(ctx, listenAddr)
	}

	return srv.Run(ctx)
}

// Helper to parse arguments into a struct, supporting JSON string argument or stdin JSON.
func parseArgs(rawArgs []string, stdin io.Reader, target any) error {
	// 1. If single argument looks like JSON:
	if len(rawArgs) == 1 {
		trimmed := strings.TrimSpace(rawArgs[0])
		if strings.HasPrefix(trimmed, "{") {
			return json.Unmarshal([]byte(trimmed), target)
		}
	}

	// 2. If rawArgs joined is JSON (e.g. unquoted JSON or shell split):
	if len(rawArgs) > 0 {
		joined := strings.TrimSpace(strings.Join(rawArgs, " "))
		if strings.HasPrefix(joined, "{") && strings.HasSuffix(joined, "}") {
			return json.Unmarshal([]byte(joined), target)
		}
	}

	// 3. If no args provided, try reading from stdin
	if len(rawArgs) == 0 && stdin != nil {
		data, err := io.ReadAll(stdin)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			trimmed := strings.TrimSpace(string(data))
			if strings.HasPrefix(trimmed, "{") {
				return json.Unmarshal([]byte(trimmed), target)
			}
		}
	}

	if len(rawArgs) == 0 {
		return errors.New("missing arguments (expected JSON string, e.g. '{\"key\": \"value\"}')")
	}

	return fmt.Errorf("invalid arguments: %v (expected JSON string, e.g. '{\"key\": \"value\"}')", rawArgs)
}

func invokeSmartEdit(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error) {
	var params smartedit.SingleEditParams
	if err := parseArgs(rawArgs, stdin, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for edit: %w", err)
	}
	res, _, err := smartedit.Handler(ctx, nil, params)
	return res, err
}

func invokeSmartBuild(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error) {
	var params smartbuild.Params
	if err := parseArgs(rawArgs, stdin, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for build: %w", err)
	}
	res, _, err := smartbuild.Handler(ctx, nil, params)
	return res, err
}

func invokeSmartTest(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error) {
	var params smarttest.Params
	if err := parseArgs(rawArgs, stdin, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for test: %w", err)
	}
	res, _, err := smarttest.Handler(ctx, nil, params)
	return res, err
}

func invokeTestQuery(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error) {
	var params testquery.Params
	if err := parseArgs(rawArgs, stdin, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for tq: %w", err)
	}
	res, _, err := testquery.Handler(ctx, nil, params)
	return res, err
}

func invokeSelene(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error) {
	var params selene.Params
	if err := parseArgs(rawArgs, stdin, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for selene: %w", err)
	}
	res, _, err := selene.Handler(ctx, nil, params)
	return res, err
}

func invokeReadDocs(ctx context.Context, rawArgs []string, stdin io.Reader) (*mcp.CallToolResult, error) {
	var params readdocs.Params
	if err := parseArgs(rawArgs, stdin, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for docs: %w", err)
	}
	res, _, err := readdocs.Handler(ctx, nil, params)
	return res, err
}
