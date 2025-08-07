// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package godoc

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the go-doc tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "godoc",
		Description: "Retrieves Go documentation for a specified package and, optionally, a specific symbol within that package. This tool is useful for understanding the functionality of a Go package or a specific symbol (function, type, etc.) within it.",
	}, getDocHandler)
}

// GetDocParams defines the input parameters for the go-doc tool.
type GetDocParams struct {
	PackagePath string `json:"package_path"`
	SymbolName  string `json:"symbol_name,omitempty"`
}

func getDocHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[GetDocParams]) (*mcp.CallToolResult, error) {
	pkgPath := request.Arguments.PackagePath
	symbolName := request.Arguments.SymbolName

	if pkgPath == "" {
		return newErrorResult("package_path cannot be empty"), nil
	}

	args := []string{"doc"}
	if symbolName == "" {
		args = append(args, pkgPath)
	} else {
		args = append(args, "-short", pkgPath, symbolName)
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return newErrorResult("`go doc` failed for package %q, symbol %q: %s", pkgPath, symbolName, out.String()), nil
	}

	docString := strings.TrimSpace(out.String())
	if docString == "" {
		docString = "documentation not found"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: docString},
		},
	}, nil
}

func newErrorResult(format string, a ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, a...)},
		},
	}
}
