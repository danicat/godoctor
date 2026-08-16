// Package config implements centralized configuration management for GoDoctor.
// It defines typed schemas, provides built-in defaults, discovers and loads .godoctor.yaml
// files hierarchically, and performs schema validation.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the master configuration schema for GoDoctor.
type Config struct {
	Version      string             `yaml:"version" mapstructure:"version" json:"version"`
	CLI          CLIConfig          `yaml:"cli" mapstructure:"cli" json:"cli"`
	Server       ServerConfig       `yaml:"server" mapstructure:"server" json:"server"`
	SafeShell    SafeShellConfig    `yaml:"safeshell" mapstructure:"safeshell" json:"safeshell"`
	Instructions InstructionsConfig `yaml:"instructions" mapstructure:"instructions" json:"instructions"`
	Tools        ToolsConfig        `yaml:"tools" mapstructure:"tools" json:"tools"`
	Features     FeaturesConfig     `yaml:"features" mapstructure:"features" json:"features"`
	Build        BuildConfig        `yaml:"build" mapstructure:"build" json:"build"`
	Autofix      AutofixConfig      `yaml:"autofix" mapstructure:"autofix" json:"autofix"`
	Edit         EditConfig         `yaml:"edit" mapstructure:"edit" json:"edit"`
	Matching     MatchingConfig     `yaml:"matching" mapstructure:"matching" json:"matching"`
	Diagnostics  DiagnosticsConfig  `yaml:"diagnostics" mapstructure:"diagnostics" json:"diagnostics"`
	Test         TestConfig         `yaml:"test" mapstructure:"test" json:"test"`
	TestQuery    TestQueryConfig    `yaml:"testquery" mapstructure:"testquery" json:"testquery"`
	Docs         DocsConfig         `yaml:"docs" mapstructure:"docs" json:"docs"`

	// LoadedFrom records the absolute path of the configuration file loaded (if any).
	LoadedFrom string `yaml:"-" mapstructure:"-" json:"-"`
}

// CLIConfig holds CLI-specific configuration settings.
type CLIConfig struct {
	DefaultOutput string `yaml:"default_output" mapstructure:"default_output" json:"default_output"`
	Quiet         bool   `yaml:"quiet" mapstructure:"quiet" json:"quiet"`
	Color         bool   `yaml:"color" mapstructure:"color" json:"color"`
}

// ServerConfig configures the MCP server and StreamableHTTP endpoints.
type ServerConfig struct {
	ListenAddr      string        `yaml:"listen_addr" mapstructure:"listen_addr" json:"listen_addr"`
	ReadTimeout     time.Duration `yaml:"read_timeout" mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout" mapstructure:"write_timeout" json:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" mapstructure:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" mapstructure:"shutdown_timeout" json:"shutdown_timeout"`
	AllowedOrigins  []string      `yaml:"allowed_origins" mapstructure:"allowed_origins" json:"allowed_origins"`
}

// SafeShellConfig configures safe shell execution parameters.
type SafeShellConfig struct {
	DefaultTimeout time.Duration `yaml:"default_timeout" mapstructure:"default_timeout" json:"default_timeout"`
	MaxOutputBytes int64         `yaml:"max_output_bytes" mapstructure:"max_output_bytes" json:"max_output_bytes"`
}

// InstructionsConfig configures dynamic agent instructions and prompts.
type InstructionsConfig struct {
	CustomPromptPath string `yaml:"custom_prompt_path,omitempty" mapstructure:"custom_prompt_path" json:"custom_prompt_path"`
	ExtraGuidance    string `yaml:"extra_guidance,omitempty" mapstructure:"extra_guidance" json:"extra_guidance"`
}

// ToolsConfig catalogs external tool specs.
type ToolsConfig struct {
	GolangCILint ToolSpec `yaml:"golangci_lint" mapstructure:"golangci_lint" json:"golangci_lint"`
	Modernize    ToolSpec `yaml:"modernize" mapstructure:"modernize" json:"modernize"`
	Deadcode     ToolSpec `yaml:"deadcode" mapstructure:"deadcode" json:"deadcode"`
	Selene       ToolSpec `yaml:"selene" mapstructure:"selene" json:"selene"`
	TestQuery    ToolSpec `yaml:"testquery" mapstructure:"testquery" json:"testquery"`
}

// ToolSpec defines execution parameters and version tracking for an external binary/package.
type ToolSpec struct {
	BinaryName         string        `yaml:"binary_name,omitempty" mapstructure:"binary_name" json:"binary_name"`
	RecommendedVersion string        `yaml:"recommended_version" mapstructure:"recommended_version" json:"recommended_version"`
	MinVersion         string        `yaml:"min_version,omitempty" mapstructure:"min_version" json:"min_version"`
	Package            string        `yaml:"pkg" mapstructure:"pkg" json:"pkg"`
	Config             string        `yaml:"config,omitempty" mapstructure:"config" json:"config"`
	Timeout            time.Duration `yaml:"timeout,omitempty" mapstructure:"timeout" json:"timeout"`
	Args               []string      `yaml:"args,omitempty" mapstructure:"args" json:"args"`
	Disabled           bool          `yaml:"disabled,omitempty" mapstructure:"disabled" json:"disabled"`
}

// FeaturesConfig holds global feature toggles.
type FeaturesConfig struct {
	Autofix                bool `yaml:"autofix" mapstructure:"autofix" json:"autofix"`
	DeadcodeCheck          bool `yaml:"deadcode_check" mapstructure:"deadcode_check" json:"deadcode_check"`
	TestQuerySync          bool `yaml:"testquery_sync" mapstructure:"testquery_sync" json:"testquery_sync"`
	VersionCheckHints      bool `yaml:"version_check_hints" mapstructure:"version_check_hints" json:"version_check_hints"`
	LevenshteinSuggestions bool `yaml:"levenshtein_suggestions" mapstructure:"levenshtein_suggestions" json:"levenshtein_suggestions"`
	AutoRollback           bool `yaml:"auto_rollback" mapstructure:"auto_rollback" json:"auto_rollback"`
	MutationTesting        bool `yaml:"mutation_testing" mapstructure:"mutation_testing" json:"mutation_testing"`
}

// BuildConfig configures the smart_build pipeline.
type BuildConfig struct {
	DefaultPackages string        `yaml:"default_packages" mapstructure:"default_packages" json:"default_packages"`
	Output          string        `yaml:"output,omitempty" mapstructure:"output" json:"output"`
	AutoTidy        bool          `yaml:"auto_tidy" mapstructure:"auto_tidy" json:"auto_tidy"`
	AutoFormat      bool          `yaml:"auto_format" mapstructure:"auto_format" json:"auto_format"`
	AutoModernize   bool          `yaml:"auto_modernize" mapstructure:"auto_modernize" json:"auto_modernize"`
	RunLinter       bool          `yaml:"run_linter" mapstructure:"run_linter" json:"run_linter"`
	RunDeadcode     bool          `yaml:"run_deadcode" mapstructure:"run_deadcode" json:"run_deadcode"`
	RunTests        bool          `yaml:"run_tests" mapstructure:"run_tests" json:"run_tests"`
	Timeout         time.Duration `yaml:"timeout" mapstructure:"timeout" json:"timeout"`
}

// AutofixConfig configures automated remediation phases.
type AutofixConfig struct {
	ModTidy   bool `yaml:"mod_tidy" mapstructure:"mod_tidy" json:"mod_tidy"`
	Modernize bool `yaml:"modernize" mapstructure:"modernize" json:"modernize"`
	Gofmt     bool `yaml:"gofmt" mapstructure:"gofmt" json:"gofmt"`
	Deadcode  bool `yaml:"deadcode" mapstructure:"deadcode" json:"deadcode"`
}

// EditConfig configures smart_edit coordinate editing.
type EditConfig struct {
	FuzzyThreshold         float64       `yaml:"fuzzy_threshold" mapstructure:"fuzzy_threshold" json:"fuzzy_threshold"`
	MaxLevenshteinDistance int           `yaml:"max_levenshtein_distance" mapstructure:"max_levenshtein_distance" json:"max_levenshtein_distance"`
	CompilerGate           string        `yaml:"compiler_gate" mapstructure:"compiler_gate" json:"compiler_gate"`
	ExcludeDirs            []string      `yaml:"exclude_dirs" mapstructure:"exclude_dirs" json:"exclude_dirs"`
	Timeout                time.Duration `yaml:"timeout" mapstructure:"timeout" json:"timeout"`
}

// MatchingConfig configures AST/fuzzy text matching parameters in smart_edit.
type MatchingConfig struct {
	DefaultThreshold float64 `yaml:"default_threshold" mapstructure:"default_threshold" json:"default_threshold"`
	MaxWindowLines   int     `yaml:"max_window_lines" mapstructure:"max_window_lines" json:"max_window_lines"`
}

// DiagnosticsConfig configures error diagnosis and Levenshtein suggestion behavior.
type DiagnosticsConfig struct {
	LevenshteinSuggestions bool `yaml:"levenshtein_suggestions" mapstructure:"levenshtein_suggestions" json:"levenshtein_suggestions"`
	MaxDistance            int  `yaml:"max_distance" mapstructure:"max_distance" json:"max_distance"`
	SuggestionsLimit       int  `yaml:"suggestions_limit" mapstructure:"suggestions_limit" json:"suggestions_limit"`
}

// TestConfig configures the smart_test runner.
type TestConfig struct {
	DefaultLevel     string        `yaml:"default_level" mapstructure:"default_level" json:"default_level"`
	DefaultPackages  string        `yaml:"default_packages" mapstructure:"default_packages" json:"default_packages"`
	CoverageProfile  string        `yaml:"coverage_profile" mapstructure:"coverage_profile" json:"coverage_profile"`
	BenchmarkPattern string        `yaml:"benchmark_pattern" mapstructure:"benchmark_pattern" json:"benchmark_pattern"`
	BenchmarkMem     bool          `yaml:"benchmark_mem" mapstructure:"benchmark_mem" json:"benchmark_mem"`
	Timeout          time.Duration `yaml:"timeout" mapstructure:"timeout" json:"timeout"`
}

// TestQueryConfig configures SQL analytics against testquery.db.
type TestQueryConfig struct {
	DatabasePath string `yaml:"db_path" mapstructure:"db_path" json:"db_path"`
	Format       string `yaml:"format" mapstructure:"format" json:"format"`
}

// DocsConfig configures read_docs AST documentation extraction.
type DocsConfig struct {
	DefaultFormat string        `yaml:"default_format" mapstructure:"default_format" json:"default_format"`
	CacheTTL      time.Duration `yaml:"cache_ttl" mapstructure:"cache_ttl" json:"cache_ttl"`
}

// NewDefaultConfig returns a fully populated Config struct with built-in production defaults.
func NewDefaultConfig() *Config {
	return &Config{
		Version: "1",
		CLI: CLIConfig{
			DefaultOutput: "text",
			Quiet:         false,
			Color:         true,
		},
		Server: ServerConfig{
			ListenAddr:      ":8080",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			AllowedOrigins: []string{
				"http://localhost",
				"http://127.0.0.1",
				"https://",
			},
		},
		SafeShell: SafeShellConfig{
			DefaultTimeout: 10 * time.Minute,
			MaxOutputBytes: 10 * 1024 * 1024, // 10MB
		},
		Instructions: InstructionsConfig{},
		Tools: ToolsConfig{
			GolangCILint: ToolSpec{
				BinaryName:         "golangci-lint",
				RecommendedVersion: "v2.12.2",
				MinVersion:         "v1.60.0",
				Package:            "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2",
				Config:             ".golangci.yml",
				Timeout:            5 * time.Minute,
				Args:               []string{"run"},
			},
			Modernize: ToolSpec{
				BinaryName:         "modernize",
				RecommendedVersion: "latest",
				Package:            "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest",
				Timeout:            2 * time.Minute,
				Args:               []string{"-fix"},
			},
			Deadcode: ToolSpec{
				BinaryName:         "deadcode",
				RecommendedVersion: "latest",
				Package:            "golang.org/x/tools/cmd/deadcode@latest",
				Timeout:            2 * time.Minute,
			},
			Selene: ToolSpec{
				BinaryName:         "selene",
				RecommendedVersion: "latest",
				Package:            "github.com/danicat/selene/cmd/selene@latest",
				Timeout:            3 * time.Minute,
			},
			TestQuery: ToolSpec{
				BinaryName:         "testquery",
				RecommendedVersion: "latest",
				Package:            "github.com/danicat/testquery@latest",
				Config:             "testquery.db",
				Timeout:            2 * time.Minute,
			},
		},
		Features: FeaturesConfig{
			Autofix:                true,
			DeadcodeCheck:          true,
			TestQuerySync:          true,
			VersionCheckHints:      true,
			LevenshteinSuggestions: true,
			AutoRollback:           true,
			MutationTesting:        true,
		},
		Build: BuildConfig{
			DefaultPackages: "./...",
			AutoTidy:        true,
			AutoFormat:      true,
			AutoModernize:   true,
			RunLinter:       true,
			RunDeadcode:     true,
			RunTests:        true,
			Timeout:         5 * time.Minute,
		},
		Autofix: AutofixConfig{
			ModTidy:   true,
			Modernize: true,
			Gofmt:     true,
			Deadcode:  true,
		},
		Edit: EditConfig{
			FuzzyThreshold:         0.95,
			MaxLevenshteinDistance: 4,
			CompilerGate:           "go vet ./...",
			ExcludeDirs: []string{
				".git", ".agents", "skills", "agents", "hooks", "bin", "vendor", "node_modules",
			},
			Timeout: 30 * time.Second,
		},
		Matching: MatchingConfig{
			DefaultThreshold: 0.95,
			MaxWindowLines:   100,
		},
		Diagnostics: DiagnosticsConfig{
			LevenshteinSuggestions: true,
			MaxDistance:            4,
			SuggestionsLimit:       5,
		},
		Test: TestConfig{
			DefaultLevel:     "basic",
			DefaultPackages:  "./...",
			CoverageProfile:  "coverage.out",
			BenchmarkPattern: ".",
			BenchmarkMem:     true,
			Timeout:          10 * time.Minute,
		},
		TestQuery: TestQueryConfig{
			DatabasePath: "testquery.db",
			Format:       "table",
		},
		Docs: DocsConfig{
			DefaultFormat: "markdown",
			CacheTTL:      24 * time.Hour,
		},
	}
}

var candidateFilenames = []string{
	".godoctor.yaml",
	".godoctor.yml",
}

// FindConfigFile searches for a GoDoctor configuration file in the following order:
// 1. startDir and its ancestor hierarchy until a project boundary (.git or go.mod) or root is reached.
// 2. User home directory (~/.godoctor.yaml, ~/.godoctor.yml, ~/.config/godoctor/config.yaml).
// Returns empty string if no config file is found.
func FindConfigFile(startDir string) string {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}

	curr, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}

	for {
		for _, name := range candidateFilenames {
			candidate := filepath.Join(curr, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}

		if isProjectBoundary(curr) {
			break
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	// Home directory fallback
	if homeDir, err := os.UserHomeDir(); err == nil {
		for _, name := range candidateFilenames {
			candidate := filepath.Join(homeDir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		xdgCandidate := filepath.Join(homeDir, ".config", "godoctor", "config.yaml")
		if info, err := os.Stat(xdgCandidate); err == nil && !info.IsDir() {
			return xdgCandidate
		}
	}

	return ""
}

func isProjectBoundary(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return true
	}
	return false
}

// Load reads and parses a configuration file from the given path.
// If configFile is empty, it returns the default configuration.
func Load(configFile string) (*Config, error) {
	cfg := NewDefaultConfig()
	if configFile == "" {
		return cfg, nil
	}

	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", configFile, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("config file not found: %s: %w", absPath, err)
	}

	v := viper.New()
	v.SetConfigFile(absPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file at %s: %w", absPath, err)
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from %s: %w", absPath, err)
	}

	cfg.LoadedFrom = absPath

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed for %s: %w", absPath, err)
	}

	return cfg, nil
}

// LoadFromWorkspace discovers and loads a configuration file for the given workspace directory.
// If no configuration file is found, it returns the default configuration without error.
func LoadFromWorkspace(workspaceDir string) (*Config, error) {
	configFile := FindConfigFile(workspaceDir)
	if configFile == "" {
		return NewDefaultConfig(), nil
	}
	return Load(configFile)
}

var semverRegex = regexp.MustCompile(`^(latest|v?[0-9]+(\.[0-9]+)*(-[0-9A-Za-z.-]+)?)$`)

// Validate checks internal consistency and parameter bounds across all configuration sections.
func (c *Config) Validate() error {
	if c.Version != "" && c.Version != "1" {
		return fmt.Errorf("unsupported configuration schema version: %q (expected '1')", c.Version)
	}

	// Validate Tools
	tools := map[string]ToolSpec{
		"golangci_lint": c.Tools.GolangCILint,
		"modernize":     c.Tools.Modernize,
		"deadcode":      c.Tools.Deadcode,
		"selene":        c.Tools.Selene,
		"testquery":     c.Tools.TestQuery,
	}

	for name, tool := range tools {
		if tool.Disabled {
			continue
		}
		if tool.RecommendedVersion != "" && !semverRegex.MatchString(tool.RecommendedVersion) {
			return fmt.Errorf("tool %q has invalid recommended_version: %q", name, tool.RecommendedVersion)
		}
		if tool.Timeout < 0 {
			return fmt.Errorf("tool %q timeout cannot be negative: %v", name, tool.Timeout)
		}
	}

	// Validate Edit
	if c.Edit.FuzzyThreshold < 0.0 || c.Edit.FuzzyThreshold > 1.0 {
		return fmt.Errorf("edit.fuzzy_threshold must be between 0.0 and 1.0 (got %f)", c.Edit.FuzzyThreshold)
	}
	if c.Edit.MaxLevenshteinDistance < 0 {
		return fmt.Errorf("edit.max_levenshtein_distance cannot be negative: %d", c.Edit.MaxLevenshteinDistance)
	}

	// Validate Matching
	if c.Matching.DefaultThreshold < 0.0 || c.Matching.DefaultThreshold > 1.0 {
		return fmt.Errorf("matching.default_threshold must be between 0.0 and 1.0 (got %f)", c.Matching.DefaultThreshold)
	}

	// Validate Diagnostics
	if c.Diagnostics.MaxDistance < 0 {
		return fmt.Errorf("diagnostics.max_distance cannot be negative: %d", c.Diagnostics.MaxDistance)
	}

	// Validate Test
	switch strings.ToLower(c.Test.DefaultLevel) {
	case "fast", "basic", "benchmark", "complete", "":
		// Valid levels (preserving fast, basic, benchmark, complete)
	default:
		return fmt.Errorf("invalid test.default_level: %q (must be fast, basic, benchmark, or complete)", c.Test.DefaultLevel)
	}

	// Validate Docs
	switch strings.ToLower(c.Docs.DefaultFormat) {
	case "markdown", "json", "":
		// Valid formats
	default:
		return fmt.Errorf("invalid docs.default_format: %q (must be markdown or json)", c.Docs.DefaultFormat)
	}

	// Validate Server
	if c.Server.ReadTimeout < 0 || c.Server.WriteTimeout < 0 || c.Server.IdleTimeout < 0 || c.Server.ShutdownTimeout < 0 {
		return errors.New("server timeouts must be non-negative")
	}

	// Validate SafeShell
	if c.SafeShell.DefaultTimeout < 0 {
		return errors.New("safeshell default_timeout must be non-negative")
	}
	if c.SafeShell.MaxOutputBytes < 0 {
		return errors.New("safeshell max_output_bytes must be non-negative")
	}

	return nil
}

// GetTool returns the ToolSpec for a given tool name or alias (case-insensitive).
// If the tool is not found, an empty ToolSpec is returned.
func (c *Config) GetTool(name string) ToolSpec {
	tool, _ := c.LookupTool(name)
	return tool
}

// LookupTool returns the ToolSpec and a boolean indicating if the tool was found.
func (c *Config) LookupTool(name string) (ToolSpec, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "golangci_lint", "golangci-lint", "linter", "lint":
		return c.Tools.GolangCILint, true
	case "modernize", "modernizer":
		return c.Tools.Modernize, true
	case "deadcode":
		return c.Tools.Deadcode, true
	case "selene", "mutation", "mutation_test":
		return c.Tools.Selene, true
	case "testquery", "tq", "test_query":
		return c.Tools.TestQuery, true
	default:
		return ToolSpec{}, false
	}
}

// IsFeatureEnabled queries whether a given feature flag is active.
func (c *Config) IsFeatureEnabled(featureName string) bool {
	switch strings.ToLower(strings.TrimSpace(featureName)) {
	case "autofix":
		return c.Features.Autofix
	case "deadcode_check", "deadcode":
		return c.Features.DeadcodeCheck
	case "testquery_sync", "testquery", "tq":
		return c.Features.TestQuerySync
	case "version_check_hints", "versioncheck", "version_hints":
		return c.Features.VersionCheckHints
	case "levenshtein_suggestions", "suggestions":
		return c.Features.LevenshteinSuggestions
	case "auto_rollback", "rollback":
		return c.Features.AutoRollback
	case "mutation_testing", "selene", "mutation":
		return c.Features.MutationTesting
	default:
		return false
	}
}

// GetLinterConfigPath returns the resolved .golangci configuration file path for the workspace.
func (c *Config) GetLinterConfigPath(workspaceDir string) string {
	if c.Tools.GolangCILint.Config != "" {
		if filepath.IsAbs(c.Tools.GolangCILint.Config) {
			return c.Tools.GolangCILint.Config
		}
		return filepath.Join(workspaceDir, c.Tools.GolangCILint.Config)
	}
	return filepath.Join(workspaceDir, ".golangci.yml")
}

// GetTestQueryDBPath returns the resolved database path for testquery.db in the workspace.
func (c *Config) GetTestQueryDBPath(workspaceDir string) string {
	dbName := c.TestQuery.DatabasePath
	if dbName == "" {
		dbName = "testquery.db"
	}
	if filepath.IsAbs(dbName) {
		return dbName
	}
	return filepath.Join(workspaceDir, dbName)
}
