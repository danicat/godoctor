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
	"encoding/json"
	"go/ast"
	godocpkg "go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/text"
)

func TestDoc_RenderJSON(t *testing.T) {
	t.Run("nil Doc", func(t *testing.T) {
		var d *Doc
		got, err := d.RenderJSON()
		if err != nil {
			t.Fatalf("RenderJSON() on nil returned error: %v", err)
		}
		if got != "{}" {
			t.Errorf("RenderJSON() on nil = %q, want %q", got, "{}")
		}
	})

	t.Run("populated Doc", func(t *testing.T) {
		d := &Doc{
			Package:      "mypkg",
			ImportPath:   "example.com/mypkg",
			ResolvedPath: "example.com/mypkg/sub",
			SymbolName:   "MyFunc",
			Type:         "function",
			Definition:   "func MyFunc() error",
			Description:  "MyFunc performs an action.",
			Examples: []Example{
				{Name: "ExampleMyFunc", Code: "mypkg.MyFunc()", Output: "nil"},
				{Name: "", Code: "mypkg.MyFunc()"},
			},
			SubPackages: []string{"example.com/mypkg/sub1", "example.com/mypkg/sub2"},
			PkgGoDevURL: "https://pkg.go.dev/example.com/mypkg#MyFunc",
			Funcs:       []string{"func OtherFunc()"},
			Types:       []string{"type MyType struct{}"},
			Vars:        []string{"var ErrSome = errors.New(\"some\")"},
			Consts:      []string{"const Version = \"1.0.0\""},
			SourcePath:  "/path/to/mypkg.go",
			Line:        42,
			References:  []string{"example.com/app/main.go:15"},
		}

		jsonStr, err := d.RenderJSON()
		if err != nil {
			t.Fatalf("RenderJSON() error = %v", err)
		}

		var unmarshaled Doc
		if err := json.Unmarshal([]byte(jsonStr), &unmarshaled); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		if unmarshaled.Package != d.Package {
			t.Errorf("unmarshaled Package = %q, want %q", unmarshaled.Package, d.Package)
		}
		if unmarshaled.ImportPath != d.ImportPath {
			t.Errorf("unmarshaled ImportPath = %q, want %q", unmarshaled.ImportPath, d.ImportPath)
		}
		if unmarshaled.ResolvedPath != d.ResolvedPath {
			t.Errorf("unmarshaled ResolvedPath = %q, want %q", unmarshaled.ResolvedPath, d.ResolvedPath)
		}
		if unmarshaled.SymbolName != d.SymbolName {
			t.Errorf("unmarshaled SymbolName = %q, want %q", unmarshaled.SymbolName, d.SymbolName)
		}
		if unmarshaled.Type != d.Type {
			t.Errorf("unmarshaled Type = %q, want %q", unmarshaled.Type, d.Type)
		}
		if unmarshaled.Definition != d.Definition {
			t.Errorf("unmarshaled Definition = %q, want %q", unmarshaled.Definition, d.Definition)
		}
		if unmarshaled.Description != d.Description {
			t.Errorf("unmarshaled Description = %q, want %q", unmarshaled.Description, d.Description)
		}
		if len(unmarshaled.Examples) != len(d.Examples) {
			t.Fatalf("unmarshaled Examples len = %d, want %d", len(unmarshaled.Examples), len(d.Examples))
		}
		if unmarshaled.Examples[0].Output != "nil" {
			t.Errorf("unmarshaled Example Output = %q, want %q", unmarshaled.Examples[0].Output, "nil")
		}
		if len(unmarshaled.SubPackages) != 2 {
			t.Errorf("unmarshaled SubPackages len = %d, want 2", len(unmarshaled.SubPackages))
		}
		if unmarshaled.PkgGoDevURL != d.PkgGoDevURL {
			t.Errorf("unmarshaled PkgGoDevURL = %q, want %q", unmarshaled.PkgGoDevURL, d.PkgGoDevURL)
		}
		if len(unmarshaled.Funcs) != 1 || len(unmarshaled.Types) != 1 ||
			len(unmarshaled.Vars) != 1 || len(unmarshaled.Consts) != 1 {
			t.Errorf("unmarshaled symbol lists mismatch: %+v", unmarshaled)
		}
		if unmarshaled.SourcePath != d.SourcePath || unmarshaled.Line != d.Line {
			t.Errorf("unmarshaled SourcePath/Line mismatch: %s:%d", unmarshaled.SourcePath, d.Line)
		}
		if len(unmarshaled.References) != 1 || unmarshaled.References[0] != d.References[0] {
			t.Errorf("unmarshaled References mismatch: %v", unmarshaled.References)
		}
	})
}

func TestDoc_RenderMarkdown_And_Render(t *testing.T) {
	t.Run("package documentation with all sections", func(t *testing.T) {
		d := &Doc{
			Package:     "samplepkg",
			ImportPath:  "example.com/samplepkg",
			Definition:  "package samplepkg // import \"example.com/samplepkg\"",
			Description: "Sample package documentation.",
			Examples: []Example{
				{Name: "ExampleSample", Code: "samplepkg.Do()", Output: "done"},
				{Name: "", Code: "samplepkg.Simple()"},
			},
			Consts:      []string{"const Answer = 42"},
			Vars:        []string{"var Status = \"ok\""},
			Funcs:       []string{"func Do() error"},
			Types:       []string{"type Config struct{}"},
			SubPackages: []string{"example.com/samplepkg/sub1", "example.com/samplepkg/sub2"},
			References:  []string{"cmd/app/main.go:10"},
			PkgGoDevURL: "https://pkg.go.dev/example.com/samplepkg",
		}

		got := d.RenderMarkdown()
		wants := []string{
			"# example.com/samplepkg\n\n",
			"```go\npackage samplepkg // import \"example.com/samplepkg\"\n```\n\n",
			"Sample package documentation.\n\n",
			"### Examples\n\n",
			"#### ExampleSample\n\n```go\nsamplepkg.Do()\n```\n\n**Output:**\n```\ndone\n```\n\n",
			"#### Package Example\n\n```go\nsamplepkg.Simple()\n```\n\n",
			"### Constants\n\n```go\nconst Answer = 42\n```\n\n",
			"### Variables\n\n```go\nvar Status = \"ok\"\n```\n\n",
			"### Functions\n\n```go\nfunc Do() error\n\n```\n\n",
			"### Types\n\n```go\ntype Config struct{}\n\n```\n\n",
			"### Usages\n\n- cmd/app/main.go:10\n\n",
			"### Sub-packages\n\n- example.com/samplepkg/sub1\n- example.com/samplepkg/sub2\n\n",
			"[View on pkg.go.dev](https://pkg.go.dev/example.com/samplepkg)\n",
		}

		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("RenderMarkdown() missing content %q.\nGot:\n%s", want, got)
			}
		}

		// Verify Render(d) produces identical output to d.RenderMarkdown()
		if gotDirect := Render(d); gotDirect != got {
			t.Errorf("Render(d) does not match d.RenderMarkdown()")
		}
	})

	t.Run("resolved path parent note", func(t *testing.T) {
		d := &Doc{
			ImportPath:   "example.com/parent",
			ResolvedPath: "example.com/parent/sub/missing",
			PkgGoDevURL:  "https://pkg.go.dev/example.com/parent",
		}
		got := Render(d)
		wantNote := "> ℹ️ **Note:** Could not find `example.com/parent/sub/missing`.\n" +
			"> Showing documentation for parent module `example.com/parent` instead.\n\n"
		if !strings.Contains(got, wantNote) {
			t.Errorf("Render() missing parent fallback note %q.\nGot:\n%s", wantNote, got)
		}
	})

	t.Run("resolved path redirect note", func(t *testing.T) {
		d := &Doc{
			ImportPath:   "google.golang.org/adk",
			ResolvedPath: "github.com/google/adk-go",
			PkgGoDevURL:  "https://pkg.go.dev/google.golang.org/adk",
		}
		got := Render(d)
		wantNote := "> **Note:** Redirected from github.com/google/adk-go\n\n"
		if !strings.Contains(got, wantNote) {
			t.Errorf("Render() missing redirect note %q.\nGot:\n%s", wantNote, got)
		}
	})

	t.Run("symbol documentation format with source and usages", func(t *testing.T) {
		d := &Doc{
			Package:     "samplepkg",
			ImportPath:  "example.com/samplepkg",
			SymbolName:  "Worker",
			Type:        "type",
			SourcePath:  "worker.go",
			Line:        28,
			Definition:  "type Worker struct{}",
			Description: "Worker processes jobs.",
			References:  []string{"cmd/app/worker.go:12", "cmd/app/pool.go:45"},
			PkgGoDevURL: "https://pkg.go.dev/example.com/samplepkg#Worker",
		}
		got := Render(d)
		wants := []string{
			"# example.com/samplepkg\n\n",
			"## type Worker\n\n",
			"Defined in: `worker.go:28`\n\n",
			"```go\ntype Worker struct{}\n```\n\n",
			"Worker processes jobs.\n\n",
			"### Usages\n\n- cmd/app/worker.go:12\n- cmd/app/pool.go:45\n\n",
			"[View on pkg.go.dev](https://pkg.go.dev/example.com/samplepkg#Worker)\n",
		}
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("Render() missing symbol documentation content %q.\nGot:\n%s", want, got)
			}
		}
	})
}

func TestLoadWithFallback_And_GetDocumentationWithFallback(t *testing.T) {
	ctx := context.Background()

	t.Run("LoadWithFallback stdlib fmt", func(t *testing.T) {
		d, err := LoadWithFallback(ctx, "fmt", "")
		if err != nil {
			t.Fatalf("LoadWithFallback(\"fmt\", \"\") error = %v", err)
		}
		if d.Package != "fmt" {
			t.Errorf("LoadWithFallback() Package = %q, want %q", d.Package, "fmt")
		}
	})

	t.Run("LoadWithFallback stdlib symbol os/exec Cmd", func(t *testing.T) {
		d, err := LoadWithFallback(ctx, "os/exec", "Cmd")
		if err != nil {
			t.Fatalf("LoadWithFallback(\"os/exec\", \"Cmd\") error = %v", err)
		}
		if d.SymbolName != "Cmd" || d.Type != "type" {
			t.Errorf("LoadWithFallback() Symbol = %q, Type = %q; want Cmd, type", d.SymbolName, d.Type)
		}
	})

	t.Run("GetDocumentationWithFallback stdlib fmt", func(t *testing.T) {
		md, err := GetDocumentationWithFallback(ctx, "fmt")
		if err != nil {
			t.Fatalf("GetDocumentationWithFallback(\"fmt\") error = %v", err)
		}
		if !strings.Contains(md, "# fmt") {
			t.Errorf("GetDocumentationWithFallback(\"fmt\") missing '# fmt'. Got:\n%s", md)
		}
	})

	t.Run("GetDocumentationWithFallback non-existent package", func(t *testing.T) {
		_, err := GetDocumentationWithFallback(ctx, "nonexistent/invalid/package/path/xyz987")
		if err == nil {
			t.Fatal("GetDocumentationWithFallback() expected error for non-existent package, got nil")
		}
		if !strings.Contains(err.Error(), "could not find documentation for") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestSymbolDocumentation_Lookups(t *testing.T) {
	ctx := context.Background()

	t.Run("function fmt.Println", func(t *testing.T) {
		d, err := Load(ctx, "fmt", "Println")
		if err != nil {
			t.Fatalf("Load(\"fmt\", \"Println\") error = %v", err)
		}
		if d.Type != "function" || d.SymbolName != "Println" {
			t.Errorf("Symbol mismatch: type=%q, name=%q", d.Type, d.SymbolName)
		}
		if !strings.Contains(d.Definition, "func Println") {
			t.Errorf("Definition missing 'func Println': %q", d.Definition)
		}
	})

	t.Run("function fmt.Sprintf with return type", func(t *testing.T) {
		d, err := Load(ctx, "fmt", "Sprintf")
		if err != nil {
			t.Fatalf("Load(\"fmt\", \"Sprintf\") error = %v", err)
		}
		if d.Type != "function" || d.SymbolName != "Sprintf" {
			t.Errorf("Symbol mismatch: type=%q, name=%q", d.Type, d.SymbolName)
		}
		if !strings.Contains(d.Definition, "func Sprintf") {
			t.Errorf("Definition missing 'func Sprintf': %q", d.Definition)
		}
	})

	t.Run("types fmt.Stringer fmt.Formatter fmt.State", func(t *testing.T) {
		for _, sym := range []string{"Stringer", "Formatter", "State"} {
			d, err := Load(ctx, "fmt", sym)
			if err != nil {
				t.Fatalf("Load(\"fmt\", %q) error = %v", sym, err)
			}
			if d.Type != "type" || d.SymbolName != sym {
				t.Errorf("Symbol %q mismatch: type=%q, name=%q", sym, d.Type, d.SymbolName)
			}
			if !strings.Contains(d.Definition, sym) {
				t.Errorf("Definition missing %q: %q", sym, d.Definition)
			}
		}
	})

	t.Run("method on type in stdlib os/exec Run", func(t *testing.T) {
		d, err := Load(ctx, "os/exec", "Run")
		if err != nil {
			t.Fatalf("Load(\"os/exec\", \"Run\") error = %v", err)
		}
		if d.Type != "method" || d.SymbolName != "Run" {
			t.Errorf("Method symbol mismatch: type=%q, name=%q", d.Type, d.SymbolName)
		}
		if !strings.Contains(d.Definition, "func (c *Cmd) Run()") {
			t.Errorf("Definition missing 'func (c *Cmd) Run()': %q", d.Definition)
		}
	})

	t.Run("variable io.EOF", func(t *testing.T) {
		d, err := Load(ctx, "io", "EOF")
		if err != nil {
			t.Fatalf("Load(\"io\", \"EOF\") error = %v", err)
		}
		if d.Type != "var" || d.SymbolName != "EOF" {
			t.Errorf("Variable mismatch: type=%q, name=%q", d.Type, d.SymbolName)
		}
	})

	t.Run("constant io.SeekStart", func(t *testing.T) {
		d, err := Load(ctx, "io", "SeekStart")
		if err != nil {
			t.Fatalf("Load(\"io\", \"SeekStart\") error = %v", err)
		}
		if d.Type != "const" || d.SymbolName != "SeekStart" {
			t.Errorf("Constant mismatch: type=%q, name=%q", d.Type, d.SymbolName)
		}
	})

	t.Run("non-existent symbol with fuzzy suggestions in fmt", func(t *testing.T) {
		_, err := Load(ctx, "fmt", "Printfln")
		if err == nil {
			t.Fatal("expected error for non-existent symbol 'Printfln', got nil")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "symbol \"Printfln\" not found in package fmt") {
			t.Errorf("error missing expected prefix: %v", errMsg)
		}
		if !strings.Contains(errMsg, "Did you mean:") {
			t.Errorf("error missing fuzzy suggestion 'Did you mean:': %v", errMsg)
		}
	})
}

func TestHandleEmptyFiles(t *testing.T) {
	t.Run("with subpackages", func(t *testing.T) {
		d := &Doc{
			SubPackages: []string{"example.com/mod/sub1", "example.com/mod/sub2"},
		}
		res, err := handleEmptyFiles(d, "example.com/mod")
		if err != nil {
			t.Fatalf("handleEmptyFiles() error = %v", err)
		}
		if res.Package != "module_root" {
			t.Errorf("Package = %q, want 'module_root'", res.Package)
		}
		if !strings.Contains(res.Description, "Module root for example.com/mod") {
			t.Errorf("Description missing module root note: %q", res.Description)
		}
	})

	t.Run("without subpackages", func(t *testing.T) {
		d := &Doc{
			SubPackages: nil,
		}
		res, err := handleEmptyFiles(d, "example.com/empty")
		if err == nil {
			t.Fatal("handleEmptyFiles() expected error, got nil")
		}
		if res != nil {
			t.Errorf("expected nil Doc, got %+v", res)
		}
		if !strings.Contains(err.Error(), "no files found in package example.com/empty") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestSuggestPackages(t *testing.T) {
	ctx := context.Background()
	suggestions := suggestPackages(ctx, "fmtr")

	found := false
	for _, s := range suggestions {
		if s == "fmt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("suggestPackages(\"fmtr\") did not include 'fmt', got %v", suggestions)
	}
}

func TestBufferCode(t *testing.T) {
	fset := token.NewFileSet()

	t.Run("nil node", func(t *testing.T) {
		if got := bufferCode(fset, nil); got != "" {
			t.Errorf("bufferCode(fset, nil) = %q, want empty string", got)
		}
	})

	t.Run("valid AST node", func(t *testing.T) {
		expr, err := parser.ParseExpr("1 + 2")
		if err != nil {
			t.Fatalf("parser.ParseExpr failed: %v", err)
		}
		got := bufferCode(fset, expr)
		if got != "1 + 2" {
			t.Errorf("bufferCode(expr) = %q, want '1 + 2'", got)
		}
	})
}

func TestFindReturnTypeDefinition(t *testing.T) {
	src := `package testpkg

type MyResult struct {
	ID string
}

func ReturnPtr() *MyResult { return nil }
func ReturnVal() MyResult { return MyResult{} }
func ReturnSliceVal() []MyResult { return nil }
func ReturnSlicePtr() []*MyResult { return nil }
func ReturnNone() {}
func ReturnBuiltin() int { return 0 }
func ReturnUnknown() *UnknownType { return nil }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "testpkg.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parser.ParseFile failed: %v", err)
	}

	docPkg, err := godocpkg.NewFromFiles(fset, []*ast.File{file}, "testpkg")
	if err != nil {
		t.Fatalf("doc.NewFromFiles failed: %v", err)
	}

	funcMap := make(map[string]*godocpkg.Func)
	for _, f := range docPkg.Funcs {
		funcMap[f.Name] = f
	}
	for _, typ := range docPkg.Types {
		for _, f := range typ.Funcs {
			funcMap[f.Name] = f
		}
	}

	tests := []struct {
		funcName string
		wantDef  bool
	}{
		{"ReturnPtr", true},
		{"ReturnVal", true},
		{"ReturnSliceVal", true},
		{"ReturnSlicePtr", true},
		{"ReturnNone", false},
		{"ReturnBuiltin", false},
	}

	for _, tt := range tests {
		t.Run(tt.funcName, func(t *testing.T) {
			fn, ok := funcMap[tt.funcName]
			if !ok {
				t.Fatalf("function %q not found in parsed package", tt.funcName)
			}
			got := findReturnTypeDefinition(fset, docPkg, fn)
			if tt.wantDef {
				if !strings.Contains(got, "type MyResult struct") {
					t.Errorf("findReturnTypeDefinition(%s) = %q, want struct definition", tt.funcName, got)
				}
			} else {
				if got != "" {
					t.Errorf("findReturnTypeDefinition(%s) = %q, want empty string", tt.funcName, got)
				}
			}
		})
	}

	t.Run("ReturnUnknown not in package types", func(t *testing.T) {
		unknownFunc := &godocpkg.Func{
			Name: "ReturnUnknown",
			Decl: &ast.FuncDecl{
				Type: &ast.FuncType{
					Results: &ast.FieldList{
						List: []*ast.Field{
							{Type: &ast.Ident{Name: "UnknownType"}},
						},
					},
				},
			},
		}
		if got := findReturnTypeDefinition(fset, docPkg, unknownFunc); got != "" {
			t.Errorf("findReturnTypeDefinition(ReturnUnknown) = %q, want empty string", got)
		}
	})

	t.Run("nil guards and edge cases", func(t *testing.T) {
		if got := findReturnTypeDefinition(fset, nil, nil); got != "" {
			t.Errorf("expected empty string for nil func, got %q", got)
		}
		if got := findReturnTypeDefinition(fset, docPkg, nil); got != "" {
			t.Errorf("expected empty string for nil func, got %q", got)
		}
		if got := findReturnTypeDefinition(fset, docPkg, &godocpkg.Func{Decl: nil}); got != "" {
			t.Errorf("expected empty string for nil Decl, got %q", got)
		}
		nilTypeFunc := &godocpkg.Func{Decl: &ast.FuncDecl{Type: nil}}
		if got := findReturnTypeDefinition(fset, docPkg, nilTypeFunc); got != "" {
			t.Errorf("expected empty string for nil Type, got %q", got)
		}
		nilResultsFunc := &godocpkg.Func{
			Decl: &ast.FuncDecl{Type: &ast.FuncType{Results: nil}},
		}
		if got := findReturnTypeDefinition(fset, docPkg, nilResultsFunc); got != "" {
			t.Errorf("expected empty string for nil Results, got %q", got)
		}
		nilFieldFunc := &godocpkg.Func{
			Decl: &ast.FuncDecl{
				Type: &ast.FuncType{
					Results: &ast.FieldList{List: []*ast.Field{nil}},
				},
			},
		}
		if got := findReturnTypeDefinition(fset, docPkg, nilFieldFunc); got != "" {
			t.Errorf("expected empty string for nil Result item, got %q", got)
		}
		nilFieldTypeFunc := &godocpkg.Func{
			Decl: &ast.FuncDecl{
				Type: &ast.FuncType{
					Results: &ast.FieldList{List: []*ast.Field{{Type: nil}}},
				},
			},
		}
		if got := findReturnTypeDefinition(fset, docPkg, nilFieldTypeFunc); got != "" {
			t.Errorf("expected empty string for nil Type in Result, got %q", got)
		}
	})
}

func TestCollectFiles(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "dummy.go", "package dummy", 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if files := collectFiles(nil); len(files) != 0 {
		t.Errorf("collectFiles(nil) = %d files, want 0", len(files))
	}
	//nolint:staticcheck // SA1019: ast.Package is deprecated but needed for collectFiles test
	if files := collectFiles(map[string]*ast.Package{"nilPkg": nil}); len(files) != 0 {
		t.Errorf("collectFiles with nil pkg = %d files, want 0", len(files))
	}
	//nolint:staticcheck // SA1019: ast.Package is deprecated but needed for collectFiles test
	pkgs := map[string]*ast.Package{
		"dummy": {
			Files: map[string]*ast.File{
				"nilFile.go": nil,
				"dummy.go":   file,
			},
		},
	}
	if files := collectFiles(pkgs); len(files) != 1 {
		t.Errorf("collectFiles expected 1 file, got %d", len(files))
	}
}

func TestSetPackageName(t *testing.T) {
	t.Run("empty package name fallback to import path basename", func(t *testing.T) {
		d := &Doc{}
		setPackageName(d, &godocpkg.Package{Name: ""}, "github.com/foo/bar")
		if d.Package != "bar" {
			t.Errorf("setPackageName() Package = %q, want 'bar'", d.Package)
		}
	})

	t.Run("explicit package name", func(t *testing.T) {
		d := &Doc{}
		setPackageName(d, &godocpkg.Package{Name: "custom"}, "github.com/foo/bar")
		if d.Package != "custom" {
			t.Errorf("setPackageName() Package = %q, want 'custom'", d.Package)
		}
	})
}

func TestInitializeDoc(t *testing.T) {
	ctx := context.Background()

	t.Run("standard initialization without redirect", func(t *testing.T) {
		d := initializeDoc(ctx, "fmt", "", "")
		if d.ImportPath != "fmt" || d.ResolvedPath != "" || d.PkgGoDevURL != "https://pkg.go.dev/fmt" {
			t.Errorf("initializeDoc unexpected: %+v", d)
		}
	})

	t.Run("matching requested path", func(t *testing.T) {
		d := initializeDoc(ctx, "fmt", "fmt", "")
		if d.ResolvedPath != "" {
			t.Errorf("initializeDoc expected empty ResolvedPath when matching, got %q", d.ResolvedPath)
		}
	})

	t.Run("different requested path (redirect/fallback)", func(t *testing.T) {
		d := initializeDoc(ctx, "google.golang.org/adk", "github.com/google/adk-go", "")
		if d.ResolvedPath != "github.com/google/adk-go" {
			t.Errorf("initializeDoc expected ResolvedPath set, got %q", d.ResolvedPath)
		}
	})
}

func TestCheckHelpers_NilPackage(t *testing.T) {
	fset := token.NewFileSet()
	d := &Doc{}
	noop := func(string) {}

	if checkFuncs(fset, nil, "any", d, noop) {
		t.Error("checkFuncs with nil pkg returned true")
	}
	if checkTypes(fset, nil, "any", d, noop) {
		t.Error("checkTypes with nil pkg returned true")
	}
	if checkVars(fset, nil, "any", d, noop) {
		t.Error("checkVars with nil pkg returned true")
	}
	if checkConsts(fset, nil, "any", d, noop) {
		t.Error("checkConsts with nil pkg returned true")
	}
}

func TestExtractExamples_And_PopulateFunc_NilGuards(t *testing.T) {
	fset := token.NewFileSet()

	exs := extractExamples(fset, []*godocpkg.Example{nil})
	if len(exs) != 0 {
		t.Errorf("extractExamples with nil element returned %d items, want 0", len(exs))
	}

	d := &Doc{}
	populateFunc(fset, nil, nil, d)
	if d.Type != "" {
		t.Errorf("populateFunc(nil) modified doc: %+v", d)
	}
}

func TestParsePackageDocs_Direct(t *testing.T) {
	tempDir := t.TempDir()
	code := `// Package mypkg provides test utilities.
package mypkg

// Add returns the sum of two integers.
func Add(a, b int) int { return a + b }

// Config holds configuration values.
type Config struct {
	Port int
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "mypkg.go"), []byte(code), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx := context.Background()

	t.Run("package level doc", func(t *testing.T) {
		d, err := parsePackageDocs(ctx, "mypkg", tempDir, "", "mypkg")
		if err != nil {
			t.Fatalf("parsePackageDocs failed: %v", err)
		}
		if d.Package != "mypkg" {
			t.Errorf("doc.Package = %q, want 'mypkg'", d.Package)
		}
		if !strings.Contains(d.Description, "Package mypkg provides test utilities.") {
			t.Errorf("doc.Description missing expected doc: %q", d.Description)
		}
		if len(d.Funcs) != 1 || len(d.Types) != 1 {
			t.Errorf("expected 1 func and 1 type, got %d funcs and %d types", len(d.Funcs), len(d.Types))
		}
	})

	t.Run("symbol level doc", func(t *testing.T) {
		docSym, err := parsePackageDocs(ctx, "mypkg", tempDir, "Add", "mypkg")
		if err != nil {
			t.Fatalf("parsePackageDocs(Add) failed: %v", err)
		}
		if docSym.SymbolName != "Add" || docSym.Type != "function" {
			t.Errorf("docSym unexpected: %+v", docSym)
		}
	})
}

func TestListSubPackages_And_ResolvePackageDir(t *testing.T) {
	ctx := context.Background()

	t.Run("ListSubPackages on non-existent directory", func(t *testing.T) {
		subs := ListSubPackages(ctx, "/nonexistent/directory/path/that/does/not/exist")
		if subs != nil {
			t.Errorf("ListSubPackages on invalid dir = %v, want nil", subs)
		}
	})

	t.Run("resolvePackageDir on invalid package", func(t *testing.T) {
		_, err := resolvePackageDir(ctx, "invalid/package/that/does/not/exist/12345")
		if err == nil {
			t.Fatal("resolvePackageDir() expected error, got nil")
		}
	})
}

func TestSetupTempModule(t *testing.T) {
	ctx := context.Background()
	dir, err := setupTempModule(ctx)
	if err != nil {
		t.Fatalf("setupTempModule() error = %v", err)
	}
	defer os.RemoveAll(dir)

	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Errorf("expected go.mod to exist in temp dir: %v", err)
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		s1, s2 string
		want   int
	}{
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"rosettacode", "raisethysword", 8},
		{"same", "same", 0},
		{"a", "", 1},
		{"", "b", 1},
	}

	for _, tt := range tests {
		got := text.Levenshtein(tt.s1, tt.s2)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
		}
	}
}

func TestFindFuzzyMatches(t *testing.T) {
	candidates := []string{"Println", "Printf", "Sprintf", "Stringer", "Scan", "fmt"}

	tests := []struct {
		query string
		want  []string
	}{
		{"Prntln", []string{"Println"}},                      // Typo
		{"printf", []string{"Println", "Printf", "Sprintf"}}, // Case insensitivity + close matches
		{"sprint", []string{"Printf", "Sprintf"}},            // Partial/Close
		{"ftm", []string{"fmt"}},                             // Package typo
		{"Xyz", nil},                                         // No match
	}

	for _, tt := range tests {
		got := findFuzzyMatches(tt.query, candidates)

		if len(got) != len(tt.want) {
			t.Errorf("findFuzzyMatches(%q) got %v, want %v", tt.query, got, tt.want)
			continue
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("findFuzzyMatches(%q) index %d: got %q, want %q", tt.query, i, got[i], tt.want[i])
			}
		}
	}
}

func TestCheckHelpers_WithNilElements(t *testing.T) {
	fset := token.NewFileSet()
	d := &Doc{}
	noop := func(string) {}

	pkg := &godocpkg.Package{
		Funcs: []*godocpkg.Func{nil, {Name: "ActualFunc"}},
		Types: []*godocpkg.Type{
			nil,
			{
				Name:    "ActualType",
				Funcs:   []*godocpkg.Func{nil, {Name: "TypeFunc"}},
				Methods: []*godocpkg.Func{nil, {Name: "TypeMethod"}},
			},
		},
		Vars:   []*godocpkg.Value{nil, {Names: []string{"ActualVar"}}},
		Consts: []*godocpkg.Value{nil, {Names: []string{"ActualConst"}}},
	}

	if !checkFuncs(fset, pkg, "ActualFunc", d, noop) {
		t.Error("checkFuncs failed to find ActualFunc with nil entries")
	}
	if !checkTypes(fset, pkg, "ActualType", d, noop) {
		t.Error("checkTypes failed to find ActualType with nil entries")
	}
	if !checkTypes(fset, pkg, "TypeFunc", d, noop) {
		t.Error("checkTypes failed to find TypeFunc with nil entries")
	}
	if !checkTypes(fset, pkg, "TypeMethod", d, noop) {
		t.Error("checkTypes failed to find TypeMethod with nil entries")
	}
	if !checkVars(fset, pkg, "ActualVar", d, noop) {
		t.Error("checkVars failed to find ActualVar with nil entries")
	}
	if !checkConsts(fset, pkg, "ActualConst", d, noop) {
		t.Error("checkConsts failed to find ActualConst with nil entries")
	}
}

func TestFindFuzzyMatches_CandidateLimit(t *testing.T) {
	candidates := []string{"item1", "item2", "item3", "item4", "item5", "item6", "item7"}
	matches := findFuzzyMatches("item", candidates)
	if len(matches) > 5 {
		t.Errorf("findFuzzyMatches expected at most 5 matches, got %d", len(matches))
	}
}

func TestLoadWithFallback_SubpackageParentWalking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx := context.Background()
	d, err := LoadWithFallback(ctx, "github.com/google/uuid/nonexistent_sub_package", "")
	if err != nil {
		t.Fatalf("LoadWithFallback failed: %v", err)
	}
	if d.Package != "uuid" {
		t.Errorf("expected parent package 'uuid', got %q", d.Package)
	}
	if d.ResolvedPath != "github.com/google/uuid/nonexistent_sub_package" {
		t.Errorf("expected ResolvedPath 'github.com/google/uuid/nonexistent_sub_package', got %q", d.ResolvedPath)
	}
}
