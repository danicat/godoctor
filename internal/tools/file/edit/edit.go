// Package edit implements the file editing tool with atomic multi-file transactions, formatting, compiler gates, and spelling aids.
package edit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/textdist"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/danicat/godoctor/internal/tools/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the smart_edit tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["smart_edit"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, toolHandler)
}

// FileEdit defines a single edit transaction within the smart_edit tool.
type FileEdit struct {
	Filename   string  `json:"filename" jsonschema:"The absolute path to the file to edit. You MUST use absolute paths in multi-root workspaces."`
	OldContent string  `json:"old_content,omitempty" jsonschema:"Optional: The block of code to find (ignores whitespace)"`
	NewContent string  `json:"new_content" jsonschema:"The new code to insert"`
	StartLine  int     `json:"start_line,omitempty" jsonschema:"Optional: restrict search to this line number and after"`
	EndLine    int     `json:"end_line,omitempty" jsonschema:"Optional: restrict search to this line number and before"`
	Threshold  float64 `json:"threshold,omitempty" jsonschema:"Similarity threshold (0.0-1.0) for fuzzy matching, default 0.95"`
	Append     bool    `json:"append,omitempty" jsonschema:"If true, append new_content to the end of the file (ignores old_content)"`
}

// Params defines the input parameters for the smart_edit tool.
type Params struct {
	Edits      []FileEdit `json:"edits,omitempty" jsonschema:"List of edits to perform atomically"`
	Filename   string     `json:"filename,omitempty" jsonschema:"Deprecated: use absolute path in edits instead"`
	OldContent string     `json:"old_content,omitempty" jsonschema:"Deprecated: use edits instead"`
	NewContent string     `json:"new_content,omitempty" jsonschema:"Deprecated: use edits instead"`
	StartLine  int        `json:"start_line,omitempty" jsonschema:"Deprecated: use edits instead"`
	EndLine    int        `json:"end_line,omitempty" jsonschema:"Deprecated: use edits instead"`
	Threshold  float64    `json:"threshold,omitempty" jsonschema:"Deprecated: use edits instead"`
	Append     bool       `json:"append,omitempty" jsonschema:"Deprecated: use edits instead"`
}

func toolHandler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}
	edits := args.Edits
	if len(edits) == 0 && args.Filename != "" {
		edits = []FileEdit{
			{
				Filename:   args.Filename,
				OldContent: args.OldContent,
				NewContent: args.NewContent,
				StartLine:  args.StartLine,
				EndLine:    args.EndLine,
				Threshold:  args.Threshold,
				Append:     args.Append,
			},
		}
	}

	if len(edits) == 0 {
		return errorResult("at least one edit transaction must be specified"), nil, nil
	}

	// Maps to hold file backups and current contents
	backups := make(map[string][]byte)
	newlyCreated := make(map[string]bool)
	currentContents := make(map[string][]byte)

	// 1. Back up all files and prepare initial contents
	for _, edit := range edits {
		absPath, err := roots.Global.Validate(session, edit.Filename)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		if _, alreadyLoaded := currentContents[absPath]; !alreadyLoaded {
			content, err := os.ReadFile(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					newlyCreated[absPath] = true
					currentContents[absPath] = []byte("")
					backups[absPath] = nil
				} else {
					return errorResult(fmt.Sprintf("failed to read file %s: %v", edit.Filename, err)), nil, nil
				}
			} else {
				currentContents[absPath] = content
				backups[absPath] = content
			}
		}
	}

	// 2. Apply edits sequentially in memory
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
		if newlyCreated[absPath] && len(original) == 0 {
			newContent = edit.NewContent
		} else if edit.Append || edit.OldContent == "" {
			if len(original) > 0 && !strings.HasSuffix(original, "\n") {
				newContent = original + "\n" + edit.NewContent
			} else {
				newContent = original + edit.NewContent
			}
		} else {
			searchStart := 0
			searchEnd := len(original)
			if edit.StartLine > 0 || edit.EndLine > 0 {
				s, e, err := shared.GetLineOffsets(original, edit.StartLine, edit.EndLine)
				if err != nil {
					return errorResult(fmt.Sprintf("line range error in %s: %v", edit.Filename, err)), nil, nil
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
				bestStartLine := shared.GetLineFromOffset(original, globalMatchStart)
				bestEndLine := shared.GetLineFromOffset(original, globalMatchEnd)

				return errorResult(fmt.Sprintf("match not found with sufficient confidence in %s (score: %.2f < %.2f).\n\nBest Match Found (Lines %d-%d):\n```go\n%s\n```\n\nSuggestions: verify old_content or lower threshold.", edit.Filename, score, threshold, bestStartLine, bestEndLine, bestMatch)), nil, nil
			}

			matchStart += searchStart
			matchEnd += searchStart
			newContent = original[:matchStart] + edit.NewContent + original[matchEnd:]
		}

		currentContents[absPath] = []byte(newContent)
	}

	// 3. Auto-Format & Import check (GO ONLY)
	for absPath, contentBytes := range currentContents {
		if strings.HasSuffix(absPath, ".go") {
			formatted, err := imports.Process(absPath, contentBytes, nil)
			if err != nil {
				snippet := shared.ExtractErrorSnippet(string(contentBytes), err)
				return errorResult(fmt.Sprintf("edit produced invalid Go code in %s: %v\n\nContext:\n```go\n%s\n```\nHint: Ensure NewContent is syntactically valid in context.", filepath.Base(absPath), err, snippet)), nil, nil
			}
			currentContents[absPath] = formatted
		}
	}

	// 4. Temporary Write to Disk for Verification Gate
	for absPath, contentBytes := range currentContents {
		if newlyCreated[absPath] {
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				rollback(backups, newlyCreated)
				return errorResult(fmt.Sprintf("failed to create directory: %v", err)), nil, nil
			}
		}
		if err := os.WriteFile(absPath, contentBytes, 0644); err != nil {
			rollback(backups, newlyCreated)
			return errorResult(fmt.Sprintf("failed to write temporary file %s: %v", filepath.Base(absPath), err)), nil, nil
		}
	}

	// 5. Run Compiler Gate (gopls check) on the entire workspace
	workspaceRoot := getWorkspaceRoot(session)

	goFiles, err := getAllGoFiles(workspaceRoot)
	if err != nil {
		rollback(backups, newlyCreated)
		return errorResult(fmt.Sprintf("failed to collect workspace Go files: %v", err)), nil, nil
	}

	if len(goFiles) > 0 {
		args := append([]string{"check"}, goFiles...)
		cmd := exec.CommandContext(ctx, "gopls", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Compiler check failed! Roll back all edits immediately.
			rollback(backups, newlyCreated)

			errorOutput := string(out)
			suggestions := findSuggestions(ctx, errorOutput)
			return errorResult(fmt.Sprintf("Post-edit diagnostics check failed. All changes rolled back.\n\nErrors:\n%s%s", errorOutput, suggestions)), nil, nil
		}
	}

	// 6. Return success
	var editedFiles []string
	for absPath := range currentContents {
		editedFiles = append(editedFiles, filepath.Base(absPath))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully edited files: %s", strings.Join(editedFiles, ", "))},
		},
	}, nil, nil
}

// rollback restores files to their original state or removes newly created files.
func rollback(backups map[string][]byte, newlyCreated map[string]bool) {
	for path, origContent := range backups {
		if newlyCreated[path] {
			_ = os.Remove(path)
		} else {
			_ = os.WriteFile(path, origContent, 0644)
		}
	}
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

func findSuggestions(ctx context.Context, errorMsg string) string {
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
			cmd := exec.CommandContext(ctx, "gopls", "symbols", filePath)
			out, err := cmd.CombinedOutput()
			if err == nil {
				knownSymbols := parseGoplsSymbols(string(out))
				bestSymbol, bestDist := findClosestSymbol(badSymbol, knownSymbols)
				if bestSymbol != "" && bestDist <= 4 {
					suggestions = append(suggestions, fmt.Sprintf("- In %s: Did you mean '%s' instead of '%s'?", filepath.Base(filePath), bestSymbol, badSymbol))
				}
			}
		}
	}

	if len(suggestions) > 0 {
		return "\n💡 **Suggestions:**\n" + strings.Join(suggestions, "\n")
	}
	return ""
}

func parseGoplsSymbols(symbolsOut string) []string {
	var symbols []string
	lines := strings.Split(symbolsOut, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) > 0 {
			symbols = append(symbols, parts[0])
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

	if idx := strings.Index(normContent, normSearch); idx != -1 {
		runeIdx := len([]rune(normContent[:idx]))
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

	bestScore := 0.0
	bestStartIdx := 0
	bestEndIdx := 0

	for startIdx := range candidates {
		endIdx := startIdx + searchLen
		if endIdx > len(normContentRunes) {
			endIdx = len(normContentRunes)
		}

		window := string(normContentRunes[startIdx:endIdx])
		score := similarity(normSearch, window)

		if score > bestScore {
			bestScore = score
			bestStartIdx = startIdx
			bestEndIdx = endIdx
		}
	}

	if bestScore > 0 {
		start := mapped[bestStartIdx].offset
		end := mapped[bestEndIdx-1].offset + 1
		return start, end, bestScore
	}

	return 0, 0, 0
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
