// Package adddependencies implements the add_dependencies tool.
package adddependencies

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/godoc"
	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_dependencies",
		Title:       "Add Dependencies",
		Description: "Manages Go module installation, initialization, and manifest updates. Consolidates the workflow by automatically initializing go.mod if absent and returning public API documentation for installed packages.",
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	//nolint:lll
	Dir        string   `json:"dir,omitempty" jsonschema:"The absolute directory path to run in. Always pass absolute paths in multi-root workspaces."`
	Packages   []string `json:"packages,omitempty" jsonschema:"Packages to get (e.g. example.com/pkg@latest)"`
	Package    string   `json:"package,omitempty" jsonschema:"Single package to get (convenience alias for packages)"`
	ModuleName string   `json:"module_name,omitempty" jsonschema:"Optional module name override when initializing go.mod"`
	Update     bool     `json:"update,omitempty" jsonschema:"If true, adds -u flag to update modules"`
	Args       []string `json:"args,omitempty" jsonschema:"Additional arguments (e.g. -t, -v)"`
}

// Handler executes the add_dependencies tool.
func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if args.Package != "" && len(args.Packages) == 0 {
		args.Packages = []string{args.Package}
	}

	return executeGoGet(ctx, req, args)
}

//nolint:funlen
func executeGoGet(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	absDir, valErr := filepath.Abs(dir)
	if valErr != nil {
		return nil, nil, valErr
	}

	var sb strings.Builder

	// Check if go.mod exists in target directory
	goModPath := filepath.Join(absDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		modName := args.ModuleName
		if strings.TrimSpace(modName) == "" {
			modName = filepath.Base(absDir)
		}
		initCmd, initErr := safeshell.CommandContext(ctx, "go", "mod", "init", modName)
		if initErr != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("failed to create go mod init command: %v", initErr)},
				},
			}, nil, nil
		}
		initCmd.Dir = absDir
		initOut, initErr := initCmd.CombinedOutput()
		if initErr != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("go mod init failed: %v\nOutput: %s", initErr, string(initOut))},
				},
			}, nil, nil
		}
		fmt.Fprintf(&sb, "Successfully initialized Go module '%s' at %s\n", modName, absDir)
	}

	if len(args.Packages) == 0 {
		if sb.Len() == 0 {
			sb.WriteString("No packages specified and go.mod is already initialized.\n")
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: sb.String()},
			},
		}, nil, nil
	}

	cmdArgs := []string{"get"}
	if args.Update {
		cmdArgs = append(cmdArgs, "-u")
	}
	cmdArgs = append(cmdArgs, args.Args...)
	cmdArgs = append(cmdArgs, args.Packages...)
	cmd, err := safeshell.CommandContext(ctx, "go", cmdArgs...)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("secure execution validation failed: %v", err)},
			},
		}, nil, nil
	}
	cmd.Dir = absDir

	output, err := cmd.CombinedOutput()
	isError := false
	if err != nil {
		isError = true
		fmt.Fprintf(&sb, "go get failed: %v\nOutput:\n%s\n", err, string(output))
	} else {
		fmt.Fprintf(&sb, "Successfully ran 'go get %s'\n", strings.Join(args.Packages, " "))
	}
	appendPackageDocs(ctx, args.Packages, &sb)

	return &mcp.CallToolResult{
		IsError: isError,
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}

func appendPackageDocs(ctx context.Context, packages []string, sb *strings.Builder) {
	for _, pkg := range packages {
		pkgPath, _, _ := strings.Cut(pkg, "@")
		if docContent, _ := godoc.GetDocumentationWithFallback(ctx, pkgPath); docContent != "" {
			sb.WriteString("\n")
			sb.WriteString(docContent)
		}
	}
}
