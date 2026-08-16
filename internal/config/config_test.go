package config

import (
	"os"
	"path/filepath"
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

	if cfg.Build.DefaultPackages != "./..." {
		t.Errorf("expected build default_packages to be './...', got %q", cfg.Build.DefaultPackages)
	}

	if cfg.Test.DefaultLevel != "basic" {
		t.Errorf("expected test default_level to be 'basic', got %q", cfg.Test.DefaultLevel)
	}

	if cfg.Edit.FuzzyThreshold != 0.95 {
		t.Errorf("expected edit fuzzy threshold to be 0.95, got %f", cfg.Edit.FuzzyThreshold)
	}

	if !cfg.Features.Autofix {
		t.Error("expected autofix feature to be enabled by default")
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to pass validation, got: %v", err)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".godoctor.yaml")

	yamlContent := `
version: "1"

tools:
  golangci_lint:
    recommended_version: "v2.15.0"
    pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
    config: "custom.golangci.yml"
    timeout: 3m
  selene:
    recommended_version: "v1.2.0"
    timeout: 45s

features:
  autofix: false
  deadcode_check: true

build:
  default_packages: "./cmd/..."
  timeout: 10m

test:
  default_level: "complete"
  benchmark_pattern: "BenchmarkJSON"

edit:
  fuzzy_threshold: 0.90
  max_levenshtein_distance: 3

testquery:
  db_path: "custom_tq.db"
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.LoadedFrom != configFile {
		t.Errorf("expected LoadedFrom to be %q, got %q", configFile, cfg.LoadedFrom)
	}

	if cfg.Tools.GolangCILint.RecommendedVersion != "v2.15.0" {
		t.Errorf("expected golangci-lint recommended_version v2.15.0, got %q", cfg.Tools.GolangCILint.RecommendedVersion)
	}

	if cfg.Tools.GolangCILint.Timeout != 3*time.Minute {
		t.Errorf("expected golangci-lint timeout 3m, got %v", cfg.Tools.GolangCILint.Timeout)
	}

	if cfg.Tools.Selene.Timeout != 45*time.Second {
		t.Errorf("expected selene timeout 45s, got %v", cfg.Tools.Selene.Timeout)
	}

	if cfg.Features.Autofix {
		t.Error("expected autofix to be false")
	}

	if cfg.Build.DefaultPackages != "./cmd/..." {
		t.Errorf("expected build packages './cmd/...', got %q", cfg.Build.DefaultPackages)
	}

	if cfg.Test.DefaultLevel != "complete" {
		t.Errorf("expected test level 'complete', got %q", cfg.Test.DefaultLevel)
	}

	if cfg.Edit.FuzzyThreshold != 0.90 {
		t.Errorf("expected edit fuzzy threshold 0.90, got %f", cfg.Edit.FuzzyThreshold)
	}

	if cfg.TestQuery.DatabasePath != "custom_tq.db" {
		t.Errorf("expected db_path 'custom_tq.db', got %q", cfg.TestQuery.DatabasePath)
	}
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
	if err := os.WriteFile(configFile, []byte(malformedContent), 0644); err != nil {
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
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create fake .git dir: %v", err)
	}

	// Place .godoctor.yaml at project root
	rootConfigFile := filepath.Join(tmpRoot, ".godoctor.yaml")
	if err := os.WriteFile(rootConfigFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatalf("failed to write root config file: %v", err)
	}

	// Create deeply nested subdirectory
	deepDir := filepath.Join(tmpRoot, "internal", "subpkg", "nested")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
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
	if err := os.WriteFile(goModFile, []byte("module example.com/test\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Place .godoctor.yml at root
	rootConfigFile := filepath.Join(tmpRoot, ".godoctor.yml")
	if err := os.WriteFile(rootConfigFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatalf("failed to write root config file: %v", err)
	}

	subDir := filepath.Join(tmpRoot, "pkg", "service")
	if err := os.MkdirAll(subDir, 0755); err != nil {
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
	if err := os.WriteFile(configFile, []byte("features:\n  autofix: false\n"), 0644); err != nil {
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

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
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
			name: "Negative tool timeout",
			mutate: func(c *Config) {
				c.Tools.Selene.Timeout = -1 * time.Second
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
			name: "Negative max Levenshtein distance",
			mutate: func(c *Config) {
				c.Edit.MaxLevenshteinDistance = -1
			},
			wantErr: true,
		},
		{
			name: "Invalid test level",
			mutate: func(c *Config) {
				c.Test.DefaultLevel = "ultra-fast"
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
			name: "Negative server timeout",
			mutate: func(c *Config) {
				c.Server.ReadTimeout = -5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "Negative SafeShell timeout",
			mutate: func(c *Config) {
				c.SafeShell.DefaultTimeout = -10 * time.Second
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
	}

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

func TestAccessorsAndHelpers(t *testing.T) {
	cfg := NewDefaultConfig()

	// 1. GetTool / LookupTool
	tool := cfg.GetTool("golangci-lint")
	if tool.BinaryName != "golangci-lint" {
		t.Errorf("expected golangci-lint binary, got %q", tool.BinaryName)
	}

	toolTQ := cfg.GetTool("tq")
	if toolTQ.BinaryName != "testquery" {
		t.Errorf("expected testquery binary for 'tq' alias, got %q", toolTQ.BinaryName)
	}

	toolSelene := cfg.GetTool("mutation_test")
	if toolSelene.BinaryName != "selene" {
		t.Errorf("expected selene binary for 'mutation_test' alias, got %q", toolSelene.BinaryName)
	}

	_, found := cfg.LookupTool("nonexistent_tool")
	if found {
		t.Error("expected nonexistent tool to not be found")
	}

	// 2. IsFeatureEnabled
	if !cfg.IsFeatureEnabled("autofix") {
		t.Error("expected autofix to be enabled")
	}
	if !cfg.IsFeatureEnabled("deadcode") {
		t.Error("expected deadcode to be enabled")
	}
	if !cfg.IsFeatureEnabled("version_hints") {
		t.Error("expected version_hints alias to be enabled")
	}
	if cfg.IsFeatureEnabled("unknown_flag") {
		t.Error("expected unknown flag to be false")
	}

	// 3. GetLinterConfigPath
	linterPath := cfg.GetLinterConfigPath("/tmp/workspace")
	if linterPath != "/tmp/workspace/.golangci.yml" {
		t.Errorf("expected '/tmp/workspace/.golangci.yml', got %q", linterPath)
	}

	// 4. GetTestQueryDBPath
	dbPath := cfg.GetTestQueryDBPath("/tmp/workspace")
	if dbPath != "/tmp/workspace/testquery.db" {
		t.Errorf("expected '/tmp/workspace/testquery.db', got %q", dbPath)
	}
}
