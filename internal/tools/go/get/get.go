// Package get implements the go get tool.
package get

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/danicat/godoctor/internal/godoc"
	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["add_dependency"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Dir      string   `json:"dir,omitempty" jsonschema:"The absolute directory path to run go get in. Always pass absolute paths in multi-root workspaces."`
	Packages []string `json:"packages,omitempty" jsonschema:"Packages to get (e.g. example.com/pkg@latest)"`
	Package  string   `json:"package,omitempty" jsonschema:"Single package to get (convenience alias for packages)"`
	Update   bool     `json:"update,omitempty" jsonschema:"If true, adds -u flag to update modules"`
	Args     []string `json:"args,omitempty" jsonschema:"Additional arguments (e.g. -t, -v)"`
}

func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	// Allow single package string as convenience
	if args.Package != "" && len(args.Packages) == 0 {
		args.Packages = []string{args.Package}
	}
	if len(args.Packages) == 0 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "at least one package must be specified (use 'package' for a single package or 'packages' for multiple)"},
			},
		}, nil, nil
	}

	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}

	absDir, valErr := roots.Global.Validate(session, dir)
	if valErr != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: valErr.Error()},
			},
		}, nil, nil
	}

	cmdArgs := []string{"get"}
	if args.Update {
		cmdArgs = append(cmdArgs, "-u")
	}
	cmdArgs = append(cmdArgs, args.Args...)
	cmdArgs = append(cmdArgs, args.Packages...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = absDir

	output, err := cmd.CombinedOutput()
	var sb strings.Builder
	isError := false
	if err != nil {
		isError = true
		fmt.Fprintf(&sb, "go get failed: %v\nOutput:\n%s\n", err, string(output))
	} else {
		fmt.Fprintf(&sb, "Successfully ran 'go get %s'\n", strings.Join(args.Packages, " "))
	}
	// Auto-fetch documentation for each package (even on failure, to provide context)
	for _, pkg := range args.Packages {
		// Strip version suffix if present (e.g., @latest, @v1.2.3)
		pkgPath := strings.Split(pkg, "@")[0]
		if docContent, _ := godoc.GetDocumentationWithFallback(ctx, pkgPath); docContent != "" {
			sb.WriteString("\n")
			sb.WriteString(docContent)
		}
	}
	return &mcp.CallToolResult{
		IsError: isError,
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}
