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

package gopretty

import (
	"context"
	"fmt"
	"go/format"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the gopretty tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gopretty",
		Description: "Formats a Go source file using goimports and gofmt. This tool is useful for ensuring that your code adheres to Go's formatting standards.",
	}, goprettyHandler)
}

// GoPrettyParams defines the input parameters for the gopretty tool.
type GoPrettyParams struct {
	FilePath string `json:"file_path"`
}

func goprettyHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[GoPrettyParams]) (*mcp.CallToolResult, error) {
	if request == nil {
		return nil, fmt.Errorf("gopretty request cannot be nil")
	}
	filePath := request.Arguments.FilePath
	if filePath == "" {
		return nil, fmt.Errorf("file_path cannot be empty")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %q: %w", filePath, err)
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", filePath, err)
	}

	importedSrc, err := imports.Process(filePath, src, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to process imports for file %q: %w", filePath, err)
	}

	formattedSrc, err := format.Source(importedSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to format file %q: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, formattedSrc, info.Mode()); err != nil {
		return nil, fmt.Errorf("failed to write file %q: %w", filePath, err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "file formatted successfully"},
		},
	}, nil
}
