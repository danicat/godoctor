package scalpel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Execute runs the specified scalpel command.
func Execute(params *ScalpelParams) (string, error) {
	if params.FilePath == "" {
		return "", fmt.Errorf("file_path is a required parameter for all scalpel operations")
	}

	originalContent, err := os.ReadFile(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", params.FilePath, err)
	}

	var newContent []byte
	var startOffset, endOffset int

	switch params.Operation {
	case "insert":
		if params.Start == nil {
			return "", fmt.Errorf("insert operation failed: the 'start' position is required")
		}
		offset, err := positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", fmt.Errorf("failed to calculate insert offset for %+v: %w", *params.Start, err)
		}
		newContent = make([]byte, 0, len(originalContent)+len(params.Content))
		newContent = append(newContent, originalContent[:offset]...)
		newContent = append(newContent, []byte(params.Content)...)
		newContent = append(newContent, originalContent[offset:]...)
	case "delete":
		if params.Start == nil || params.End == nil {
			return "", fmt.Errorf("delete operation failed: 'start' and 'end' positions are required")
		}
		startOffset, err = positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", fmt.Errorf("failed to calculate start offset for delete for %+v: %w", *params.Start, err)
		}
		endOffset, err = positionToOffset(originalContent, *params.End)
		if err != nil {
			return "", fmt.Errorf("failed to calculate end offset for delete for %+v: %w", *params.End, err)
		}
		if startOffset > endOffset {
			return "", fmt.Errorf("invalid range: start offset %d cannot be greater than end offset %d", startOffset, endOffset)
		}
		newContent = make([]byte, 0, len(originalContent)-(endOffset-startOffset))
		newContent = append(newContent, originalContent[:startOffset]...)
		newContent = append(newContent, originalContent[endOffset:]...)
	case "replace":
		if params.Start == nil || params.End == nil {
			return "", fmt.Errorf("replace operation failed: 'start' and 'end' positions are required")
		}
		startOffset, err = positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", fmt.Errorf("failed to calculate start offset for replace for %+v: %w", *params.Start, err)
		}
		endOffset, err = positionToOffset(originalContent, *params.End)
		if err != nil {
			return "", fmt.Errorf("failed to calculate end offset for replace for %+v: %w", *params.End, err)
		}
		if startOffset > endOffset {
			return "", fmt.Errorf("invalid range: start offset %d cannot be greater than end offset %d", startOffset, endOffset)
		}
		newContent = make([]byte, 0, len(originalContent)-(endOffset-startOffset)+len(params.Content))
		newContent = append(newContent, originalContent[:startOffset]...)
		newContent = append(newContent, []byte(params.Content)...)
		newContent = append(newContent, originalContent[endOffset:]...)
	case "replaceAll":
		if params.Pattern == "" || params.Replacement == "" {
			return "", fmt.Errorf("replaceAll operation failed: 'pattern' and 'replacement' are required")
		}
		re, err := regexp.Compile(params.Pattern)
		if err != nil {
			return "", fmt.Errorf("failed to compile regex pattern: %w", err)
		}
		newContent = re.ReplaceAll(originalContent, []byte(params.Replacement))
	case "search":
		if params.Pattern == "" {
			return "", fmt.Errorf("search operation failed: 'pattern' is required")
		}
		re, err := regexp.Compile(params.Pattern)
		if err != nil {
			return "", fmt.Errorf("failed to compile regex pattern: %w", err)
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
			return "", fmt.Errorf("failed to marshal search matches to JSON: %w", err)
		}
		return string(jsonMatches), nil
	case "read":
		if params.Start == nil && params.End == nil {
			return formatWithLineNumbers(originalContent), nil
		}
		if params.Start == nil || params.End == nil {
			return "", fmt.Errorf("read operation failed: both 'start' and 'end' positions must be provided, or neither")
		}
		startOffset, err = positionToOffset(originalContent, *params.Start)
		if err != nil {
			return "", fmt.Errorf("failed to calculate start offset for read for %+v: %w", *params.Start, err)
		}
		endOffset, err = positionToOffset(originalContent, *params.End)
		if err != nil {
			return "", fmt.Errorf("failed to calculate end offset for read for %+v: %w", *params.End, err)
		}
		if startOffset > endOffset {
			return "", fmt.Errorf("invalid range: start offset %d cannot be greater than end offset %d", startOffset, endOffset)
		}
		return formatWithLineNumbers(originalContent[startOffset:endOffset]), nil
	default:
		return "", fmt.Errorf("unknown operation: %q", params.Operation)
	}

	if err := os.WriteFile(params.FilePath, newContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return formatWithLineNumbers(newContent), nil
}

// positionToOffset converts a Position (line/column or offset) to a byte offset.
func positionToOffset(content []byte, pos Position) (int, error) {
	if pos.Offset != 0 {
		if pos.Offset < 0 || pos.Offset > len(content) {
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

	if pos.Column-1 > len(lines[pos.Line-1]) {
		return 0, fmt.Errorf("column %d is out of bounds for line %d", pos.Column, pos.Line)
	}
	offset += pos.Column - 1

	return offset, nil
}

func formatWithLineNumbers(content []byte) string {
	var builder strings.Builder
	lines := bytes.Split(content, []byte("\n"))
	for i, line := range lines {
		if i == len(lines)-1 && len(line) == 0 {
			continue
		}
		builder.WriteString(fmt.Sprintf("%d: %s\n", i+1, line))
	}
	return builder.String()
}

// Register registers the scalpel tool with the MCP server.
// It handles incoming tool calls, executes the corresponding scalpel operation,
// and returns the result or an error. Errors from the Execute function are
// propagated back to the client.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scalpel",
        Description: "A tool for surgical file editing. It operates on a file specified by `file_path` and performs an `operation`.\n\n**Operations:**\n\n*   **`insert`**: Inserts `content` at a specific position.\n    *   Requires: `file_path`, `operation: \"insert\"`, `content`, `start`.\n    *   `start`: A `Position` object (`{line, column}` or `offset`) specifying where to insert.\n*   **`delete`**: Deletes a range of content.\n    *   Requires: `file_path`, `operation: \"delete\"`, `start`, `end`.\n    *   `start`, `end`: `Position` objects defining the range to delete.\n*   **`replace`**: Replaces a range of content with new `content`.\n    *   Requires: `file_path`, `operation: \"replace\"`, `content`, `start`, `end`.\n    *   `start`, `end`: `Position` objects defining the range to replace.\n*   **`replaceAll`**: Replaces all occurrences of a `pattern` with a `replacement`.\n    *   Requires: `file_path`, `operation: \"replaceAll\"`, `pattern` (regex), `replacement`.\n*   **`search`**: Searches for a `pattern` (regex) and returns match details.\n    *   Requires: `file_path`, `operation: \"search\"`, `pattern`.\n    *   Returns: A JSON array of `Match` objects, each with range and captured groups.\n*   **`read`**: Reads the entire file or a specific range.\n    *   Requires: `file_path`, `operation: \"read\"`.\n    *   Optional: `start`, `end` `Position` objects to read a specific range.\n\n**`Position` Object:**\nA position can be specified in two ways:\n1.  `{ \"offset\": <number> }`: A zero-based byte offset from the beginning of the file.\n2.  `{ \"line\": <number>, \"column\": <number> }`: A one-based line and column number.",
	}, func(ctx context.Context, session *mcp.ServerSession, request *mcp.CallToolParamsFor[ScalpelParams]) (*mcp.CallToolResult, error) {
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