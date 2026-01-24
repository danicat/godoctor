// Package build implements the go build tool.
package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/danicat/godoctor/internal/tools/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["verify_build"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Dir      string   `json:"dir,omitempty" jsonschema:"Directory to build in (default: current)"`
	Packages []string `json:"packages,omitempty" jsonschema:"Packages to build (default: ./...)"`
	Args     []string `json:"args,omitempty" jsonschema:"Additional build arguments (e.g. -o, -race, -tags, -v)"`
}

func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}
	pkgs := args.Packages
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}

	cmdArgs := append([]string{"build"}, args.Args...)
	cmdArgs = append(cmdArgs, pkgs...)

	//nolint:gosec // G204: Subprocess launched with variable is expected behavior for this tool.
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	output := string(out)

	var sb strings.Builder
	if err != nil {
		sb.WriteString("## 🔴 Build Failed\n\n")

		// Parse errors into a list
		lines := strings.Split(output, "\n")
		var buildErrors []string
		
		// To avoid spamming context, we only show snippets for the first 3 unique files
		snippetCount := 0

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			
			// Look for typical Go error format: "file:line:col: message" or "file:line: message"
			if strings.Contains(line, ".go:") {
				buildErrors = append(buildErrors, "- ⚠️ "+line)
				
				// Extract Snippet?
				if snippetCount < 3 {
					parts := strings.Split(line, ":")
					if len(parts) >= 3 {
						file := parts[0]
						lineNumStr := parts[1]
						
						// Basic validation that it is a file path
						if strings.HasSuffix(file, ".go") {
							// Resolve file path relative to build dir
							absPath := file
							if !filepath.IsAbs(file) {
								absPath = filepath.Join(dir, file)
							}
							
							// Parse line number
							if lineNum, err := strconv.Atoi(lineNumStr); err == nil {
								// Read snippet
								if content, err := os.ReadFile(absPath); err == nil {
									snippet := shared.GetSnippet(string(content), lineNum)
									if snippet != "" {
										buildErrors = append(buildErrors, fmt.Sprintf("  ```go\n%s  ```", snippet))
										snippetCount++
									}
								}
							}
						}
					}
				}
			}
		}

		if len(buildErrors) > 0 {
			sb.WriteString("### Analysis (Problems)\n")
			for _, e := range buildErrors {
				sb.WriteString(e + "\n")
			}
			sb.WriteString("\n")
		} else {
			// Fallback for non-parseable errors (e.g. linker errors)
			sb.WriteString("### Error Details\n```text\n")
			sb.WriteString(output)
			sb.WriteString("\n```\n")
		}

		sb.WriteString(shared.GetDocHintFromOutput(output))
	} else {
		sb.WriteString("## 🟢 Build Successful\n")
		if output != "" {
			sb.WriteString("\n```text\n")
			sb.WriteString(output)
			sb.WriteString("\n```\n")
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}
