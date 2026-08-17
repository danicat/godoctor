package smartedit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danicat/godoctor/internal/text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// FileBackup stores original file state for rollback and permission restoration.
type FileBackup struct {
	Content []byte
	Mode    os.FileMode
	Existed bool
}

// ExecuteEdits performs a set of file edits atomically in a compiler-verified transaction.
func ExecuteEdits(ctx context.Context, session *mcp.ServerSession, edits []FileEdit) (*mcp.CallToolResult, error) {
	if len(edits) == 0 {
		return errorResult("no edit operations provided"), nil
	}

	backups := make(map[string]FileBackup)
	currentContents := make(map[string][]byte)

	if err := backupFiles(edits, backups, currentContents); err != nil {
		return errorResult(err.Error()), nil
	}

	if errResult := applyMemoryEdits(edits, backups, currentContents); errResult != nil {
		return errResult, nil
	}

	if errResult := autoFormatContents(currentContents); errResult != nil {
		return errResult, nil
	}

	return writeAndVerify(ctx, session, currentContents, backups)
}

func backupFiles(
	edits []FileEdit,
	backups map[string]FileBackup,
	currentContents map[string][]byte,
) error {
	for _, edit := range edits {
		filename := strings.TrimSpace(edit.Filename)
		if filename == "" || !filepath.IsAbs(filename) {
			return errors.New("filename is required and must be an absolute path")
		}
		absPath := filepath.Clean(filename)

		if _, alreadyLoaded := currentContents[absPath]; !alreadyLoaded {
			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					backups[absPath] = FileBackup{
						Content: nil,
						Mode:    0666,
						Existed: false,
					}
					currentContents[absPath] = []byte("")
				} else {
					return fmt.Errorf("failed to read file %s: %v", edit.Filename, err)
				}
			} else {
				content, err := os.ReadFile(absPath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %v", edit.Filename, err)
				}
				currentContents[absPath] = content
				backups[absPath] = FileBackup{
					Content: content,
					Mode:    info.Mode(),
					Existed: true,
				}
			}
		}
	}
	return nil
}

func applyMemoryEdits(
	edits []FileEdit,
	backups map[string]FileBackup,
	currentContents map[string][]byte,
) *mcp.CallToolResult {
	for _, edit := range edits {
		absPath := filepath.Clean(edit.Filename)
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
		case !backups[absPath].Existed && len(original) == 0:
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

func commitWrite(path string, content []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmpFile := filepath.Clean(filepath.Join(dir, fmt.Sprintf(".%s.tmp.%d", base, time.Now().UnixNano())))

	perm := mode.Perm()
	if perm == 0 {
		perm = 0666
	}

	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpFile)
		}
	}()

	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpFile, perm); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}

func ensureDir(dir string, createdDirs *[]string) error {
	var toCreate []string
	curr := filepath.Clean(dir)
	for curr != "" && curr != "." && curr != string(filepath.Separator) && filepath.Dir(curr) != curr {
		if _, err := os.Stat(curr); err == nil {
			break
		}
		toCreate = append(toCreate, curr)
		curr = filepath.Dir(curr)
	}
	for i := len(toCreate) - 1; i >= 0; i-- {
		d := toCreate[i]
		if err := os.Mkdir(d, 0750); err != nil && !os.IsExist(err) {
			return err
		}
		if createdDirs != nil {
			*createdDirs = append(*createdDirs, d)
		}
	}
	return nil
}

func writeContents(
	currentContents map[string][]byte,
	backups map[string]FileBackup,
	createdDirs *[]string,
) (*mcp.CallToolResult, error) {
	for absPath, contentBytes := range currentContents {
		bk := backups[absPath]
		if !bk.Existed {
			if err := ensureDir(filepath.Dir(absPath), createdDirs); err != nil {
				var dirs []string
				if createdDirs != nil {
					dirs = *createdDirs
				}
				rbErr := rollback(backups, dirs)
				if rbErr != nil {
					msg := fmt.Sprintf("failed to create directory: %v (rollback failure: %v)", err, rbErr)
					return errorResult(msg), errors.Join(err, rbErr)
				}
				return errorResult(fmt.Sprintf("failed to create directory: %v", err)), err
			}
		}
		mode := bk.Mode
		if !bk.Existed {
			mode = 0666
		}
		if err := commitWrite(absPath, contentBytes, mode); err != nil {
			var dirs []string
			if createdDirs != nil {
				dirs = *createdDirs
			}
			rbErr := rollback(backups, dirs)
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
func rollback(backups map[string]FileBackup, createdDirs []string) error {
	var errs []error
	for path, bk := range backups {
		if !bk.Existed {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("rollback: failed to remove %s: %w", path, err))
			}
		} else {
			if err := commitWrite(path, bk.Content, bk.Mode); err != nil {
				errs = append(errs, fmt.Errorf("rollback: failed to restore %s: %w", path, err))
			}
		}
	}
	for i := len(createdDirs) - 1; i >= 0; i-- {
		_ = os.Remove(createdDirs[i])
	}
	return errors.Join(errs...)
}
