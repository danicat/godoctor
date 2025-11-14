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

package getdocs

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandler(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		params      Params
		wantErr     bool
		wantContent string
	}{
		{
			name: "Standard Library Function",
			params: Params{
				PackagePath: "fmt",
				SymbolName:  "Println",
			},
			wantErr:     false,
			wantContent: "func Println(a ...any) (n int, err error)",
		},
		{
			name: "Package-Level Documentation",
			params: Params{
				PackagePath: "os",
			},
			wantErr:     false,
			wantContent: "package os",
		},
		{
			name: "Symbol Not Found",
			params: Params{
				PackagePath: "fmt",
				SymbolName:  "NonExistentSymbol",
			},
			wantErr:     true, // Expect an error because the symbol doesn't exist.
			wantContent: "no symbol NonExistentSymbol",
		},
		{
			name: "Package Not Found",
			params: Params{
				PackagePath: "non/existent/package",
			},
			wantErr:     true, // Expect an error because the package doesn't exist.
			wantContent: "is not in std",
		},
		{
			name: "Empty Package Path",
			params: Params{
				PackagePath: "",
			},
			wantErr:     true,
			wantContent: "package_path cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _, err := Handler(ctx, nil, tc.params)
			if err != nil {
				t.Fatalf("Handler returned an unexpected error: %v", err)
			}
			verifyResult(t, result, tc.wantErr, tc.wantContent)
		})
	}
}

func verifyResult(t *testing.T, result *mcp.CallToolResult, wantErr bool, wantContent string) {
	t.Helper()
	if wantErr {
		if !result.IsError {
			t.Errorf("Expected an error, but got none.")
		}
		if len(result.Content) == 0 {
			t.Fatal("Expected error content, but got none.")
		}
		textContent, ok := result.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatal("Expected TextContent, but got a different type.")
		}
		if !strings.Contains(textContent.Text, wantContent) {
			t.Errorf("Expected error content to contain %q, but got %q", wantContent, textContent.Text)
		}
	} else {
		if result.IsError {
			t.Errorf("Did not expect an error, but got one: %v", result.Content)
		}
		if len(result.Content) == 0 {
			t.Fatal("Expected content, but got none.")
		}
		textContent, ok := result.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatal("Expected TextContent, but got a different type.")
		}
		if !strings.Contains(textContent.Text, wantContent) {
			t.Errorf("Expected content to contain %q, but got %q", wantContent, textContent.Text)
		}
	}
}