package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}

	if cfg.Version != "1" {
		t.Errorf("expected version 1, got %q", cfg.Version)
	}

	// CLI defaults
	if cfg.CLI.OutputFormat != "text" || cfg.CLI.DefaultOutput != "text" {
		t.Errorf("expected output format text, got %q", cfg.CLI.OutputFormat)
	}
	if cfg.CLI.Timeout != 60*time.Second {
		t.Errorf("expected CLI timeout 60s, got %v", cfg.CLI.Timeout)
	}

	// Server defaults
	if cfg.Server.HTTP.Listen != ":8080" || cfg.Server.ListenAddr != ":8080" {
		t.Errorf("expected server listen :8080, got %q", cfg.Server.HTTP.Listen)
	}
	if cfg.Server.HTTP.WriteTimeout != 5*time.Minute {
		t.Errorf("expected server write timeout 5m, got %v", cfg.Server.HTTP.WriteTimeout)
	}

	// SafeShell defaults
	if cfg.SafeShell.Mode != "standard" {
		t.Errorf("expected safeshell mode standard, got %q", cfg.SafeShell.Mode)
	}
	if cfg.SafeShell.CommandTimeout != 120*time.Second {
		t.Errorf("expected safeshell timeout 120s, got %v", cfg.SafeShell.CommandTimeout)
	}

	// Build defaults
	if cfg.Build.DefaultPackages != "./..." {
		t.Errorf("expected build default_packages to be './...', got %q", cfg.Build.DefaultPackages)
	}
	if cfg.Build.Timeout != 5*time.Minute {
		t.Errorf("expected build timeout 5m, got %v", cfg.Build.Timeout)
	}

	// Test defaults
	if cfg.Test.DefaultLevel != "basic" {
		t.Errorf("expected test default_level to be 'basic', got %q", cfg.Test.DefaultLevel)
	}

	// Edit defaults
	if cfg.Edit.DefaultThreshold != 0.95 || cfg.Edit.FuzzyThreshold != 0.95 {
		t.Errorf("expected edit fuzzy threshold to be 0.95, got %f", cfg.Edit.DefaultThreshold)
	}

	// Matching defaults
	if cfg.Matching.SimilarityThreshold != 0.95 {
		t.Errorf("expected similarity threshold 0.95, got %f", cfg.Matching.SimilarityThreshold)
	}

	// Diagnostics defaults
	if !cfg.Diagnostics.EnableSuggestions || !cfg.Diagnostics.LevenshteinSuggestions {
		t.Error("expected diagnostics suggestions to be enabled")
	}

	// Features defaults
	if !cfg.Features.Autofix {
		t.Error("expected autofix feature to be enabled by default")
	}
	if !cfg.Features.ModTidy {
		t.Error("expected mod_tidy feature to be enabled by default")
	}

	// TestQuery defaults
	if cfg.TestQuery.DatabasePath != ".godoctor/testquery.db" {
		t.Errorf("expected db_path '.godoctor/testquery.db', got %q", cfg.TestQuery.DatabasePath)
	}

	// Docs defaults
	if cfg.Docs.CacheTTL != 15*time.Minute {
		t.Errorf("expected docs cache_ttl 15m, got %v", cfg.Docs.CacheTTL)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to pass validation, got: %v", err)
	}
}

func verifyLoadedCLIAndServer(t *testing.T, cfg *Config) {
	t.Helper()
	if cfg.CLI.OutputFormat != "json" || cfg.CLI.DefaultOutput != "json" {
		t.Errorf("expected CLI output format json, got %q", cfg.CLI.OutputFormat)
	}
	if cfg.CLI.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %q", cfg.CLI.LogLevel)
	}
	if cfg.Server.Name != "custom-godoctor" {
		t.Errorf("expected server name 'custom-godoctor', got %q", cfg.Server.Name)
	}
	if cfg.Server.HTTP.Listen != ":9090" || cfg.Server.ListenAddr != ":9090" {
		t.Errorf("expected listen :9090, got %q", cfg.Server.HTTP.Listen)
	}
	if cfg.Server.HTTP.WriteTimeout != 10*time.Minute {
		t.Errorf("expected write timeout 10m, got %v", cfg.Server.HTTP.WriteTimeout)
	}
}

func verifyLoadedSafeShellAndTools(t *testing.T, cfg *Config) {
	t.Helper()
	if cfg.SafeShell.Mode != "strict" {
		t.Errorf("expected safeshell mode strict, got %q", cfg.SafeShell.Mode)
	}
	if cfg.SafeShell.CommandTimeout != 90*time.Second ||
		cfg.SafeShell.DefaultTimeout != 90*time.Second {
		t.Errorf("expected safeshell timeout 90s, got %v", cfg.SafeShell.CommandTimeout)
	}
	if !cfg.Instructions.Compact || cfg.Instructions.DynamicTools {
		t.Errorf("unexpected instructions settings: %+v", cfg.Instructions)
	}
	if cfg.Instructions.RulesFile != ".godoctor/rules.md" {
		t.Errorf("expected rules file .godoctor/rules.md, got %q", cfg.Instructions.RulesFile)
	}
	if cfg.Tools.GolangCILint.RecommendedVersion != "v2.15.0" {
		t.Errorf("expected recommended_version v2.15.0, got %q", cfg.Tools.GolangCILint.RecommendedVersion)
	}
	if cfg.Tools.GolangCILint.BinaryName != "golangci-lint" ||
		cfg.Tools.GolangCILint.Command != "golangci-lint" {
		t.Errorf("expected golangci-lint binary & command match")
	}
	expectedPkg := "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
	if cfg.Tools.GolangCILint.Pkg != expectedPkg ||
		cfg.Tools.GolangCILint.Package != expectedPkg {
		t.Errorf("expected golangci-lint pkg & package match")
	}
	if cfg.Tools.Selene.Timeout != 45*time.Second {
		t.Errorf("expected selene timeout 45s, got %v", cfg.Tools.Selene.Timeout)
	}
}

func verifyLoadedBuildAndAutofix(t *testing.T, cfg *Config) {
	t.Helper()
	if cfg.Build.DefaultPackages != "./cmd/..." {
		t.Errorf("expected build packages './cmd/...', got %q", cfg.Build.DefaultPackages)
	}
	if !cfg.Build.Race || !cfg.Build.Trimpath {
		t.Errorf("expected race and trimpath to be true")
	}
	if !cfg.Autofix.DryRun || !cfg.Autofix.ModTidy || cfg.Autofix.Gofmt {
		t.Errorf("unexpected autofix settings: %+v", cfg.Autofix)
	}
}

func verifyLoadedEditAndServices(t *testing.T, cfg *Config) {
	t.Helper()
	if cfg.Edit.BackupStrategy != "git" ||
		cfg.Edit.DefaultThreshold != 0.90 ||
		cfg.Edit.FuzzyThreshold != 0.90 {
		t.Errorf("unexpected edit settings: %+v", cfg.Edit)
	}
	if cfg.Matching.FuzzyFallback ||
		cfg.Matching.SimilarityThreshold != 0.92 ||
		cfg.Matching.DefaultThreshold != 0.92 {
		t.Errorf("unexpected matching settings: %+v", cfg.Matching)
	}
	if cfg.Diagnostics.EnableSuggestions || cfg.Diagnostics.LevenshteinSuggestions {
		t.Errorf("expected enable suggestions false, got %v", cfg.Diagnostics.EnableSuggestions)
	}
	if cfg.Diagnostics.MaxLevenshteinDistance != 2 || cfg.Diagnostics.MaxDistance != 2 {
		t.Errorf("expected max distance 2, got %d", cfg.Diagnostics.MaxLevenshteinDistance)
	}
	if cfg.Test.DefaultLevel != "complete" {
		t.Errorf("expected test level 'complete', got %q", cfg.Test.DefaultLevel)
	}
	if !cfg.Test.RaceDetector || cfg.Test.CoverageThreshold != 90.0 {
		t.Errorf("unexpected test settings: %+v", cfg.Test)
	}
	if cfg.TestQuery.DatabasePath != "custom_tq.db" || cfg.TestQuery.WALMode {
		t.Errorf("unexpected testquery settings: %+v", cfg.TestQuery)
	}
	if cfg.Docs.CacheEnabled || cfg.Docs.DefaultFormat != "json" {
		t.Errorf("unexpected docs settings: %+v", cfg.Docs)
	}
	if cfg.Features.Autofix || !cfg.Features.CoverageGate || !cfg.Features.StrictMode {
		t.Errorf("unexpected features settings: %+v", cfg.Features)
	}
}

const fullRFC0001YAML = `
version: "1"

cli:
  timeout: "60s"
  output_format: "json"
  color: true
  log_level: "debug"

server:
  name: "custom-godoctor"
  transport: "http"
  http:
    listen: ":9090"
    read_timeout: "45s"
    write_timeout: "10m"
    idle_timeout: "180s"
    shutdown_timeout: "15s"
    allowed_origins:
      - "http://localhost:3000"
    allow_credentials: true
  logging:
    level: "debug"
    format: "json"
    trace_mcp_payloads: true

safeshell:
  mode: "strict"
  command_timeout: "90s"
  allowed_binaries:
    - "go"
    - "golangci-lint"
  max_output_bytes: 20971520

instructions:
  enabled: true
  compact: true
  dynamic_tools: false
  rules_file: ".godoctor/rules.md"
  custom_rules: "Custom prompt rules"

tools:
  golangci_lint:
    binary_name: "golangci-lint"
    recommended_version: "v2.15.0"
    min_version: "v1.60.0"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
    config: "custom.golangci.yml"
    timeout: "3m"
    args: ["run", "--fast"]
    disabled: false
  modernize:
    binary_name: "modernize"
    recommended_version: "v0.1.0"
    pkg: "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@v0.1.0"
    timeout: "1m"
    args: ["-fix"]
    disabled: false
  deadcode:
    binary_name: "deadcode"
    recommended_version: "latest"
    pkg: "golang.org/x/tools/cmd/deadcode@latest"
    timeout: "1m"
    disabled: false
  selene:
    binary_name: "selene"
    recommended_version: "v1.2.0"
    pkg: "github.com/danicat/selene/cmd/selene@v1.2.0"
    packages: "./pkg/..."
    timeout: "45s"
    disabled: false
  testquery:
    binary_name: "testquery"
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    db_path: "custom_tq.db"
    format: "json"
    timeout: "1m"
    disabled: false

build:
  default_packages: "./cmd/..."
  output: "bin/custom"
  tags: ["integration"]
  race: true
  trimpath: true
  timeout: "10m"
  flags: ["-v"]

autofix:
  enabled: true
  dry_run: true
  order:
    - tidy
    - modernize
  mod_tidy: true
  modernize: true
  gofmt: false
  deadcode: false

edit:
  backup_strategy: "git"
  atomic_write: false
  preserve_permissions: true
  default_threshold: 0.90
  format_on_save: "gofmt"
  exclude_paths:
    - ".git"
    - "build"
  max_levenshtein_distance: 2
  compiler_gate: "go build ./..."
  timeout: "45s"

matching:
  fuzzy_fallback: false
  similarity_threshold: 0.92
  normalize_unicode: false
  min_seed_length: 5
  window_expansion_delta: 2
  max_window_lines: 50

diagnostics:
  collect_on_edit: false
  verification_scope: "package"
  check_command: ["go", "test", "-c"]
  timeout: "20s"
  enable_suggestions: false
  max_levenshtein_distance: 2
  max_suggestions: 3
  snippet_context_lines: 3

test:
  default_level: "complete"
  default_packages: "./pkg/..."
  timeout: "120s"
  verbose: false
  race_detector: true
  coverage_threshold: 90.0
  coverage_profile: "custom_cov.out"
  coverage_output_dir: "/tmp/coverage"
  benchmark_pattern: "BenchmarkJSON"
  benchmark_flags: ["-benchmem"]
  benchmark_mem: false

testquery:
  db_path: "custom_tq.db"
  wal_mode: false
  busy_timeout: "10s"
  format: "json"

docs:
  cache_enabled: false
  cache_ttl: "30m"
  cache_max_entries: 1000
  external_fetch: false
  offline_mode: true
  pkg_go_dev_url: "https://custom.pkg.go.dev"
  max_symbols_rendered: 50
  fuzzy_suggestions: false
  max_fuzzy_suggestions: 3
  fuzzy_distance_threshold: 1
  default_format: "json"
  temp_dir: "/tmp/docs"

features:
  autofix: false
  mod_tidy: false
  modernize_check: false
  deadcode_check: true
  format_on_build: false
  format_on_edit: false
  vet_gate: false
  auto_rollback: false
  testquery_sync: false
  version_check_hints: false
  coverage_gate: true
  race_detector: true
  strict_mode: true
  remote_doc_fetch: false
  vanity_resolution: false
  docs_cache: false
`

func TestLoad_FullRFC0001YAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".godoctor.yaml")

	if err := os.WriteFile(configFile, []byte(fullRFC0001YAML), 0o600); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.LoadedFrom != configFile {
		t.Errorf("expected LoadedFrom to be %q, got %q", configFile, cfg.LoadedFrom)
	}

	verifyLoadedCLIAndServer(t, cfg)
	verifyLoadedSafeShellAndTools(t, cfg)
	verifyLoadedBuildAndAutofix(t, cfg)
	verifyLoadedEditAndServices(t, cfg)
}

func TestLoad_EmptyFileFallback(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected nil error on empty path, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Version != "1" {
		t.Errorf("expected default version 1, got %q", cfg.Version)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/path/.godoctor.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".godoctor.yaml")

	malformedContent := `
version: "1"
tools:
  golangci_lint: [unbalanced list
`
	if err := os.WriteFile(configFile, []byte(malformedContent), 0o600); err != nil {
		t.Fatalf("failed to write malformed config file: %v", err)
	}

	_, err := Load(configFile)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestFindConfigFile_UpwardTraversal(t *testing.T) {
	tmpRoot := t.TempDir()

	// Simulate git repo root
	gitDir := filepath.Join(tmpRoot, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatalf("failed to create fake .git dir: %v", err)
	}

	// Place .godoctor.yaml at project root
	rootConfigFile := filepath.Join(tmpRoot, ".godoctor.yaml")
	if err := os.WriteFile(rootConfigFile, []byte("version: \"1\"\n"), 0o600); err != nil {
		t.Fatalf("failed to write root config file: %v", err)
	}

	// Create deeply nested subdirectory
	deepDir := filepath.Join(tmpRoot, "internal", "subpkg", "nested")
	if err := os.MkdirAll(deepDir, 0o750); err != nil {
		t.Fatalf("failed to create deep subdirectories: %v", err)
	}

	// Should find config file from deep directory
	found := FindConfigFile(deepDir)
	if found != rootConfigFile {
		t.Errorf("expected to find %q from %q, got %q", rootConfigFile, deepDir, found)
	}
}

func TestFindConfigFile_GoModBoundary(t *testing.T) {
	tmpRoot := t.TempDir()

	// Place go.mod at root
	goModFile := filepath.Join(tmpRoot, "go.mod")
	if err := os.WriteFile(goModFile, []byte("module example.com/test\n"), 0o600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Place .godoctor.yml at root
	rootConfigFile := filepath.Join(tmpRoot, ".godoctor.yml")
	if err := os.WriteFile(rootConfigFile, []byte("version: \"1\"\n"), 0o600); err != nil {
		t.Fatalf("failed to write root config file: %v", err)
	}

	subDir := filepath.Join(tmpRoot, "pkg", "service")
	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatalf("failed to create subDir: %v", err)
	}

	found := FindConfigFile(subDir)
	if found != rootConfigFile {
		t.Errorf("expected to find %q, got %q", rootConfigFile, found)
	}
}

func TestLoadFromWorkspace(t *testing.T) {
	tmpRoot := t.TempDir()
	configFile := filepath.Join(tmpRoot, ".godoctor.yaml")
	if err := os.WriteFile(configFile, []byte("features:\n  autofix: false\n"), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromWorkspace(tmpRoot)
	if err != nil {
		t.Fatalf("expected successful load from workspace, got: %v", err)
	}
	if cfg.Features.Autofix {
		t.Error("expected autofix to be false")
	}

	// Workspace without config returns default config
	emptyWorkspace := t.TempDir()
	defCfg, err := LoadFromWorkspace(emptyWorkspace)
	if err != nil {
		t.Fatalf("expected nil error on empty workspace, got: %v", err)
	}
	if !defCfg.Features.Autofix {
		t.Error("expected autofix to be true in default config")
	}
}

type validationTestCase struct {
	name    string
	mutate  func(c *Config)
	wantErr bool
}

func validationToolCases() []validationTestCase {
	return []validationTestCase{
		{
			name: "Invalid schema version",
			mutate: func(c *Config) {
				c.Version = "2"
			},
			wantErr: true,
		},
		{
			name: "Invalid recommended_version format",
			mutate: func(c *Config) {
				c.Tools.GolangCILint.RecommendedVersion = "invalid-version-$$$"
			},
			wantErr: true,
		},
		{
			name: "Invalid min_version format",
			mutate: func(c *Config) {
				c.Tools.GolangCILint.MinVersion = "bad@version"
			},
			wantErr: true,
		},
		{
			name: "Negative tool timeout",
			mutate: func(c *Config) {
				c.Tools.Selene.Timeout = -1 * time.Second
			},
			wantErr: true,
		},
	}
}

func validationEditCases() []validationTestCase {
	return []validationTestCase{
		{
			name: "Default threshold greater than 1.0",
			mutate: func(c *Config) {
				c.Edit.DefaultThreshold = 1.5
			},
			wantErr: true,
		},
		{
			name: "Fuzzy threshold greater than 1.0",
			mutate: func(c *Config) {
				c.Edit.FuzzyThreshold = 1.5
			},
			wantErr: true,
		},
		{
			name: "Fuzzy threshold less than 0.0",
			mutate: func(c *Config) {
				c.Edit.FuzzyThreshold = -0.1
			},
			wantErr: true,
		},
		{
			name: "Negative max Levenshtein distance in Edit",
			mutate: func(c *Config) {
				c.Edit.MaxLevenshteinDistance = -1
			},
			wantErr: true,
		},
		{
			name: "Negative edit timeout",
			mutate: func(c *Config) {
				c.Edit.Timeout = -5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Matching similarity threshold out of range high",
			mutate: func(c *Config) {
				c.Matching.SimilarityThreshold = 1.2
			},
			wantErr: true,
		},
		{
			name: "Matching similarity threshold out of range low",
			mutate: func(c *Config) {
				c.Matching.SimilarityThreshold = -0.2
			},
			wantErr: true,
		},
		{
			name: "Matching default threshold out of range high",
			mutate: func(c *Config) {
				c.Matching.DefaultThreshold = 1.2
			},
			wantErr: true,
		},
		{
			name: "Matching default threshold out of range low",
			mutate: func(c *Config) {
				c.Matching.DefaultThreshold = -0.2
			},
			wantErr: true,
		},
		{
			name: "Matching negative min seed length",
			mutate: func(c *Config) {
				c.Matching.MinSeedLength = -1
			},
			wantErr: true,
		},
		{
			name: "Matching negative window expansion delta",
			mutate: func(c *Config) {
				c.Matching.WindowExpansionDelta = -1
			},
			wantErr: true,
		},
		{
			name: "Matching negative max window lines",
			mutate: func(c *Config) {
				c.Matching.MaxWindowLines = -1
			},
			wantErr: true,
		},
	}
}

func validationDiagnosticsCases() []validationTestCase {
	return []validationTestCase{
		{
			name: "Diagnostics negative max Levenshtein distance",
			mutate: func(c *Config) {
				c.Diagnostics.MaxLevenshteinDistance = -1
			},
			wantErr: true,
		},
		{
			name: "Diagnostics negative max distance",
			mutate: func(c *Config) {
				c.Diagnostics.MaxDistance = -1
			},
			wantErr: true,
		},
		{
			name: "Diagnostics negative max suggestions",
			mutate: func(c *Config) {
				c.Diagnostics.MaxSuggestions = -1
			},
			wantErr: true,
		},
		{
			name: "Diagnostics negative suggestions limit",
			mutate: func(c *Config) {
				c.Diagnostics.SuggestionsLimit = -1
			},
			wantErr: true,
		},
		{
			name: "Diagnostics negative snippet context lines",
			mutate: func(c *Config) {
				c.Diagnostics.SnippetContextLines = -1
			},
			wantErr: true,
		},
		{
			name: "Diagnostics negative timeout",
			mutate: func(c *Config) {
				c.Diagnostics.Timeout = -1 * time.Second
			},
			wantErr: true,
		},
	}
}

func validationTestDocsCases() []validationTestCase {
	return []validationTestCase{
		{
			name: "Invalid test level",
			mutate: func(c *Config) {
				c.Test.DefaultLevel = "ultra-fast"
			},
			wantErr: true,
		},
		{
			name: "Negative test timeout",
			mutate: func(c *Config) {
				c.Test.Timeout = -10 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Coverage threshold greater than 100",
			mutate: func(c *Config) {
				c.Test.CoverageThreshold = 150.0
			},
			wantErr: true,
		},
		{
			name: "Coverage threshold less than 0",
			mutate: func(c *Config) {
				c.Test.CoverageThreshold = -5.0
			},
			wantErr: true,
		},
		{
			name: "Invalid docs format",
			mutate: func(c *Config) {
				c.Docs.DefaultFormat = "pdf"
			},
			wantErr: true,
		},
		{
			name: "Negative docs cache TTL",
			mutate: func(c *Config) {
				c.Docs.CacheTTL = -1 * time.Minute
			},
			wantErr: true,
		},
		{
			name: "Negative docs cache max entries",
			mutate: func(c *Config) {
				c.Docs.CacheMaxEntries = -10
			},
			wantErr: true,
		},
		{
			name: "Negative docs max symbols rendered",
			mutate: func(c *Config) {
				c.Docs.MaxSymbolsRendered = -1
			},
			wantErr: true,
		},
		{
			name: "Negative docs max fuzzy suggestions",
			mutate: func(c *Config) {
				c.Docs.MaxFuzzySuggestions = -1
			},
			wantErr: true,
		},
		{
			name: "Negative docs fuzzy distance threshold",
			mutate: func(c *Config) {
				c.Docs.FuzzyDistanceThreshold = -1
			},
			wantErr: true,
		},
		{
			name: "Negative testquery busy timeout",
			mutate: func(c *Config) {
				c.TestQuery.BusyTimeout = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Invalid testquery format",
			mutate: func(c *Config) {
				c.TestQuery.Format = "xml"
			},
			wantErr: true,
		},
	}
}

func validationServerCases() []validationTestCase {
	return []validationTestCase{
		{
			name: "Negative server HTTP read timeout",
			mutate: func(c *Config) {
				c.Server.HTTP.ReadTimeout = -5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative server HTTP write timeout",
			mutate: func(c *Config) {
				c.Server.HTTP.WriteTimeout = -5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative server HTTP idle timeout",
			mutate: func(c *Config) {
				c.Server.HTTP.IdleTimeout = -5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative server HTTP shutdown timeout",
			mutate: func(c *Config) {
				c.Server.HTTP.ShutdownTimeout = -5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative SafeShell timeout",
			mutate: func(c *Config) {
				c.SafeShell.CommandTimeout = -10 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative SafeShell max output bytes",
			mutate: func(c *Config) {
				c.SafeShell.MaxOutputBytes = -1024
			},
			wantErr: true,
		},
		{
			name: "Negative CLI timeout",
			mutate: func(c *Config) {
				c.CLI.Timeout = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative Build timeout",
			mutate: func(c *Config) {
				c.Build.Timeout = -1 * time.Second
			},
			wantErr: true,
		},
	}
}

func TestValidate_Errors(t *testing.T) {
	c1, c2, c3 := validationToolCases(), validationEditCases(), validationDiagnosticsCases()
	c4, c5 := validationTestDocsCases(), validationServerCases()

	tests := make([]validationTestCase, 0, len(c1)+len(c2)+len(c3)+len(c4)+len(c5))
	tests = append(tests, c1...)
	tests = append(tests, c2...)
	tests = append(tests, c3...)
	tests = append(tests, c4...)
	tests = append(tests, c5...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefaultConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLookupTool(t *testing.T) {
	cfg := NewDefaultConfig()
	toolsToTest := []struct {
		query       string
		expectedCmd string
		found       bool
	}{
		{"golangci-lint", "golangci-lint", true},
		{"golangci_lint", "golangci-lint", true},
		{"linter", "golangci-lint", true},
		{"lint", "golangci-lint", true},
		{"modernize", "modernize", true},
		{"modernizer", "modernize", true},
		{"deadcode", "deadcode", true},
		{"selene", "selene", true},
		{"mutation", "selene", true},
		{"mutation_test", "selene", true},
		{"mutation-test", "selene", true},
		{"testquery", "testquery", true},
		{"tq", "testquery", true},
		{"test_query", "testquery", true},
		{"test-query", "testquery", true},
		{"nonexistent", "", false},
	}

	for _, tt := range toolsToTest {
		tool, ok := cfg.LookupTool(tt.query)
		if ok != tt.found {
			t.Errorf("LookupTool(%q) found = %v, expected %v", tt.query, ok, tt.found)
		}
		if tt.found && tool.Command != tt.expectedCmd {
			t.Errorf("LookupTool(%q) command = %q, expected %q", tt.query, tool.Command, tt.expectedCmd)
		}
		getTool := cfg.GetTool(tt.query)
		if tt.found && getTool.Command != tt.expectedCmd {
			t.Errorf("GetTool(%q) command = %q, expected %q", tt.query, getTool.Command, tt.expectedCmd)
		}
	}
}

func TestIsFeatureEnabled(t *testing.T) {
	cfg := NewDefaultConfig()
	featureTests := []struct {
		name     string
		expected bool
	}{
		{"autofix", true},
		{"mod_tidy", true},
		{"modtidy", true},
		{"tidy", true},
		{"modernize_check", true},
		{"modernize", true},
		{"deadcode_check", true},
		{"deadcode", true},
		{"format_on_build", true},
		{"formatonbuild", true},
		{"format_on_edit", true},
		{"formatonedit", true},
		{"vet_gate", true},
		{"vetgate", true},
		{"vet", true},
		{"auto_rollback", true},
		{"autorollback", true},
		{"rollback", true},
		{"testquery_sync", true},
		{"testquery", true},
		{"tq", true},
		{"test_query_sync", true},
		{"version_check_hints", true},
		{"versioncheck", true},
		{"version_hints", true},
		{"version_check", true},
		{"coverage_gate", false},
		{"coveragegate", false},
		{"coverage", false},
		{"race_detector", false},
		{"racedetector", false},
		{"race", false},
		{"strict_mode", false},
		{"strictmode", false},
		{"strict", false},
		{"remote_doc_fetch", true},
		{"remotedocfetch", true},
		{"vanity_resolution", true},
		{"vanityresolution", true},
		{"docs_cache", true},
		{"docscache", true},
		{"levenshtein_suggestions", true},
		{"suggestions", true},
		{"mutation_testing", true},
		{"selene", true},
		{"mutation", true},
		{"unknown_flag", false},
	}

	for _, ft := range featureTests {
		got := cfg.IsFeatureEnabled(ft.name)
		if got != ft.expected {
			t.Errorf("IsFeatureEnabled(%q) = %v, expected %v", ft.name, got, ft.expected)
		}
	}
}

func TestGetLinterConfigPath(t *testing.T) {
	cfg := NewDefaultConfig()
	linterPath := cfg.GetLinterConfigPath("/tmp/workspace")
	if linterPath != "/tmp/workspace/.golangci.yml" {
		t.Errorf("expected '/tmp/workspace/.golangci.yml', got %q", linterPath)
	}

	cfg.Tools.GolangCILint.Config = ""
	if defaultPath := cfg.GetLinterConfigPath("/tmp/workspace"); defaultPath != "/tmp/workspace/.golangci.yml" {
		t.Errorf("expected default .golangci.yml, got %q", defaultPath)
	}

	cfg.Tools.GolangCILint.Config = "/abs/path/.golangci.yml"
	if absLinter := cfg.GetLinterConfigPath("/tmp/workspace"); absLinter != "/abs/path/.golangci.yml" {
		t.Errorf("expected '/abs/path/.golangci.yml', got %q", absLinter)
	}
}

func TestGetTestQueryDBPath(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.TestQuery.DatabasePath = ".godoctor/testquery.db"
	dbPath := cfg.GetTestQueryDBPath("/tmp/workspace")
	if dbPath != "/tmp/workspace/.godoctor/testquery.db" {
		t.Errorf("expected '/tmp/workspace/.godoctor/testquery.db', got %q", dbPath)
	}

	cfg.TestQuery.DatabasePath = ""
	cfg.Tools.TestQuery.DbPath = "custom_tool.db"
	if toolDB := cfg.GetTestQueryDBPath("/tmp/workspace"); toolDB != "/tmp/workspace/custom_tool.db" {
		t.Errorf("expected '/tmp/workspace/custom_tool.db', got %q", toolDB)
	}

	cfg.TestQuery.DatabasePath = ""
	cfg.Tools.TestQuery.DbPath = ""
	if fallbackDB := cfg.GetTestQueryDBPath("/tmp/workspace"); fallbackDB != "/tmp/workspace/.godoctor/testquery.db" {
		t.Errorf("expected '/tmp/workspace/.godoctor/testquery.db', got %q", fallbackDB)
	}

	cfg.TestQuery.DatabasePath = "/var/data/custom.db"
	if absDB := cfg.GetTestQueryDBPath("/tmp/workspace"); absDB != "/var/data/custom.db" {
		t.Errorf("expected '/var/data/custom.db', got %q", absDB)
	}
}

func TestToolSpecNormalization(t *testing.T) {
	ts1 := ToolSpec{BinaryName: "custom-bin", Pkg: "example.com/tool@latest"}
	ts1.normalize()
	if ts1.Command != "custom-bin" || ts1.Package != "example.com/tool@latest" {
		t.Errorf("expected command & package to be populated: %+v", ts1)
	}

	ts2 := ToolSpec{Command: "custom-cmd", Package: "example.com/pkg@v1.0"}
	ts2.normalize()
	if ts2.BinaryName != "custom-cmd" || ts2.Pkg != "example.com/pkg@v1.0" {
		t.Errorf("expected binary_name & pkg to be populated: %+v", ts2)
	}
}

func TestSeleneConfigHelpers(t *testing.T) {
	cfg := NewDefaultConfig()

	// Default workers should equal runtime.GOMAXPROCS(0)
	if workers := cfg.GetSeleneWorkers(); workers != runtime.GOMAXPROCS(0) {
		t.Errorf("expected default workers %d, got %d", runtime.GOMAXPROCS(0), workers)
	}

	// Custom workers override
	cfg.Tools.Selene.Workers = 4
	if workers := cfg.GetSeleneWorkers(); workers != 4 {
		t.Errorf("expected workers 4, got %d", workers)
	}

	// TestQuery compat defaults
	if !cfg.IsSeleneTestQueryCompat() {
		t.Errorf("expected IsSeleneTestQueryCompat to default to true")
	}

	// DB path resolution
	dbPath := cfg.GetSeleneDBPath("/tmp/workspace")
	if dbPath != "/tmp/workspace/.godoctor/testquery.db" {
		t.Errorf("expected '/tmp/workspace/.godoctor/testquery.db', got %q", dbPath)
	}

	// Custom db_path in tools.selene
	cfg.Tools.Selene.DbPath = "custom_selene.db"
	if dbPath := cfg.GetSeleneDBPath("/tmp/workspace"); dbPath != "/tmp/workspace/custom_selene.db" {
		t.Errorf("expected '/tmp/workspace/custom_selene.db', got %q", dbPath)
	}

	// Disable compat
	cfg.Tools.Selene.TestQueryCompat = false
	cfg.Features.TestQueryCompat = false
	if cfg.IsSeleneTestQueryCompat() {
		t.Errorf("expected IsSeleneTestQueryCompat to be false when both are false")
	}
}
