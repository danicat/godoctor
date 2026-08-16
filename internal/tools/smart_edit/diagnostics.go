package smartedit

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/danicat/godoctor/internal/text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func writeAndVerify(
	ctx context.Context,
	_ *mcp.ServerSession,
	currentContents map[string][]byte,
	backups map[string]FileBackup,
) (*mcp.CallToolResult, error) {
	var createdDirs []string
	if res, err := writeContents(currentContents, backups, &createdDirs); err != nil || res != nil {
		return res, err
	}

	var editedPaths []string
	for absPath := range currentContents {
		editedPaths = append(editedPaths, absPath)
	}

	workspaceRoot := findWorkspaceRoot(editedPaths)
	if workspaceRoot == "" && len(editedPaths) > 0 {
		workspaceRoot = filepath.Dir(editedPaths[0])
	}

	goFiles, walkErr := getAllGoFiles(workspaceRoot)
	if walkErr != nil {
		rbErr := rollback(backups, createdDirs)
		if rbErr != nil {
			msg := fmt.Sprintf("failed to collect workspace Go files: %v (rollback failure: %v)", walkErr, rbErr)
			return errorResult(msg), errors.Join(walkErr, rbErr)
		}
		return errorResult(fmt.Sprintf("failed to collect workspace Go files: %v", walkErr)), walkErr
	}

	if len(goFiles) > 0 {
		cmd, err := safeshell.CommandContext(ctx, "go", "vet", "./...")
		if err != nil {
			rbErr := rollback(backups, createdDirs)
			msg := fmt.Sprintf("Post-edit secure validation failed: %v", err)
			if rbErr != nil {
				msg += fmt.Sprintf(" (rollback failure: %v)", rbErr)
				return errorResult(msg), errors.Join(err, rbErr)
			}
			return errorResult(msg), err
		}
		cmd.Dir = workspaceRoot
		out, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			rbErr := rollback(backups, createdDirs)

			errorOutput := string(out)
			suggestions := findSuggestions(ctx, errorOutput)
			if rbErr != nil {
				msg := fmt.Sprintf("Post-edit diagnostics check failed. All changes rolled back.\n\n"+
					"Errors:\n%s%s\n\nRollback Failure:\n%v", errorOutput, suggestions, rbErr)
				return errorResult(msg), errors.Join(cmdErr, rbErr)
			}
			msg := fmt.Sprintf("Post-edit diagnostics check failed. All changes rolled back.\n\n"+
				"Errors:\n%s%s", errorOutput, suggestions)
			return errorResult(msg), cmdErr
		}
	}

	var editedFiles []string
	for absPath := range currentContents {
		editedFiles = append(editedFiles, filepath.Base(absPath))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully edited files: %s", strings.Join(editedFiles, ", "))},
		},
	}, nil
}

// findWorkspaceRoot searches upwards from edited files to locate the module or workspace root.
func findWorkspaceRoot(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	for _, p := range paths {
		dir := filepath.Dir(p)
		if root := findModuleOrGitRoot(dir); root != "" {
			return root
		}
	}
	return commonAncestor(paths)
}

func findModuleOrGitRoot(startDir string) string {
	dir := filepath.Clean(startDir)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func commonAncestor(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	common := filepath.Dir(paths[0])
	for _, p := range paths[1:] {
		dir := filepath.Dir(p)
		for !strings.HasPrefix(dir, common) {
			parent := filepath.Dir(common)
			if parent == common {
				return common
			}
			common = parent
		}
	}
	return common
}

// getAllGoFiles collects all relevant Go files to check, avoiding skills and assets directories.
func getAllGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "skills" || info.Name() == "agents" || info.Name() == "hooks" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

var (
	undeclaredRegex = regexp.MustCompile(`undeclared name:\s*([a-zA-Z0-9_]+)`)
	undefinedRegex  = regexp.MustCompile(`([a-zA-Z0-9_]+)\s+undefined`)
	noFieldRegex    = regexp.MustCompile(`no field or method\s*([a-zA-Z0-9_]+)`)
	fileErrorRegex  = regexp.MustCompile(`^((?:[a-zA-Z]:)?[^:]+):(\d+):(\d+):\s*(.*)$`)
)

func findSuggestions(_ context.Context, errorMsg string) string {
	lines := strings.Split(errorMsg, "\n")
	var suggestions []string
	astCache := make(map[string][]string)

	for _, line := range lines {
		matches := fileErrorRegex.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}
		filePath := matches[1]
		msg := matches[4]

		var badSymbol string
		if m := undeclaredRegex.FindStringSubmatch(msg); len(m) > 1 {
			badSymbol = m[1]
		} else if m := undefinedRegex.FindStringSubmatch(msg); len(m) > 1 {
			badSymbol = m[1]
		} else if m := noFieldRegex.FindStringSubmatch(msg); len(m) > 1 {
			badSymbol = m[1]
		}

		if badSymbol != "" {
			dir := filepath.Dir(filePath)
			knownSymbols, ok := astCache[dir]
			if !ok {
				knownSymbols = extractASTSymbols(filePath)
				astCache[dir] = knownSymbols
			}
			bestSymbol, bestDist := findClosestSymbol(badSymbol, knownSymbols)
			if bestSymbol != "" && bestDist <= 4 {
				suggestions = append(suggestions, fmt.Sprintf("- In %s: Did you mean '%s' instead of '%s'?",
					filepath.Base(filePath), bestSymbol, badSymbol))
			}
		}
	}

	if len(suggestions) > 0 {
		return "\n💡 **Suggestions:**\n" + strings.Join(suggestions, "\n")
	}
	return ""
}

func extractASTSymbols(filePath string) []string {
	dir := filepath.Dir(filePath)
	fset := token.NewFileSet()
	//nolint:staticcheck // ParseDir is used for fast symbol extraction
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil && len(pkgs) == 0 {
		return nil
	}

	var symbols []string
	seen := make(map[string]bool)
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			symbols = append(symbols, name)
		}
	}

	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					add(node.Name.Name)
				case *ast.TypeSpec:
					add(node.Name.Name)
				case *ast.ValueSpec:
					for _, name := range node.Names {
						add(name.Name)
					}
				case *ast.Field:
					for _, name := range node.Names {
						add(name.Name)
					}
				}
				return true
			})
		}
	}

	return symbols
}

func findClosestSymbol(bad string, known []string) (string, int) {
	bestDist := 999
	bestSymbol := ""
	for _, k := range known {
		if k == bad {
			continue
		}
		dist := text.Levenshtein(strings.ToLower(bad), strings.ToLower(k))
		if dist < bestDist {
			bestDist = dist
			bestSymbol = k
		}
	}
	return bestSymbol, bestDist
}

func extractErrorSnippet(content string, err error) string {
	errMsg := err.Error()
	parts := strings.Split(errMsg, ":")

	var lineNum int
	for _, part := range parts {
		var n int
		if _, e := fmt.Sscanf(strings.TrimSpace(part), "%d", &n); e == nil {
			lineNum = n
			break
		}
	}

	if lineNum == 0 {
		return "Could not determine error line."
	}

	return text.GetSnippet(content, lineNum)
}
