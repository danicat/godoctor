// Package smartedit implements the file editing tool with atomic multi-file transactions,
// formatting, compiler gates, and spelling aids.
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

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/danicat/godoctor/internal/textdist"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the smart_edit tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart_edit",
		Title:       "Smart Edit",
		Description: "Single-file coordinate editing transaction. Automatically applies the edit, formats using gofmt/goimports, and runs type verification (go vet) across the workspace. If compiler checks fail, the edit is rolled back, and Levenshtein-based spelling suggestions are returned for misspelled symbols.",
	}, singleEditHandler)
}

// FileEdit represents a single edit operation inside an atomic transaction.
type FileEdit struct {
	Filename   string `json:"filename" jsonschema:"The absolute path to the file to edit"`
	OldContent string `json:"old_content,omitempty" jsonschema:"Optional: The block of code to find (ignores whitespace)"`
	NewContent string `json:"new_content" jsonschema:"The new code to insert"`
	//nolint:lll
	StartLine int `json:"start_line,omitempty" jsonschema:"Optional: restrict search window to line number >= start_line"`
	EndLine   int `json:"end_line,omitempty" jsonschema:"Optional: restrict search window to line number <= end_line"`
	//nolint:lll
	Threshold float64 `json:"threshold,omitempty" jsonschema:"Optional: similarity threshold (0.0-1.0) for fuzzy matching (default 0.95)"`
	Append    bool    `json:"append,omitempty" jsonschema:"Optional: append new_content to end of file"`
}

// SingleEditParams defines the input parameters for the smart_edit single file edit tool.
type SingleEditParams struct {
	Filename   string `json:"filename" jsonschema:"The absolute path to the file to edit"`
	OldContent string `json:"old_content,omitempty" jsonschema:"Optional: The block of code to find (ignores whitespace)"`
	NewContent string `json:"new_content" jsonschema:"The new code to insert"`
	//nolint:lll
	StartLine int `json:"start_line,omitempty" jsonschema:"Optional: restrict search window to line number >= start_line"`
	EndLine   int `json:"end_line,omitempty" jsonschema:"Optional: restrict search window to line number <= end_line"`
	//nolint:lll
	Threshold float64 `json:"threshold,omitempty" jsonschema:"Optional: similarity threshold (0.0-1.0) for fuzzy matching (default 0.95)"`
	Append    bool    `json:"append,omitempty" jsonschema:"Optional: append new_content to end of file"`
}

//nolint:lll
func singleEditHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args SingleEditParams,
) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Filename) == "" {
		return errorResult("filename is required"), nil, nil
	}

	edit := FileEdit(args)

	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}

	res, err := ExecuteEdits(ctx, session, []FileEdit{edit})
	return res, nil, err
}

// ExecuteEdits performs a set of file edits atomically in a compiler-verified transaction.
func ExecuteEdits(ctx context.Context, session *mcp.ServerSession, edits []FileEdit) (*mcp.CallToolResult, error) {
	if len(edits) == 0 {
		return errorResult("no edit operations provided"), nil
	}

	backups := make(map[string][]byte)
	newlyCreated := make(map[string]bool)
	currentContents := make(map[string][]byte)

	if err := backupFiles(session, edits, backups, newlyCreated, currentContents); err != nil {
		return errorResult(err.Error()), nil
	}

	if errResult := applyMemoryEdits(session, edits, newlyCreated, currentContents); errResult != nil {
		return errResult, nil
	}

	if errResult := autoFormatContents(currentContents); errResult != nil {
		return errResult, nil
	}

	return writeAndVerify(ctx, session, currentContents, backups, newlyCreated)
}

func backupFiles(
	session *mcp.ServerSession,
	edits []FileEdit,
	backups map[string][]byte,
	newlyCreated map[string]bool,
	currentContents map[string][]byte,
) error {
	for _, edit := range edits {
		absPath, err := roots.Global.Validate(session, edit.Filename)
		if err != nil {
			return err
		}

		if _, alreadyLoaded := currentContents[absPath]; !alreadyLoaded {
			//nolint:gosec
			content, err := os.ReadFile(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					newlyCreated[absPath] = true
					currentContents[absPath] = []byte("")
					backups[absPath] = nil
				} else {
					return fmt.Errorf("failed to read file %s: %v", edit.Filename, err)
				}
			} else {
				currentContents[absPath] = content
				backups[absPath] = content
			}
		}
	}
	return nil
}

func applyMemoryEdits(
	session *mcp.ServerSession,
	edits []FileEdit,
	newlyCreated map[string]bool,
	currentContents map[string][]byte,
) *mcp.CallToolResult {
	for _, edit := range edits {
		absPath, _ := roots.Global.Validate(session, edit.Filename)
		original := string(currentContents[absPath])
		threshold := edit.Threshold
		if threshold == 0 {
			threshold = 0.95
		}
		if threshold > 1.0 {
			threshold = 1.0
		}
		if threshold < 0.0 {
			threshold = 0.0
		}

		var newContent string
		switch {
		case newlyCreated[absPath] && len(original) == 0:
			newContent = edit.NewContent
		case edit.Append || edit.OldContent == "":
			if len(original) > 0 && !strings.HasSuffix(original, "\n") {
				newContent = original + "\n" + edit.NewContent
			} else {
				newContent = original + edit.NewContent
			}
		default:
			var errResult *mcp.CallToolResult
			newContent, errResult = applySingleMemoryEdit(edit, original, threshold)
			if errResult != nil {
				return errResult
			}
		}

		currentContents[absPath] = []byte(newContent)
	}
	return nil
}

func applySingleMemoryEdit(edit FileEdit, original string, threshold float64) (string, *mcp.CallToolResult) {
	searchStart := 0
	searchEnd := len(original)
	if edit.StartLine > 0 || edit.EndLine > 0 {
		s, e, err := getLineOffsets(original, edit.StartLine, edit.EndLine)
		if err != nil {
			return "", errorResult(fmt.Sprintf("line range error in %s: %v", edit.Filename, err))
		}
		searchStart = s
		searchEnd = e
	}

	searchArea := original[searchStart:searchEnd]
	matchStart, matchEnd, score := findBestMatch(searchArea, edit.OldContent)

	if score < threshold {
		bestMatch := ""
		if matchStart < matchEnd && matchEnd <= len(searchArea) {
			bestMatch = searchArea[matchStart:matchEnd]
		}

		globalMatchStart := searchStart + matchStart
		globalMatchEnd := searchStart + matchEnd
		bestStartLine := getLineFromOffset(original, globalMatchStart)
		bestEndLine := getLineFromOffset(original, globalMatchEnd)

		return "", errorResult(fmt.Sprintf(
			"match not found with sufficient confidence in %s (score: %.2f < %.2f).\n\n"+
				"Best Match Found (Lines %d-%d):\n```go\n%s\n```\n\n"+
				"Suggestions: verify old_content or lower threshold.",
			edit.Filename, score, threshold, bestStartLine, bestEndLine, bestMatch))
	}

	matchStart += searchStart
	matchEnd += searchStart
	return original[:matchStart] + edit.NewContent + original[matchEnd:], nil
}

func autoFormatContents(currentContents map[string][]byte) *mcp.CallToolResult {
	for absPath, contentBytes := range currentContents {
		if strings.HasSuffix(absPath, ".go") {
			formatted, err := imports.Process(absPath, contentBytes, nil)
			if err != nil {
				snippet := extractErrorSnippet(string(contentBytes), err)
				return errorResult(fmt.Sprintf(
					"edit produced invalid Go code in %s: %v\n\nContext:\n```go\n%s\n```\n"+
						"Hint: Ensure NewContent is syntactically valid in context.",
					filepath.Base(absPath), err, snippet))
			}
			currentContents[absPath] = formatted
		}
	}
	return nil
}

func commitWrite(path string, content []byte, isNew bool) (err error) {
	var f *os.File
	if isNew {
		// os.Create opens with O_RDWR|O_CREATE|O_TRUNC and mode 0666,
		// delegating file permissions entirely to the OS umask.
		//nolint:gosec
		f, err = os.Create(path)
	} else {
		// Open the existing file for writing/truncating without O_CREATE.
		// Passing 0 permission has no effect and avoids hardcoding modes.
		//nolint:gosec
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	}
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}()
	_, err = f.Write(content)
	return err
}

func writeContents(
	currentContents map[string][]byte,
	backups map[string][]byte,
	newlyCreated map[string]bool,
) (*mcp.CallToolResult, error) {
	for absPath, contentBytes := range currentContents {
		if newlyCreated[absPath] {
			if err := os.MkdirAll(filepath.Dir(absPath), 0750); err != nil {
				rbErr := rollback(backups, newlyCreated)
				if rbErr != nil {
					msg := fmt.Sprintf("failed to create directory: %v (rollback failure: %v)", err, rbErr)
					return errorResult(msg), errors.Join(err, rbErr)
				}
				return errorResult(fmt.Sprintf("failed to create directory: %v", err)), err
			}
		}
		if err := commitWrite(absPath, contentBytes, newlyCreated[absPath]); err != nil {
			rbErr := rollback(backups, newlyCreated)
			if rbErr != nil {
				msg := fmt.Sprintf("failed to write temporary file %s: %v (rollback failure: %v)",
					filepath.Base(absPath), err, rbErr)
				return errorResult(msg), errors.Join(err, rbErr)
			}
			msg := fmt.Sprintf("failed to write temporary file %s: %v", filepath.Base(absPath), err)
			return errorResult(msg), err
		}
	}
	return nil, nil
}

func writeAndVerify(
	ctx context.Context,
	session *mcp.ServerSession,
	currentContents map[string][]byte,
	backups map[string][]byte,
	newlyCreated map[string]bool,
) (*mcp.CallToolResult, error) {
	if res, err := writeContents(currentContents, backups, newlyCreated); err != nil || res != nil {
		return res, err
	}

	workspaceRoot := getWorkspaceRoot(session)

	goFiles, walkErr := getAllGoFiles(workspaceRoot)
	if walkErr != nil {
		rbErr := rollback(backups, newlyCreated)
		if rbErr != nil {
			msg := fmt.Sprintf("failed to collect workspace Go files: %v (rollback failure: %v)", walkErr, rbErr)
			return errorResult(msg), errors.Join(walkErr, rbErr)
		}
		return errorResult(fmt.Sprintf("failed to collect workspace Go files: %v", walkErr)), walkErr
	}

	if len(goFiles) > 0 {
		cmd, err := safeshell.CommandContext(ctx, "go", "vet", "./...")
		if err != nil {
			rbErr := rollback(backups, newlyCreated)
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
			rbErr := rollback(backups, newlyCreated)

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

// rollback restores files to their original state or removes newly created files.
func rollback(backups map[string][]byte, newlyCreated map[string]bool) error {
	var errs []error
	for path, origContent := range backups {
		if newlyCreated[path] {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("rollback: failed to remove %s: %w", path, err))
			}
		} else {
			if err := commitWrite(path, origContent, false); err != nil {
				errs = append(errs, fmt.Errorf("rollback: failed to restore %s: %w", path, err))
			}
		}
	}
	return errors.Join(errs...)
}

// getAllGoFiles collects all relevant Go files to check, avoiding skills and assets directories.
func getAllGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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
	fileErrorRegex  = regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.*)$`)
)

func findSuggestions(_ context.Context, errorMsg string) string {
	lines := strings.Split(errorMsg, "\n")
	var suggestions []string

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
			knownSymbols := extractASTSymbols(filePath)
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
		dist := textdist.Levenshtein(strings.ToLower(bad), strings.ToLower(k))
		if dist < bestDist {
			bestDist = dist
			bestSymbol = k
		}
	}
	return bestSymbol, bestDist
}

// findBestMatch locates the best match for 'search' within 'content' ignoring whitespace and newlines.
// It returns the start and end byte offsets in the original content and a similarity score (0-1).
func findBestMatch(content, search string) (int, int, float64) {
	normSearch := normalize(search)
	if normSearch == "" {
		return 0, 0, 0
	}

	type charMap struct {
		char   rune
		offset int
	}
	var mapped []charMap
	for offset, char := range content {
		if !isWhitespace(char) {
			mapped = append(mapped, charMap{char, offset})
		}
	}
	normContentRunes := make([]rune, len(mapped))
	for i, cm := range mapped {
		normContentRunes[i] = cm.char
	}
	normContent := string(normContentRunes)

	if before, _, ok := strings.Cut(normContent, normSearch); ok {
		runeIdx := len([]rune(before))
		start := mapped[runeIdx].offset
		end := mapped[runeIdx+len([]rune(normSearch))-1].offset + 1
		return start, end, 1.0
	}

	searchRunes := []rune(normSearch)
	searchLen := len(searchRunes)
	contentLen := len(normContentRunes)

	if searchLen > contentLen {
		score := similarity(normSearch, normContent)
		return 0, len(content), score
	}

	candidates := collectCandidates(normContent, normContentRunes, searchRunes, searchLen)
	bestScore, bestStartIdx, bestEndIdx := evaluateCandidates(normContentRunes, normSearch, searchLen, candidates)

	if bestScore > 0 {
		start := mapped[bestStartIdx].offset
		end := mapped[bestEndIdx-1].offset + 1
		return start, end, bestScore
	}

	return 0, 0, 0
}

func collectCandidates(normContent string, normContentRunes, searchRunes []rune, searchLen int) map[int]int {
	seedLen := 16
	step := 8

	if searchLen < 64 {
		seedLen = 8
		step = 4
	}
	if searchLen < seedLen {
		seedLen = 4
		step = 2
	}

	candidates := make(map[int]int)

	checkSeed := func(offset int) {
		seed := string(searchRunes[offset : offset+seedLen])
		startSearch := 0
		for {
			idx := strings.Index(normContent[startSearch:], seed)
			if idx == -1 {
				break
			}
			realIdx := startSearch + idx
			projectedStart := realIdx - offset
			if projectedStart >= 0 && projectedStart <= len(normContentRunes)-searchLen {
				candidates[projectedStart]++
			}
			startSearch = realIdx + 1
		}
	}

	for i := 0; i <= searchLen-seedLen; i += step {
		checkSeed(i)
	}

	if searchLen >= seedLen {
		tailOffset := searchLen - seedLen
		if tailOffset%step != 0 {
			checkSeed(tailOffset)
		}
	}
	return candidates
}

func evaluateCandidates(
	normContentRunes []rune,
	normSearch string,
	searchLen int,
	candidates map[int]int,
) (float64, int, int) {
	bestScore := 0.0
	bestStartIdx := 0
	bestEndIdx := 0

	for startIdx := range candidates {
		endIdx := min(startIdx+searchLen, len(normContentRunes))

		window := string(normContentRunes[startIdx:endIdx])
		score := similarity(normSearch, window)

		if score > bestScore {
			bestScore = score
			bestStartIdx = startIdx
			bestEndIdx = endIdx
		}
	}
	return bestScore, bestStartIdx, bestEndIdx
}

func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

func normalize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if !isWhitespace(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func similarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	d := textdist.Levenshtein(s1, s2)
	maxLen := len([]rune(s1))
	if l2 := len([]rune(s2)); l2 > maxLen {
		maxLen = l2
	}
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(d)/float64(maxLen)
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}

func getWorkspaceRoot(session *mcp.ServerSession) string {
	rts := roots.Global.Get(session)
	if len(rts) > 0 {
		return rts[0]
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func getLineOffsets(content string, startLine, endLine int) (int, int, error) {
	currentLine := 1
	startOffset := 0
	endOffset := len(content)
	foundStart := false

	if startLine <= 1 {
		startOffset = 0
		foundStart = true
	}

	for i, char := range content {
		if char == '\n' {
			currentLine++
			if !foundStart && currentLine == startLine {
				startOffset = i + 1
				foundStart = true
			}
			if endLine > 0 && currentLine > endLine {
				endOffset = i + 1
				break
			}
		}
	}

	if startLine > currentLine && startLine > 1 {
		return 0, 0, fmt.Errorf("start_line %d is beyond file length (%d lines)", startLine, currentLine)
	}

	return startOffset, endOffset, nil
}

func getSnippet(content string, lineNum int) string {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return ""
	}

	start := max(lineNum-5, 1)
	end := min(lineNum+5, len(lines))

	var sb strings.Builder
	for i := start; i <= end; i++ {
		prefix := "  "
		if i == lineNum {
			prefix = "-> "
		}
		fmt.Fprintf(&sb, "%s%d | %s\n", prefix, i, lines[i-1])
	}
	return sb.String()
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

	return getSnippet(content, lineNum)
}

func getLineFromOffset(content string, offset int) int {
	if offset < 0 || offset > len(content) {
		return 0
	}
	return strings.Count(content[:offset], "\n") + 1
}
