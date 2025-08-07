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
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MockServerSession provides a mock implementation of the mcp.ServerSession for testing.
type MockServerSession struct {
	// Define any fields needed for your mock session, e.g., to control behavior.
}

func TestGetDocHandler(t *testing.T) {
	ctx := context.Background()
	mockSession := &mcp.ServerSession{} // Adjust as needed for your mock implementation.

	testCases := []struct {
		name        string
		params      *mcp.CallToolParamsFor[GetDocParams]
		wantErr     bool
		wantContent string
	}{
		{
			name: "Standard Library Function",
			params: &mcp.CallToolParamsFor[GetDocParams]{
				Arguments: GetDocParams{
					PackagePath: "fmt",
					SymbolName:  "Println",
				},
			},
			wantErr:     false,
			wantContent: "func Println(a ...any) (n int, err error)",
		},
		{
			name: "Package-Level Documentation",
			params: &mcp.CallToolParamsFor[GetDocParams]{
				Arguments: GetDocParams{
					PackagePath: "os",
				},
			},
			wantErr:     false,
			wantContent: "package os",
		},
		{
			name: "Symbol Not Found",
			params: &mcp.CallToolParamsFor[GetDocParams]{
				Arguments: GetDocParams{
					PackagePath: "fmt",
					SymbolName:  "NonExistentSymbol",
				},
			},
			wantErr:     true, // Expect an error because the symbol doesn't exist.
			wantContent: "no symbol NonExistentSymbol",
		},
		{
			name: "Package Not Found",
			params: &mcp.CallToolParamsFor[GetDocParams]{
				Arguments: GetDocParams{
					PackagePath: "non/existent/package",
				},
			},
			wantErr:     true, // Expect an error because the package doesn't exist.
			wantContent: "is not in std",
		},
		{
			name: "Empty Package Path",
			params: &mcp.CallToolParamsFor[GetDocParams]{
				Arguments: GetDocParams{
					PackagePath: "",
				},
			},
			wantErr:     true,
			wantContent: "package_path cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getDocHandler(ctx, mockSession, tc.params)
			if err != nil {
				t.Fatalf("getDocHandler returned an unexpected error: %v", err)
			}

			if tc.wantErr {
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
				if !contains(textContent.Text, tc.wantContent) {
					t.Errorf("Expected error content to contain %q, but got %q", tc.wantContent, textContent.Text)
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
				if !contains(textContent.Text, tc.wantContent) {
					t.Errorf("Expected content to contain %q, but got %q", tc.wantContent, textContent.Text)
				}
			}
		})
	}
}

// contains is a helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
