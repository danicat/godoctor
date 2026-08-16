package versioncheck

import (
	"context"
	"debug/buildinfo"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/safeshell"
)

// Runner abstracts CLI execution, path lookup, and binary inspection for testability.
type Runner interface {
	LookPath(file string) (string, error)
	RunCommand(ctx context.Context, name string, args ...string) ([]byte, error)
	ReadBuildInfo(path string) (*buildinfo.BuildInfo, error)
	Stat(path string) (os.FileInfo, error)
}

// stdRunner is the default production Runner implementation.
type stdRunner struct{}

func (r *stdRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (r *stdRunner) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd, err := safeshell.CommandContext(ctx, name, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

func (r *stdRunner) ReadBuildInfo(path string) (*buildinfo.BuildInfo, error) {
	return buildinfo.ReadFile(path)
}

func (r *stdRunner) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Option configures Checker instances.
type Option func(*Checker)

// WithRunner sets a custom Runner (useful for testing and mocks).
func WithRunner(r Runner) Option {
	return func(c *Checker) {
		if r != nil {
			c.runner = r
		}
	}
}

// WithCache sets a custom VersionCache.
func WithCache(cache *VersionCache) Option {
	return func(c *Checker) {
		c.cache = cache
	}
}

// WithTimeout sets per-tool CLI invocation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Checker) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithNoCache disables reading from or writing to the cache.
func WithNoCache(noCache bool) Option {
	return func(c *Checker) {
		c.noCache = noCache
	}
}

// Checker coordinates binary discovery, version extraction, semver evaluation, and caching.
type Checker struct {
	runner  Runner
	cache   *VersionCache
	timeout time.Duration
	noCache bool
}

// NewChecker initializes a new Checker with optional overrides.
func NewChecker(opts ...Option) *Checker {
	c := &Checker{
		runner:  &stdRunner{},
		cache:   DefaultCache,
		timeout: 2 * time.Second,
		noCache: false,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// DefaultChecker is the package-level default Checker instance.
var DefaultChecker = NewChecker()

// CheckTool inspects a single tool spec and returns its ToolStatus.
func (c *Checker) CheckTool(ctx context.Context, spec ToolSpec) (ToolStatus, error) {
	status := ToolStatus{
		ID:                 spec.ID,
		DisplayName:        spec.DisplayName,
		RecommendedVersion: spec.DefaultRecommended,
		Category:           spec.Category,
		Required:           spec.Required,
	}

	// 1. Discovery: LookPath across candidate binaries
	var foundPath string
	for _, bin := range spec.Binaries {
		if p, err := c.runner.LookPath(bin); err == nil && p != "" {
			foundPath = p
			break
		}
	}

	if foundPath == "" {
		status.Status = StatusMissing
		status.Satisfies = false
		status.UpgradeCommand = BuildUpgradeCommand(spec, spec.DefaultRecommended)
		return status, nil
	}

	status.BinaryPath = foundPath

	// 2. Cache check
	if !c.noCache && c.cache != nil {
		if cached, ok := c.cache.Get(spec.ID, foundPath); ok {
			// Retain recommendation from spec in case of overrides
			cached.RecommendedVersion = spec.DefaultRecommended
			cached.Satisfies = Satisfies(ParseVersion(cached.InstalledVersion), spec.DefaultRecommended)
			if !cached.Satisfies && cached.Status != StatusMissing {
				cached.Status = StatusOutdated
				cached.UpgradeCommand = BuildUpgradeCommand(spec, spec.DefaultRecommended)
			}
			return cached, nil
		}
	}

	// 3. CLI Version Probe
	var rawVer string
	for _, args := range spec.VersionArgs {
		cmdCtx, cancel := context.WithTimeout(ctx, c.timeout)
		out, err := c.runner.RunCommand(cmdCtx, foundPath, args...)
		cancel()

		if err == nil || len(out) > 0 {
			outStr := string(out)
			if extracted := ExtractVersion(outStr, spec.OutputRegex); extracted != "" {
				rawVer = extracted
				break
			}
			if extracted := ExtractVersion(outStr, GenericSemverRe); extracted != "" {
				rawVer = extracted
				break
			}
		}
	}

	// 4. Binary Header Inspection Fallback (debug/buildinfo)
	if rawVer == "" || rawVer == "devel" || rawVer == "(devel)" {
		if info, err := c.runner.ReadBuildInfo(foundPath); err == nil && info != nil {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				rawVer = info.Main.Version
			} else {
				var revision string
				for _, setting := range info.Settings {
					if setting.Key == "vcs.revision" {
						revision = setting.Value
						if len(revision) > 7 {
							revision = revision[:7]
						}
						break
					}
				}
				if revision != "" {
					rawVer = fmt.Sprintf("devel (%s)", revision)
				} else if info.GoVersion != "" && spec.ID == "go" {
					rawVer = info.GoVersion
				} else {
					rawVer = "devel"
				}
			}
		}
	}

	// 5. Evaluate Semver and Status
	status.InstalledVersion = rawVer
	if rawVer == "" {
		status.Status = StatusUnknown
		status.Satisfies = true // Do not block on unparseable local binary
	} else {
		parsed := ParseVersion(rawVer)
		status.Satisfies = Satisfies(parsed, spec.DefaultRecommended)
		if status.Satisfies {
			status.Status = StatusOk
		} else {
			status.Status = StatusOutdated
			status.UpgradeCommand = BuildUpgradeCommand(spec, spec.DefaultRecommended)
		}
	}

	// 6. Cache store
	if !c.noCache && c.cache != nil {
		c.cache.Set(spec.ID, foundPath, status)
	}

	return status, nil
}

// CheckAll evaluates all provided tool specifications (or DefaultRegistry if empty).
func (c *Checker) CheckAll(ctx context.Context, specs ...ToolSpec) ([]ToolStatus, error) {
	if len(specs) == 0 {
		specs = DefaultRegistry()
	}

	results := make([]ToolStatus, 0, len(specs))
	for _, spec := range specs {
		st, err := c.CheckTool(ctx, spec)
		if err != nil {
			return nil, err
		}
		results = append(results, st)
	}
	return results, nil
}

// CheckAll evaluates all tools, applying custom versions or package paths from GoDoctor config if supplied.
func CheckAll(ctx context.Context, cfg *config.Config) ([]ToolStatus, error) {
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}
	specs := DefaultRegistry()
	for i := range specs {
		if toolCfg, found := cfg.LookupTool(specs[i].ID); found {
			if toolCfg.RecommendedVersion != "" {
				specs[i].DefaultRecommended = toolCfg.RecommendedVersion
			}
			if toolCfg.Package != "" {
				specs[i].PackagePath = toolCfg.Package
			}
			if toolCfg.BinaryName != "" {
				specs[i].Binaries = []string{toolCfg.BinaryName}
			}
		}
	}
	return DefaultChecker.CheckAll(ctx, specs...)
}

// CheckTool evaluates an individual tool by name, optionally with a config.ToolSpec override.
func CheckTool(ctx context.Context, name string, spec ...config.ToolSpec) (ToolStatus, error) {
	tSpec, found := FindToolSpec(name)
	if !found {
		tSpec = ToolSpec{
			ID:          name,
			DisplayName: name,
			Binaries:    []string{name},
		}
	}
	if len(spec) > 0 {
		if spec[0].BinaryName != "" {
			tSpec.Binaries = []string{spec[0].BinaryName}
		}
		if spec[0].RecommendedVersion != "" {
			tSpec.DefaultRecommended = spec[0].RecommendedVersion
		}
		if spec[0].Package != "" {
			tSpec.PackagePath = spec[0].Package
		}
	}
	return DefaultChecker.CheckTool(ctx, tSpec)
}

// CheckToolByName evaluates a single tool by its ID or name against defaults.
func CheckToolByName(ctx context.Context, idOrName string) (ToolStatus, error) {
	return CheckTool(ctx, idOrName)
}

// FormatStatusTable renders a formatted ASCII / Unicode table suitable for terminal display.
func FormatStatusTable(statuses []ToolStatus) string {
	var sb strings.Builder

	sb.WriteString("========================================================================================\n")
	sb.WriteString("                   🩺 GoDoctor Environment & Tool Diagnostic Check                     \n")
	sb.WriteString("========================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("  %-18s %-12s %-12s %-14s %s\n", "TOOL", "STATUS", "INSTALLED", "RECOMMENDED", "UPGRADE COMMAND"))
	sb.WriteString("  ──────────────────────────────────────────────────────────────────────────────────────\n")

	var healthyCount, outdatedCount, missingCount int
	var recommendations []string

	for _, st := range statuses {
		statusBadge := string(st.Status)
		switch st.Status {
		case StatusOk:
			statusBadge = "✓ OK"
			healthyCount++
		case StatusOutdated:
			statusBadge = "⚠️ OUTDATED"
			outdatedCount++
			if st.UpgradeCommand != "" {
				recommendations = append(recommendations, fmt.Sprintf("• Upgrade %s:\n    $ %s", st.DisplayName, st.UpgradeCommand))
			}
		case StatusMissing:
			statusBadge = "✗ MISSING"
			missingCount++
			if st.UpgradeCommand != "" {
				recommendations = append(recommendations, fmt.Sprintf("• Install %s:\n    $ %s", st.DisplayName, st.UpgradeCommand))
			}
		case StatusUnknown:
			statusBadge = "? UNKNOWN"
			healthyCount++
		}

		installed := st.InstalledVersion
		if installed == "" {
			installed = "none"
		}

		upgradeCmd := st.UpgradeCommand
		if upgradeCmd == "" {
			upgradeCmd = "(up to date)"
		}

		sb.WriteString(fmt.Sprintf("  %-18s %-12s %-12s %-14s %s\n",
			st.DisplayName, statusBadge, installed, st.RecommendedVersion, upgradeCmd))
	}

	sb.WriteString("  ──────────────────────────────────────────────────────────────────────────────────────\n")
	fmt.Fprintf(&sb, "Summary: %d/%d tools healthy", healthyCount, len(statuses))
	if outdatedCount > 0 {
		fmt.Fprintf(&sb, ", %d outdated", outdatedCount)
	}
	if missingCount > 0 {
		fmt.Fprintf(&sb, ", %d missing", missingCount)
	}
	sb.WriteString(".\n")

	if len(recommendations) > 0 {
		sb.WriteString("\n💡 Recommended Actions:\n")
		for _, rec := range recommendations {
			sb.WriteString("  " + rec + "\n")
		}
	}
	sb.WriteString("========================================================================================\n")

	return sb.String()
}

// FormatRecommendationsMarkdown formats non-blocking diagnostic hints in GitHub-flavored Markdown.
// Returns an empty string if all tools satisfy recommendations.
func FormatRecommendationsMarkdown(statuses []ToolStatus) string {
	var hints []string

	for _, st := range statuses {
		if st.Status == StatusOutdated {
			hints = append(hints, fmt.Sprintf("- ⚠️ `%s`: Installed `%s` is older than recommended `%s`.\n  Upgrade: `%s`",
				st.DisplayName, st.InstalledVersion, st.RecommendedVersion, st.UpgradeCommand))
		} else if st.Status == StatusMissing && !st.Required {
			hints = append(hints, fmt.Sprintf("- 💡 `%s`: Missing local binary (falling back to slower `go run` execution).\n  Install: `%s`",
				st.DisplayName, st.UpgradeCommand))
		}
	}

	if len(hints) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("> [!TIP]\n")
	sb.WriteString("> **GoDoctor Tool Upgrade Recommendations**\n")
	for _, hint := range hints {
		for _, line := range strings.Split(hint, "\n") {
			sb.WriteString("> " + line + "\n")
		}
	}

	return sb.String()
}

// GetUpgradeHintsMarkdown inspects the requested tool IDs and returns Markdown hints if any are sub-optimal.
func GetUpgradeHintsMarkdown(ctx context.Context, toolIDs []string) string {
	var specs []ToolSpec
	for _, id := range toolIDs {
		if spec, found := FindToolSpec(id); found {
			specs = append(specs, spec)
		}
	}
	if len(specs) == 0 {
		return ""
	}

	statuses, err := DefaultChecker.CheckAll(ctx, specs...)
	if err != nil || len(statuses) == 0 {
		return ""
	}

	return FormatRecommendationsMarkdown(statuses)
}
