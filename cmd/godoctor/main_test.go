package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupTestSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "godoctor_test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build server binary: %v", err)
	}

	runCmd := exec.Command(binaryPath)
	transport := mcp.NewCommandTransport(runCmd)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(context.Background(), transport)
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	t.Cleanup(func() {
		session.Close()
	})

	return session
}

func TestGetDocTool(t *testing.T) {
	testCases := []struct {
		name              string
		pkgPath           string
		symbolName        string
		expectError       bool
		expectedSubstring string
	}{
		{
			name:              "std library symbol",
			pkgPath:           "fmt",
			symbolName:        "Println",
			expectError:       false,
			expectedSubstring: "func Println(a ...any)",
		},
		{
			name:              "std library package",
			pkgPath:           "fmt",
			symbolName:        "",
			expectError:       false,
			expectedSubstring: "Package fmt implements formatted I/O",
		},
		{
			name:              "third party symbol",
			pkgPath:           "github.com/google/generative-ai-go/genai",
			symbolName:        "NewClient",
			expectError:       false,
			expectedSubstring: "func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error)",
		},
		{
			name:              "package not found",
			pkgPath:           "nonexistent/pkg",
			symbolName:        "Symbol",
			expectError:       true,
			expectedSubstring: "no such package",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := setupTestSession(t)
			params := &mcp.CallToolParams{
				Name: "go-doc",
				Arguments: map[string]any{
					"package_path": tc.pkgPath,
					"symbol_name":  tc.symbolName,
				},
			}
			res, err := session.CallTool(context.Background(), params)
			if err != nil {
				t.Errorf("CallTool failed: %v", err)
				return
			}

			if tc.expectError {
				if !res.IsError {
					t.Error("expected an error, but got none")
					return
				}
				textContent, ok := res.Content[0].(*mcp.TextContent)
				if !ok {
					t.Errorf("expected TextContent, got %T", res.Content[0])
					return
				}
				if !strings.Contains(textContent.Text, tc.expectedSubstring) {
					t.Errorf("expected error to contain '%s', got '%s'", tc.expectedSubstring, textContent.Text)
				}
			} else {
				if res.IsError {
					t.Errorf("tool returned an error: %+v", res.Content)
					if textContent, ok := res.Content[0].(*mcp.TextContent); ok {
						t.Logf("Error content: %s", textContent.Text)
					}
					return
				}
				textContent, ok := res.Content[0].(*mcp.TextContent)
				if !ok {
					t.Errorf("expected TextContent, got %T", res.Content[0])
					return
				}
				if !strings.Contains(textContent.Text, tc.expectedSubstring) {
					t.Errorf("expected doc to contain '%s', got '%s'", tc.expectedSubstring, textContent.Text)
				}
			}
		})
	}
}