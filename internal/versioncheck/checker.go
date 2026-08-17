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

func (c *Checker) probeCLIVersion(ctx context.Context, foundPath string, spec ToolSpec) string {
	timeout := c.timeout
	if spec.Timeout > 0 {
		timeout = spec.Timeout
	}
	for _, args := range spec.VersionArgs {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		out, err := c.runner.RunCommand(cmdCtx, foundPath, args...)
		cancel()

		if err == nil || len(out) > 0 {
			outStr := string(out)
			if extracted := ExtractVersion(outStr, spec.OutputRegex); extracted != "" {
				return extracted
			}
			if extracted := ExtractVersion(outStr, GenericSemverRe); extracted != "" {
				return extracted
			}
		}
	}
	return ""
}

func (c *Checker) inspectBuildInfo(foundPath string, spec ToolSpec) string {
	info, err := c.runner.ReadBuildInfo(foundPath)
	if err != nil || info == nil {
		return ""
	}
	if info.Main.Version != "" && info.Main.Version != DevelParenVersion {
		return info.Main.Version
	}

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

	switch {
	case revision != "":
		return fmt.Sprintf("devel (%s)", revision)
	case info.GoVersion != "" && spec.ID == ToolGo:
		return info.GoVersion
	default:
		return DevelVersion
	}
}

func (c *Checker) findBinaryPath(spec ToolSpec) string {
	for _, bin := range spec.Binaries {
		if p, err := c.runner.LookPath(bin); err == nil && p != "" {
			return p
		}
	}
	return ""
}

func (c *Checker) evaluateToolStatus(spec ToolSpec, foundPath, rawVer string) ToolStatus {
	status := ToolStatus{
		ID:                 spec.ID,
		DisplayName:        spec.DisplayName,
		RecommendedVersion: spec.DefaultRecommended,
		Category:           spec.Category,
		Required:           spec.Required,
		BinaryPath:         foundPath,
		InstalledVersion:   rawVer,
	}

	if rawVer == "" {
		status.Status = StatusUnknown
		status.Satisfies = true
		return status
	}

	parsed := ParseVersion(rawVer)
	status.Satisfies = Satisfies(parsed, spec.DefaultRecommended)
	if status.Satisfies {
		status.Status = StatusOk
	} else {
		status.Status = StatusOutdated
		status.UpgradeCommand = BuildUpgradeCommand(spec, spec.DefaultRecommended)
	}
	return status
}

// CheckTool inspects a single tool spec and returns its ToolStatus.
func (c *Checker) CheckTool(ctx context.Context, spec ToolSpec) (ToolStatus, error) {
	if spec.Disabled {
		return ToolStatus{
			ID:                 spec.ID,
			DisplayName:        spec.DisplayName,
			RecommendedVersion: spec.DefaultRecommended,
			Category:           spec.Category,
			Required:           spec.Required,
			Status:             StatusOk,
			Satisfies:          true,
		}, nil
	}

	foundPath := c.findBinaryPath(spec)
	if foundPath == "" {
		return ToolStatus{
			ID:                 spec.ID,
			DisplayName:        spec.DisplayName,
			RecommendedVersion: spec.DefaultRecommended,
			Category:           spec.Category,
			Required:           spec.Required,
			Status:             StatusMissing,
			Satisfies:          false,
			UpgradeCommand:     BuildUpgradeCommand(spec, spec.DefaultRecommended),
		}, nil
	}

	if !c.noCache && c.cache != nil {
		if cached, ok := c.cache.Get(spec.ID, foundPath); ok {
			cached.RecommendedVersion = spec.DefaultRecommended
			cached.Satisfies = Satisfies(ParseVersion(cached.InstalledVersion), spec.DefaultRecommended)
			if !cached.Satisfies && cached.Status != StatusMissing {
				cached.Status = StatusOutdated
				cached.UpgradeCommand = BuildUpgradeCommand(spec, spec.DefaultRecommended)
			}
			return cached, nil
		}
	}

	rawVer := c.probeCLIVersion(ctx, foundPath, spec)
	if rawVer == "" || rawVer == DevelVersion || rawVer == DevelParenVersion {
		if buildVer := c.inspectBuildInfo(foundPath, spec); buildVer != "" {
			rawVer = buildVer
		}
	}

	status := c.evaluateToolStatus(spec, foundPath, rawVer)
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

func applyConfigToSpec(spec *ToolSpec, toolCfg config.ToolSpec) {
	if toolCfg.RecommendedVersion != "" {
		spec.DefaultRecommended = toolCfg.RecommendedVersion
	}
	if toolCfg.Package != "" {
		spec.PackagePath = toolCfg.Package
	}
	if toolCfg.Command != "" {
		spec.Binaries = []string{toolCfg.Command}
	}
	if toolCfg.Timeout > 0 {
		spec.Timeout = toolCfg.Timeout
	}
	if toolCfg.Disabled {
		spec.Disabled = true
	}
}

// CheckAll evaluates all tools, applying custom versions, package paths, timeouts,
// and filtering disabled tools from GoDoctor config if supplied.
func CheckAll(ctx context.Context, cfg *config.Config) ([]ToolStatus, error) {
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}
	specs := DefaultRegistry()
	activeSpecs := make([]ToolSpec, 0, len(specs))
	for _, spec := range specs {
		if toolCfg, found := cfg.LookupTool(spec.ID); found {
			if toolCfg.Disabled {
				continue
			}
			applyConfigToSpec(&spec, toolCfg)
		}
		activeSpecs = append(activeSpecs, spec)
	}
	return DefaultChecker.CheckAll(ctx, activeSpecs...)
}

func formatStatusBadgeAndAction(st ToolStatus) (badge string, isHealthy, isOutdated, isMissing bool, rec string) {
	badge = string(st.Status)
	switch st.Status {
	case StatusOk:
		badge = "✓ OK"
		isHealthy = true
	case StatusOutdated:
		badge = "⚠️ OUTDATED"
		isOutdated = true
		if st.UpgradeCommand != "" {
			rec = fmt.Sprintf("• Upgrade %s:\n    $ %s", st.DisplayName, st.UpgradeCommand)
		}
	case StatusMissing:
		badge = "✗ MISSING"
		isMissing = true
		if st.UpgradeCommand != "" {
			rec = fmt.Sprintf("• Install %s:\n    $ %s", st.DisplayName, st.UpgradeCommand)
		}
	case StatusUnknown:
		badge = "? UNKNOWN"
		isHealthy = true
	}
	return badge, isHealthy, isOutdated, isMissing, rec
}

func formatStatusSummaryAndRecommendations(sb *strings.Builder, healthy, outdated, missing, total int, recs []string) {
	fmt.Fprintf(sb, "Summary: %d/%d tools healthy", healthy, total)
	if outdated > 0 {
		fmt.Fprintf(sb, ", %d outdated", outdated)
	}
	if missing > 0 {
		fmt.Fprintf(sb, ", %d missing", missing)
	}
	sb.WriteString(".\n")

	if len(recs) > 0 {
		sb.WriteString("\n💡 Recommended Actions:\n")
		for _, rec := range recs {
			sb.WriteString("  " + rec + "\n")
		}
	}
	sb.WriteString("========================================================================================\n")
}

// FormatStatusTable renders a formatted ASCII / Unicode table suitable for terminal display.
func FormatStatusTable(statuses []ToolStatus) string {
	var sb strings.Builder

	sb.WriteString("========================================================================================\n")
	sb.WriteString("                   🩺 GoDoctor Environment & Tool Diagnostic Check                     \n")
	sb.WriteString("========================================================================================\n\n")

	fmt.Fprintf(&sb, "  %-18s %-12s %-12s %-14s %s\n",
		"TOOL", "STATUS", "INSTALLED", "RECOMMENDED", "UPGRADE COMMAND")
	sb.WriteString("  ──────────────────────────────────────────────────────────────────────────────────────\n")

	var healthyCount, outdatedCount, missingCount int
	var recommendations []string

	for _, st := range statuses {
		statusBadge, isHealthy, isOutdated, isMissing, rec := formatStatusBadgeAndAction(st)
		if isHealthy {
			healthyCount++
		}
		if isOutdated {
			outdatedCount++
		}
		if isMissing {
			missingCount++
		}
		if rec != "" {
			recommendations = append(recommendations, rec)
		}

		installed := st.InstalledVersion
		if installed == "" {
			installed = "none"
		}

		upgradeCmd := st.UpgradeCommand
		if upgradeCmd == "" {
			upgradeCmd = "(up to date)"
		}

		fmt.Fprintf(&sb, "  %-18s %-12s %-12s %-14s %s\n",
			st.DisplayName, statusBadge, installed, st.RecommendedVersion, upgradeCmd)
	}

	sb.WriteString("  ──────────────────────────────────────────────────────────────────────────────────────\n")
	formatStatusSummaryAndRecommendations(&sb, healthyCount, outdatedCount, missingCount, len(statuses), recommendations)
	return sb.String()
}
