
## General Workflow

Before executing the first task the user asks, you should read the README.md file at least once.

When working with a Go codebase, a typical workflow involves understanding the code, making changes, and then reviewing those changes. The godoctor tools are designed to assist with each of these stages.

## Tool: get_documentation

Retrieves documentation for a specified Go package or a specific symbol (like a function or type). This is the primary tool for code comprehension and exploration. Use it to understand a package's public API, function signatures, and purpose before attempting to use or modify it.

### When to Use

Use the get_documentation tool whenever you need to understand a piece of Go code. This could be before you modify it, when you are trying to debug it, or when you are exploring a new codebase. It is your primary tool for code comprehension.

**Key Scenarios:**

- **Before Modifying Code:** Before changing a function or type, use get_documentation to understand its purpose, parameters, and return values.
- **Debugging:** When you encounter a bug, use get_documentation to inspect the functions involved and understand their expected behavior.
- **Code Exploration:** When you are new to a project, use get_documentation to explore the public API of different packages.

### How to Use

The get_documentation tool takes a package_path and an optional symbol_name. See the tool's description for detailed parameter information.

## Tool: write_code

Creates or replaces an entire Go source file with the provided content. Use this tool when the extent of edits to a file is substantial, affecting more than 25% of the file's content. It automatically formats the code and manages imports.

### When to Use

Use the write_code tool to create new Go source files. This tool ensures that the file is created with the correct content and also checks for any initial errors.

**Key Scenarios:**

- **Creating a new Go file:** When you need to create a new Go file with some initial content.

### How to Use

The write_code tool takes the path of the Go file to create and the content of the file as input. See the tool's description for detailed parameter information.

## Tool: edit_code

Edits a Go source file by replacing the first occurrence of a specified 'old_string' with a 'new_string'. Use this for surgical edits like adding, deleting, or renaming code when the changes affect less than 25% of the file. To ensure precision, the 'old_string' must be a unique anchor string that includes enough context to target only the desired location.

### When to Use

Use the edit_code tool to edit existing Go source files. This tool is useful for making small changes to a file, such as renaming a variable or changing a function signature.

**Key Scenarios:**

- **Refactoring:** When you are refactoring code, use the edit_code tool to make small, targeted changes.
- **Fixing Bugs:** When you are fixing a bug, use the edit_code tool to apply a patch to a file.

### How to Use

The edit_code tool takes the path of the Go file to edit, the old string to replace, and the new string to replace it with. See the tool's description for detailed parameter information.

## Tool: review_code

Performs an expert code review of Go source code. The tool returns a JSON array of suggestions, each containing a 'line_number', a 'finding' describing the issue, and a 'comment' with a recommendation. Use this tool to verify the quality of your changes before finalizing your work.

### When to Use

Use the review_code tool after you have made changes to the code and before you commit them. This tool acts as an expert Go developer, providing feedback on your changes to ensure they meet the standards of the Go community.

**Key Scenarios:**

- **After Making Changes:** Once you have implemented a new feature or fixed a bug, use the review_code tool to get feedback on your work.
- **Improving Code Quality:** If you are refactoring code, use the review_code tool to ensure your changes are an improvement.
- **Learning Go:** The review_code tool is a great way to learn idiomatic Go. By reviewing your code, you can see where you are deviating from best practices.

### How to Use

The review_code tool takes the content of a Go file as input. See the tool's description for detailed parameter information.

## Tool: crawl_webpage

Crawls a website to a specified depth, returning the text-only content of each page. This tool is useful for extracting documentation from websites.

### When to Use

Use the crawl_webpage tool when you need to retrieve the content of a website. This could be to extract documentation, to analyze the content of a page, or to answer questions about a website's content.

**Key Scenarios:**

- **Extracting Documentation:** Use the crawl_webpage tool to retrieve the content of a web page and then use a large language model to extract documentation.
- **Content Analysis:** The crawl_webpage tool can be used to retrieve the content of a website for analysis. This could be to identify keywords, to extract data, or to perform sentiment analysis.
- **Answering Questions:** The crawl_webpage tool can be used to retrieve the content of a website to answer questions about it. For example, you could use it to find the contact information for a company or to get the latest news from a news website.

### How to Use

The crawl_webpage tool takes a URL, a recursion level, and a boolean to indicate whether to crawl external sites. See the tool's description for detailed parameter information.
