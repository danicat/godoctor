# GoDoctor Agent Instructions

This document provides instructions for an AI agent on how to effectively use the `godoctor` tool suite.

## General Workflow

When working with a Go codebase, a typical workflow involves understanding the code, making changes, and then reviewing those changes. The godoctor tools are designed to assist with each of these stages.

## Tool: godoc

### Description
Retrieves Go documentation for a specified package and, optionally, a specific symbol within that package. This tool is useful for understanding the functionality of a Go package or a specific symbol (function, type, etc.) within it.

### When to Use

Use the `godoc` tool whenever you need to understand a piece of Go code. This could be before you modify it, when you are trying to debug it, or when you are exploring a new codebase. It is your primary tool for code comprehension.

**Key Scenarios:**

- **Before Modifying Code:** Before changing a function or type, use `godoc` to understand its purpose, parameters, and return values.
- **Debugging:** When you encounter a bug, use `godoc` to inspect the functions involved and understand their expected behavior.
- **Code Exploration:** When you are new to a project, use `godoc` to explore the public API of different packages.

### How to Use

The `godoc` tool takes the following parameters:
- `package_path` (string, required): The full import path of the Go package (e.g., "fmt", "github.com/spf13/cobra").
- `symbol_name` (string, optional): The name of a specific symbol within the package (e.g., "Println", "Command").

## Tool: gopretty

### Description
Formats a Go source file using goimports and gofmt. This tool is useful for ensuring that your code adheres to Go's formatting standards.

### When to Use

Use the `gopretty` tool to format your Go code. This tool runs both `goimports` and `gofmt` on a file to ensure it is correctly formatted and all necessary imports are present.

**Key Scenarios:**

- **After Making Changes:** After you have modified a file, run `gopretty` on it to ensure it is correctly formatted.
- **Before Committing:** Before you commit your changes, run `gopretty` on all the files you have changed to ensure they are all correctly formatted.

### How to Use

The `gopretty` tool takes the following parameter:
- `file_path` (string, required): The path of a Go file to format.

## Tool: scribble

### Description
Writes content to a new Go source file and checks it for errors. This tool should be used whenever you are creating a new Go file.

### When to Use

Use the `scribble` tool to create new Go source files. This tool ensures that the file is created with the correct content and also checks for any initial errors.

**Key Scenarios:**

- **Creating a new Go file:** When you need to create a new Go file with some initial content.

### How to Use

The `scribble` tool takes the following parameters:
- `file_path` (string, required): The path of the Go file to create.
- `content` (string, required): The content of the Go file.

## Tool: code_review

### Description
Provides an expert-level, AI-powered review of a given Go source file. This tool is useful for improving code quality before committing changes.

### When to Use

Use the `code_review` tool after you have made changes to the code and before you commit them. This tool acts as an expert Go developer, providing feedback on your changes to ensure they meet the standards of the Go community.

**Key Scenarios:**

- **After Making Changes:** Once you have implemented a new feature or fixed a bug, use the `code_review` tool to get feedback on your work.
- **Improving Code Quality:** If you are refactoring code, use the `code_review` tool to ensure your changes are an improvement.
- **Learning Go:** The `code_review` tool is a great way to learn idiomatic Go. By reviewing your code, you can see where you are deviating from best practices.

### How to Use

The `code_review` tool takes the following parameters:
- `file_content` (string, required): The full content of the Go source file to be reviewed.
- `model_name` (string, optional): The specific generative AI model to use for the review. If omitted, it defaults to a pre-configured model.
- `hint` (string, optional): A natural language hint to guide the AI's review, focusing it on a specific concern (e.g., performance, clarity, error handling).