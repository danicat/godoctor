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

// Package getdocs implements the documentation retrieval tool.
package getdocs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	// ErrPackageMissing indicates that the requested package could not be found locally.
	ErrPackageMissing = errors.New("package missing")
)

// Register registers the get_docs tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "get_docs",
		Title: "Go Documentation",
		Description: "Retrieves Go package or symbol documentation. " +
			"Use this to understand APIs and function signatures before coding.",
	}, Handler)
}

// Params defines the input parameters for the get_docs tool.
type Params struct {
	PackagePath string `json:"package_path"`
	SymbolName  string `json:"symbol_name,omitempty"`
}

// Handler handles the get_docs tool execution.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
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

	docString, err := runGoDoc(ctx, pkgPath, symbolName, "")
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

	// Check if the error is due to a missing package
	if errors.Is(classifyError(docString), ErrPackageMissing) {
		return fetchAndRetry(ctx, pkgPath, symbolName, docString)
	}

	// If it wasn't a missing package error, or we couldn't recover, return the original error.
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("`go doc` failed for package %q, symbol %q: %s",
				pkgPath, symbolName, docString)},
		},
	}, nil, nil
}

func classifyError(output string) error {
	if strings.Contains(output, "no required module provides package") ||
		strings.Contains(output, "is not in GOROOT") ||
		strings.Contains(output, "cannot find package") {
		return ErrPackageMissing
	}
	return nil
}

func runGoDoc(ctx context.Context, pkgPath, symbolName, dir string) (string, error) {
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

func fetchAndRetry(ctx context.Context, pkgPath, symbolName, originalErr string) (*mcp.CallToolResult, any, error) {
	tempDir, err := os.MkdirTemp("", "godoctor_docs_*")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to create temp dir for fallback: %v", err)},
			},
		}, nil, nil
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

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
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to download package %q: %v\nOutput: %s\n\nOriginal error: %s",
					pkgPath, err, out, originalErr)},
			},
		}, nil, nil
	}

	// Try go doc again in the temp dir
	docString, err := runGoDoc(ctx, pkgPath, symbolName, tempDir)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("`go doc` failed for package %q (after download): %s",
					pkgPath, docString)},
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
