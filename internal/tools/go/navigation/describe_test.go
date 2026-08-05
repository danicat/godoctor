package navigation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDescribeSymbol(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	src := `package main

import "fmt"

type User struct {
	Name string
}

func (u *User) String() string {
	return u.Name
}

func main() {
	u := &User{Name: "Alice"}
	fmt.Println(u.String())
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}

	res, _, err := Handler(context.Background(), nil, Params{
		Filename: srcFile,
		Line:     5,
		Col:      6,
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("Handler returned error: %v", res.Content)
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "User") {
		t.Errorf("Expected output to contain 'User', got:\n%s", text)
	}
	if !strings.Contains(text, "Workspace References") {
		t.Errorf("Expected output to contain 'Workspace References', got:\n%s", text)
	}
}
