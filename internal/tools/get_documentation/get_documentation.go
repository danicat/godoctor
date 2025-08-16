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

package get_documentation

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/danicat/godoctor/internal/mcp/result"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the get_documentation tool with the server.
func Register(server *mcp.Server, namespace string) {
	name := "get_documentation"
	if namespace != "" {
		name = namespace + ":" + name
	}
	schema, err := jsonschema.For[GetDocumentationParams]()
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       "Go Documentation",
		Description: "Retrieves documentation for a specified Go package or a specific symbol (like a function or type). This is the primary tool for code comprehension and exploration. Use it to understand a package's public API, function signatures, and purpose before attempting to use or modify it.",
		InputSchema: schema,
	}, getDocumentationHandler)
}

// GetDocumentationParams defines the input parameters for the get_documentation tool.
type GetDocumentationParams struct {
	PackagePath string `json:"package_path"`
	SymbolName  string `json:"symbol_name,omitempty"`
}

func getDocumentationHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[GetDocumentationParams]) (*mcp.CallToolResult, error) {
	pkgPath := request.Arguments.PackagePath
	symbolName := request.Arguments.SymbolName

	if pkgPath == "" {
		return result.NewError("package_path cannot be empty"), nil
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
			return result.NewText(docString), nil
		}
		// For other errors, we'll consider it a tool execution error.
		return result.NewError("`go doc` failed for package %q, symbol %q: %s", pkgPath, symbolName, docString), nil
	}

	if docString == "" {
		docString = "documentation not found"
	}

	return result.NewText(docString), nil
}
