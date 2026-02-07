// Package modernize implements the modernize tool.
package modernize

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["modernize_code"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, toolHandler)
}

// Params defines the input parameters.
type Params struct {
	Dir string `json:"dir,omitempty" jsonschema:"Directory to run analysis in (default: current)"`
	Fix bool   `json:"fix,omitempty" jsonschema:"Whether to automatically apply the suggested fixes (default: false)"`
}

func toolHandler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}
	
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to resolve absolute path: %v", err)), nil, nil
	}

	toolPath := "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest"
	
	// 1. Always run in check mode first to identify what needs fixing
	checkCmd := exec.CommandContext(ctx, "go", "run", toolPath, "./...")
	checkCmd.Dir = absDir
	checkOut, checkErr := checkCmd.CombinedOutput()
	diagnostics := string(checkOut)

	// If check failed with a non-exit-code error, fail immediately
	if checkErr != nil && diagnostics == "" {
		return errorResult(fmt.Sprintf("modernize check failed to run: %v", checkErr)), nil, nil
	}

	// Clean up diagnostics string: remove trailing exit status messages
	diagnostics = strings.TrimSpace(diagnostics)
	if strings.Contains(diagnostics, "exit status") {
		lines := strings.Split(diagnostics, "\n")
		var filtered []string
		for _, line := range lines {
			if !strings.Contains(line, "exit status") {
				filtered = append(filtered, line)
			}
		}
		diagnostics = strings.Join(filtered, "\n")
	}
	diagnostics = strings.TrimSpace(diagnostics)

	// 2. If nothing to fix, we're done
	if diagnostics == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "✅ No modernization issues found."},
			},
		}, nil, nil
	}

	// 3. If user requested fix, apply it
	if args.Fix {
		fixCmd := exec.CommandContext(ctx, "go", "run", toolPath, "-fix", "./...")
		fixCmd.Dir = absDir
		fixOut, fixErr := fixCmd.CombinedOutput()
		
		if fixErr != nil {
			// If fix failed, report the error and the output
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("⚠️ Modernization fix attempted but encountered errors:\n\n%s", string(fixOut))},
				},
			}, nil, nil
		}

		// Success! Report what was fixed (based on the check we ran earlier)
		report := fmt.Sprintf("⚠️ Found modernization opportunities:\n\n%s\n\n✅ Automatically applied modernization fixes.", diagnostics)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: report},
			},
		}, nil, nil
	}

	// 4. Check mode only - report findings
	report := fmt.Sprintf("⚠️ Found modernization opportunities:\n\n%s", diagnostics)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: report},
		},
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