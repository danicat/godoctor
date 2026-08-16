// Package cli implements the command-line interface for godoctor using Cobra.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/server"
	readdocs "github.com/danicat/godoctor/internal/tools/read_docs"
	"github.com/danicat/godoctor/internal/tools/selene"
	smartbuild "github.com/danicat/godoctor/internal/tools/smart_build"
	smartedit "github.com/danicat/godoctor/internal/tools/smart_edit"
	smarttest "github.com/danicat/godoctor/internal/tools/smart_test"
	testquery "github.com/danicat/godoctor/internal/tools/test_query"
	"github.com/danicat/godoctor/internal/versioncheck"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// DefaultConfigFileTemplate is the template for generated .godoctor.yaml.
const DefaultConfigFileTemplate = `# ==============================================================================
# GoDoctor Configuration File (.godoctor.yaml)
# Centralized configuration for Go Developer Intelligence & MCP Server
# ==============================================================================
version: "1"

# CLI & Runtime Execution Settings
cli:
  timeout: "60s"          # Default execution timeout for tool invocations
  output_format: "text"   # Output format: text | json | yaml
  color: true             # Enable ANSI color in terminal output
  log_level: "info"       # Logging level: debug | info | warn | error

# MCP Server Settings
mcp:
  listen_address: ""      # HTTP address (e.g. ":8080") or empty for stdio
  instructions_file: ""   # Optional path to custom instruction markdown

# External Tool Version Tracking & Execution Config
tools:
  golangci_lint:
    recommended_version: "v2.12.2"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
    config: ".golangci.yml"
  modernize:
    recommended_version: "latest"
    pkg: "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest"
  deadcode:
    recommended_version: "latest"
    pkg: "golang.org/x/tools/cmd/deadcode@latest"
  selene:
    recommended_version: "latest"
    pkg: "github.com/danicat/selene/cmd/selene@latest"
    timeout: "60s"
  testquery:
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: "testquery.db"

# Global Feature Flags
features:
  autofix: true               # Enable automatic code fixes in build pipeline
  deadcode_check: true        # Run dead code analysis in build pipeline
  testquery_sync: true        # Automatically index test runs into testquery.db
  version_check_hints: true   # Display upgrade recommendations when tools are outdated
`

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

// GlobalOptions holds global CLI flags.
type GlobalOptions struct {
	ConfigPath string
	Verbose    bool
	Quiet      bool
}

// NewRootCmd constructs the main Cobra command tree.
func NewRootCmd(version string, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var globalOpts GlobalOptions

	rootCmd := &cobra.Command{
		Use:           "godoctor",
		Short:         "Go Developer Intelligence and MCP Server",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.SetIn(stdin)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Global Flags
	rootCmd.PersistentFlags().StringVarP(&globalOpts.ConfigPath, "config", "c", "", "Path to .godoctor.yaml configuration file")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Verbose, "verbose", "V", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Quiet, "quiet", "q", false, "Quiet output")

	// Subcommands
	rootCmd.AddCommand(newCallCmd(stdin, stdout, stderr))
	rootCmd.AddCommand(newMCPCmd(version, stdout, stderr))
	rootCmd.AddCommand(newInitCmd(stdout, stderr))
	rootCmd.AddCommand(newCheckCmd(stdout, stderr))
	rootCmd.AddCommand(newInstallCmd(stdout, stderr))
	rootCmd.AddCommand(newUninstallCmd(stdout, stderr))
	rootCmd.AddCommand(newListCmd(stdout))
	rootCmd.AddCommand(newVersionCmd(version, stdout))

	return rootCmd
}

// Run executes the CLI with the given arguments and streams.
func Run(ctx context.Context, version string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	rootCmd := NewRootCmd(version, stdin, stdout, stderr)
	rootCmd.SetArgs(args)

	if len(args) == 0 {
		return rootCmd.Help()
	}

	return rootCmd.ExecuteContext(ctx)
}

func newCallCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "call <tool-name> [json-arguments]",
		Short: "Invoke a tool directly from the CLI",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("missing tool name\nUsage: godoctor call <tool-name> '<json-arguments>'")
			}

			toolName := args[0]
			tool := FindTool(toolName)
			if tool == nil {
				return fmt.Errorf("unknown tool: %q\nRun 'godoctor list' to see available tools", toolName)
			}

			res, err := tool.Invoke(cmd.Context(), args[1:], stdin)
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
		},
	}
}

func newMCPCmd(version string, stdout, stderr io.Writer) *cobra.Command {
	_ = stdout
	_ = stderr
	var listenAddr string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run in Model Context Protocol (MCP) server mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			srv := server.New(version)
			if listenAddr != "" {
				return srv.ServeHTTP(cmd.Context(), listenAddr)
			}
			return srv.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&listenAddr, "listen", "l", "", "HTTP listen address (e.g. :8080)")
	return cmd
}

func newInitCmd(stdout, stderr io.Writer) *cobra.Command {
	_ = stderr
	var force bool
	var targetDir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a commented .godoctor.yaml configuration file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if targetDir == "" {
				targetDir = "."
			}

			targetPath := filepath.Join(targetDir, ".godoctor.yaml")
			if _, err := os.Stat(targetPath); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", targetPath)
			}

			if err := os.WriteFile(targetPath, []byte(DefaultConfigFileTemplate), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", targetPath, err)
			}

			fmt.Fprintf(stdout, "Created %s successfully.\n", targetPath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of existing .godoctor.yaml")
	cmd.Flags().StringVarP(&targetDir, "dir", "d", "", "Directory to create .godoctor.yaml in (default: current directory)")
	return cmd
}

func newCheckCmd(stdout, stderr io.Writer) *cobra.Command {
	_ = stderr
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Inspect installed external tools, versions, and health status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			statuses, err := versioncheck.CheckAll(cmd.Context(), nil)
			if err != nil {
				return fmt.Errorf("tool check failed: %w", err)
			}

			if jsonOutput {
				data, err := json.MarshalIndent(statuses, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to format JSON: %w", err)
				}
				fmt.Fprintln(stdout, string(data))
				return nil
			}

			table := versioncheck.FormatStatusTable(statuses)
			fmt.Fprint(stdout, table)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output check results in JSON format")
	return cmd
}

func newInstallCmd(stdout, stderr io.Writer) *cobra.Command {
	var opts InstallOptions

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Configure MCP server registration and unpack agent skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !opts.InstallMCP && !opts.InstallSkills {
				opts.InstallMCP = true
				opts.InstallSkills = true
			}
			return ExecuteInstall(opts, stdout, stderr)
		},
	}

	cmd.Flags().BoolVar(&opts.InstallMCP, "mcp", false, "Register the MCP server in mcp_config.json")
	cmd.Flags().BoolVar(&opts.InstallSkills, "skills", false, "Unpack embedded skills (@godoctor, @selene, @testquery)")
	cmd.Flags().BoolVarP(&opts.Workspace, "workspace", "w", false, "Install to workspace scope (.agents/)")
	cmd.Flags().BoolVarP(&opts.Global, "global", "g", false, "Install to global user config (Default: ~/.gemini/config)")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Force overwrite of existing skill files")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "Quiet / script-friendly output")
	cmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", "", "Explicit path to mcp_config.json")
	cmd.Flags().StringVarP(&opts.SkillsDir, "skills-dir", "s", "", "Explicit directory for skills installation")

	return cmd
}

func newUninstallCmd(stdout, stderr io.Writer) *cobra.Command {
	var opts UninstallOptions

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove MCP server registration and agent skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !opts.UninstallMCP && !opts.UninstallSkills {
				opts.UninstallMCP = true
				opts.UninstallSkills = true
			}
			return ExecuteUninstall(opts, stdout, stderr)
		},
	}

	cmd.Flags().BoolVar(&opts.UninstallMCP, "mcp", false, "Remove GoDoctor from mcp_config.json")
	cmd.Flags().BoolVar(&opts.UninstallSkills, "skills", false, "Remove GoDoctor skills (@godoctor, @selene, @testquery)")
	cmd.Flags().BoolVarP(&opts.Workspace, "workspace", "w", false, "Uninstall from workspace scope (.agents/)")
	cmd.Flags().BoolVarP(&opts.Global, "global", "g", false, "Uninstall from global user config (Default: ~/.gemini/config)")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "Quiet / script-friendly output")
	cmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", "", "Explicit path to mcp_config.json")
	cmd.Flags().StringVarP(&opts.SkillsDir, "skills-dir", "s", "", "Explicit directory for skills removal")

	return cmd
}

func newListCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available intelligence tools",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(stdout)
		},
	}
}

func newVersionCmd(version string, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the godoctor version",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintln(stdout, version)
			return nil
		},
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
  init           Generate a commented .godoctor.yaml configuration file
  check          Inspect installed external tools, versions, and health status
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

Configuration & Health:
  godoctor init                 Generate default .godoctor.yaml in current repository
  godoctor check                Inspect versions and health of all workspace tools

MCP Server Mode:
  godoctor mcp                  Run MCP server using standard I/O (default for MCP clients)
  godoctor mcp -listen=:8080    Run MCP server as Streamable HTTP service on specified address

Tool Invocation:
  godoctor call <tool-name> '<json-arguments>'

Tools:
  edit, build, test, docs, selene, tq

Examples:
  godoctor init
  godoctor check
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

// parseArgs parses arguments into a struct, supporting JSON string argument or stdin JSON.
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
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if len(trimmed) > 0 {
			if strings.HasPrefix(trimmed, "{") {
				return json.Unmarshal([]byte(trimmed), target)
			}
			return fmt.Errorf("invalid JSON from stdin: expected object starting with '{'")
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
