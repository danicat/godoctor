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

	"github.com/danicat/godoctor/internal/config"
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

// DefaultConfigFileTemplate is the master .godoctor.yaml configuration template matching RFC-0001 (Section 3.1).
const DefaultConfigFileTemplate = `# ==============================================================================
# GoDoctor Master Configuration (.godoctor.yaml)
# ==============================================================================
version: "1"

# CLI Global Execution Settings
cli:
  timeout: "60s"
  output_format: "text"           # "text" | "json" | "yaml"
  color: true
  log_level: "info"               # "debug" | "info" | "warn" | "error"

# MCP Server Settings
server:
  name: "godoctor"
  transport: "stdio"              # "stdio" | "http"
  http:
    listen: ":8080"
    read_timeout: "30s"
    write_timeout: "5m"           # Extended to accommodate long-running builds/tests
    idle_timeout: "120s"
    shutdown_timeout: "10s"
    allowed_origins:
      - "http://localhost"
      - "http://localhost:*"
      - "http://127.0.0.1"
      - "http://127.0.0.1:*"
    allow_credentials: true
  logging:
    level: "info"                 # "debug" | "info" | "warn" | "error"
    format: "text"                # "text" | "json"
    trace_mcp_payloads: false
    log_file: ""

# SafeShell Subprocess Safety & Allowed Executables
safeshell:
  mode: "standard"                # "standard" | "strict" | "allowlist" | "disabled"
  command_timeout: "120s"
  allowed_binaries:
    - "go"
    - "gofmt"
    - "golangci-lint"
    - "selene"
    - "testquery"
    - "tq"
    - "deadcode"
    - "modernize"

# System Instructions Prompt Configuration
instructions:
  enabled: true
  compact: false
  dynamic_tools: true            # Filter instructions to only active tools
  rules_file: ""                 # Optional repo-specific markdown rules (e.g. .godoctor/rules.md)
  custom_rules: ""

# External Utility Definitions & RFC-0001 Version Tracking
tools:
  golangci_lint:
    binary_name: "golangci-lint"
    recommended_version: "v2.12.2"
    min_version: "v1.60.0"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
    config: ".golangci.yml"
    timeout: "5m"
    args: ["run"]
    disabled: false

  modernize:
    binary_name: "modernize"
    recommended_version: "latest"
    pkg: "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest"
    timeout: "2m"
    args: ["-fix"]
    disabled: false

  deadcode:
    binary_name: "deadcode"
    recommended_version: "latest"
    pkg: "golang.org/x/tools/cmd/deadcode@latest"
    timeout: "2m"
    disabled: false

  selene:
    binary_name: "selene"
    recommended_version: "latest"
    pkg: "github.com/danicat/selene/cmd/selene@latest"
    packages: "./..."
    timeout: "3m"
    workers: 0                    # 0 defaults to runtime.GOMAXPROCS workers
    testquery_compat: true        # Enable TestQuery SQLite DB integration
    db_path: ".godoctor/testquery.db"
    disabled: false

  testquery:
    binary_name: "testquery"
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: ".godoctor/testquery.db"
    format: "table"
    timeout: "2m"
    disabled: false

# Build Pipeline Configuration (smart_build)
build:
  default_packages: "./..."
  output: ""
  tags: []
  race: false
  trimpath: false
  timeout: "5m"
  flags: []

# Auto-Fix Pipeline Configuration (smart_build)
autofix:
  enabled: true
  dry_run: false
  order:
    - tidy
    - modernize
    - gofmt
  mod_tidy: true
  modernize: true
  gofmt: true
  deadcode: true

# Transactional Edit Engine (smart_edit)
edit:
  backup_strategy: "memory"       # "memory" | "temp_file" | "git"
  atomic_write: true              # Write-to-temp + atomic os.Rename
  preserve_permissions: true      # Retain original os.FileMode
  default_threshold: 0.95
  format_on_save: "goimports"     # "goimports" | "gofmt" | "none"
  exclude_paths:
    - ".git"
    - "vendor"
    - "node_modules"
    - "skills"
    - "agents"
    - "hooks"

# Fuzzy Coordinate Matching Engine (match.go)
matching:
  fuzzy_fallback: true
  similarity_threshold: 0.95
  normalize_unicode: true         # Strip non-breaking spaces (\u00A0) and Unicode whitespace
  min_seed_length: 3
  window_expansion_delta: 4

# Compiler Diagnostics & Verification Gate (diagnostics.go)
diagnostics:
  collect_on_edit: true
  verification_scope: "module"    # "module" | "package" | "edited_files"
  check_command:
    - "go"
    - "vet"
    - "./..."
  timeout: "30s"
  enable_suggestions: true
  max_levenshtein_distance: 3
  max_suggestions: 5
  snippet_context_lines: 5

# Test & Benchmark Runner (smart_test)
test:
  default_level: "basic"           # "fast" | "basic" | "benchmark" | "complete"
  default_packages: "./..."
  timeout: "60s"
  verbose: true
  race_detector: false
  coverage_threshold: 80.0
  coverage_profile: "coverage.out"
  coverage_output_dir: ""          # Empty string uses os.TempDir()
  benchmark_pattern: "."
  benchmark_flags:
    - "-benchmem"
    - "-run=NONE"

# SQLite Test Analytics (test_query)
testquery:
  db_path: ".godoctor/testquery.db"
  wal_mode: true
  busy_timeout: "5s"
  format: "table"                  # "table" | "json" | "csv"

# AST Documentation Engine (read_docs, godoc)
docs:
  cache_enabled: true
  cache_ttl: "15m"
  cache_max_entries: 500
  external_fetch: true
  offline_mode: false
  pkg_go_dev_url: "https://pkg.go.dev"
  max_symbols_rendered: 100
  fuzzy_suggestions: true
  max_fuzzy_suggestions: 5
  fuzzy_distance_threshold: 2
  default_format: "markdown"       # "markdown" | "json"
  temp_dir: ""

# Global Feature Flags
features:
  autofix: true
  mod_tidy: true
  modernize_check: true
  deadcode_check: true
  format_on_build: true
  format_on_edit: true
  vet_gate: true
  auto_rollback: true
  testquery_sync: true
  testquery_compat: true
  version_check_hints: true
  coverage_gate: false
  race_detector: false
  strict_mode: false
  remote_doc_fetch: true
  vanity_resolution: true
  docs_cache: true
`

// MinimalConfigFileTemplate is a concise configuration template for minimal setups.
const MinimalConfigFileTemplate = `# ==============================================================================
# GoDoctor Minimal Configuration (.godoctor.yaml)
# ==============================================================================
version: "1"

cli:
  timeout: "60s"
  output_format: "text"

server:
  transport: "stdio"

features:
  autofix: true
  deadcode_check: true
  testquery_sync: true
  version_check_hints: true
`

// CLI tool name and command constants
const (
	ToolNameEdit      = "edit"
	ToolNameBuild     = "build"
	ToolNameTest      = "test"
	ToolNameDocs      = "docs"
	ToolNameSelene    = "selene"
	ToolNameTestQuery = "testquery"
	ToolNameTQ        = "tq"
	AppName           = "godoctor"
	CommandMCP        = "mcp"
	CommandInit       = "init"
	CommandCheck      = "check"
	CommandList       = "list"
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
			Name:        ToolNameEdit,
			Aliases:     []string{"smart_edit"},
			Description: "Single-file coordinate editing transaction with compiler verification and rollback.",
			Usage:       `godoctor call edit '{"filename": "/abs/path/file.go", "old_content": "...", "new_content": "..."}'`,
			Invoke:      invokeSmartEdit,
		},
		{
			Name:        ToolNameBuild,
			Aliases:     []string{"smart_build"},
			Description: "Build Go packages and binaries with automated compilation, testing, linting, and quality checks.",
			Usage:       `godoctor call build '{"dir": "/abs/path"}'`,
			Invoke:      invokeSmartBuild,
		},
		{
			Name:        ToolNameTest,
			Aliases:     []string{"smart_test"},
			Description: "Multi-tier test and benchmark runner (fast, basic, benchmark, complete) with testquery.db indexing.",
			Usage:       `godoctor call test '{"dir": "/abs/path", "level": "basic"}'`,
			Invoke:      invokeSmartTest,
		},
		{
			Name:        ToolNameDocs,
			Aliases:     []string{"read_docs"},
			Description: "AST documentation reader with ephemeral remote package downloading and symbol extraction.",
			Usage:       `godoctor call docs '{"import_path": "net/http", "symbol_name": "Get"}'`,
			Invoke:      invokeReadDocs,
		},
		{
			Name:        ToolNameSelene,
			Aliases:     []string{"mutation_test", "mutation"},
			Description: "Selene-powered AST mutation testing evaluating unit test quality by introducing code defects.",
			Usage:       `godoctor call selene '{"dir": "/abs/path"}'`,
			Invoke:      invokeSelene,
		},
		{
			Name:        ToolNameTQ,
			Aliases:     []string{"test_query", ToolNameTestQuery},
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
		Use:           AppName,
		Short:         "Go Developer Intelligence and MCP Server",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.SetIn(stdin)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Global Flags
	rootCmd.PersistentFlags().StringVarP(
		&globalOpts.ConfigPath, "config", "c", "", "Path to .godoctor.yaml configuration file",
	)
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Verbose, "verbose", "V", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Quiet, "quiet", "q", false, "Quiet output")

	// Subcommands
	rootCmd.AddCommand(newCallCmd(stdin, stdout, stderr))
	rootCmd.AddCommand(newMCPCmd(version, &globalOpts, stdout, stderr))
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
						_, _ = fmt.Fprintln(stderr, tc.Text)
					} else {
						_, _ = fmt.Fprintln(stdout, tc.Text)
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

func newMCPCmd(version string, opts *GlobalOptions, stdout, stderr io.Writer) *cobra.Command {
	_ = stdout
	_ = stderr
	var listenAddr string
	var transport string

	cmd := &cobra.Command{
		Use:   CommandMCP,
		Short: "Run in Model Context Protocol (MCP) server mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var configPath string
			if opts != nil {
				configPath = opts.ConfigPath
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				cfg = config.NewDefaultConfig()
			}
			srv := server.New(version, server.WithServerConfig(cfg.Server))
			transport = strings.ToLower(strings.TrimSpace(transport))
			if transport == "http" || listenAddr != "" {
				if listenAddr == "" {
					listenAddr = cfg.Server.ListenAddr
					if listenAddr == "" {
						listenAddr = config.DefaultListenAddr
					}
				}
				return srv.ServeHTTP(cmd.Context(), listenAddr)
			}
			if transport != "" && transport != "stdio" {
				return fmt.Errorf("unsupported transport %q (must be 'stdio' or 'http')", transport)
			}
			return srv.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&listenAddr, "listen", "l", "", "HTTP listen address (e.g. :8080)")
	cmd.Flags().StringVarP(&transport, "transport", "t", "stdio", "MCP transport protocol (stdio | http)")
	return cmd
}

func newInitCmd(stdout, stderr io.Writer) *cobra.Command {
	_ = stderr
	var force bool
	var minimal bool
	var targetDir string

	cmd := &cobra.Command{
		Use:   CommandInit,
		Short: "Generate a .godoctor.yaml configuration file",
		RunE: func(_ *cobra.Command, _ []string) error {
			if targetDir == "" {
				targetDir = "."
			}

			targetPath := filepath.Join(targetDir, ".godoctor.yaml")
			if _, err := os.Stat(targetPath); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", targetPath)
			}

			content := DefaultConfigFileTemplate
			if minimal {
				content = MinimalConfigFileTemplate
			}

			if err := os.WriteFile(targetPath, []byte(content), 0600); err != nil {
				return fmt.Errorf("failed to write %s: %w", targetPath, err)
			}

			_, _ = fmt.Fprintf(stdout, "Created %s successfully.\n", targetPath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of existing .godoctor.yaml")
	cmd.Flags().BoolVarP(&minimal, "minimal", "m", false, "Generate minimal .godoctor.yaml configuration")
	cmd.Flags().StringVarP(
		&targetDir, "dir", "d", "", "Directory to create .godoctor.yaml in (default: current directory)",
	)
	return cmd
}

func resolveCheckStatuses(
	ctx context.Context,
	dir, configFlag string,
	noCache bool,
) ([]versioncheck.ToolStatus, error) {
	var cfg *config.Config
	var err error

	if configFlag != "" {
		cfg, err = config.Load(configFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", configFlag, err)
		}
	} else {
		targetDir := dir
		if targetDir == "" {
			targetDir = "."
		}
		cfg, err = config.LoadFromWorkspace(targetDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load workspace config: %w", err)
		}
	}

	if noCache {
		checker := versioncheck.NewChecker(versioncheck.WithNoCache(true))
		specs := versioncheck.DefaultRegistry()
		if cfg != nil {
			for i := range specs {
				if toolCfg, found := cfg.LookupTool(specs[i].ID); found {
					if toolCfg.RecommendedVersion != "" {
						specs[i].DefaultRecommended = toolCfg.RecommendedVersion
					}
					if toolCfg.Package != "" {
						specs[i].PackagePath = toolCfg.Package
					}
					if toolCfg.Command != "" {
						specs[i].Binaries = []string{toolCfg.Command}
					}
				}
			}
		}
		return checker.CheckAll(ctx, specs...)
	}
	return versioncheck.CheckAll(ctx, cfg)
}

func outputCheckResults(stdout io.Writer, statuses []versioncheck.ToolStatus, jsonOutput bool) error {
	if jsonOutput {
		data, err := json.MarshalIndent(statuses, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		_, _ = fmt.Fprintln(stdout, string(data))
		return nil
	}
	table := versioncheck.FormatStatusTable(statuses)
	_, _ = fmt.Fprint(stdout, table)
	return nil
}

func newCheckCmd(stdout, stderr io.Writer) *cobra.Command {
	_ = stderr
	var jsonOutput bool
	var strict bool
	var noCache bool
	var dir string

	cmd := &cobra.Command{
		Use:   CommandCheck,
		Short: "Inspect installed external tools, versions, and health status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFlag, _ := cmd.Flags().GetString("config")
			statuses, err := resolveCheckStatuses(cmd.Context(), dir, configFlag, noCache)
			if err != nil {
				return fmt.Errorf("tool check failed: %w", err)
			}

			if err := outputCheckResults(stdout, statuses, jsonOutput); err != nil {
				return err
			}

			if strict {
				var unhealthy []string
				for _, st := range statuses {
					if st.Status == versioncheck.StatusMissing || st.Status == versioncheck.StatusOutdated {
						unhealthy = append(unhealthy, fmt.Sprintf("%s (%s)", st.DisplayName, st.Status))
					}
				}
				if len(unhealthy) > 0 {
					return fmt.Errorf("strict check failed: %d unhealthy tools: %s", len(unhealthy), strings.Join(unhealthy, ", "))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output check results in JSON format")
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail with non-zero exit code if any tool is missing or outdated")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Bypass version cache and re-probe all binaries")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Workspace directory to check (default: current directory)")
	return cmd
}

func newInstallCmd(stdout, stderr io.Writer) *cobra.Command {
	var opts InstallOptions

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Configure MCP server registration and unpack agent skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.InstallAll || (!opts.InstallMCP && !opts.InstallSkills) {
				opts.InstallMCP = true
				opts.InstallSkills = true
			}
			return ExecuteInstall(cmd.Context(), opts, stdout, stderr)
		},
	}

	cmd.Flags().BoolVarP(&opts.InstallAll, "all", "a", false, "Install both MCP server and agent skills")
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
			if opts.UninstallAll || (!opts.UninstallMCP && !opts.UninstallSkills) {
				opts.UninstallMCP = true
				opts.UninstallSkills = true
			}
			return ExecuteUninstall(cmd.Context(), opts, stdout, stderr)
		},
	}

	cmd.Flags().BoolVarP(&opts.UninstallAll, "all", "a", false, "Remove both MCP server registration and agent skills")
	cmd.Flags().BoolVar(&opts.UninstallMCP, "mcp", false, "Remove the MCP server from mcp_config.json")
	cmd.Flags().BoolVar(&opts.UninstallSkills, "skills", false, "Remove GoDoctor skills (@godoctor, @selene, @testquery)")
	cmd.Flags().BoolVarP(&opts.Workspace, "workspace", "w", false, "Uninstall from workspace scope (.agents/)")
	cmd.Flags().BoolVarP(
		&opts.Global, "global", "g", false, "Uninstall from global user config (Default: ~/.gemini/config)",
	)
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "Quiet / script-friendly output")
	cmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", "", "Explicit path to mcp_config.json")
	cmd.Flags().StringVarP(&opts.SkillsDir, "skills-dir", "s", "", "Explicit directory for skills removal")

	return cmd
}

func newListCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   CommandList,
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
			_, _ = fmt.Fprintln(stdout, version)
			return nil
		},
	}
}

func runList(w io.Writer) error {
	_, _ = fmt.Fprintln(w, "Available godoctor tools:")
	_, _ = fmt.Fprintln(w)
	for _, tool := range GetTools() {
		aliasStr := ""
		if len(tool.Aliases) > 0 {
			aliasStr = fmt.Sprintf(" (aliases: %s)", strings.Join(tool.Aliases, ", "))
		}
		_, _ = fmt.Fprintf(w, "• %s%s\n", tool.Name, aliasStr)
		_, _ = fmt.Fprintf(w, "  %s\n", tool.Description)
		_, _ = fmt.Fprintf(w, "  Usage: %s\n\n", tool.Usage)
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
