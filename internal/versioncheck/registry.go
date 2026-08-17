package versioncheck

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// CLI tool argument and identity constants.
const (
	ArgVersion     = "version"
	ArgDashVersion = "--version"
	ArgShortV      = "-v"
	ArgDashHelp    = "-help"

	ToolGo                      = "go"
	ToolGoDisplayName           = "Go Toolchain"
	ToolGolangCILint            = "golangci_lint"
	ToolGolangCILintDisplayName = "golangci-lint"
	ToolModernize               = "modernize"
	ToolDeadcode                = "deadcode"
	ToolSelene                  = "selene"
	ToolTestQuery               = "testquery"

	DefaultGoVersion       = ">=1.24.0"
	DefaultGolangCILintVer = "v2.12.2"
	DefaultLatestVer       = "latest"
)

var (
	// golangciLintRe matches "golangci-lint has version 2.12.2" or "golangci-lint version 1.64.0"
	golangciLintRe = regexp.MustCompile(
		`(?i)golangci-lint(?:\.exe)?\s+(?:has\s+)?version\s+v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`,
	)

	// goVersionRe matches "go version go1.26.3 darwin/arm64" or "go version 1.24.0"
	goVersionRe = regexp.MustCompile(
		`(?i)go\s+version\s+(?:go)?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:[a-zA-Z0-9.-]+)?)`,
	)

	// seleneRe matches "selene version 0.3.1" or "selene 0.3.1"
	seleneRe = regexp.MustCompile(
		`(?i)selene(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`,
	)

	// testqueryRe matches "testquery version 0.4.0" or "tq version 0.4.0"
	testqueryRe = regexp.MustCompile(
		`(?i)(?:testquery|tq)(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`,
	)

	// modernizeRe matches "modernize version 0.1.0" or "modernize v0.1.0"
	modernizeRe = regexp.MustCompile(
		`(?i)modernize(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`,
	)

	// deadcodeRe matches "deadcode version 0.1.0"
	deadcodeRe = regexp.MustCompile(
		`(?i)deadcode(?:\.exe)?\s+(?:version\s+)?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9a-zA-Z.-]+)?)`,
	)

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
	Timeout            time.Duration       `json:"timeout,omitempty"`
	Disabled           bool                `json:"disabled,omitempty"`
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

func specGo() ToolSpec {
	return ToolSpec{
		ID:                 ToolGo,
		DisplayName:        ToolGoDisplayName,
		Binaries:           []string{"go"},
		VersionArgs:        [][]string{{ArgVersion}},
		OutputRegex:        goVersionRe,
		DefaultRecommended: DefaultGoVersion,
		Category:           "compiler",
		Required:           true,
		InstallGuide: InstallInstructions{
			Homebrew: "brew upgrade go",
			DocsURL:  "https://go.dev/dl/",
		},
	}
}

func specGolangCILint() ToolSpec {
	return ToolSpec{
		ID:                 ToolGolangCILint,
		DisplayName:        ToolGolangCILintDisplayName,
		Binaries:           []string{ToolGolangCILintDisplayName},
		VersionArgs:        [][]string{{ArgDashVersion}, {ArgVersion}},
		OutputRegex:        golangciLintRe,
		DefaultRecommended: DefaultGolangCILintVer,
		PackagePath:        "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		Category:           "linter",
		Required:           false,
		InstallGuide: InstallInstructions{
			GoInstall: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + DefaultGolangCILintVer,
			Homebrew:  "brew install golangci-lint",
			Script: "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh" +
				" | sh -s -- -b $(go env GOPATH)/bin " + DefaultGolangCILintVer,
			DocsURL: "https://golangci-lint.run/welcome/install/",
		},
	}
}

func specModernize() ToolSpec {
	return ToolSpec{
		ID:                 ToolModernize,
		DisplayName:        ToolModernize,
		Binaries:           []string{ToolModernize},
		VersionArgs:        [][]string{{"-V"}, {ArgDashVersion}, {ArgDashHelp}},
		OutputRegex:        modernizeRe,
		DefaultRecommended: DefaultLatestVer,
		PackagePath:        "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize",
		Category:           "refactor",
		Required:           false,
		InstallGuide: InstallInstructions{
			GoInstall: "go install golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@" + DefaultLatestVer,
			DocsURL:   "https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize",
		},
	}
}

func specDeadcode() ToolSpec {
	return ToolSpec{
		ID:                 ToolDeadcode,
		DisplayName:        ToolDeadcode,
		Binaries:           []string{ToolDeadcode},
		VersionArgs:        [][]string{{ArgDashVersion}, {ArgShortV}, {ArgDashHelp}},
		OutputRegex:        deadcodeRe,
		DefaultRecommended: DefaultLatestVer,
		PackagePath:        "golang.org/x/tools/cmd/deadcode",
		Category:           "linter",
		Required:           false,
		InstallGuide: InstallInstructions{
			GoInstall: "go install golang.org/x/tools/cmd/deadcode@" + DefaultLatestVer,
			DocsURL:   "https://pkg.go.dev/golang.org/x/tools/cmd/deadcode",
		},
	}
}

func specSelene() ToolSpec {
	return ToolSpec{
		ID:                 ToolSelene,
		DisplayName:        ToolSelene,
		Binaries:           []string{ToolSelene},
		VersionArgs:        [][]string{{ArgDashVersion}, {ArgShortV}, {ArgVersion}},
		OutputRegex:        seleneRe,
		DefaultRecommended: DefaultLatestVer,
		PackagePath:        "github.com/danicat/selene/cmd/selene",
		Category:           "test",
		Required:           false,
		InstallGuide: InstallInstructions{
			GoInstall: "go install github.com/danicat/selene/cmd/selene@" + DefaultLatestVer,
			DocsURL:   "https://github.com/danicat/selene",
		},
	}
}

func specTestQuery() ToolSpec {
	return ToolSpec{
		ID:                 ToolTestQuery,
		DisplayName:        "testquery (tq)",
		Binaries:           []string{ToolTestQuery, "tq"},
		VersionArgs:        [][]string{{ArgVersion}, {ArgDashVersion}, {ArgShortV}},
		OutputRegex:        testqueryRe,
		DefaultRecommended: DefaultLatestVer,
		PackagePath:        "github.com/danicat/testquery",
		Category:           "test",
		Required:           false,
		InstallGuide: InstallInstructions{
			GoInstall: "go install github.com/danicat/testquery@" + DefaultLatestVer,
			DocsURL:   "https://github.com/danicat/testquery",
		},
	}
}

// DefaultRegistry returns the catalog of all external tools tracked by GoDoctor.
func DefaultRegistry() []ToolSpec {
	return []ToolSpec{
		specGo(),
		specGolangCILint(),
		specModernize(),
		specDeadcode(),
		specSelene(),
		specTestQuery(),
	}
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
			targetVer = DefaultLatestVer
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
