package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "dev"
)

func main() {
	versionFlag := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "godoctor", Version: version}, nil)
	addTools(server)
	if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
		log.Fatalf("Error running server: %v", err)
	}
}

func addTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "getDoc"}, getDocHandler)
}

type GetDocParams struct {
	Symbol string `json:"symbol"`
}

func getDocHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[GetDocParams]) (*mcp.CallToolResult, error) {
	pkgPath, symbolName := splitSymbol(request.Arguments.Symbol)
	dir, err := findPkgDir(pkgPath)
	if err != nil {
		return newErrorResult("could not find package %q: %v", pkgPath, err), nil
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return newErrorResult("failed to parse package %q: %v", pkgPath, err), nil
	}

	if len(pkgs) == 0 {
		return newErrorResult("no Go packages found in %s", dir), nil
	}

	var docString string
	for _, pkgAST := range pkgs {
		files := make([]*ast.File, 0, len(pkgAST.Files))
		for _, file := range pkgAST.Files {
			files = append(files, file)
		}
		p, err := doc.NewFromFiles(fset, files, pkgPath)
		if err != nil {
			return newErrorResult("failed to create doc package for %q: %v", pkgPath, err), nil
		}

		if symbolName == "" {
			docString = p.Doc
			break
		}
		for _, f := range p.Funcs {
			if f.Name == symbolName {
				docString = f.Doc
				break
			}
		}
		if docString != "" {
			break
		}
		for _, t := range p.Types {
			if t.Name == symbolName {
				docString = t.Doc
				break
			}
			for _, f := range t.Funcs {
				if f.Name == symbolName {
					docString = f.Doc
					break
				}
			}
			if docString != "" {
				break
			}
			for _, m := range t.Methods {
				if m.Name == symbolName {
					docString = m.Doc
					break
				}
			}
		}
	}

	if docString == "" {
		docString = "documentation not found"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.TrimSpace(docString)},
		},
	}, nil
}

func newErrorResult(format string, a ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, a...)},
		},
	}
}

func findPkgDir(pkgPath string) (string, error) {
	if goroot := os.Getenv("GO_DOCTOR_GOROOT"); goroot != "" {
		dir := filepath.Join(goroot, "src", pkgPath)
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("could not find package %q in GOROOT %s", pkgPath, goroot)
		}
		return dir, nil
	}

	// Use `go list` to find the package directory.
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", pkgPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // Capture stderr as well
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not find package %q: `go list` failed: %s", pkgPath, out.String())
	}
	return strings.TrimSpace(out.String()), nil
}

func splitSymbol(symbol string) (string, string) {
	lastDot := strings.LastIndex(symbol, ".")
	if lastDot == -1 {
		return symbol, ""
	}
	return symbol[:lastDot], symbol[lastDot+1:]
}