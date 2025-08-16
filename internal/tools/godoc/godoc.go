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
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

// Register registers the go-doc tool with the server.
func Register(server *mcp.Server, namespace string) {
	name := "godoc"
	if namespace != "" {
		name = namespace + ":" + name
	}
	schema, err := jsonschema.For[GetDocParams]()
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       "Go Documentation",
		Description: "Retrieves documentation for a specified Go package or a specific symbol (like a function or type). This is the primary tool for code comprehension and exploration. Use it to understand a package's public API, function signatures, and purpose before attempting to use or modify it.",
		InputSchema: schema,
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
	err := cmd.Run()

	docString := strings.TrimSpace(out.String())
	if err != nil {
		// If the command fails, it might be because the package doesn't exist.
		// This is a valid result from the tool, not a tool execution error.
		if strings.Contains(docString, "no required module provides package") {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: docString},
				},
			}, nil
		}
		// For other errors, we'll consider it a tool execution error.
		return newErrorResult("`go doc` failed for package %q, symbol %q: %s", pkgPath, symbolName, docString), nil
	}

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
