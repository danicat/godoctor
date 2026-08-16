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

func TestRegistry(t *testing.T) {
	registry := DefaultRegistry()
	if len(registry) < 6 {
		t.Fatalf("DefaultRegistry() returned %d tools, expected at least 6", len(registry))
	}

	expectedIDs := []string{"go", "golangci_lint", "modernize", "deadcode", "selene", "testquery"}
	for _, id := range expectedIDs {
		spec, found := FindToolSpec(id)
		if !found {
			t.Errorf("FindToolSpec(%q) not found", id)
		}
		if spec.DisplayName == "" {
			t.Errorf("spec %q has empty DisplayName", id)
		}
	}

	// Test aliases
	if _, found := FindToolSpec("golangci-lint"); !found {
		t.Errorf("FindToolSpec('golangci-lint') hyphen alias should be found")
	}
	if _, found := FindToolSpec("tq"); !found {
		t.Errorf("FindToolSpec('tq') alias should be found")
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
	cache.stat = func(path string) (os.FileInfo, error) {
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
	cache.stat = func(path string) (os.FileInfo, error) {
		return mockFileInfo{modTime: fixedTime.Add(1 * time.Minute)}, nil
	}
	_, ok = cache.Get("golangci_lint", "/usr/local/bin/golangci-lint")
	if ok {
		t.Errorf("Cache.Get should invalidate when binary mtime changes on disk")
	}

	// 4. Invalidate and Clear
	cache.Clear()
	if len(cache.entries) != 0 {
		t.Errorf("Cache.Clear did not empty entries")
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

func TestChecker_CheckTool(t *testing.T) {
	ctx := context.Background()

	t.Run("Missing binary", func(t *testing.T) {
		runner := &mockRunner{
			lookPathFunc: func(file string) (string, error) {
				return "", errors.New("not found")
			},
		}
		checker := NewChecker(WithRunner(runner), WithNoCache(true))
		spec, _ := FindToolSpec("golangci_lint")

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
	})

	t.Run("Valid CLI version - OK", func(t *testing.T) {
		runner := &mockRunner{
			lookPathFunc: func(file string) (string, error) {
				return "/usr/bin/golangci-lint", nil
			},
			runCommandFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("golangci-lint has version 2.12.2 built with go1.24.0"), nil
			},
		}
		checker := NewChecker(WithRunner(runner), WithNoCache(true))
		spec, _ := FindToolSpec("golangci_lint")

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
	})

	t.Run("Outdated CLI version", func(t *testing.T) {
		runner := &mockRunner{
			lookPathFunc: func(file string) (string, error) {
				return "/usr/bin/golangci-lint", nil
			},
			runCommandFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("golangci-lint version 1.60.0 built from (devel)"), nil
			},
		}
		checker := NewChecker(WithRunner(runner), WithNoCache(true))
		spec, _ := FindToolSpec("golangci_lint")

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
	})

	t.Run("Fallback to ReadBuildInfo when CLI fails", func(t *testing.T) {
		runner := &mockRunner{
			lookPathFunc: func(file string) (string, error) {
				return "/usr/bin/modernize", nil
			},
			runCommandFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return nil, errors.New("unrecognized flag")
			},
			readBuildInfoFunc: func(path string) (*buildinfo.BuildInfo, error) {
				return &buildinfo.BuildInfo{
					Main: debug.Module{
						Path:    "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize",
						Version: "v0.25.0",
					},
				}, nil
			},
		}
		checker := NewChecker(WithRunner(runner), WithNoCache(true))
		spec, _ := FindToolSpec("modernize")

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
	})

	t.Run("Fallback to VCS revision for devel build", func(t *testing.T) {
		runner := &mockRunner{
			lookPathFunc: func(file string) (string, error) {
				return "/usr/bin/deadcode", nil
			},
			runCommandFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("deadcode: unknown flag"), errors.New("exit status 1")
			},
			readBuildInfoFunc: func(path string) (*buildinfo.BuildInfo, error) {
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
		spec, _ := FindToolSpec("deadcode")

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
	})
}

func TestChecker_CheckAll(t *testing.T) {
	runner := &mockRunner{
		lookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		runCommandFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
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

	// Package-level CheckAll with custom config
	statuses, err := CheckAll(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CheckAll with config failed: %v", err)
	}
	if len(statuses) < 6 {
		t.Errorf("expected at least 6 statuses, got %d", len(statuses))
	}

	for _, s := range statuses {
		if s.ID == "golangci_lint" {
			if s.RecommendedVersion != "v2.15.0" {
				t.Errorf("expected recommended version v2.15.0 from config override, got %s", s.RecommendedVersion)
			}
		}
	}
}

func TestCheckTool_WithConfig(t *testing.T) {
	toolSpec := config.ToolSpec{
		BinaryName:         "custom-tool",
		RecommendedVersion: "v1.5.0",
		Package:            "example.com/custom/tool@v1.5.0",
	}

	st, err := CheckTool(context.Background(), "custom-tool", toolSpec)
	if err != nil {
		t.Fatalf("CheckTool failed: %v", err)
	}
	if st.RecommendedVersion != "v1.5.0" {
		t.Errorf("expected recommended version v1.5.0, got %s", st.RecommendedVersion)
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

func TestFormatRecommendationsMarkdown(t *testing.T) {
	t.Run("Has recommendations", func(t *testing.T) {
		statuses := []ToolStatus{
			{DisplayName: "golangci-lint", Status: StatusOutdated, InstalledVersion: "v1.64.0", RecommendedVersion: "v2.12.2", UpgradeCommand: "go install pkg@v2.12.2"},
			{DisplayName: "selene", Status: StatusMissing, Required: false, UpgradeCommand: "go install selene@latest"},
		}

		md := FormatRecommendationsMarkdown(statuses)
		if !strings.Contains(md, "> [!TIP]") {
			t.Errorf("markdown missing TIP block: %s", md)
		}
		if !strings.Contains(md, "golangci-lint") || !strings.Contains(md, "selene") {
			t.Errorf("markdown missing tool names: %s", md)
		}
	})

	t.Run("All tools healthy - returns empty", func(t *testing.T) {
		statuses := []ToolStatus{
			{DisplayName: "Go Toolchain", Status: StatusOk, InstalledVersion: "1.26.3", RecommendedVersion: ">=1.24.0"},
			{DisplayName: "golangci-lint", Status: StatusOk, InstalledVersion: "v2.12.2", RecommendedVersion: "v2.12.2"},
		}

		md := FormatRecommendationsMarkdown(statuses)
		if md != "" {
			t.Errorf("expected empty string when all tools are healthy, got: %q", md)
		}
	})
}

func TestVersion_String(t *testing.T) {
	v1 := Version{Major: 1, Minor: 2, Patch: 3}
	if v1.String() != "1.2.3" {
		t.Errorf("v1.String() = %q, want '1.2.3'", v1.String())
	}

	v2 := Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc1"}
	if v2.String() != "1.2.3-rc1" {
		t.Errorf("v2.String() = %q, want '1.2.3-rc1'", v2.String())
	}

	v3 := Version{IsDevel: true, CommitHash: "abcdef1"}
	if v3.String() != "devel (abcdef1)" {
		t.Errorf("v3.String() = %q, want 'devel (abcdef1)'", v3.String())
	}
}
