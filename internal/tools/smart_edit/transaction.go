package smartedit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// ExecuteEdits performs a set of file edits atomically in a compiler-verified transaction.
func ExecuteEdits(ctx context.Context, session *mcp.ServerSession, edits []FileEdit) (*mcp.CallToolResult, error) {
	if len(edits) == 0 {
		return errorResult("no edit operations provided"), nil
	}

	backups := make(map[string][]byte)
	newlyCreated := make(map[string]bool)
	currentContents := make(map[string][]byte)

	if err := backupFiles(edits, backups, newlyCreated, currentContents); err != nil {
		return errorResult(err.Error()), nil
	}

	if errResult := applyMemoryEdits(edits, newlyCreated, currentContents); errResult != nil {
		return errResult, nil
	}

	if errResult := autoFormatContents(currentContents); errResult != nil {
		return errResult, nil
	}

	return writeAndVerify(ctx, session, currentContents, backups, newlyCreated)
}

func backupFiles(
	edits []FileEdit,
	backups map[string][]byte,
	newlyCreated map[string]bool,
	currentContents map[string][]byte,
) error {
	for _, edit := range edits {
		filename := edit.Filename
		if filename == "" {
			filename = "."
		}
		absPath, err := filepath.Abs(filename)
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
	edits []FileEdit,
	newlyCreated map[string]bool,
	currentContents map[string][]byte,
) *mcp.CallToolResult {
	for _, edit := range edits {
		filename := edit.Filename
		if filename == "" {
			filename = "."
		}
		absPath, _ := filepath.Abs(filename)
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
		s, e, err := text.GetLineOffsets(original, edit.StartLine, edit.EndLine)
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
		bestStartLine := text.GetLineFromOffset(original, globalMatchStart)
		bestEndLine := text.GetLineFromOffset(original, globalMatchEnd)

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
