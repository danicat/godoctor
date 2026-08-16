// Package smartedit implements the file editing tool with atomic multi-file transactions,
// formatting, compiler gates, and spelling aids.
package smartedit

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the smart_edit tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart_edit",
		Title:       "Smart Edit",
		Description: "Single-file coordinate editing transaction. Automatically applies the edit, formats using gofmt/goimports, and runs type verification (go vet) across the workspace. If compiler checks fail, the edit is rolled back, and Levenshtein-based spelling suggestions are returned for misspelled symbols.",
	}, Handler)
}

// FileEdit represents a single edit operation inside an atomic transaction.
type FileEdit struct {
	//nolint:lll
	Filename string `json:"filename" jsonschema:"The absolute path to the file to edit. Required. Relative paths are rejected."`
	//nolint:lll
	OldContent string `json:"old_content,omitempty" jsonschema:"Optional: The block of code to find (ignores whitespace)"`
	NewContent string `json:"new_content" jsonschema:"The new code to insert"`
	//nolint:lll
	StartLine int `json:"start_line,omitempty" jsonschema:"Optional: restrict search window to line number >= start_line"`
	//nolint:lll
	EndLine int `json:"end_line,omitempty" jsonschema:"Optional: restrict search window to line number <= end_line"`
	//nolint:lll
	Threshold float64 `json:"threshold,omitempty" jsonschema:"Optional: similarity threshold (0.0-1.0) for fuzzy matching (default 0.95)"`
	//nolint:lll
	Append bool `json:"append,omitempty" jsonschema:"Optional: append new_content to end of file"`
}

// SingleEditParams defines the input parameters for the smart_edit single file edit tool.
type SingleEditParams = FileEdit

// Handler handles the smart_edit tool execution.
//
//nolint:lll
func Handler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args SingleEditParams,
) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Filename) == "" || !filepath.IsAbs(args.Filename) {
		return errorResult("filename is required and must be an absolute path"), nil, nil
	}

	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}

	res, err := ExecuteEdits(ctx, session, []FileEdit{args})
	if res != nil {
		return res, nil, nil
	}
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}
	return nil, nil, nil
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
