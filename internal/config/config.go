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
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Tool binary names and default constants
const (
	ToolGolangCILint = "golangci-lint"
	ToolModernize    = "modernize"
	ToolDeadcode     = "deadcode"
	ToolSelene       = "selene"
	ToolTestQuery    = "testquery"

	VersionLatest = "latest"

	DefaultTestQueryDB      = ".godoctor/testquery.db"
	DefaultWildcardPackages = "./..."
	DefaultTestLevel        = "basic"
	DefaultOutputText       = "text"
	DefaultFormatMarkdown   = "markdown"
	DefaultFormatTable      = "table"
	DefaultFormatJSON       = "json"
	DefaultListenAddr       = ":8080"
	AutofixTidy             = "tidy"
	ToolVet                 = "vet"
	ToolMutation            = "mutation"
	ToolStrict              = "strict"
	ToolGolangCILintKey     = "golangci_lint"
)

// Config represents the master configuration schema for GoDoctor.
type Config struct {
	Version      string             `mapstructure:"version" yaml:"version" json:"version"`
	CLI          CLIConfig          `mapstructure:"cli" yaml:"cli" json:"cli"`
	Server       ServerConfig       `mapstructure:"server" yaml:"server" json:"server"`
	SafeShell    SafeShellConfig    `mapstructure:"safeshell" yaml:"safeshell" json:"safeshell"`
	Instructions InstructionsConfig `mapstructure:"instructions" yaml:"instructions" json:"instructions"`
	Tools        ToolsConfig        `mapstructure:"tools" yaml:"tools" json:"tools"`
	Features     FeaturesConfig     `mapstructure:"features" yaml:"features" json:"features"`
	Build        BuildConfig        `mapstructure:"build" yaml:"build" json:"build"`
	Autofix      AutofixConfig      `mapstructure:"autofix" yaml:"autofix" json:"autofix"`
	Edit         EditConfig         `mapstructure:"edit" yaml:"edit" json:"edit"`
	Matching     MatchingConfig     `mapstructure:"matching" yaml:"matching" json:"matching"`
	Diagnostics  DiagnosticsConfig  `mapstructure:"diagnostics" yaml:"diagnostics" json:"diagnostics"`
	Test         TestConfig         `mapstructure:"test" yaml:"test" json:"test"`
	TestQuery    TestQueryConfig    `mapstructure:"testquery" yaml:"testquery" json:"testquery"`
	Docs         DocsConfig         `mapstructure:"docs" yaml:"docs" json:"docs"`
	LoadedFrom   string             `mapstructure:"-" yaml:"-" json:"-"`
}

// CLIConfig holds CLI-specific configuration settings.
type CLIConfig struct {
	Timeout       time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	OutputFormat  string        `mapstructure:"output_format" yaml:"output_format" json:"output_format"`
	Color         bool          `mapstructure:"color" yaml:"color" json:"color"`
	LogLevel      string        `mapstructure:"log_level" yaml:"log_level" json:"log_level"`
	DefaultOutput string        `mapstructure:"default_output,omitempty" yaml:"default_output,omitempty"`
	Quiet         bool          `mapstructure:"quiet,omitempty" yaml:"quiet,omitempty"`
}

// ServerConfig configures the MCP server and StreamableHTTP endpoints.
type ServerConfig struct {
	Name            string              `mapstructure:"name" yaml:"name" json:"name"`
	Transport       string              `mapstructure:"transport" yaml:"transport" json:"transport"`
	HTTP            ServerHTTPConfig    `mapstructure:"http" yaml:"http" json:"http"`
	Logging         ServerLoggingConfig `mapstructure:"logging" yaml:"logging" json:"logging"`
	ListenAddr      string              `mapstructure:"listen_addr,omitempty" yaml:"listen_addr,omitempty"`
	ReadTimeout     time.Duration       `mapstructure:"read_timeout,omitempty" yaml:"read_timeout,omitempty"`
	WriteTimeout    time.Duration       `mapstructure:"write_timeout,omitempty" yaml:"write_timeout,omitempty"`
	IdleTimeout     time.Duration       `mapstructure:"idle_timeout,omitempty" yaml:"idle_timeout,omitempty"`
	ShutdownTimeout time.Duration       `mapstructure:"shutdown_timeout,omitempty" yaml:"shutdown_timeout,omitempty"`
	AllowedOrigins  []string            `mapstructure:"allowed_origins,omitempty" yaml:"allowed_origins,omitempty"`
}

// ServerHTTPConfig configures HTTP-specific server options.
type ServerHTTPConfig struct {
	Listen           string        `mapstructure:"listen" yaml:"listen" json:"listen"`
	ListenAddr       string        `mapstructure:"listen_addr,omitempty" yaml:"listen_addr,omitempty"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout      time.Duration `mapstructure:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	AllowedOrigins   []string      `mapstructure:"allowed_origins" yaml:"allowed_origins"`
	AllowCredentials bool          `mapstructure:"allow_credentials" yaml:"allow_credentials"`
}

// ServerLoggingConfig configures logging options for the server.
type ServerLoggingConfig struct {
	Level            string `mapstructure:"level" yaml:"level" json:"level"`
	Format           string `mapstructure:"format" yaml:"format" json:"format"`
	TraceMCPPayloads bool   `mapstructure:"trace_mcp_payloads" yaml:"trace_mcp_payloads"`
	LogFile          string `mapstructure:"log_file,omitempty" yaml:"log_file,omitempty"`
}

// SafeShellConfig configures safe shell execution parameters.
type SafeShellConfig struct {
	Mode            string        `mapstructure:"mode" yaml:"mode" json:"mode"`
	CommandTimeout  time.Duration `mapstructure:"command_timeout" yaml:"command_timeout"`
	DefaultTimeout  time.Duration `mapstructure:"default_timeout,omitempty" yaml:"default_timeout,omitempty"`
	AllowedBinaries []string      `mapstructure:"allowed_binaries" yaml:"allowed_binaries"`
	MaxOutputBytes  int64         `mapstructure:"max_output_bytes" yaml:"max_output_bytes"`
}

// InstructionsConfig configures dynamic agent instructions and prompts.
type InstructionsConfig struct {
	Enabled          bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Compact          bool   `mapstructure:"compact" yaml:"compact" json:"compact"`
	DynamicTools     bool   `mapstructure:"dynamic_tools" yaml:"dynamic_tools"`
	RulesFile        string `mapstructure:"rules_file,omitempty" yaml:"rules_file,omitempty"`
	CustomRules      string `mapstructure:"custom_rules,omitempty" yaml:"custom_rules,omitempty"`
	CustomPromptPath string `mapstructure:"custom_prompt_path,omitempty" yaml:"custom_prompt_path,omitempty"`
	ExtraGuidance    string `mapstructure:"extra_guidance,omitempty" yaml:"extra_guidance,omitempty"`
}

// ToolsConfig catalogs external tool specs.
type ToolsConfig struct {
	GolangCILint ToolSpec `mapstructure:"golangci_lint" yaml:"golangci_lint"`
	Modernize    ToolSpec `mapstructure:"modernize" yaml:"modernize"`
	Deadcode     ToolSpec `mapstructure:"deadcode" yaml:"deadcode"`
	Selene       ToolSpec `mapstructure:"selene" yaml:"selene"`
	TestQuery    ToolSpec `mapstructure:"testquery" yaml:"testquery"`
}

// ToolSpec defines execution parameters and version tracking for an external binary/package.
type ToolSpec struct {
	BinaryName         string        `mapstructure:"binary_name" yaml:"binary_name"`
	Command            string        `mapstructure:"command,omitempty" yaml:"command,omitempty"`
	RecommendedVersion string        `mapstructure:"recommended_version" yaml:"recommended_version"`
	MinVersion         string        `mapstructure:"min_version,omitempty" yaml:"min_version,omitempty"`
	Pkg                string        `mapstructure:"pkg,omitempty" yaml:"pkg,omitempty"`
	Package            string        `mapstructure:"package,omitempty" yaml:"package,omitempty"`
	Config             string        `mapstructure:"config,omitempty" yaml:"config,omitempty"`
	Packages           string        `mapstructure:"packages,omitempty" yaml:"packages,omitempty"`
	DbPath             string        `mapstructure:"db_path,omitempty" yaml:"db_path,omitempty"`
	Format             string        `mapstructure:"format,omitempty" yaml:"format,omitempty"`
	Timeout            time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Args               []string      `mapstructure:"args,omitempty" yaml:"args,omitempty"`
	Disabled           bool          `mapstructure:"disabled" yaml:"disabled"`
	Workers            int           `mapstructure:"workers,omitempty" yaml:"workers,omitempty"`
	TestQueryCompat    bool          `mapstructure:"testquery_compat,omitempty" yaml:"testquery_compat,omitempty"`
}

func (ts *ToolSpec) normalize() {
	if ts.BinaryName != "" {
		ts.Command = ts.BinaryName
	} else if ts.Command != "" {
		ts.BinaryName = ts.Command
	}
	if ts.Pkg != "" {
		ts.Package = ts.Pkg
	} else if ts.Package != "" {
		ts.Pkg = ts.Package
	}
}

// FeaturesConfig holds global feature toggles.
type FeaturesConfig struct {
	Autofix                bool `mapstructure:"autofix" yaml:"autofix"`
	ModTidy                bool `mapstructure:"mod_tidy" yaml:"mod_tidy"`
	ModernizeCheck         bool `mapstructure:"modernize_check" yaml:"modernize_check"`
	DeadcodeCheck          bool `mapstructure:"deadcode_check" yaml:"deadcode_check"`
	FormatOnBuild          bool `mapstructure:"format_on_build" yaml:"format_on_build"`
	FormatOnEdit           bool `mapstructure:"format_on_edit" yaml:"format_on_edit"`
	VetGate                bool `mapstructure:"vet_gate" yaml:"vet_gate"`
	AutoRollback           bool `mapstructure:"auto_rollback" yaml:"auto_rollback"`
	TestQuerySync          bool `mapstructure:"testquery_sync" yaml:"testquery_sync"`
	TestQueryCompat        bool `mapstructure:"testquery_compat,omitempty" yaml:"testquery_compat,omitempty"`
	VersionCheckHints      bool `mapstructure:"version_check_hints" yaml:"version_check_hints"`
	CoverageGate           bool `mapstructure:"coverage_gate" yaml:"coverage_gate"`
	RaceDetector           bool `mapstructure:"race_detector" yaml:"race_detector"`
	StrictMode             bool `mapstructure:"strict_mode" yaml:"strict_mode"`
	RemoteDocFetch         bool `mapstructure:"remote_doc_fetch" yaml:"remote_doc_fetch"`
	VanityResolution       bool `mapstructure:"vanity_resolution" yaml:"vanity_resolution"`
	DocsCache              bool `mapstructure:"docs_cache" yaml:"docs_cache"`
	LevenshteinSuggestions bool `mapstructure:"levenshtein_suggestions,omitempty" yaml:"levenshtein_suggestions,omitempty"`
	MutationTesting        bool `mapstructure:"mutation_testing,omitempty" yaml:"mutation_testing,omitempty"`
}

// BuildConfig configures the smart_build pipeline.
type BuildConfig struct {
	DefaultPackages string        `mapstructure:"default_packages" yaml:"default_packages"`
	Output          string        `mapstructure:"output,omitempty" yaml:"output,omitempty"`
	Tags            []string      `mapstructure:"tags,omitempty" yaml:"tags,omitempty"`
	Race            bool          `mapstructure:"race" yaml:"race" json:"race"`
	Trimpath        bool          `mapstructure:"trimpath" yaml:"trimpath" json:"trimpath"`
	Timeout         time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Flags           []string      `mapstructure:"flags,omitempty" yaml:"flags,omitempty"`
	AutoTidy        bool          `mapstructure:"auto_tidy,omitempty" yaml:"auto_tidy,omitempty"`
	AutoFormat      bool          `mapstructure:"auto_format,omitempty" yaml:"auto_format,omitempty"`
	AutoModernize   bool          `mapstructure:"auto_modernize,omitempty" yaml:"auto_modernize,omitempty"`
	RunLinter       bool          `mapstructure:"run_linter,omitempty" yaml:"run_linter,omitempty"`
	RunDeadcode     bool          `mapstructure:"run_deadcode,omitempty" yaml:"run_deadcode,omitempty"`
	RunTests        bool          `mapstructure:"run_tests,omitempty" yaml:"run_tests,omitempty"`
}

// AutofixConfig configures automated remediation phases.
type AutofixConfig struct {
	Enabled   bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	DryRun    bool     `mapstructure:"dry_run" yaml:"dry_run" json:"dry_run"`
	Order     []string `mapstructure:"order,omitempty" yaml:"order,omitempty"`
	ModTidy   bool     `mapstructure:"mod_tidy,omitempty" yaml:"mod_tidy,omitempty"`
	Modernize bool     `mapstructure:"modernize,omitempty" yaml:"modernize,omitempty"`
	Gofmt     bool     `mapstructure:"gofmt,omitempty" yaml:"gofmt,omitempty"`
	Deadcode  bool     `mapstructure:"deadcode,omitempty" yaml:"deadcode,omitempty"`
}

// EditConfig configures smart_edit coordinate editing.
type EditConfig struct {
	BackupStrategy         string        `mapstructure:"backup_strategy" yaml:"backup_strategy"`
	AtomicWrite            bool          `mapstructure:"atomic_write" yaml:"atomic_write"`
	PreservePermissions    bool          `mapstructure:"preserve_permissions" yaml:"preserve_permissions"`
	DefaultThreshold       float64       `mapstructure:"default_threshold" yaml:"default_threshold"`
	FuzzyThreshold         float64       `mapstructure:"fuzzy_threshold,omitempty" yaml:"fuzzy_threshold,omitempty"`
	FormatOnSave           string        `mapstructure:"format_on_save" yaml:"format_on_save"`
	ExcludePaths           []string      `mapstructure:"exclude_paths" yaml:"exclude_paths"`
	ExcludeDirs            []string      `mapstructure:"exclude_dirs,omitempty" yaml:"exclude_dirs,omitempty"`
	MaxLevenshteinDistance int           `mapstructure:"max_levenshtein_distance" yaml:"max_levenshtein_distance"`
	CompilerGate           string        `mapstructure:"compiler_gate" yaml:"compiler_gate"`
	Timeout                time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
}

// MatchingConfig configures AST/fuzzy text matching parameters in smart_edit.
type MatchingConfig struct {
	FuzzyFallback        bool    `mapstructure:"fuzzy_fallback" yaml:"fuzzy_fallback"`
	SimilarityThreshold  float64 `mapstructure:"similarity_threshold" yaml:"similarity_threshold"`
	DefaultThreshold     float64 `mapstructure:"default_threshold,omitempty" yaml:"default_threshold,omitempty"`
	NormalizeUnicode     bool    `mapstructure:"normalize_unicode" yaml:"normalize_unicode"`
	MinSeedLength        int     `mapstructure:"min_seed_length" yaml:"min_seed_length"`
	WindowExpansionDelta int     `mapstructure:"window_expansion_delta" yaml:"window_expansion_delta"`
	MaxWindowLines       int     `mapstructure:"max_window_lines" yaml:"max_window_lines"`
}

// DiagnosticsConfig configures error diagnosis and Levenshtein suggestion behavior.
type DiagnosticsConfig struct {
	CollectOnEdit          bool          `mapstructure:"collect_on_edit" yaml:"collect_on_edit"`
	VerificationScope      string        `mapstructure:"verification_scope" yaml:"verification_scope"`
	CheckCommand           []string      `mapstructure:"check_command" yaml:"check_command"`
	Timeout                time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	EnableSuggestions      bool          `mapstructure:"enable_suggestions" yaml:"enable_suggestions"`
	LevenshteinSuggestions bool          `mapstructure:"levenshtein_suggestions" yaml:"levenshtein_suggestions"`
	MaxLevenshteinDistance int           `mapstructure:"max_levenshtein_distance" yaml:"max_levenshtein_distance"`
	MaxDistance            int           `mapstructure:"max_distance,omitempty" yaml:"max_distance,omitempty"`
	MaxSuggestions         int           `mapstructure:"max_suggestions" yaml:"max_suggestions"`
	SuggestionsLimit       int           `mapstructure:"suggestions_limit,omitempty" yaml:"suggestions_limit,omitempty"`
	SnippetContextLines    int           `mapstructure:"snippet_context_lines" yaml:"snippet_context_lines"`
}

// TestConfig configures the smart_test runner.
type TestConfig struct {
	DefaultLevel      string        `mapstructure:"default_level" yaml:"default_level"`
	DefaultPackages   string        `mapstructure:"default_packages" yaml:"default_packages"`
	Timeout           time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Verbose           bool          `mapstructure:"verbose" yaml:"verbose" json:"verbose"`
	RaceDetector      bool          `mapstructure:"race_detector" yaml:"race_detector"`
	CoverageThreshold float64       `mapstructure:"coverage_threshold" yaml:"coverage_threshold"`
	CoverageProfile   string        `mapstructure:"coverage_profile" yaml:"coverage_profile"`
	CoverageOutputDir string        `mapstructure:"coverage_output_dir,omitempty" yaml:"coverage_output_dir,omitempty"`
	BenchmarkPattern  string        `mapstructure:"benchmark_pattern" yaml:"benchmark_pattern"`
	BenchmarkFlags    []string      `mapstructure:"benchmark_flags,omitempty" yaml:"benchmark_flags,omitempty"`
	BenchmarkMem      bool          `mapstructure:"benchmark_mem" yaml:"benchmark_mem"`
}

// TestQueryConfig configures SQL analytics against testquery.db.
type TestQueryConfig struct {
	DatabasePath string        `mapstructure:"db_path" yaml:"db_path" json:"db_path"`
	WALMode      bool          `mapstructure:"wal_mode" yaml:"wal_mode" json:"wal_mode"`
	BusyTimeout  time.Duration `mapstructure:"busy_timeout" yaml:"busy_timeout" json:"busy_timeout"`
	Format       string        `mapstructure:"format" yaml:"format" json:"format"`
}

// DocsConfig configures read_docs AST documentation extraction.
type DocsConfig struct {
	CacheEnabled           bool          `mapstructure:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL               time.Duration `mapstructure:"cache_ttl" yaml:"cache_ttl"`
	CacheMaxEntries        int           `mapstructure:"cache_max_entries" yaml:"cache_max_entries"`
	ExternalFetch          bool          `mapstructure:"external_fetch" yaml:"external_fetch"`
	OfflineMode            bool          `mapstructure:"offline_mode" yaml:"offline_mode"`
	PkgGoDevURL            string        `mapstructure:"pkg_go_dev_url" yaml:"pkg_go_dev_url"`
	MaxSymbolsRendered     int           `mapstructure:"max_symbols_rendered" yaml:"max_symbols_rendered"`
	FuzzySuggestions       bool          `mapstructure:"fuzzy_suggestions" yaml:"fuzzy_suggestions"`
	MaxFuzzySuggestions    int           `mapstructure:"max_fuzzy_suggestions" yaml:"max_fuzzy_suggestions"`
	FuzzyDistanceThreshold int           `mapstructure:"fuzzy_distance_threshold" yaml:"fuzzy_distance_threshold"`
	DefaultFormat          string        `mapstructure:"default_format" yaml:"default_format"`
	TempDir                string        `mapstructure:"temp_dir,omitempty" yaml:"temp_dir,omitempty"`
}

func defaultToolsConfig() ToolsConfig {
	return ToolsConfig{
		GolangCILint: ToolSpec{
			BinaryName:         ToolGolangCILint,
			Command:            ToolGolangCILint,
			RecommendedVersion: "v2.12.2",
			MinVersion:         "v1.60.0",
			Pkg:                "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2",
			Package:            "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2",
			Config:             ".golangci.yml",
			Timeout:            5 * time.Minute,
			Args:               []string{"run"},
			Disabled:           false,
		},
		Modernize: ToolSpec{
			BinaryName:         ToolModernize,
			Command:            ToolModernize,
			RecommendedVersion: VersionLatest,
			Pkg:                "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest",
			Package:            "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest",
			Timeout:            2 * time.Minute,
			Args:               []string{"-fix"},
			Disabled:           false,
		},
		Deadcode: ToolSpec{
			BinaryName:         ToolDeadcode,
			Command:            ToolDeadcode,
			RecommendedVersion: VersionLatest,
			Pkg:                "golang.org/x/tools/cmd/deadcode@latest",
			Package:            "golang.org/x/tools/cmd/deadcode@latest",
			Timeout:            2 * time.Minute,
			Disabled:           false,
		},
		Selene: ToolSpec{
			BinaryName:         ToolSelene,
			Command:            ToolSelene,
			RecommendedVersion: VersionLatest,
			Pkg:                "github.com/danicat/selene/cmd/selene@latest",
			Package:            "github.com/danicat/selene/cmd/selene@latest",
			Packages:           DefaultWildcardPackages,
			Timeout:            3 * time.Minute,
			Workers:            0,
			TestQueryCompat:    true,
			DbPath:             DefaultTestQueryDB,
			Disabled:           false,
		},
		TestQuery: ToolSpec{
			BinaryName:         ToolTestQuery,
			Command:            ToolTestQuery,
			RecommendedVersion: VersionLatest,
			Pkg:                "github.com/danicat/testquery@latest",
			Package:            "github.com/danicat/testquery@latest",
			DbPath:             DefaultTestQueryDB,
			Config:             DefaultTestQueryDB,
			Format:             DefaultFormatTable,
			Timeout:            2 * time.Minute,
			Disabled:           false,
		},
	}
}

func defaultServerConfig() ServerConfig {
	return ServerConfig{
		Name:      "godoctor",
		Transport: "stdio",
		HTTP: ServerHTTPConfig{
			Listen:          DefaultListenAddr,
			ListenAddr:      DefaultListenAddr,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    5 * time.Minute,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			AllowedOrigins: []string{
				"http://localhost",
				"http://localhost:*",
				"http://127.0.0.1",
				"http://127.0.0.1:*",
			},
			AllowCredentials: true,
		},
		Logging: ServerLoggingConfig{
			Level:            "info",
			Format:           DefaultOutputText,
			TraceMCPPayloads: false,
			LogFile:          "",
		},
		ListenAddr:      DefaultListenAddr,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    5 * time.Minute,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		AllowedOrigins: []string{
			"http://localhost",
			"http://localhost:*",
			"http://127.0.0.1",
			"http://127.0.0.1:*",
		},
	}
}

func defaultBuildConfig() BuildConfig {
	return BuildConfig{
		DefaultPackages: DefaultWildcardPackages,
		Output:          "",
		Tags:            nil,
		Race:            false,
		Trimpath:        false,
		Timeout:         5 * time.Minute,
		Flags:           nil,
		AutoTidy:        true,
		AutoFormat:      true,
		AutoModernize:   true,
		RunLinter:       true,
		RunDeadcode:     true,
		RunTests:        true,
	}
}

func defaultEditConfig() EditConfig {
	return EditConfig{
		BackupStrategy:         "memory",
		AtomicWrite:            true,
		PreservePermissions:    true,
		DefaultThreshold:       0.95,
		FuzzyThreshold:         0.95,
		FormatOnSave:           "goimports",
		ExcludePaths:           []string{".git", "vendor", "node_modules", "skills", "agents", "hooks"},
		ExcludeDirs:            []string{".git", "vendor", "node_modules", "skills", "agents", "hooks"},
		MaxLevenshteinDistance: 3,
		CompilerGate:           "go vet ./...",
		Timeout:                30 * time.Second,
	}
}

func defaultTestConfig() TestConfig {
	return TestConfig{
		DefaultLevel:      DefaultTestLevel,
		DefaultPackages:   DefaultWildcardPackages,
		Timeout:           60 * time.Second,
		Verbose:           true,
		RaceDetector:      false,
		CoverageThreshold: 80.0,
		CoverageProfile:   "coverage.out",
		CoverageOutputDir: "",
		BenchmarkPattern:  ".",
		BenchmarkFlags:    []string{"-benchmem", "-run=NONE"},
		BenchmarkMem:      true,
	}
}

// NewDefaultConfig returns a fully populated Config struct with built-in production defaults matching RFC-0001.
func NewDefaultConfig() *Config {
	return &Config{
		Version: "1",
		CLI: CLIConfig{
			Timeout:       60 * time.Second,
			OutputFormat:  DefaultOutputText,
			Color:         true,
			LogLevel:      "info",
			DefaultOutput: DefaultOutputText,
			Quiet:         false,
		},
		Server: defaultServerConfig(),
		SafeShell: SafeShellConfig{
			Mode:           "standard",
			CommandTimeout: 120 * time.Second,
			DefaultTimeout: 120 * time.Second,
			AllowedBinaries: []string{
				"go",
				"gofmt",
				ToolGolangCILint,
				ToolSelene,
				ToolTestQuery,
				"tq",
				ToolDeadcode,
				ToolModernize,
			},
			MaxOutputBytes: 10 * 1024 * 1024, // 10MB
		},
		Instructions: InstructionsConfig{
			Enabled:          true,
			Compact:          false,
			DynamicTools:     true,
			RulesFile:        "",
			CustomRules:      "",
			CustomPromptPath: "",
			ExtraGuidance:    "",
		},
		Tools: defaultToolsConfig(),
		Features: FeaturesConfig{
			Autofix:                true,
			ModTidy:                true,
			ModernizeCheck:         true,
			DeadcodeCheck:          true,
			FormatOnBuild:          true,
			FormatOnEdit:           true,
			VetGate:                true,
			AutoRollback:           true,
			TestQuerySync:          true,
			TestQueryCompat:        true,
			VersionCheckHints:      true,
			CoverageGate:           false,
			RaceDetector:           false,
			StrictMode:             false,
			RemoteDocFetch:         true,
			VanityResolution:       true,
			DocsCache:              true,
			LevenshteinSuggestions: true,
			MutationTesting:        true,
		},
		Build: defaultBuildConfig(),
		Autofix: AutofixConfig{
			Enabled:   true,
			DryRun:    false,
			Order:     []string{AutofixTidy, ToolModernize, "gofmt"},
			ModTidy:   true,
			Modernize: true,
			Gofmt:     true,
			Deadcode:  true,
		},
		Edit: defaultEditConfig(),
		Matching: MatchingConfig{
			FuzzyFallback:        true,
			SimilarityThreshold:  0.95,
			DefaultThreshold:     0.95,
			NormalizeUnicode:     true,
			MinSeedLength:        3,
			WindowExpansionDelta: 4,
			MaxWindowLines:       100,
		},
		Diagnostics: DiagnosticsConfig{
			CollectOnEdit:          true,
			VerificationScope:      "module",
			CheckCommand:           []string{"go", ToolVet, DefaultWildcardPackages},
			Timeout:                30 * time.Second,
			EnableSuggestions:      true,
			LevenshteinSuggestions: true,
			MaxLevenshteinDistance: 3,
			MaxDistance:            3,
			MaxSuggestions:         5,
			SuggestionsLimit:       5,
			SnippetContextLines:    5,
		},
		Test: defaultTestConfig(),
		TestQuery: TestQueryConfig{
			DatabasePath: DefaultTestQueryDB,
			WALMode:      true,
			BusyTimeout:  5 * time.Second,
			Format:       DefaultFormatTable,
		},
		Docs: DocsConfig{
			CacheEnabled:           true,
			CacheTTL:               15 * time.Minute,
			CacheMaxEntries:        500,
			ExternalFetch:          true,
			OfflineMode:            false,
			PkgGoDevURL:            "https://pkg.go.dev",
			MaxSymbolsRendered:     100,
			FuzzySuggestions:       true,
			MaxFuzzySuggestions:    5,
			FuzzyDistanceThreshold: 2,
			DefaultFormat:          DefaultFormatMarkdown,
			TempDir:                "",
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

	cfg.normalize()
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

func (c *Config) normalize() {
	// CLI
	if c.CLI.OutputFormat != "" {
		c.CLI.DefaultOutput = c.CLI.OutputFormat
	} else if c.CLI.DefaultOutput != "" {
		c.CLI.OutputFormat = c.CLI.DefaultOutput
	}

	// Server
	if c.Server.HTTP.Listen != "" {
		c.Server.ListenAddr = c.Server.HTTP.Listen
		c.Server.HTTP.ListenAddr = c.Server.HTTP.Listen
	} else if c.Server.ListenAddr != "" {
		c.Server.HTTP.Listen = c.Server.ListenAddr
		c.Server.HTTP.ListenAddr = c.Server.ListenAddr
	}
	if c.Server.HTTP.ReadTimeout != 0 {
		c.Server.ReadTimeout = c.Server.HTTP.ReadTimeout
	} else if c.Server.ReadTimeout != 0 {
		c.Server.HTTP.ReadTimeout = c.Server.ReadTimeout
	}
	if c.Server.HTTP.WriteTimeout != 0 {
		c.Server.WriteTimeout = c.Server.HTTP.WriteTimeout
	} else if c.Server.WriteTimeout != 0 {
		c.Server.HTTP.WriteTimeout = c.Server.WriteTimeout
	}
	if c.Server.HTTP.IdleTimeout != 0 {
		c.Server.IdleTimeout = c.Server.HTTP.IdleTimeout
	} else if c.Server.IdleTimeout != 0 {
		c.Server.HTTP.IdleTimeout = c.Server.IdleTimeout
	}
	if c.Server.HTTP.ShutdownTimeout != 0 {
		c.Server.ShutdownTimeout = c.Server.HTTP.ShutdownTimeout
	} else if c.Server.ShutdownTimeout != 0 {
		c.Server.HTTP.ShutdownTimeout = c.Server.ShutdownTimeout
	}
	if len(c.Server.HTTP.AllowedOrigins) > 0 {
		c.Server.AllowedOrigins = c.Server.HTTP.AllowedOrigins
	} else if len(c.Server.AllowedOrigins) > 0 {
		c.Server.HTTP.AllowedOrigins = c.Server.AllowedOrigins
	}

	// SafeShell
	if c.SafeShell.CommandTimeout != 0 {
		c.SafeShell.DefaultTimeout = c.SafeShell.CommandTimeout
	} else if c.SafeShell.DefaultTimeout != 0 {
		c.SafeShell.CommandTimeout = c.SafeShell.DefaultTimeout
	}

	// Tools
	c.Tools.GolangCILint.normalize()
	c.Tools.Modernize.normalize()
	c.Tools.Deadcode.normalize()
	c.Tools.Selene.normalize()
	c.Tools.TestQuery.normalize()

	// Edit
	if c.Edit.DefaultThreshold != 0 {
		c.Edit.FuzzyThreshold = c.Edit.DefaultThreshold
	} else if c.Edit.FuzzyThreshold != 0 {
		c.Edit.DefaultThreshold = c.Edit.FuzzyThreshold
	}
	if len(c.Edit.ExcludePaths) > 0 {
		c.Edit.ExcludeDirs = c.Edit.ExcludePaths
	} else if len(c.Edit.ExcludeDirs) > 0 {
		c.Edit.ExcludePaths = c.Edit.ExcludeDirs
	}

	// Matching
	if c.Matching.SimilarityThreshold != 0 {
		c.Matching.DefaultThreshold = c.Matching.SimilarityThreshold
	} else if c.Matching.DefaultThreshold != 0 {
		c.Matching.SimilarityThreshold = c.Matching.DefaultThreshold
	}

	// Diagnostics
	if c.Diagnostics.EnableSuggestions {
		c.Diagnostics.LevenshteinSuggestions = true
	} else if !c.Diagnostics.EnableSuggestions {
		c.Diagnostics.LevenshteinSuggestions = false
	}
	if c.Diagnostics.MaxLevenshteinDistance != 0 {
		c.Diagnostics.MaxDistance = c.Diagnostics.MaxLevenshteinDistance
	} else if c.Diagnostics.MaxDistance != 0 {
		c.Diagnostics.MaxLevenshteinDistance = c.Diagnostics.MaxDistance
	}
	if c.Diagnostics.MaxSuggestions != 0 {
		c.Diagnostics.SuggestionsLimit = c.Diagnostics.MaxSuggestions
	} else if c.Diagnostics.SuggestionsLimit != 0 {
		c.Diagnostics.MaxSuggestions = c.Diagnostics.SuggestionsLimit
	}
}

var semverRegex = regexp.MustCompile(`^(latest|v?[0-9]+(\.[0-9]+)*(-[0-9A-Za-z.-]+)?)$`)

// Validate checks internal consistency and parameter bounds across all configuration sections.
func (c *Config) Validate() error {
	if c.Version != "" && c.Version != "1" {
		return fmt.Errorf("unsupported configuration schema version: %q (expected '1')", c.Version)
	}
	if err := validateTools(c.Tools); err != nil {
		return err
	}
	if err := validateEditAndMatching(c.Edit, c.Matching, c.Diagnostics); err != nil {
		return err
	}
	if err := validateTestAndDocs(c.Test, c.Docs, c.TestQuery); err != nil {
		return err
	}
	return validateServerAndShell(c.CLI, c.Build, c.Server, c.SafeShell)
}

func validateTools(tools ToolsConfig) error {
	specs := map[string]ToolSpec{
		ToolGolangCILintKey: tools.GolangCILint,
		ToolModernize:       tools.Modernize,
		ToolDeadcode:        tools.Deadcode,
		ToolSelene:          tools.Selene,
		ToolTestQuery:       tools.TestQuery,
	}

	for name, tool := range specs {
		if tool.Disabled {
			continue
		}
		if tool.RecommendedVersion != "" && !semverRegex.MatchString(tool.RecommendedVersion) {
			return fmt.Errorf("tool %q has invalid recommended_version: %q", name, tool.RecommendedVersion)
		}
		if tool.MinVersion != "" && !semverRegex.MatchString(tool.MinVersion) {
			return fmt.Errorf("tool %q has invalid min_version: %q", name, tool.MinVersion)
		}
		if tool.Timeout < 0 {
			return fmt.Errorf("tool %q timeout cannot be negative: %v", name, tool.Timeout)
		}
	}
	return nil
}

func validateEditAndMatching(edit EditConfig, matching MatchingConfig, diag DiagnosticsConfig) error {
	if edit.DefaultThreshold < 0.0 || edit.DefaultThreshold > 1.0 {
		return fmt.Errorf("edit.default_threshold must be between 0.0 and 1.0 (got %f)", edit.DefaultThreshold)
	}
	if edit.FuzzyThreshold < 0.0 || edit.FuzzyThreshold > 1.0 {
		return fmt.Errorf("edit.fuzzy_threshold must be between 0.0 and 1.0 (got %f)", edit.FuzzyThreshold)
	}
	if edit.MaxLevenshteinDistance < 0 {
		return fmt.Errorf("edit.max_levenshtein_distance cannot be negative: %d", edit.MaxLevenshteinDistance)
	}
	if edit.Timeout < 0 {
		return fmt.Errorf("edit.timeout cannot be negative: %v", edit.Timeout)
	}
	if matching.SimilarityThreshold < 0.0 || matching.SimilarityThreshold > 1.0 {
		return fmt.Errorf("matching.similarity_threshold must be between 0.0 and 1.0 (got %f)", matching.SimilarityThreshold)
	}
	if matching.DefaultThreshold < 0.0 || matching.DefaultThreshold > 1.0 {
		return fmt.Errorf("matching.default_threshold must be between 0.0 and 1.0 (got %f)", matching.DefaultThreshold)
	}
	if matching.MinSeedLength < 0 {
		return fmt.Errorf("matching.min_seed_length cannot be negative: %d", matching.MinSeedLength)
	}
	if matching.WindowExpansionDelta < 0 {
		return fmt.Errorf("matching.window_expansion_delta cannot be negative: %d", matching.WindowExpansionDelta)
	}
	if matching.MaxWindowLines < 0 {
		return fmt.Errorf("matching.max_window_lines cannot be negative: %d", matching.MaxWindowLines)
	}
	if diag.MaxLevenshteinDistance < 0 {
		return fmt.Errorf("diagnostics.max_levenshtein_distance cannot be negative: %d", diag.MaxLevenshteinDistance)
	}
	if diag.MaxDistance < 0 {
		return fmt.Errorf("diagnostics.max_distance cannot be negative: %d", diag.MaxDistance)
	}
	if diag.MaxSuggestions < 0 {
		return fmt.Errorf("diagnostics.max_suggestions cannot be negative: %d", diag.MaxSuggestions)
	}
	if diag.SuggestionsLimit < 0 {
		return fmt.Errorf("diagnostics.suggestions_limit cannot be negative: %d", diag.SuggestionsLimit)
	}
	if diag.SnippetContextLines < 0 {
		return fmt.Errorf("diagnostics.snippet_context_lines cannot be negative: %d", diag.SnippetContextLines)
	}
	if diag.Timeout < 0 {
		return fmt.Errorf("diagnostics.timeout cannot be negative: %v", diag.Timeout)
	}
	return nil
}

func validateTestAndDocs(test TestConfig, docs DocsConfig, tq TestQueryConfig) error {
	switch strings.ToLower(test.DefaultLevel) {
	case "fast", "basic", "benchmark", "complete", "":
	default:
		return fmt.Errorf("invalid test.default_level: %q (must be fast, basic, benchmark, or complete)", test.DefaultLevel)
	}
	if test.Timeout < 0 {
		return fmt.Errorf("test.timeout cannot be negative: %v", test.Timeout)
	}
	if test.CoverageThreshold < 0.0 || test.CoverageThreshold > 100.0 {
		return fmt.Errorf("test.coverage_threshold must be between 0.0 and 100.0 (got %f)", test.CoverageThreshold)
	}

	switch strings.ToLower(docs.DefaultFormat) {
	case DefaultFormatMarkdown, DefaultFormatJSON, "":
	default:
		return fmt.Errorf("invalid docs.default_format: %q (must be markdown or json)", docs.DefaultFormat)
	}
	if docs.CacheTTL < 0 {
		return fmt.Errorf("docs.cache_ttl cannot be negative: %v", docs.CacheTTL)
	}
	if docs.CacheMaxEntries < 0 {
		return fmt.Errorf("docs.cache_max_entries cannot be negative: %d", docs.CacheMaxEntries)
	}
	if docs.MaxSymbolsRendered < 0 {
		return fmt.Errorf("docs.max_symbols_rendered cannot be negative: %d", docs.MaxSymbolsRendered)
	}
	if docs.MaxFuzzySuggestions < 0 {
		return fmt.Errorf("docs.max_fuzzy_suggestions cannot be negative: %d", docs.MaxFuzzySuggestions)
	}
	if docs.FuzzyDistanceThreshold < 0 {
		return fmt.Errorf("docs.fuzzy_distance_threshold cannot be negative: %d", docs.FuzzyDistanceThreshold)
	}

	if tq.BusyTimeout < 0 {
		return fmt.Errorf("testquery.busy_timeout cannot be negative: %v", tq.BusyTimeout)
	}
	switch strings.ToLower(tq.Format) {
	case DefaultFormatTable, DefaultFormatJSON, "csv", "":
	default:
		return fmt.Errorf("invalid testquery.format: %q (must be table, json, or csv)", tq.Format)
	}

	return nil
}

func validateServerAndShell(cli CLIConfig, build BuildConfig, server ServerConfig, shell SafeShellConfig) error {
	if cli.Timeout < 0 {
		return errors.New("cli timeout must be non-negative")
	}
	if build.Timeout < 0 {
		return errors.New("build timeout must be non-negative")
	}
	if server.ReadTimeout < 0 || server.WriteTimeout < 0 ||
		server.IdleTimeout < 0 || server.ShutdownTimeout < 0 {
		return errors.New("server timeouts must be non-negative")
	}
	if server.HTTP.ReadTimeout < 0 ||
		server.HTTP.WriteTimeout < 0 ||
		server.HTTP.IdleTimeout < 0 ||
		server.HTTP.ShutdownTimeout < 0 {
		return errors.New("server HTTP timeouts must be non-negative")
	}
	if shell.CommandTimeout < 0 || shell.DefaultTimeout < 0 {
		return errors.New("safeshell command_timeout must be non-negative")
	}
	if shell.MaxOutputBytes < 0 {
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
	case ToolGolangCILintKey, ToolGolangCILint, "linter", "lint":
		return c.Tools.GolangCILint, true
	case ToolModernize, "modernizer":
		return c.Tools.Modernize, true
	case ToolDeadcode:
		return c.Tools.Deadcode, true
	case ToolSelene, ToolMutation, "mutation_test", "mutation-test":
		return c.Tools.Selene, true
	case ToolTestQuery, "tq", "test_query", "test-query":
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
	case "mod_tidy", "modtidy", "tidy":
		return c.Features.ModTidy
	case "modernize_check", ToolModernize:
		return c.Features.ModernizeCheck
	case "deadcode_check", ToolDeadcode:
		return c.Features.DeadcodeCheck
	case "format_on_build", "formatonbuild":
		return c.Features.FormatOnBuild
	case "format_on_edit", "formatonedit":
		return c.Features.FormatOnEdit
	case "vet_gate", "vetgate", ToolVet:
		return c.Features.VetGate
	case "auto_rollback", "autorollback", "rollback":
		return c.Features.AutoRollback
	case "testquery_sync", ToolTestQuery, "tq", "test_query_sync":
		return c.Features.TestQuerySync
	case "version_check_hints", "versioncheck", "version_hints", "version_check":
		return c.Features.VersionCheckHints
	case "coverage_gate", "coveragegate", "coverage":
		return c.Features.CoverageGate
	case "race_detector", "racedetector", "race":
		return c.Features.RaceDetector
	case "strict_mode", "strictmode", ToolStrict:
		return c.Features.StrictMode
	case "remote_doc_fetch", "remotedocfetch":
		return c.Features.RemoteDocFetch
	case "vanity_resolution", "vanityresolution":
		return c.Features.VanityResolution
	case "docs_cache", "docscache":
		return c.Features.DocsCache
	case "levenshtein_suggestions", "suggestions":
		return c.Features.LevenshteinSuggestions
	case "mutation_testing", ToolSelene, ToolMutation:
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
		dbName = c.Tools.TestQuery.DbPath
	}
	if dbName == "" {
		dbName = DefaultTestQueryDB
	}
	if filepath.IsAbs(dbName) {
		return dbName
	}
	return filepath.Join(workspaceDir, dbName)
}

// GetSeleneWorkers returns the configured workers or runtime.GOMAXPROCS(0) if 0 or negative.
func (c *Config) GetSeleneWorkers() int {
	if c != nil && c.Tools.Selene.Workers > 0 {
		return c.Tools.Selene.Workers
	}
	workers := runtime.GOMAXPROCS(0)
	if workers <= 0 {
		workers = 1
	}
	return workers
}

// GetSeleneDBPath returns the resolved database path for Selene testquery integration.
func (c *Config) GetSeleneDBPath(workspaceDir string) string {
	if c == nil {
		return filepath.Join(workspaceDir, DefaultTestQueryDB)
	}
	if c.Tools.Selene.DbPath != "" {
		if filepath.IsAbs(c.Tools.Selene.DbPath) {
			return c.Tools.Selene.DbPath
		}
		return filepath.Join(workspaceDir, c.Tools.Selene.DbPath)
	}
	return c.GetTestQueryDBPath(workspaceDir)
}

// IsSeleneTestQueryCompat returns whether testquery compatibility mode is enabled for Selene.
func (c *Config) IsSeleneTestQueryCompat() bool {
	if c == nil {
		return true
	}
	if c.Tools.Selene.TestQueryCompat {
		return true
	}
	return c.Features.TestQueryCompat
}
