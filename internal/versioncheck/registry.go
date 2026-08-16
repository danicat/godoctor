package versioncheck

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// golangciLintRe matches "golangci-lint has version 2.12.2" or "golangci-lint version 1.64.0"
	golangciLintRe = regexp.MustCompile(`(?i)golangci-lint(?:\.exe)?\s+(?:has\s+)?version\s+v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`)

	// goVersionRe matches "go version go1.26.3 darwin/arm64" or "go version 1.24.0"
	goVersionRe = regexp.MustCompile(`(?i)go\s+version\s+(?:go)?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:[a-zA-Z0-9.-]+)?)`)

	// seleneRe matches "selene version 0.3.1" or "selene 0.3.1"
	seleneRe = regexp.MustCompile(`(?i)selene(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`)

	// testqueryRe matches "testquery version 0.4.0" or "tq version 0.4.0"
	testqueryRe = regexp.MustCompile(`(?i)(?:testquery|tq)(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`)

	// modernizeRe matches "modernize version 0.1.0" or "modernize v0.1.0"
	modernizeRe = regexp.MustCompile(`(?i)modernize(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`)

	// deadcodeRe matches "deadcode version 0.1.0"
	deadcodeRe = regexp.MustCompile(`(?i)deadcode(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`)

	// GenericSemverRe fallback semver pattern
	GenericSemverRe = regexp.MustCompile(`v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.+_-]+)?)`)
)

// InstallInstructions provides platform and package manager specific install guidance.
type InstallInstructions struct {
	GoInstall string `json:"go_install,omitempty"`
	Homebrew  string `json:"homebrew,omitempty"`
	Script    string `json:"script,omitempty"`
	DocsURL   string `json:"docs_url,omitempty"`
}

// ToolSpec defines inspection metadata and upgrade guidance for an external utility.
type ToolSpec struct {
	ID                 string              `json:"id"`
	DisplayName        string              `json:"display_name"`
	Binaries           []string            `json:"binaries"`
	VersionArgs        [][]string          `json:"version_args"`
	OutputRegex        *regexp.Regexp      `json:"-"`
	DefaultRecommended string              `json:"default_recommended"`
	PackagePath        string              `json:"package_path"`
	Category           string              `json:"category"` // "compiler", "linter", "refactor", "test"
	Required           bool                `json:"required"`
	InstallGuide       InstallInstructions `json:"install_guide"`
}

// ToolStatus represents the evaluation result of an external utility.
type ToolStatus struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"display_name"`
	Status             Status `json:"status"` // OK, OUTDATED, MISSING, UNKNOWN
	BinaryPath         string `json:"binary_path,omitempty"`
	InstalledVersion   string `json:"installed_version,omitempty"`
	RecommendedVersion string `json:"recommended_version"`
	Satisfies          bool   `json:"satisfies"`
	UpgradeCommand     string `json:"upgrade_command,omitempty"`
	Category           string `json:"category,omitempty"`
	Required           bool   `json:"required"`
}

// DefaultRegistry returns the catalog of all external tools tracked by GoDoctor.
func DefaultRegistry() []ToolSpec {
	return []ToolSpec{
		{
			ID:                 "go",
			DisplayName:        "Go Toolchain",
			Binaries:           []string{"go"},
			VersionArgs:        [][]string{{"version"}},
			OutputRegex:        goVersionRe,
			DefaultRecommended: ">=1.24.0",
			Category:           "compiler",
			Required:           true,
			InstallGuide: InstallInstructions{
				Homebrew: "brew upgrade go",
				DocsURL:  "https://go.dev/dl/",
			},
		},
		{
			ID:                 "golangci_lint",
			DisplayName:        "golangci-lint",
			Binaries:           []string{"golangci-lint"},
			VersionArgs:        [][]string{{"--version"}, {"version"}},
			OutputRegex:        golangciLintRe,
			DefaultRecommended: "v2.12.2",
			PackagePath:        "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
			Category:           "linter",
			Required:           false,
			InstallGuide: InstallInstructions{
				GoInstall: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2",
				Homebrew:  "brew install golangci-lint",
				Script:    "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2",
				DocsURL:   "https://golangci-lint.run/welcome/install/",
			},
		},
		{
			ID:                 "modernize",
			DisplayName:        "modernize",
			Binaries:           []string{"modernize"},
			VersionArgs:        [][]string{{"-V"}, {"--version"}, {"-help"}},
			OutputRegex:        modernizeRe,
			DefaultRecommended: "latest",
			PackagePath:        "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize",
			Category:           "refactor",
			Required:           false,
			InstallGuide: InstallInstructions{
				GoInstall: "go install golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest",
				DocsURL:   "https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize",
			},
		},
		{
			ID:                 "deadcode",
			DisplayName:        "deadcode",
			Binaries:           []string{"deadcode"},
			VersionArgs:        [][]string{{"--version"}, {"-v"}, {"-help"}},
			OutputRegex:        deadcodeRe,
			DefaultRecommended: "latest",
			PackagePath:        "golang.org/x/tools/cmd/deadcode",
			Category:           "linter",
			Required:           false,
			InstallGuide: InstallInstructions{
				GoInstall: "go install golang.org/x/tools/cmd/deadcode@latest",
				DocsURL:   "https://pkg.go.dev/golang.org/x/tools/cmd/deadcode",
			},
		},
		{
			ID:                 "selene",
			DisplayName:        "selene",
			Binaries:           []string{"selene"},
			VersionArgs:        [][]string{{"--version"}, {"-v"}, {"version"}},
			OutputRegex:        seleneRe,
			DefaultRecommended: "latest",
			PackagePath:        "github.com/danicat/selene/cmd/selene",
			Category:           "test",
			Required:           false,
			InstallGuide: InstallInstructions{
				GoInstall: "go install github.com/danicat/selene/cmd/selene@latest",
				DocsURL:   "https://github.com/danicat/selene",
			},
		},
		{
			ID:                 "testquery",
			DisplayName:        "testquery (tq)",
			Binaries:           []string{"testquery", "tq"},
			VersionArgs:        [][]string{{"version"}, {"--version"}, {"-v"}},
			OutputRegex:        testqueryRe,
			DefaultRecommended: "latest",
			PackagePath:        "github.com/danicat/testquery",
			Category:           "test",
			Required:           false,
			InstallGuide: InstallInstructions{
				GoInstall: "go install github.com/danicat/testquery@latest",
				DocsURL:   "https://github.com/danicat/testquery",
			},
		},
	}
}

// FindToolSpec searches DefaultRegistry for a tool matching id or name.
func FindToolSpec(idOrName string) (ToolSpec, bool) {
	norm := strings.ToLower(strings.TrimSpace(idOrName))
	norm = strings.ReplaceAll(norm, "-", "_")

	for _, spec := range DefaultRegistry() {
		specIDNorm := strings.ReplaceAll(strings.ToLower(spec.ID), "-", "_")
		if specIDNorm == norm || strings.ToLower(spec.DisplayName) == norm {
			return spec, true
		}
		for _, b := range spec.Binaries {
			if strings.ToLower(b) == norm {
				return spec, true
			}
		}
	}
	return ToolSpec{}, false
}

// BuildUpgradeCommand constructs a clean install or upgrade command for a tool.
func BuildUpgradeCommand(spec ToolSpec, recommended string) string {
	if spec.PackagePath != "" {
		pkg := spec.PackagePath
		if idx := strings.Index(pkg, "@"); idx != -1 {
			pkg = pkg[:idx]
		}
		targetVer := recommended
		if targetVer == "" || strings.HasPrefix(targetVer, ">=") || strings.HasPrefix(targetVer, ">") {
			targetVer = "latest"
		}
		return fmt.Sprintf("go install %s@%s", pkg, targetVer)
	}

	if spec.InstallGuide.GoInstall != "" {
		return spec.InstallGuide.GoInstall
	}
	if spec.InstallGuide.Homebrew != "" {
		return spec.InstallGuide.Homebrew
	}
	if spec.InstallGuide.DocsURL != "" {
		return spec.InstallGuide.DocsURL
	}
	return ""
}
