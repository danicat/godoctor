// Package list implements the file listing tool.
package list

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["list_files"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Path  string `json:"path" jsonschema:"The root path to list (default: .)"`
	Depth int    `json:"depth,omitempty" jsonschema:"Maximum recursion depth (0 for default of 5, 1 for non-recursive)"`
}

func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	absRoot, err := roots.Global.Validate(args.Path)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	maxDepth := args.Depth
	if maxDepth == 0 {
		maxDepth = 5
	}

	// Try git ls-files first for .gitignore-aware listing
	if result, ok := tryGitLsFiles(ctx, absRoot, maxDepth); ok {
		return result, nil, nil
	}

	// Fallback to manual walk
	return walkDir(absRoot, maxDepth)
}

// tryGitLsFiles attempts to list files using git ls-files, which respects .gitignore.
func tryGitLsFiles(ctx context.Context, absRoot string, maxDepth int) (*mcp.CallToolResult, bool) {
	// Check if we're in a git repo
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = absRoot
	if _, err := cmd.Output(); err != nil {
		return nil, false
	}

	// Use git ls-files for tracked + untracked (but not ignored) files
	cmd = exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = absRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Listing files in %s (Depth: %d, git-aware)\n\n", absRoot, maxDepth))

	fileCount := 0
	dirsSeen := make(map[string]bool)
	const maxFiles = 1000

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}

		// Depth filter
		depth := strings.Count(line, "/") + 1
		if depth > maxDepth {
			continue
		}

		if fileCount >= maxFiles {
			sb.WriteString(fmt.Sprintf("\n(Limit of %d files reached, output truncated)\n", maxFiles))
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
			}, true
		}

		// Track directories
		dir := filepath.Dir(line)
		if dir != "." {
			parts := strings.Split(dir, "/")
			for i := range parts {
				d := strings.Join(parts[:i+1], "/")
				if !dirsSeen[d] {
					dirsSeen[d] = true
					sb.WriteString(fmt.Sprintf("%s/\n", d))
				}
			}
		}

		sb.WriteString(fmt.Sprintf("%s\n", line))
		fileCount++
	}

	sb.WriteString(fmt.Sprintf("\nFound %d files, %d directories.\n", fileCount, len(dirsSeen)))
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, true
}

// walkDir is the fallback directory walker for non-git directories.
func walkDir(absRoot string, maxDepth int) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Listing files in %s (Depth: %d)\n\n", absRoot, maxDepth))

	fileCount := 0
	dirCount := 0
	limitReached := false
	const maxFiles = 1000

	err := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			sb.WriteString(fmt.Sprintf("Warning: skipping %s: %v\n", path, err))
			return nil
		}

		relPath, _ := filepath.Rel(absRoot, path)
		if relPath == "." {
			return nil
		}

		depth := strings.Count(relPath, string(os.PathSeparator)) + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() && (d.Name() == ".git" || d.Name() == ".idea" || d.Name() == ".vscode" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}

		if fileCount >= maxFiles {
			limitReached = true
			return filepath.SkipAll
		}

		if d.IsDir() {
			sb.WriteString(fmt.Sprintf("%s/\n", relPath))
			dirCount++
		} else {
			sb.WriteString(fmt.Sprintf("%s\n", relPath))
			fileCount++
		}

		return nil
	})

	if err != nil {
		sb.WriteString(fmt.Sprintf("\nError walking: %v\n", err))
	}

	if limitReached {
		sb.WriteString(fmt.Sprintf("\n(Limit of %d files reached, output truncated)\n", maxFiles))
	} else {
		sb.WriteString(fmt.Sprintf("\nFound %d files, %d directories.\n", fileCount, dirCount))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
