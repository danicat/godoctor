// Package list implements the file listing tool.
package list

import (
	"context"
	"fmt"
	"io/fs"
	"os"
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
	Path  string `json:"path" jsonschema:"The absolute root path to list. You MUST pass the absolute path in multi-root workspaces."`
	Depth int    `json:"depth,omitempty" jsonschema:"Maximum recursion depth (0 for default of 5, 1 for non-recursive)"`
}

func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}

	absRoot, err := roots.Global.Validate(session, args.Path)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	maxDepth := args.Depth
	if maxDepth == 0 {
		maxDepth = 5
	}

	return walkDir(absRoot, maxDepth)
}

// walkDir is the directory walker that lists all files and directories, ignoring only `.git`.
func walkDir(absRoot string, maxDepth int) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Listing files in %s (Depth: %d)\n\n", absRoot, maxDepth)

	fileCount := 0
	dirCount := 0
	limitReached := false
	const maxFiles = 1000

	err := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(&sb, "Warning: skipping %s: %v\n", path, err)
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

		// Strictly exclude ONLY .git to prevent infinite recursion/extraneous noise
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if fileCount >= maxFiles {
			limitReached = true
			return filepath.SkipAll
		}

		if d.IsDir() {
			fmt.Fprintf(&sb, "%s/\n", relPath)
			dirCount++
		} else {
			fmt.Fprintf(&sb, "%s\n", relPath)
			fileCount++
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(&sb, "\nError walking: %v\n", err)
	}

	if limitReached {
		fmt.Fprintf(&sb, "\n(Limit of %d files reached, output truncated)\n", maxFiles)
	} else {
		fmt.Fprintf(&sb, "\nFound %d files, %d directories.\n", fileCount, dirCount)
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
