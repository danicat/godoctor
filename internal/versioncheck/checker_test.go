package versioncheck

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/danicat/godoctor/internal/config"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input      string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantDevel  bool
		wantCommit string
	}{
		{input: "v1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "2.12.2", wantMajor: 2, wantMinor: 12, wantPatch: 2},
		{input: "go1.26.0", wantMajor: 1, wantMinor: 26, wantPatch: 0},
		{input: "1.24", wantMajor: 1, wantMinor: 24, wantPatch: 0},
		{input: "v2.0.0-rc1", wantMajor: 2, wantMinor: 0, wantPatch: 0, wantPre: "rc1"},
		{input: "1.25.0-alpha.2+build123", wantMajor: 1, wantMinor: 25, wantPatch: 0, wantPre: "alpha.2+build123"},
		{input: "devel", wantDevel: true},
		{input: "(devel)", wantDevel: true},
		{input: "devel (abcdef1)", wantDevel: true, wantCommit: "abcdef1"},
		{input: "", wantMajor: 0, wantMinor: 0, wantPatch: 0},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			v := ParseVersion(tc.input)
			if v.IsDevel != tc.wantDevel {
				t.Errorf("ParseVersion(%q).IsDevel = %v, want %v", tc.input, v.IsDevel, tc.wantDevel)
			}
			if tc.wantCommit != "" && v.CommitHash != tc.wantCommit {
				t.Errorf("ParseVersion(%q).CommitHash = %q, want %q", tc.input, v.CommitHash, tc.wantCommit)
			}
			if !tc.wantDevel {
				if v.Major != tc.wantMajor || v.Minor != tc.wantMinor || v.Patch != tc.wantPatch {
					t.Errorf("ParseVersion(%q) = (%d.%d.%d), want (%d.%d.%d)",
						tc.input, v.Major, v.Minor, v.Patch, tc.wantMajor, tc.wantMinor, tc.wantPatch)
				}
				if v.Prerelease != tc.wantPre {
					t.Errorf("ParseVersion(%q).Prerelease = %q, want %q", tc.input, v.Prerelease, tc.wantPre)
				}
			}
		})
	}
}

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.2.0", "1.1.9", 1},
		{"1.1.9", "1.2.0", -1},
		{"2.0.0", "1.99.99", 1},
		{"1.24.0-rc1", "1.24.0", -1},
		{"1.24.0", "1.24.0-rc1", 1},
		{"1.24.0-rc1", "1.24.0-rc2", -1},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tc.v1, tc.v2), func(t *testing.T) {
			parsed1 := ParseVersion(tc.v1)
			parsed2 := ParseVersion(tc.v2)
			got := parsed1.Compare(parsed2)
			if got != tc.want {
				t.Errorf("Compare(%s, %s) = %d, want %d", tc.v1, tc.v2, got, tc.want)
			}
		})
	}
}

func TestSatisfies(t *testing.T) {
	tests := []struct {
		installed  string
		constraint string
		want       bool
	}{
		{installed: "1.26.0", constraint: "latest", want: true},
		{installed: "1.26.0", constraint: "", want: true},
		{installed: "devel", constraint: ">=1.24.0", want: true},
		{installed: "1.26.0", constraint: ">=1.24.0", want: true},
		{installed: "1.23.4", constraint: ">=1.24.0", want: false},
		{installed: "v2.12.2", constraint: "v2.12.2", want: true},
		{installed: "v2.13.0", constraint: "v2.12.2", want: true},
		{installed: "v1.64.0", constraint: "v2.12.2", want: false},
		{installed: "1.25.0", constraint: ">1.24", want: true},
		{installed: "1.24.0", constraint: ">1.24", want: false},
		{installed: "1.24.0", constraint: "<=1.24.0", want: true},
		{installed: "1.25.0", constraint: "<=1.24.0", want: false},
		{installed: "1.24.0", constraint: "=1.24.0", want: true},
		{installed: "1.24.1", constraint: "=1.24.0", want: false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s_sat_%s", tc.installed, tc.constraint), func(t *testing.T) {
			parsed := ParseVersion(tc.installed)
			got := Satisfies(parsed, tc.constraint)
			if got != tc.want {
				t.Errorf("Satisfies(%s, %s) = %v, want %v", tc.installed, tc.constraint, got, tc.want)
			}
		})
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "golangci-lint new format",
			text: "golangci-lint has version 2.12.2 built with go1.24.0 from 0a1b2c3 on 2025-01-15T12:00:00Z",
			want: "2.12.2",
		},
		{
			name: "golangci-lint old format",
			text: "golangci-lint version 1.64.0 built from (devel) on (unknown)",
			want: "1.64.0",
		},
		{
			name: "go version darwin",
			text: "go version go1.26.3 darwin/arm64",
			want: "1.26.3",
		},
		{
			name: "selene output",
			text: "selene version 0.3.1\nAST mutation engine",
			want: "0.3.1",
		},
		{
			name: "testquery output",
			text: "testquery version 0.4.0\nSQL test analytics",
			want: "0.4.0",
		},
		{
			name: "tq alias output",
			text: "tq version 0.4.0",
			want: "0.4.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var extracted string
			switch {
			case strings.Contains(tc.text, "golangci-lint"):
				extracted = ExtractVersion(tc.text, golangciLintRe)
			case strings.Contains(tc.text, "go version"):
				extracted = ExtractVersion(tc.text, goVersionRe)
			case strings.Contains(tc.text, "selene"):
				extracted = ExtractVersion(tc.text, seleneRe)
			case strings.Contains(tc.text, "testquery") || strings.Contains(tc.text, "tq"):
				extracted = ExtractVersion(tc.text, testqueryRe)
			}

			if extracted != tc.want {
				t.Errorf("ExtractVersion(%q) = %q, want %q", tc.text, extracted, tc.want)
			}
		})
	}
}

func findToolSpecForTest(id string) ToolSpec {
	for _, s := range DefaultRegistry() {
		if s.ID == id {
			return s
		}
	}
	return ToolSpec{}
}

func TestRegistry(t *testing.T) {
	registry := DefaultRegistry()
	if len(registry) < 6 {
		t.Fatalf("DefaultRegistry() returned %d tools, expected at least 6", len(registry))
	}

	expectedIDs := map[string]bool{
		"go":            false,
		"golangci_lint": false,
		"modernize":     false,
		"deadcode":      false,
		"selene":        false,
		"testquery":     false,
	}
	for _, spec := range registry {
		if _, ok := expectedIDs[spec.ID]; ok {
			expectedIDs[spec.ID] = true
		}
		if spec.DisplayName == "" {
			t.Errorf("spec %q has empty DisplayName", spec.ID)
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected tool ID %q not found in DefaultRegistry()", id)
		}
	}
}

// Mock file info for cache tests
type mockFileInfo struct {
	modTime time.Time
}

func (m mockFileInfo) Name() string       { return "mock" }
func (m mockFileInfo) Size() int64        { return 100 }
func (m mockFileInfo) Mode() os.FileMode  { return 0755 }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() any           { return nil }

func TestVersionCache(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC)
	currentTime := fixedTime
	mtime := fixedTime.Add(-1 * time.Hour)

	cache := NewVersionCache(5 * time.Minute)
	cache.nowFunc = func() time.Time { return currentTime }
	cache.stat = func(_ string) (os.FileInfo, error) {
		return mockFileInfo{modTime: mtime}, nil
	}

	status := ToolStatus{
		ID:                 "golangci_lint",
		DisplayName:        "golangci-lint",
		Status:             StatusOk,
		BinaryPath:         "/usr/local/bin/golangci-lint",
		InstalledVersion:   "v2.12.2",
		RecommendedVersion: "v2.12.2",
		Satisfies:          true,
	}

	// 1. Initial Set and Get
	cache.Set("golangci_lint", "/usr/local/bin/golangci-lint", status)
	got, ok := cache.Get("golangci_lint", "/usr/local/bin/golangci-lint")
	if !ok || got.InstalledVersion != "v2.12.2" {
		t.Fatalf("Cache.Get failed on fresh entry: got %+v, ok=%v", got, ok)
	}

	// 2. TTL Expiration
	currentTime = fixedTime.Add(6 * time.Minute)
	_, ok = cache.Get("golangci_lint", "/usr/local/bin/golangci-lint")
	if ok {
		t.Errorf("Cache.Get should have expired after TTL")
	}

	// 3. Reset and test MTime invalidation
	currentTime = fixedTime
	cache.Set("golangci_lint", "/usr/local/bin/golangci-lint", status)
	// Change mtime to simulate binary upgrade on disk
	cache.stat = func(_ string) (os.FileInfo, error) {
		return mockFileInfo{modTime: fixedTime.Add(1 * time.Minute)}, nil
	}
	_, ok = cache.Get("golangci_lint", "/usr/local/bin/golangci-lint")
	if ok {
		t.Errorf("Cache.Get should invalidate when binary mtime changes on disk")
	}
}

// MockRunner for Checker unit tests
type mockRunner struct {
	lookPathFunc      func(file string) (string, error)
	runCommandFunc    func(ctx context.Context, name string, args ...string) ([]byte, error)
	readBuildInfoFunc func(path string) (*buildinfo.BuildInfo, error)
	statFunc          func(path string) (os.FileInfo, error)
}

func (m *mockRunner) LookPath(file string) (string, error) {
	if m.lookPathFunc != nil {
		return m.lookPathFunc(file)
	}
	return "/bin/" + file, nil
}

func (m *mockRunner) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	if m.runCommandFunc != nil {
		return m.runCommandFunc(ctx, name, args...)
	}
	return []byte("version 1.0.0"), nil
}

func (m *mockRunner) ReadBuildInfo(path string) (*buildinfo.BuildInfo, error) {
	if m.readBuildInfoFunc != nil {
		return m.readBuildInfoFunc(path)
	}
	return nil, errors.New("no buildinfo")
}

func (m *mockRunner) Stat(path string) (os.FileInfo, error) {
	if m.statFunc != nil {
		return m.statFunc(path)
	}
	return mockFileInfo{modTime: time.Now()}, nil
}

func testCheckerMissing(ctx context.Context, t *testing.T) {
	t.Helper()
	runner := &mockRunner{
		lookPathFunc: func(_ string) (string, error) {
			return "", errors.New("not found")
		},
	}
	checker := NewChecker(WithRunner(runner), WithNoCache(true))
	spec := findToolSpecForTest("golangci_lint")

	st, err := checker.CheckTool(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StatusMissing {
		t.Errorf("expected StatusMissing, got %s", st.Status)
	}
	if st.Satisfies {
		t.Errorf("missing tool should not satisfy")
	}
	if st.UpgradeCommand == "" {
		t.Errorf("missing tool should provide upgrade command")
	}
}

func testCheckerValidCLI(ctx context.Context, t *testing.T) {
	t.Helper()
	runner := &mockRunner{
		lookPathFunc: func(_ string) (string, error) {
			return "/usr/bin/golangci-lint", nil
		},
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("golangci-lint has version 2.12.2 built with go1.24.0"), nil
		},
	}
	checker := NewChecker(WithRunner(runner), WithNoCache(true))
	spec := findToolSpecForTest("golangci_lint")

	st, err := checker.CheckTool(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StatusOk {
		t.Errorf("expected StatusOk, got %s", st.Status)
	}
	if st.InstalledVersion != "2.12.2" {
		t.Errorf("expected 2.12.2, got %s", st.InstalledVersion)
	}
}

func testCheckerOutdated(ctx context.Context, t *testing.T) {
	t.Helper()
	runner := &mockRunner{
		lookPathFunc: func(_ string) (string, error) {
			return "/usr/bin/golangci-lint", nil
		},
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("golangci-lint version 1.60.0 built from (devel)"), nil
		},
	}
	checker := NewChecker(WithRunner(runner), WithNoCache(true))
	spec := findToolSpecForTest("golangci_lint")

	st, err := checker.CheckTool(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StatusOutdated {
		t.Errorf("expected StatusOutdated, got %s", st.Status)
	}
	if st.InstalledVersion != "1.60.0" {
		t.Errorf("expected 1.60.0, got %s", st.InstalledVersion)
	}
	if !strings.Contains(st.UpgradeCommand, "golangci-lint@v2.12.2") {
		t.Errorf("expected upgrade command to pin v2.12.2, got %q", st.UpgradeCommand)
	}
}

func testCheckerBuildInfoFallback(ctx context.Context, t *testing.T) {
	t.Helper()
	runner := &mockRunner{
		lookPathFunc: func(_ string) (string, error) {
			return "/usr/bin/modernize", nil
		},
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("unrecognized flag")
		},
		readBuildInfoFunc: func(_ string) (*buildinfo.BuildInfo, error) {
			return &buildinfo.BuildInfo{
				Main: debug.Module{
					Path:    "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize",
					Version: "v0.25.0",
				},
			}, nil
		},
	}
	checker := NewChecker(WithRunner(runner), WithNoCache(true))
	spec := findToolSpecForTest("modernize")

	st, err := checker.CheckTool(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StatusOk {
		t.Errorf("expected StatusOk via buildinfo fallback, got %s", st.Status)
	}
	if st.InstalledVersion != "v0.25.0" {
		t.Errorf("expected v0.25.0, got %s", st.InstalledVersion)
	}
}

func testCheckerVCSDevelFallback(ctx context.Context, t *testing.T) {
	t.Helper()
	runner := &mockRunner{
		lookPathFunc: func(_ string) (string, error) {
			return "/usr/bin/deadcode", nil
		},
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("deadcode: unknown flag"), errors.New("exit status 1")
		},
		readBuildInfoFunc: func(_ string) (*buildinfo.BuildInfo, error) {
			return &buildinfo.BuildInfo{
				Main: debug.Module{
					Version: "(devel)",
				},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "1a2b3c4d5e6f7a8b9c"},
				},
			}, nil
		},
	}
	checker := NewChecker(WithRunner(runner), WithNoCache(true))
	spec := findToolSpecForTest("deadcode")

	st, err := checker.CheckTool(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StatusOk {
		t.Errorf("expected StatusOk for devel build with 'latest' rec, got %s", st.Status)
	}
	if st.InstalledVersion != "devel (1a2b3c4)" {
		t.Errorf("expected 'devel (1a2b3c4)', got %q", st.InstalledVersion)
	}
}

func TestChecker_CheckTool(t *testing.T) {
	ctx := context.Background()

	t.Run("Missing binary", func(t *testing.T) {
		testCheckerMissing(ctx, t)
	})

	t.Run("Valid CLI version - OK", func(t *testing.T) {
		testCheckerValidCLI(ctx, t)
	})

	t.Run("Outdated CLI version", func(t *testing.T) {
		testCheckerOutdated(ctx, t)
	})

	t.Run("Fallback to ReadBuildInfo when CLI fails", func(t *testing.T) {
		testCheckerBuildInfoFallback(ctx, t)
	})

	t.Run("Fallback to VCS revision for devel build", func(t *testing.T) {
		testCheckerVCSDevelFallback(ctx, t)
	})

	t.Run("Disabled tool returns OK status", func(t *testing.T) {
		checker := NewChecker(WithNoCache(true))
		spec := ToolSpec{
			ID:          "custom-disabled",
			DisplayName: "custom-disabled",
			Disabled:    true,
		}
		st, err := checker.CheckTool(ctx, spec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.Status != StatusOk || !st.Satisfies {
			t.Errorf("expected StatusOk and satisfies=true for disabled tool, got status=%s, satisfies=%v", st.Status, st.Satisfies)
		}
	})

	t.Run("Tool with custom timeout", func(t *testing.T) {
		timeoutObserved := false
		runner := &mockRunner{
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/bin/tool", nil
			},
			runCommandFunc: func(cmdCtx context.Context, _ string, _ ...string) ([]byte, error) {
				deadline, ok := cmdCtx.Deadline()
				if ok && time.Until(deadline) <= 500*time.Millisecond {
					timeoutObserved = true
				}
				return []byte("tool version 1.0.0"), nil
			},
		}
		checker := NewChecker(WithRunner(runner), WithNoCache(true), WithTimeout(5*time.Second))
		spec := ToolSpec{
			ID:          "custom-timeout",
			DisplayName: "custom-timeout",
			Binaries:    []string{"tool"},
			VersionArgs: [][]string{{"version"}},
			Timeout:     100 * time.Millisecond,
		}
		st, err := checker.CheckTool(ctx, spec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !timeoutObserved {
			t.Errorf("expected custom spec timeout to be applied to context deadline")
		}
		if st.InstalledVersion != "1.0.0" {
			t.Errorf("expected installed version 1.0.0, got %s", st.InstalledVersion)
		}
	})
}

func TestChecker_CheckAll(t *testing.T) {
	runner := &mockRunner{
		lookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		runCommandFunc: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if strings.Contains(name, "go") {
				return []byte("go version go1.26.0 darwin/arm64"), nil
			}
			return []byte("version 1.0.0"), nil
		},
	}
	checker := NewChecker(WithRunner(runner), WithNoCache(true))
	statuses, err := checker.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("CheckAll failed: %v", err)
	}
	if len(statuses) != 6 {
		t.Errorf("CheckAll returned %d statuses, want 6", len(statuses))
	}
}

func TestCheckAll_WithConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Tools.GolangCILint.RecommendedVersion = "v2.15.0"
	cfg.Tools.Deadcode.Disabled = true

	// Package-level CheckAll with custom config
	statuses, err := CheckAll(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CheckAll with config failed: %v", err)
	}
	// Total 6 tools in registry, 1 disabled -> 5 statuses
	if len(statuses) != 5 {
		t.Errorf("expected 5 statuses (deadcode disabled), got %d", len(statuses))
	}

	for _, s := range statuses {
		if s.ID == "deadcode" {
			t.Errorf("disabled tool 'deadcode' should not be present in CheckAll results")
		}
		if s.ID == "golangci_lint" {
			if s.RecommendedVersion != "v2.15.0" {
				t.Errorf("expected recommended version v2.15.0 from config override, got %s", s.RecommendedVersion)
			}
		}
	}
}

func TestWorkspaceConfig_CheckAll(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 1. Without config file, loads default
	cfg, err := config.LoadFromWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("config.LoadFromWorkspace failed on empty dir: %v", err)
	}
	statuses, err := CheckAll(ctx, cfg)
	if err != nil {
		t.Fatalf("CheckAll failed on default workspace config: %v", err)
	}
	if len(statuses) < 6 {
		t.Errorf("expected 6 statuses for default workspace, got %d", len(statuses))
	}

	// 2. With .godoctor.yaml in workspace
	configYaml := `
version: "1"
features:
  version_check_hints: false
tools:
  deadcode:
    disabled: true
  golangci_lint:
    recommended_version: "v2.20.0"
`
	if err := os.WriteFile(tmpDir+"/.godoctor.yaml", []byte(configYaml), 0600); err != nil {
		t.Fatalf("failed to write test .godoctor.yaml: %v", err)
	}

	cfgWithYaml, err := config.LoadFromWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("config.LoadFromWorkspace failed with yaml: %v", err)
	}
	statusesWithCfg, err := CheckAll(ctx, cfgWithYaml)
	if err != nil {
		t.Fatalf("CheckAll with yaml failed: %v", err)
	}
	if len(statusesWithCfg) != 5 {
		t.Errorf("expected 5 statuses (deadcode disabled in yaml), got %d", len(statusesWithCfg))
	}
}

func TestFormatStatusTable(t *testing.T) {
	statuses := []ToolStatus{
		{DisplayName: "Go Toolchain", Status: StatusOk, InstalledVersion: "1.26.3", RecommendedVersion: ">=1.24.0"},
		{DisplayName: "golangci-lint", Status: StatusOutdated, InstalledVersion: "v1.64.0", RecommendedVersion: "v2.12.2", UpgradeCommand: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"},
		{DisplayName: "selene", Status: StatusMissing, RecommendedVersion: "latest", UpgradeCommand: "go install github.com/danicat/selene/cmd/selene@latest"},
	}

	table := FormatStatusTable(statuses)
	if !strings.Contains(table, "Go Toolchain") {
		t.Errorf("table missing Go Toolchain")
	}
	if !strings.Contains(table, "⚠️ OUTDATED") {
		t.Errorf("table missing OUTDATED badge")
	}
	if !strings.Contains(table, "✗ MISSING") {
		t.Errorf("table missing MISSING badge")
	}
	if !strings.Contains(table, "Summary: 1/3 tools healthy, 1 outdated, 1 missing.") {
		t.Errorf("table missing or malformed summary line: %s", table)
	}
}
