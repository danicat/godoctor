package smartedit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExtractASTSymbols_Declarations(t *testing.T) {
	tmpDir := t.TempDir()
	sourceCode := `package testpkg

const (
	MyConst1 = 1
	MyConst2 = 2
)

const SingleConst = "hello"

var (
	VarA, VarB = 10, 20
)

var SingleVar = true

type MyType int

type Bar struct {
	FieldX    string
	FieldY    int
	unexported bool
}

type AnonymousContainer struct {
	Bar
}

type Greeter interface {
	Greet(name string) string
}

func Foo() {}

func (b Bar) MethodOnBar() {}
`
	filePath := filepath.Join(tmpDir, "sample.go")
	if err := os.WriteFile(filePath, []byte(sourceCode), 0600); err != nil {
		t.Fatalf("failed to write sample Go file: %v", err)
	}

	symbols := extractASTSymbols(filePath)

	expectedSymbols := []string{
		"MyConst1", "MyConst2", "SingleConst",
		"VarA", "VarB", "SingleVar",
		"MyType", "Bar", "FieldX", "FieldY", "unexported",
		"AnonymousContainer",
		"Greeter", "Greet", "name",
		"Foo", "MethodOnBar",
	}

	for _, expected := range expectedSymbols {
		if !slices.Contains(symbols, expected) {
			t.Errorf("expected symbol %q was not found in extracted symbols: %v", expected, symbols)
		}
	}
}

func TestExtractASTSymbols_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "a.go")
	fileAContent := "package testpkg\n\nfunc FuncA() {}\nvar SharedVar = 1\n"
	if err := os.WriteFile(fileA, []byte(fileAContent), 0600); err != nil {
		t.Fatalf("failed to write a.go: %v", err)
	}

	fileB := filepath.Join(tmpDir, "b.go")
	fileBContent := "package testpkg\n\nfunc FuncB() {}\nvar SharedVar = 2\n"
	if err := os.WriteFile(fileB, []byte(fileBContent), 0600); err != nil {
		t.Fatalf("failed to write b.go: %v", err)
	}

	symbols := extractASTSymbols(fileA)

	if !slices.Contains(symbols, "FuncA") {
		t.Errorf("expected FuncA in symbols: %v", symbols)
	}
	if !slices.Contains(symbols, "FuncB") {
		t.Errorf("expected FuncB in symbols: %v", symbols)
	}
	if !slices.Contains(symbols, "SharedVar") {
		t.Errorf("expected SharedVar in symbols: %v", symbols)
	}

	// Ensure deduplication
	count := 0
	for _, sym := range symbols {
		if sym == "SharedVar" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected SharedVar to appear exactly once, got %d occurrences", count)
	}
}

func TestExtractASTSymbols_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("empty directory", func(t *testing.T) {
		dummyPath := filepath.Join(tmpDir, "dummy.go")
		symbols := extractASTSymbols(dummyPath)
		if len(symbols) != 0 {
			t.Errorf("expected empty symbols for empty directory, got %v", symbols)
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		nonExistentPath := filepath.Join(t.TempDir(), "nonexistent_dir", "file.go")
		symbols := extractASTSymbols(nonExistentPath)
		if symbols != nil {
			t.Errorf("expected nil symbols for non-existent directory, got %v", symbols)
		}
	})

	t.Run("syntax errors handled gracefully", func(t *testing.T) {
		brokenCode := "package broken\n\nfunc BrokenFunc( {\ninvalid syntax\n"
		brokenPath := filepath.Join(tmpDir, "broken.go")
		if err := os.WriteFile(brokenPath, []byte(brokenCode), 0600); err != nil {
			t.Fatalf("failed to write broken.go: %v", err)
		}
		_ = extractASTSymbols(brokenPath)
	})

	t.Run("non Go files only", func(t *testing.T) {
		txtPath := filepath.Join(tmpDir, "notes.txt")
		if err := os.WriteFile(txtPath, []byte("some text"), 0600); err != nil {
			t.Fatalf("failed to write notes.txt: %v", err)
		}
		symbols := extractASTSymbols(filepath.Join(tmpDir, "anything.go"))
		if len(symbols) != 0 {
			t.Errorf("expected empty symbols for dir without Go files, got %v", symbols)
		}
	})
}

func TestFindClosestSymbol_Matching(t *testing.T) {
	t.Run("exact match exclusion", func(t *testing.T) {
		known := []string{"Foo", "Bar", "Baz"}
		bestSym, bestDist := findClosestSymbol("Foo", known)
		if bestSym == "Foo" {
			t.Errorf("exact match should be excluded, got %q", bestSym)
		}
		if bestDist != 3 {
			t.Errorf("expected best distance 3, got %d", bestDist)
		}
	})

	t.Run("fuzzy matching on typos", func(t *testing.T) {
		tests := []struct {
			bad          string
			known        []string
			expectedSym  string
			expectedDist int
		}{
			{
				bad:          "MyFuncton",
				known:        []string{"OtherFunction", "MyFunction", "Helper"},
				expectedSym:  "MyFunction",
				expectedDist: 1,
			},
			{
				bad:          "val",
				known:        []string{"Val", "Value", "Variable"},
				expectedSym:  "Val",
				expectedDist: 0,
			},
			{
				bad:          "ProccessData",
				known:        []string{"ProcessData", "CalculateSum"},
				expectedSym:  "ProcessData",
				expectedDist: 1,
			},
		}

		for _, tc := range tests {
			sym, dist := findClosestSymbol(tc.bad, tc.known)
			if sym != tc.expectedSym {
				t.Errorf("for bad=%q: expected symbol %q, got %q", tc.bad, tc.expectedSym, sym)
			}
			if dist != tc.expectedDist {
				t.Errorf("for bad=%q: expected distance %d, got %d", tc.bad, tc.expectedDist, dist)
			}
		}
	})
}

func TestFindClosestSymbol_EdgeCases(t *testing.T) {
	t.Run("case insensitivity", func(t *testing.T) {
		known := []string{"MyFunction", "CalculateSum"}
		sym, dist := findClosestSymbol("myfunction", known)
		if sym != "MyFunction" {
			t.Errorf("expected %q, got %q", "MyFunction", sym)
		}
		if dist != 0 {
			t.Errorf("expected distance 0 for case difference, got %d", dist)
		}
	})

	t.Run("empty known list", func(t *testing.T) {
		sym, dist := findClosestSymbol("Target", nil)
		if sym != "" || dist != 999 {
			t.Errorf("expected ('', 999) for nil known list, got (%q, %d)", sym, dist)
		}

		sym, dist = findClosestSymbol("Target", []string{})
		if sym != "" || dist != 999 {
			t.Errorf("expected ('', 999) for empty known list, got (%q, %d)", sym, dist)
		}
	})

	t.Run("large distance", func(t *testing.T) {
		known := []string{"Zebra", "Elephant"}
		sym, dist := findClosestSymbol("Alpha", known)
		if dist < 4 {
			t.Errorf("expected large distance for unrelated strings, got %d for %q", dist, sym)
		}
	})
}

func setupTestWorkspace(t *testing.T) string {
	tmpDir := t.TempDir()
	source := `package testpkg

type S struct {
	GoodField string
}

func GoodSym() {}
func HandleRequest() {}
`
	filePath := filepath.Join(tmpDir, "file.go")
	if err := os.WriteFile(filePath, []byte(source), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return filePath
}

func TestFindSuggestions_Patterns(t *testing.T) {
	ctx := context.Background()

	t.Run("undeclared name pattern", func(t *testing.T) {
		filePath := setupTestWorkspace(t)
		errMsg := filePath + ":10:5: undeclared name: GoodSim"

		suggestion := findSuggestions(ctx, errMsg)
		expected := "- In file.go: Did you mean 'GoodSym' instead of 'GoodSim'?"
		if !strings.Contains(suggestion, expected) {
			t.Errorf("expected suggestion to contain %q, got: %s", expected, suggestion)
		}
		if !strings.Contains(suggestion, "💡 **Suggestions:**") {
			t.Errorf("expected suggestion header, got: %s", suggestion)
		}
	})

	t.Run("undefined pattern", func(t *testing.T) {
		filePath := setupTestWorkspace(t)
		errMsg := filePath + ":12:8: HandleReqest undefined"

		suggestion := findSuggestions(ctx, errMsg)
		expected := "- In file.go: Did you mean 'HandleRequest' instead of 'HandleReqest'?"
		if !strings.Contains(suggestion, expected) {
			t.Errorf("expected suggestion to contain %q, got: %s", expected, suggestion)
		}
	})

	t.Run("no field or method pattern", func(t *testing.T) {
		filePath := setupTestWorkspace(t)
		errMsg := filePath + ":15:3: type S has no field or method GoodFeild"

		suggestion := findSuggestions(ctx, errMsg)
		expected := "- In file.go: Did you mean 'GoodField' instead of 'GoodFeild'?"
		if !strings.Contains(suggestion, expected) {
			t.Errorf("expected suggestion to contain %q, got: %s", expected, suggestion)
		}
	})
}

func TestFindSuggestions_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("distance greater than 4 yields no suggestions", func(t *testing.T) {
		filePath := setupTestWorkspace(t)
		errMsg := filePath + ":10:5: undeclared name: CompletelyUnrelatedLongIdentifier"

		suggestion := findSuggestions(ctx, errMsg)
		if suggestion != "" {
			t.Errorf("expected empty suggestions for large distance, got: %s", suggestion)
		}
	})

	t.Run("invalid or unrelated error messages", func(t *testing.T) {
		testCases := []string{
			"",
			"syntax error: unexpected semicolon, expecting )",
			"cannot find package \"example.com/mod\"",
			"main.go:1:1: unexpected error without known pattern",
		}

		for _, msg := range testCases {
			suggestion := findSuggestions(ctx, msg)
			if suggestion != "" {
				t.Errorf("expected empty suggestion for %q, got: %s", msg, suggestion)
			}
		}
	})
}

func TestFindSuggestions_MultiFileAndWindows(t *testing.T) {
	ctx := context.Background()

	t.Run("multiple error lines across multiple files", func(t *testing.T) {
		tmpDir := t.TempDir()

		file1 := filepath.Join(tmpDir, "file1.go")
		file1Content := "package multi\n\nfunc ProcessAlpha() {}\n"
		if err := os.WriteFile(file1, []byte(file1Content), 0600); err != nil {
			t.Fatalf("failed to write file1.go: %v", err)
		}

		file2 := filepath.Join(tmpDir, "file2.go")
		file2Content := "package multi\n\nfunc ProcessBeta() {}\n"
		if err := os.WriteFile(file2, []byte(file2Content), 0600); err != nil {
			t.Fatalf("failed to write file2.go: %v", err)
		}

		errMsg := strings.Join([]string{
			file1 + ":5:2: undeclared name: ProcessAlfa",
			file2 + ":8:4: ProcessBata undefined",
		}, "\n")

		suggestion := findSuggestions(ctx, errMsg)
		expected1 := "- In file1.go: Did you mean 'ProcessAlpha' instead of 'ProcessAlfa'?"
		expected2 := "- In file2.go: Did you mean 'ProcessBeta' instead of 'ProcessBata'?"

		if !strings.Contains(suggestion, expected1) {
			t.Errorf("expected suggestion 1 %q, got: %s", expected1, suggestion)
		}
		if !strings.Contains(suggestion, expected2) {
			t.Errorf("expected suggestion 2 %q, got: %s", expected2, suggestion)
		}
	})

	t.Run("windows path pattern", func(t *testing.T) {
		filePath := setupTestWorkspace(t)
		winPath := `C:\workspace\` + filepath.Base(filePath)
		errMsg := winPath + ":10:5: undeclared name: GoodSim"

		matches := fileErrorRegex.FindStringSubmatch(errMsg)
		if len(matches) < 5 {
			t.Fatalf("expected at least 5 matches for Windows path, got %d", len(matches))
		}
		if matches[1] != winPath {
			t.Errorf("expected filePath %q, got %q", winPath, matches[1])
		}
		if matches[2] != "10" || matches[3] != "5" {
			t.Errorf("expected line 10, col 5, got line %s, col %s", matches[2], matches[3])
		}
	})
}

func TestFindWorkspaceRoot(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "pkg", "subpkg", "deep")
	if err := os.MkdirAll(nestedDir, 0750); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	// Case 1: go.mod at root
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testroot\n"), 0600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	deepFile := filepath.Join(nestedDir, "deep.go")
	root := findWorkspaceRoot([]string{deepFile})
	if root != tmpDir {
		t.Errorf("expected workspace root %q, got %q", tmpDir, root)
	}

	// Case 2: empty paths returns empty string
	if r := findWorkspaceRoot(nil); r != "" {
		t.Errorf("expected empty string for nil paths, got %q", r)
	}
}

func TestExtractErrorSnippet(t *testing.T) {
	content := `package main

import "fmt"

func main() {
	fmt.Println("line 5")
	fmt.Println("line 6")
	fmt.Println("line 7")
}
`

	t.Run("error matching line number", func(t *testing.T) {
		err := errors.New("main.go:5:2: syntax error: unexpected newline")
		snippet := extractErrorSnippet(content, err)

		if !strings.Contains(snippet, "-> 5 |") {
			t.Errorf("expected snippet to point to line 5, got:\n%s", snippet)
		}
		if !strings.Contains(snippet, "fmt.Println(\"line 5\")") {
			t.Errorf("expected snippet to include line 5 content, got:\n%s", snippet)
		}
	})

	t.Run("error without line number", func(t *testing.T) {
		err := errors.New("cannot find package \"github.com/foo/bar\"")
		snippet := extractErrorSnippet(content, err)
		expected := "Could not determine error line."
		if snippet != expected {
			t.Errorf("expected %q, got %q", expected, snippet)
		}
	})

	t.Run("error with line number out of range", func(t *testing.T) {
		err := errors.New("main.go:999:1: some error")
		snippet := extractErrorSnippet(content, err)
		if snippet != "" {
			t.Errorf("expected empty snippet for out-of-range line, got %q", snippet)
		}
	})

	t.Run("error with multiple colons and prefix path", func(t *testing.T) {
		err := errors.New("/Users/test/dir:sub/main.go:6:10: undefined: myVar")
		snippet := extractErrorSnippet(content, err)
		if !strings.Contains(snippet, "-> 6 |") {
			t.Errorf("expected snippet to identify line 6, got:\n%s", snippet)
		}
	})
}

func setupGoFilesTree(t *testing.T, tmpDir string) ([]string, []string) {
	dirsToCreate := []string{
		filepath.Join(tmpDir, "pkg", "subpkg"),
		filepath.Join(tmpDir, ".git", "objects"),
		filepath.Join(tmpDir, "skills", "edit"),
		filepath.Join(tmpDir, "agents", "tester"),
		filepath.Join(tmpDir, "hooks", "pre-commit"),
		filepath.Join(tmpDir, "normal_dir"),
	}
	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	filesToCreate := map[string]string{
		filepath.Join(tmpDir, "root.go"):                  "package main",
		filepath.Join(tmpDir, "pkg", "pkg.go"):            "package pkg",
		filepath.Join(tmpDir, "pkg", "subpkg", "deep.go"): "package subpkg",
		filepath.Join(tmpDir, "normal_dir", "normal.go"):  "package normal",
		filepath.Join(tmpDir, "README.md"):                "# Readme",
		filepath.Join(tmpDir, "notes.txt"):                "Notes",
		filepath.Join(tmpDir, ".git", "git.go"):           "package git",
		filepath.Join(tmpDir, "skills", "skill.go"):       "package skills",
		filepath.Join(tmpDir, "agents", "agent.go"):       "package agents",
		filepath.Join(tmpDir, "hooks", "hook.go"):         "package hooks",
	}

	for path, content := range filesToCreate {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	expectedAllowed := []string{
		filepath.Join(tmpDir, "root.go"),
		filepath.Join(tmpDir, "pkg", "pkg.go"),
		filepath.Join(tmpDir, "pkg", "subpkg", "deep.go"),
		filepath.Join(tmpDir, "normal_dir", "normal.go"),
	}

	forbiddenDisallowed := []string{
		filepath.Join(tmpDir, "README.md"),
		filepath.Join(tmpDir, "notes.txt"),
		filepath.Join(tmpDir, ".git", "git.go"),
		filepath.Join(tmpDir, "skills", "skill.go"),
		filepath.Join(tmpDir, "agents", "agent.go"),
		filepath.Join(tmpDir, "hooks", "hook.go"),
	}

	return expectedAllowed, forbiddenDisallowed
}

func TestGetAllGoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	expectedAllowed, forbiddenDisallowed := setupGoFilesTree(t, tmpDir)

	goFiles, err := getAllGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("getAllGoFiles failed: %v", err)
	}

	for _, expected := range expectedAllowed {
		if !slices.Contains(goFiles, expected) {
			t.Errorf("expected %s in collected Go files, but not found in: %v", expected, goFiles)
		}
	}

	for _, forbidden := range forbiddenDisallowed {
		if slices.Contains(goFiles, forbidden) {
			t.Errorf("forbidden file %s found in collected Go files: %v", forbidden, goFiles)
		}
	}

	if len(goFiles) != len(expectedAllowed) {
		t.Errorf("expected exactly %d files, got %d: %v", len(expectedAllowed), len(goFiles), goFiles)
	}

	t.Run("non-existent root returns empty list without error", func(t *testing.T) {
		nonExistentRoot := filepath.Join(tmpDir, "does_not_exist")
		files, err := getAllGoFiles(nonExistentRoot)
		if err != nil {
			t.Errorf("expected nil error for non-existent root, got: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files for non-existent root, got: %v", files)
		}
	})

	t.Run("empty directory returns empty slice", func(t *testing.T) {
		emptyDir := t.TempDir()
		files, err := getAllGoFiles(emptyDir)
		if err != nil {
			t.Fatalf("unexpected error for empty dir: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})
}

func TestWriteAndVerify_Success(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmod\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	origContent := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"original\")\n}\n")
	if err := os.WriteFile(mainFile, origContent, 0600); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	newContent := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"updated\")\n}\n")
	currentContents := map[string][]byte{mainFile: newContent}
	backups := map[string]FileBackup{
		mainFile: {Content: origContent, Mode: 0600, Existed: true},
	}

	res, err := writeAndVerify(ctx, nil, currentContents, backups)
	if err != nil {
		t.Fatalf("unexpected error from writeAndVerify: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %v", res.Content[0].(*mcp.TextContent).Text)
	}

	written, err := os.ReadFile(filepath.Clean(mainFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(written) != string(newContent) {
		t.Errorf("file content mismatch, expected %q, got %q", string(newContent), string(written))
	}
}

func TestWriteAndVerify_PermissionPreservation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmod\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	scriptFile := filepath.Join(tmpDir, "script.go")
	origContent := []byte("package main\n\nfunc main() {}\n")
	// #nosec G306 -- Testing executable permission preservation
	if err := os.WriteFile(scriptFile, origContent, 0755); err != nil {
		t.Fatalf("failed to write script.go: %v", err)
	}

	// Edit with vet error to trigger rollback
	badContent := []byte("package main\n\nvar _ int = \"type mismatch\"\n\nfunc main() {}\n")
	currentContents := map[string][]byte{scriptFile: badContent}
	backups := map[string]FileBackup{
		scriptFile: {Content: origContent, Mode: 0755, Existed: true},
	}

	res, err := writeAndVerify(ctx, nil, currentContents, backups)
	if err == nil || !res.IsError {
		t.Fatalf("expected vet error, got: res=%v, err=%v", res, err)
	}

	info, err := os.Stat(scriptFile)
	if err != nil {
		t.Fatalf("failed to stat restored file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected restored permissions 0755, got: %o", info.Mode().Perm())
	}
}

func TestWriteAndVerify_RollbackVet(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmod\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	origContent := []byte("package main\n\nimport \"fmt\"\n\nfunc ValidFunction() {\n\tfmt.Println(\"ok\")\n}\nfunc main() {\n\tValidFunction()\n}\n")
	if err := os.WriteFile(mainFile, origContent, 0600); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	badContent := []byte("package main\n\nfunc ValidFunction() {\n\tvar _ int = \"type mismatch\"\n}\nfunc main() {\n\tValidFunction()\n}\n")
	currentContents := map[string][]byte{mainFile: badContent}
	backups := map[string]FileBackup{
		mainFile: {Content: origContent, Mode: 0600, Existed: true},
	}

	res, err := writeAndVerify(ctx, nil, currentContents, backups)
	if err == nil || !res.IsError {
		t.Fatalf("expected error result from writeAndVerify on vet failure, got res=%v, err=%v", res, err)
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Post-edit diagnostics check failed. All changes rolled back.") {
		t.Errorf("expected diagnostics failure message, got: %s", text)
	}

	restored, err := os.ReadFile(filepath.Clean(mainFile))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(restored) != string(origContent) {
		t.Errorf("file was not restored to original content")
	}
}

func TestWriteAndVerify_RollbackCreatesAndRemoves(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmod\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	origMainContent := []byte("package main\n\nfunc main() {}\n")
	if err := os.WriteFile(mainFile, origMainContent, 0600); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	nestedDir := filepath.Join(tmpDir, "newpkg")
	newFile := filepath.Join(nestedDir, "extra.go")
	newFileContent := []byte("package main\n\nfunc Extra() {\n\tvar _ int = \"type mismatch\"\n}\n")

	currentContents := map[string][]byte{newFile: newFileContent}
	backups := map[string]FileBackup{
		newFile: {Content: nil, Mode: 0600, Existed: false},
	}

	res, err := writeAndVerify(ctx, nil, currentContents, backups)
	if err == nil || !res.IsError {
		t.Fatalf("expected error on vet failure, got: res=%v, err=%v", res, err)
	}

	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Errorf("expected newly created file %s to be removed on rollback", newFile)
	}
	if _, err := os.Stat(nestedDir); !os.IsNotExist(err) {
		t.Errorf("expected newly created dir %s to be removed on rollback", nestedDir)
	}
}

func TestWriteAndVerify_NoGoFiles(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	txtFile := filepath.Join(tmpDir, "data.txt")
	origContent := []byte("initial")
	if err := os.WriteFile(txtFile, origContent, 0600); err != nil {
		t.Fatalf("failed to write data.txt: %v", err)
	}

	newContent := []byte("updated data")
	currentContents := map[string][]byte{txtFile: newContent}
	backups := map[string]FileBackup{
		txtFile: {Content: origContent, Mode: 0600, Existed: true},
	}

	res, err := writeAndVerify(ctx, nil, currentContents, backups)
	if err != nil || res.IsError {
		t.Fatalf("expected success, got res=%v, err=%v", res, err)
	}

	written, err := os.ReadFile(filepath.Clean(txtFile))
	if err != nil {
		t.Fatalf("failed to read data.txt: %v", err)
	}
	if string(written) != string(newContent) {
		t.Errorf("content mismatch, expected %q, got %q", string(newContent), string(written))
	}
}

func TestWriteAndVerify_WriteError(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mainFile := filepath.Join(tmpDir, "main2.go")
	origContent := []byte("package main\n\nfunc main() {}\n")
	if err := os.WriteFile(mainFile, origContent, 0600); err != nil {
		t.Fatalf("failed to write main2.go: %v", err)
	}

	conflictFile := filepath.Join(tmpDir, "conflict_as_file")
	if err := os.WriteFile(conflictFile, []byte("i am a file"), 0600); err != nil {
		t.Fatalf("failed to write conflict file: %v", err)
	}

	unwritableFile := filepath.Join(conflictFile, "sub.go")
	currentContents := map[string][]byte{
		mainFile:       []byte("package main\n\nfunc main() { /* changed */ }\n"),
		unwritableFile: []byte("package main\n"),
	}
	backups := map[string]FileBackup{
		mainFile:       {Content: origContent, Mode: 0600, Existed: true},
		unwritableFile: {Content: nil, Mode: 0600, Existed: false},
	}

	res, err := writeAndVerify(ctx, nil, currentContents, backups)
	if err == nil || !res.IsError {
		t.Fatalf("expected write error, got res=%v, err=%v", res, err)
	}

	restored, err := os.ReadFile(filepath.Clean(mainFile))
	if err != nil {
		t.Fatalf("failed to read main2.go: %v", err)
	}
	if string(restored) != string(origContent) {
		t.Errorf("expected main2.go to be restored to %q, got %q", string(origContent), string(restored))
	}
}

func TestWriteAndVerify_RollbackFailures(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmod\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	t.Run("rollback failure on go vet error", func(t *testing.T) {
		mainFile := filepath.Join(tmpDir, "main_rf.go")
		origContent := []byte("package main\n\nfunc main() {}\n")
		if err := os.WriteFile(mainFile, origContent, 0600); err != nil {
			t.Fatalf("failed to write main_rf.go: %v", err)
		}

		badContent := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%d\", \"bad\")\n}\n")
		nonExistentFile := filepath.Join(tmpDir, "non_existent_dir", "ghost.go")

		currentContents := map[string][]byte{mainFile: badContent}
		backups := map[string]FileBackup{
			mainFile:        {Content: origContent, Mode: 0600, Existed: true},
			nonExistentFile: {Content: []byte("cannot restore"), Mode: 0600, Existed: true},
		}

		res, err := writeAndVerify(ctx, nil, currentContents, backups)
		if err == nil || !res.IsError {
			t.Fatalf("expected combined error on vet and rollback failure, got res=%v, err=%v", res, err)
		}

		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "Rollback Failure:") {
			t.Errorf("expected text to mention Rollback Failure, got: %s", text)
		}
	})

	t.Run("rollback failure on writeContents error", func(t *testing.T) {
		conflictDir := filepath.Join(tmpDir, "conflict_file")
		if err := os.WriteFile(conflictDir, []byte("file content"), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		unwritableFile := filepath.Join(conflictDir, "child.go")
		nonExistentFile := filepath.Join(tmpDir, "non_existent_dir_2", "ghost.go")

		currentContents := map[string][]byte{
			unwritableFile: []byte("package main\n"),
		}
		backups := map[string]FileBackup{
			unwritableFile:  {Content: nil, Mode: 0600, Existed: false},
			nonExistentFile: {Content: []byte("content"), Mode: 0600, Existed: true},
		}

		res, err := writeAndVerify(ctx, nil, currentContents, backups)
		if err == nil || !res.IsError {
			t.Fatalf("expected error, got res=%v, err=%v", res, err)
		}
	})
}
