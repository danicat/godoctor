package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupTestSession(t *testing.T, env ...string) *mcp.ClientSession {
	t.Helper()

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "godoctor_test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build server binary: %v", err)
	}

	runCmd := exec.Command(binaryPath)
	runCmd.Env = append(os.Environ(), env...)
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
		symbol            string
		env               []string
		expectError       bool
		expectedSubstring string
	}{
		{
			name:              "std library symbol",
			symbol:            "fmt.Println",
			expectError:       false,
			expectedSubstring: "Println formats using the default formats",
		},
		{
			name:              "third party symbol",
			symbol:            "github.com/modelcontextprotocol/go-sdk/mcp.NewClient",
			expectError:       false,
			expectedSubstring: "NewClient creates a new",
		},
		{
			name:              "package not found",
			symbol:            "nonexistent/pkg.Symbol",
			expectError:       true,
			expectedSubstring: "could not find package",
		},
		{
			name:              "custom goroot",
			symbol:            "fmt.Println",
			env:               []string{"GO_DOCTOR_GOROOT=/nonexistent"},
			expectError:       true,
			expectedSubstring: `could not find package "fmt"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := setupTestSession(t, tc.env...)
			params := &mcp.CallToolParams{
				Name: "getDoc",
				Arguments: map[string]any{
					"symbol": tc.symbol,
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
					t.Errorf("tool returned an error: %v", res.Content)
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
