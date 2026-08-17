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
	mcp.AddTool(server, &mcp.Tool{
		Name:  "smart_edit",
		Title: "Smart Edit",
		Description: "Single-file coordinate editing transaction. Automatically applies " +
			"the edit, formats using gofmt/goimports, and runs type verification (go vet) " +
			"across the workspace. If compiler checks fail, the edit is rolled back, and " +
			"Levenshtein-based spelling suggestions are returned for misspelled symbols.",
	}, Handler)
}

// FileEdit represents a single edit operation inside an atomic transaction.
type FileEdit struct {
	Filename   string  `json:"filename" jsonschema:"Absolute file path to edit. Required. Relative paths are rejected."`
	OldContent string  `json:"old_content,omitempty" jsonschema:"The block of code to find (ignores whitespace)"`
	NewContent string  `json:"new_content" jsonschema:"The new code to insert"`
	StartLine  int     `json:"start_line,omitempty" jsonschema:"Restrict search window to line number >= start_line"`
	EndLine    int     `json:"end_line,omitempty" jsonschema:"Restrict search window to line number <= end_line"`
	Threshold  float64 `json:"threshold,omitempty" jsonschema:"Similarity threshold (0.0-1.0) for fuzzy match"`
	Append     bool    `json:"append,omitempty" jsonschema:"Append new_content to end of file"`
}

// SingleEditParams defines the input parameters for the smart_edit single file edit tool.
type SingleEditParams = FileEdit

// Handler handles the smart_edit tool execution.
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
	if err != nil {
		if res != nil {
			return res, nil, nil
		}
		return errorResult(err.Error()), nil, nil
	}
	return res, nil, nil
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
