// Package shared provides shared utilities for GoDoctor tools.
package shared

import (
	"fmt"
	"regexp"
)

var (
	// undefinedPkg match: "undefined: pkgname.Symbol"
	undefinedPkgRe = regexp.MustCompile(`undefined:\s+([a-zA-Z0-9_]+)\.`)
	// importError match: "could not import github.com/foo/bar" or "package github.com/foo/bar is not in GOROOT"
	importErrorRe = regexp.MustCompile(`(?:could not import|package)\s+([a-zA-Z0-9_./-]+)`)
)

// GetDocHintFromOutput checks a raw output string for API usage issues and returns a generic doc hint.
func GetDocHintFromOutput(output string) string {
	return generateHint(output)
}

func generateHint(msg string) string {
	// Check for "undefined: pkg.Symbol"
	if matches := undefinedPkgRe.FindStringSubmatch(msg); len(matches) > 1 {
		pkgName := matches[1]
		return fmt.Sprintf("\n\n**HINT:** usage of '%s' failed. "+
			"Try calling `go_docs` on that package to see the correct API.", pkgName)
	}

	// Check for "could not import ..."
	if matches := importErrorRe.FindStringSubmatch(msg); len(matches) > 1 {
		pkgPath := matches[1]
		return fmt.Sprintf("\n\n**HINT:** import '%s' failed. "+
			"Try calling `go_docs` on \"%s\" to verify the package path and exports.", pkgPath, pkgPath)
	}

	return ""
}
