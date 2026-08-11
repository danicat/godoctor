package smartread

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReadCodeTool_SingleFileAndEnrichment(t *testing.T) {
	tmpDir := t.TempDir()

	modPath := filepath.Join(tmpDir, "go.mod")
	modData := []byte("module example.com/test\ngo 1.21\n")
	if err := os.WriteFile(modPath, modData, 0600); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(tmpDir, "main.go")
	src := `package main

import (
	"fmt"
)

type MyStruct struct {
	Name string
}

func (s *MyStruct) Greet() string {
	return "Hello " + s.Name
}

func main() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}

	roots.Global.Add(nil, tmpDir)

	res, _, err := readCodeHandler(context.Background(), nil, Params{Filenames: []string{srcFile}})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if res.IsError {
		t.Errorf("tool returned error: %v", res.Content)
	}

	if len(res.Content) == 0 {
		t.Fatal("no content returned")
	}

	output := res.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(output, "MyStruct") {
		t.Errorf("expected MyStruct in output, got: %s", output)
	}

	if !strings.Contains(output, "<types>") {
		t.Errorf("expected <types> enrichment block, got: %s", output)
	}
}

func TestReadCodeTool_MultiFile(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "a.go")
	file2 := filepath.Join(tmpDir, "b.go")

	if err := os.WriteFile(file1, []byte("package main\ntype TypeA struct{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("package main\ntype TypeB struct{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	roots.Global.Add(nil, tmpDir)

	res, _, err := readCodeHandler(context.Background(), nil, Params{
		Filenames: []string{file1, file2},
	})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}

	output := res.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(output, "TypeA") || !strings.Contains(output, "TypeB") {
		t.Errorf("expected both TypeA and TypeB in output, got: %s", output)
	}
}

func TestReadCodeTool_OutlineMode(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "outline.go")
	src := `package main

import "net/http"

type User struct {
	ID string
}

func ProcessUser(u User) {}
`
	if err := os.WriteFile(srcFile, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}

	roots.Global.Add(nil, tmpDir)

	res, _, err := readCodeHandler(context.Background(), nil, Params{
		Filenames: []string{srcFile},
		Outline:   true,
	})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	output := res.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(output, "Outline") {
		t.Errorf("expected outline header in output, got: %s", output)
	}
	if !strings.Contains(output, "User") {
		t.Errorf("expected User symbol in outline, got: %s", output)
	}
}

func TestReadCodeTool_DeprecatedSingleFilenameFallback(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "single.go")
	if err := os.WriteFile(srcFile, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}

	roots.Global.Add(nil, tmpDir)

	res, _, err := readCodeHandler(context.Background(), nil, Params{
		Filename: srcFile,
	})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if res.IsError {
		t.Errorf("expected success with deprecated Filename argument, got error: %v", res.Content)
	}
}

func TestReadCodeTool_Errors(t *testing.T) {
	// No filenames specified
	res, _, _ := readCodeHandler(context.Background(), nil, Params{})
	if !res.IsError {
		t.Error("expected error when no filenames are specified")
	}

	// Line range error
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "short.go")
	_ = os.WriteFile(srcFile, []byte("line 1\n"), 0600)
	roots.Global.Add(nil, tmpDir)

	resErr, _, _ := readCodeHandler(context.Background(), nil, Params{
		Filenames: []string{srcFile},
		StartLine: 100,
	})
	if !resErr.IsError {
		t.Error("expected error when start_line is beyond file length")
	}
}

func TestReadCodeTool_Partial(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "partial.go")
	src := `line 1
line 2
line 3
line 4
line 5`
	if err := os.WriteFile(srcFile, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}

	roots.Global.Add(nil, tmpDir)

	res, _, err := readCodeHandler(context.Background(), nil, Params{
		Filenames: []string{srcFile},
		StartLine: 2,
		EndLine:   4,
	})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	text := res.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(text, "   2 | line 2") {
		t.Errorf("expected line 2, got: %s", text)
	}
	if !strings.Contains(text, "   4 | line 4") {
		t.Errorf("expected line 4, got: %s", text)
	}
	if strings.Contains(text, "   1 | line 1") {
		t.Errorf("did not expect line 1, got: %s", text)
	}
	if strings.Contains(text, "   5 | line 5") {
		t.Errorf("did not expect line 5, got: %s", text)
	}
}
