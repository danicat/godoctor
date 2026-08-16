// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package readdocs_test

import (
	"context"
	"strings"
	"testing"

	readdocs "github.com/danicat/godoctor/internal/tools/read_docs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolHandler(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		params      readdocs.Params
		wantErr     bool
		wantContent string
	}{
		{
			name:        "Standard Library Function (Markdown Check)",
			params:      readdocs.Params{ImportPath: "fmt", SymbolName: "Println"},
			wantErr:     false,
			wantContent: "```go\nfunc Println", // Verifies Markdown code block start
		},
		{
			name:        "Fuzzy Match Symbol",
			params:      readdocs.Params{ImportPath: "fmt", SymbolName: "Pritln"}, // Typo
			wantErr:     true,
			wantContent: "Println", // Expect the correct suggestion to appear in the error
		},
		{
			name:        "Package-Level Documentation",
			params:      readdocs.Params{ImportPath: "os"},
			wantErr:     false,
			wantContent: "# os", // Markdown header
		},
		{
			name:        "Symbol Not Found",
			params:      readdocs.Params{ImportPath: "fmt", SymbolName: "NonExistentSymbol"},
			wantErr:     true,
			wantContent: "symbol \"NonExistentSymbol\" not found in package fmt",
		},
		{
			name:        "Package Not Found",
			params:      readdocs.Params{ImportPath: "non/existent/package"},
			wantErr:     true,
			wantContent: "failed to download package",
		},
		{
			name:        "Empty Package Path",
			params:      readdocs.Params{ImportPath: ""},
			wantErr:     true,
			wantContent: "import_path cannot be empty",
		},
		{
			name:        "JSON Format Output",
			params:      readdocs.Params{ImportPath: "fmt", SymbolName: "Println", Format: "json"},
			wantErr:     false,
			wantContent: `"package": "fmt"`,
		},
		{
			name:        "Invalid Format",
			params:      readdocs.Params{ImportPath: "fmt", Format: "yaml"},
			wantErr:     true,
			wantContent: "invalid format: must be 'markdown' or 'json'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _, err := readdocs.Handler(ctx, nil, tc.params)
			if err != nil {
				t.Fatalf("ToolHandler returned an unexpected error: %v", err)
			}
			verifyResult(t, result, tc.wantErr, tc.wantContent)
		})
	}
}

func verifyResult(t *testing.T, result *mcp.CallToolResult, wantErr bool, wantContent string) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("Expected content, but got none.")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent, but got a different type.")
	}

	if wantErr {
		if !result.IsError {
			t.Errorf("Expected an error, but got none.")
		}
		if !strings.Contains(textContent.Text, wantContent) {
			t.Errorf("Expected error content to contain %q, but got %q", wantContent, textContent.Text)
		}
	} else {
		if result.IsError {
			t.Errorf("Did not expect an error, but got one: %v", result.Content)
		}
		if !strings.Contains(textContent.Text, wantContent) {
			t.Errorf("Expected content to contain %q, but got %q", wantContent, textContent.Text)
		}
	}
}
