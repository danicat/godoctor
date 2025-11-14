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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the get_documentation tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_docs",
		Title:       "Go Documentation",
		Description: "Retrieves Go package or symbol documentation. Use this to understand APIs and function signatures before coding.",
	}, getDocumentationHandler)
}

// GetDocumentationParams defines the input parameters for the get_documentation tool.
type GetDocumentationParams struct {
	PackagePath string `json:"package_path"`
	SymbolName  string `json:"symbol_name,omitempty"`
}

func getDocumentationHandler(ctx context.Context, request *mcp.CallToolRequest, args GetDocumentationParams) (*mcp.CallToolResult, any, error) {
	pkgPath := args.PackagePath
	symbolName := args.SymbolName

	if pkgPath == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "package_path cannot be empty"},
			},
		}, nil, nil
	}

	// Helper function to run go doc
	runGoDoc := func(dir string) (string, error) {
		cmdArgs := []string{"doc"}
		if symbolName == "" {
			cmdArgs = append(cmdArgs, pkgPath)
		} else {
			cmdArgs = append(cmdArgs, "-short", pkgPath, symbolName)
		}
		cmd := exec.CommandContext(ctx, "go", cmdArgs...)
		if dir != "" {
			cmd.Dir = dir
		}
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return strings.TrimSpace(out.String()), err
	}

	docString, err := runGoDoc("")
	if err == nil {
		if docString == "" {
			docString = "documentation not found"
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: docString},
			},
		}, nil, nil
	}

	// If the first attempt failed, check if it's because the package is missing.
	// Common error messages:
	// - "no required module provides package"
	// - "package ... is not in GOROOT"
	// - "cannot find package"
	isMissingPackage := strings.Contains(docString, "no required module provides package") ||
		strings.Contains(docString, "is not in GOROOT") ||
		strings.Contains(docString, "cannot find package")

	if isMissingPackage {
		// Try to fetch the package in a temporary directory
		tempDir, err := os.MkdirTemp("", "godoctor_docs_*")
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("failed to create temp dir for fallback: %v", err)},
				},
			}, nil, nil
		}
		defer os.RemoveAll(tempDir)

		// Initialize a temporary module
		initCmd := exec.CommandContext(ctx, "go", "mod", "init", "temp_docs_fetcher")
		initCmd.Dir = tempDir
		if out, err := initCmd.CombinedOutput(); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("failed to init temp module: %v\nOutput: %s", err, out)},
				},
			}, nil, nil
		}

		// Download the package
		getCmd := exec.CommandContext(ctx, "go", "get", pkgPath)
		getCmd.Dir = tempDir
		if out, err := getCmd.CombinedOutput(); err != nil {
			// If go get fails, we can't retrieve docs. Return the original error or the go get error.
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("failed to download package %q: %v\nOutput: %s\n\nOriginal error: %s", pkgPath, err, out, docString)},
				},
			}, nil, nil
		}

		// Try go doc again in the temp dir
		docString, err = runGoDoc(tempDir)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("`go doc` failed for package %q (after download): %s", pkgPath, docString)},
				},
			}, nil, nil
		}

		if docString == "" {
			docString = "documentation not found"
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: docString},
			},
		}, nil, nil
	}

	// If it wasn't a missing package error, or we couldn't recover, return the original error.
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("`go doc` failed for package %q, symbol %q: %s", pkgPath, symbolName, docString)},
		},
	}, nil, nil
}