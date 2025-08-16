package prompts

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const describePrompt = `
## General Workflow

When working with a Go codebase, a typical workflow involves understanding the code, making changes, and then reviewing those changes. The godoctor tools are designed to assist with each of these stages.

## Tool: godoc

Retrieves documentation for a specified Go package or a specific symbol (like a function or type). This is the primary tool for code comprehension and exploration. Use it to understand a package's public API, function signatures, and purpose before attempting to use or modify it.

### When to Use

Use the godoc tool whenever you need to understand a piece of Go code. This could be before you modify it, when you are trying to debug it, or when you are exploring a new codebase. It is your primary tool for code comprehension.

**Key Scenarios:**

- **Before Modifying Code:** Before changing a function or type, use godoc to understand its purpose, parameters, and return values.
- **Debugging:** When you encounter a bug, use godoc to inspect the functions involved and understand their expected behavior.
- **Code Exploration:** When you are new to a project, use godoc to explore the public API of different packages.

### How to Use

The godoc tool takes a package_path and an optional symbol_name. See the tool's description for detailed parameter information.

## Tool: scribble

Creates or replaces an entire Go source file with the provided content. Use this tool when the extent of edits to a file is substantial, affecting more than 25% of the file's content. It automatically formats the code and manages imports.

### When to Use

Use the scribble tool to create new Go source files. This tool ensures that the file is created with the correct content and also checks for any initial errors.

**Key Scenarios:**

- **Creating a new Go file:** When you need to create a new Go file with some initial content.

### How to Use

The scribble tool takes the path of the Go file to create and the content of the file as input. See the tool's description for detailed parameter information.

## Tool: scalpel

Edits a Go source file by replacing the first occurrence of a specified 'old_string' with a 'new_string'. Use this for surgical edits like adding, deleting, or renaming code when the changes affect less than 25% of the file. To ensure precision, the 'old_string' must be a unique anchor string that includes enough context to target only the desired location.

### When to Use

Use the scalpel tool to edit existing Go source files. This tool is useful for making small changes to a file, such as renaming a variable or changing a function signature.

**Key Scenarios:**

- **Refactoring:** When you are refactoring code, use the scalpel tool to make small, targeted changes.
- **Fixing Bugs:** When you are fixing a bug, use the scalpel tool to apply a patch to a file.

### How to Use

The scalpel tool takes the path of the Go file to edit, the old string to replace, and the new string to replace it with. See the tool's description for detailed parameter information.

## Tool: code_review

Performs an expert code review of Go source code. The tool returns a JSON array of suggestions, each containing a 'line_number', a 'finding' describing the issue, and a 'comment' with a recommendation. Use this tool to verify the quality of your changes before finalizing your work.

### When to Use

Use the code_review tool after you have made changes to the code and before you commit them. This tool acts as an expert Go developer, providing feedback on your changes to ensure they meet the standards of the Go community.

**Key Scenarios:**

- **After Making Changes:** Once you have implemented a new feature or fixed a bug, use the code_review tool to get feedback on your work.
- **Improving Code Quality:** If you are refactoring code, use the code_review tool to ensure your changes are an improvement.
- **Learning Go:** The code_review tool is a great way to learn idiomatic Go. By reviewing your code, you can see where you are deviating from best practices.

### How to Use

The code_review tool takes the content of a Go file as input. See the tool's description for detailed parameter information.
`


// Describe creates the definition for the 'describe' prompt.
func Describe(namespace string) *mcp.Prompt {
	name := "describe"
	if namespace != "" {
		name = namespace + ":" + name
	}
	return &mcp.Prompt{
		Name:        name,
		Description: "Provides instructions on how to use the godoctor tools.",
	}
}

// DescribeHandler is the handler that generates the content for the 'describe' prompt.
func DescribeHandler(_ context.Context, _ *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: describePrompt,
				},
			},
		},
	}, nil
}
