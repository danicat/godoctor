// Package versioncheck provides tools for discovering, inspecting, comparing semantic versions,
// and generating actionable diagnostic upgrade recommendations for GoDoctor external utilities.
package versioncheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Status represents the health status of an external tool.
type Status string

const (
	// StatusOk indicates the tool is installed and meets or exceeds recommended version.
	StatusOk Status = "OK"
	// StatusOutdated indicates the installed version is older than recommended.
	StatusOutdated Status = "OUTDATED"
	// StatusMissing indicates the tool was not found in $PATH.
	StatusMissing Status = "MISSING"
	// StatusUnknown indicates the tool is present but its version could not be parsed.
	StatusUnknown Status = "UNKNOWN"
)

// Version models a parsed semantic version string.
type Version struct {
	Raw        string
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	IsDevel    bool
	CommitHash string
}

// ParseVersion converts raw version strings (e.g. "v1.24.0", "go1.26.3", "v2.12.2-rc1", "devel (abc1234)")
// into a structured Version.
func ParseVersion(vStr string) Version {
	vStr = strings.TrimSpace(vStr)
	v := Version{Raw: vStr}

	if vStr == "" {
		return v
	}

	lower := strings.ToLower(vStr)
	if lower == "(devel)" || strings.HasPrefix(lower, "devel") {
		v.IsDevel = true
		// Check if commit hash is present, e.g. "devel (abc1234)" or "devel +abc1234"
		if hash := extractCommitHash(vStr); hash != "" {
			v.CommitHash = hash
		}
		return v
	}

	clean := strings.TrimPrefix(vStr, "v")
	clean = strings.TrimPrefix(clean, "V")
	clean = strings.TrimPrefix(clean, "go")

	// Separate prerelease/build metadata: "1.24.0-rc1+build" -> main "1.24.0", prerelease "rc1"
	mainPart := clean
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		mainPart = clean[:idx]
		v.Prerelease = clean[idx+1:]
	}

	parts := strings.Split(mainPart, ".")
	if len(parts) >= 1 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		v.Patch, _ = strconv.Atoi(parts[2])
	}

	return v
}

// Compare compares version v to target.
// Returns:
//   - -1 if v < target
//   -  0 if v == target
//   - +1 if v > target
func (v Version) Compare(target Version) int {
	if v.Major != target.Major {
		if v.Major < target.Major {
			return -1
		}
		return 1
	}
	if v.Minor != target.Minor {
		if v.Minor < target.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != target.Patch {
		if v.Patch < target.Patch {
			return -1
		}
		return 1
	}

	// Prerelease comparison: "1.24.0-rc1" < "1.24.0" (a release version is higher than a prerelease of same M.m.p)
	if v.Prerelease != "" && target.Prerelease == "" {
		return -1
	}
	if v.Prerelease == "" && target.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && target.Prerelease != "" {
		return strings.Compare(v.Prerelease, target.Prerelease)
	}

	return 0
}

// Satisfies determines if the installed version satisfies the recommended constraint.
// Supports:
//   - "latest" or empty constraint (always satisfied if installed)
//   - Range constraint e.g. ">=1.24.0" or ">=1.24"
//   - Exact / Minimum pinned version e.g. "v2.12.2" or "1.64.0"
func Satisfies(installed Version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)

	// "latest" or empty means any installed version satisfies the recommendation
	if constraint == "" || strings.EqualFold(constraint, "latest") {
		return true
	}

	// Devel builds satisfy constraints unless an explicit strict pin is enforced
	if installed.IsDevel {
		return true
	}

	if strings.HasPrefix(constraint, ">=") {
		targetStr := strings.TrimSpace(strings.TrimPrefix(constraint, ">="))
		target := ParseVersion(targetStr)
		return installed.Compare(target) >= 0
	}

	if strings.HasPrefix(constraint, ">") {
		targetStr := strings.TrimSpace(strings.TrimPrefix(constraint, ">"))
		target := ParseVersion(targetStr)
		return installed.Compare(target) > 0
	}

	if strings.HasPrefix(constraint, "<=") {
		targetStr := strings.TrimSpace(strings.TrimPrefix(constraint, "<="))
		target := ParseVersion(targetStr)
		return installed.Compare(target) <= 0
	}

	if strings.HasPrefix(constraint, "=") {
		targetStr := strings.TrimSpace(strings.TrimPrefix(constraint, "="))
		target := ParseVersion(targetStr)
		return installed.Compare(target) == 0
	}

	// Default: treated as minimum recommended version (Installed >= Recommended)
	target := ParseVersion(constraint)
	return installed.Compare(target) >= 0
}

// ExtractVersion searches text using the given regex pattern and returns the first capture group.
func ExtractVersion(text string, re *regexp.Regexp) string {
	if re == nil || text == "" {
		return ""
	}
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 && matches[1] != "" {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

var hashRe = regexp.MustCompile(`[0-9a-fA-F]{7,40}`)

func extractCommitHash(text string) string {
	return hashRe.FindString(text)
}

// String returns formatted version string.
func (v Version) String() string {
	if v.IsDevel {
		if v.CommitHash != "" {
			return fmt.Sprintf("devel (%s)", v.CommitHash)
		}
		return "devel"
	}
	if v.Raw != "" {
		return v.Raw
	}
	res := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		res += "-" + v.Prerelease
	}
	return res
}
