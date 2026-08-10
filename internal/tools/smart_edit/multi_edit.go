package smartedit

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterMultiEdit registers the smart_multi_edit tool with the server.
func RegisterMultiEdit(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart_multi_edit",
		Title:       "Smart Multi Edit",
		Description: "Atomic, multi-file batch coordinate editing transaction. Performs multiple file edits atomically across the workspace in a single compiler-verified transaction. If any edit causes a compiler error, all edits are completely rolled back.",
	}, multiEditHandler)
}

// MultiEditParams defines the input parameters for the smart_multi_edit tool.
type MultiEditParams struct {
	//nolint:lll
	Operations []FileEdit `json:"operations" jsonschema:"List of edit operations to perform atomically in a single compiler-verified transaction"`
}

//nolint:lll
func multiEditHandler(ctx context.Context, req *mcp.CallToolRequest, args MultiEditParams) (*mcp.CallToolResult, any, error) {
	if len(args.Operations) == 0 {
		return errorResult("operations array is required and must contain at least one edit operation"), nil, nil
	}

	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}

	res, err := ExecuteEdits(ctx, session, args.Operations)
	return res, nil, err
}
