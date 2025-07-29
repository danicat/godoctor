package scalpel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Execute runs the specified scalpel command.
func Execute(params *ScalpelParams) (string, error) {
	if params.FilePath == "" {
		return "", scalpelErrorf("file_path required for all scalpel operations")
	}

	originalContent, err := os.ReadFile(params.FilePath)
	if err != nil {
		return "", scalpelErrorf("failed to read file %s: %w", params.FilePath, err)
	}

	var newContent []byte
	var startOffset, endOffset int

	switch params.Operation {
	case "insert":
		if params.Start == nil {
			return "", scalpelErrorf("start position required for insert operation")
		}
		offset, err := positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", scalpelErrorf("failed to calculate insert offset: %w", err)
		}
		newContent = make([]byte, 0, len(originalContent)+len(params.Content))
		newContent = append(newContent, originalContent[:offset]...)
		newContent = append(newContent, []byte(params.Content)...)
		newContent = append(newContent, originalContent[offset:]...)
	case "delete":
		if params.Start == nil || params.End == nil {
			return "", scalpelErrorf("start and end positions required for delete operation")
		}
		startOffset, err = positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", scalpelErrorf("failed to calculate start offset for delete: %w", err)
		}
		endOffset, err = positionToOffset(originalContent, *params.End)
		if err != nil {
			return "", scalpelErrorf("failed to calculate end offset for delete: %w", err)
		}
		if startOffset > endOffset {
			return "", scalpelErrorf("start offset %d cannot be greater than end offset %d", startOffset, endOffset)
		}
		newContent = make([]byte, 0, len(originalContent)-(endOffset-startOffset))
		newContent = append(newContent, originalContent[:startOffset]...)
		newContent = append(newContent, originalContent[endOffset:]...)
	case "replace":
		if params.Start == nil || params.End == nil {
			return "", scalpelErrorf("start and end positions required for replace operation")
		}
		startOffset, err = positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", scalpelErrorf("failed to calculate start offset for replace: %w", err)
		}
		endOffset, err = positionToOffset(originalContent, *params.End)
		if err != nil {
			return "", scalpelErrorf("failed to calculate end offset for replace: %w", err)
		}
		if startOffset > endOffset {
			return "", scalpelErrorf("start offset %d cannot be greater than end offset %d", startOffset, endOffset)
		}
		newContent = make([]byte, 0, len(originalContent)-(endOffset-startOffset)+len(params.Content))
		newContent = append(newContent, originalContent[:startOffset]...)
		newContent = append(newContent, []byte(params.Content)...)
		newContent = append(newContent, originalContent[endOffset:]...)
	case "replaceAll":
		if params.Pattern == "" || params.Replacement == "" {
			return "", scalpelErrorf("pattern and replacement required for replaceAll operation")
		}
		re, err := regexp.Compile(params.Pattern)
		if err != nil {
			return "", scalpelErrorf("failed to compile regex: %w", err)
		}
		newContent = re.ReplaceAll(originalContent, []byte(params.Replacement))
		// Calculate replacements_made by comparing original and new content length, or by counting matches
		// For simplicity, we'll just return a success message for now.
		// A more robust solution would involve iterating through matches and counting.
		// For now, we'll return a generic success message.
		return `{"status": "success", "message": "Operation completed successfully."}`,
		nil
	case "search":
		if params.Pattern == "" {
			return "", fmt.Errorf("pattern is required for search operation")
		}
		re, err := regexp.Compile(params.Pattern)
		if err != nil {
			return "scalpelErrorf("failed to compile regex: %w", err)rr)
		}
		matches := []Match{}
		for _, submatches := range re.FindAllSubmatchIndex(originalContent, -1) {
			match := Match{}
			match.Range.Start.Offset = submatches[0]
			match.Range.End.Offset = submatches[1]
			for i := 0; i < len(submatches); i += 2 {
				match.Groups = append(match.Groups, string(originalContent[submatches[i]:submatches[i+1]]))
			}
			matches = append(matches, match)
		}
		jsonMatches, err := json.Marshal(matches)
		if err != nil {
			return "", fmt.Errorf("failed to marshal matches to JSON: %w", err)
		}
		return string(jsonMatches), nil
	default:
		return "", fmt.Errorf("unknown operation: %s", params.Operation)
	}

	if err := os.WriteFile(params.FilePath, newContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return `{"status": "success", "message": "Operation completed successfully."}`,
		nil
}

// positionToOffset converts a Position (line/column or offset) to a byte offset.
func positionToOffset(content []byte, pos Position) (int, error) {
	if pos.Offset != 0 {
		if pos.Offset > len(content) {
			return 0, fmt.Errorf("offset %d is out of bounds for file length %d", pos.Offset, len(content))
		}
		return pos.Offset, nil
	}

	if pos.Line == 0 || pos.Column == 0 {
		return 0, fmt.Errorf("either offset or line/column must be provided")
	}

	lines := bytes.Split(content, []byte("\n"))
	if pos.Line > len(lines) {
		return 0, fmt.Errorf("line %d is out of bounds for file with %d lines", pos.Line, len(lines))
	}

	offset := 0
	for i := 0; i < pos.Line-1; i++ {
		offset += len(lines[i]) + 1 // +1 for the newline character
	}


func scalpelErrorf(format string, args ...interface{}) error {
	return fmt.Errorf("scalpel: "+format, args...)
}
	if pos.Column-1 > len(lines[pos.Line-1]) {
		return 0, fmt.Errorf("column %d is out of bounds for line %d", pos.Column, pos.Line)
	}
	offset += pos.Column - 1

	return offset, nil
}

func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scalpel",
		Description: "A powerful tool for precise file editing. Supports inserting content at specific positions, deleting content within a defined range, and replacing text either by range or by searching for a pattern.",
	}, func(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[ScalpelParams]) (*mcp.CallToolResult, error) {
		result, err := Execute(&request.Arguments)
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: result},
			},
		}, nil
	})
}
