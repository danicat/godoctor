# GoDoctor

GoDoctor is an intelligent, AI-powered companion for the modern Go developer. It integrates seamlessly with AI-powered IDEs and other development tools through the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/), providing a suite of powerful features to enhance your workflow.

This project was developed and refined through an iterative process of AI-driven self-review, where GoDoctor's own code review tool was used to improve its own source code.

## Configuring for Gemini CLI

To use GoDoctor as a tool with the Gemini CLI, you need to make the `godoctor` binary accessible to the CLI.

1.  **Place in PATH:** Ensure the `godoctor` binary (located in the `bin/` directory after building) is in a directory that is part of your system's `PATH`. A common practice is to copy it to a dedicated user scripts directory:
    ```bash
    # After running 'make build'
    cp ./bin/godoctor ~/.local/bin/
    ```
    *Note: Ensure `~/.local/bin` is in your `PATH`.*

2.  **Configure Gemini CLI:** Open the Gemini CLI's configuration file (usually located at `~/.gemini/settings.json`) and add `godoctor` to your tool search paths.

## Features

*   **AI-Powered Code Review:** Get instant, context-aware feedback on your Go code. The `code_review` tool analyzes your code for quality, clarity, and adherence to Go best practices. You can guide the review with natural language hints (e.g., "focus on readability" or "check for security issues").
*   **On-Demand Documentation:** Instantly retrieve documentation for any symbol in the Go standard library or your project's dependencies using the `go-doc` tool.
*   **Flexible CLI:** A powerful and intuitive command-line interface (`godoctor-cli`) for direct interaction with the GoDoctor server.
*   **stdin Support:** Pipe code directly into the code reviewer from other commands (e.g., `git show HEAD:main.go | godoctor-cli -review -`).
*   **MCP Compliant:** Built on the Model Context Protocol for broad compatibility with modern development tools.

## Installation

1.  **Prerequisites:**
    *   Go 1.18 or later
    *   `make`
    *   A Gemini API Key (for the code review tool). Set it as an environment variable:
        ```bash
        export GEMINI_API_KEY="your-api-key"
        ```

2.  **Clone and Build:**
    ```bash
    git clone https://github.com/danicat/godoctor.git
    cd godoctor
    make build
    ```
    This will create the `godoctor` server and `godoctor-cli` client in the `bin/` directory.

## Usage

The `godoctor-cli` is the primary way to interact with GoDoctor from the command line.

### Code Review

Review a file by providing its path. You can use the `-hint` flag to guide the reviewer.

```bash
# Review a file
./bin/godo_ctor-cli -review cmd/godoctor-cli/main.go

# Review a file with a hint
./bin/godoctor-cli -review internal/tool/codereview/codereview.go -hint "Focus on improving error handling"
```

Review code from `stdin`:

```bash
# Review the current staging changes in git
git diff --staged | ./bin/godoctor-cli -review -
```

### Get Documentation

Retrieve documentation for a package or a specific symbol.

```bash
# Get package documentation for 'fmt'
./bin/godoctor-cli fmt

# Get documentation for 'fmt.Println'
./bin/godoctor-cli fmt Println
```

### Help

For a full list of commands and flags:

```bash
./bin/godoctor-cli -help
```

## Manual Testing

For development or debugging, you can run the `godoctor-cli` directly. It will automatically start the `godoctor` server process in the background.

The client finds the `godoctor` server binary by searching your system's `PATH`. For development, the most reliable method is to explicitly provide the path to the server binary using the `-server` flag.

You can run the client using `go run`:
```bash
# Example: Get documentation (requires 'godoctor' in your PATH)
go run ./cmd/godoctor-cli/main.go fmt Println

# Example: Review a file using the -server flag
go run ./cmd/godoctor-cli/main.go -server ./bin/godoctor -review cmd/godoctor/main.go
```

Or you can use the compiled binary after running `make build`:
```bash
# This requires './bin/godoctor' to be in your PATH
./bin/godoctor-cli fmt Println

# A more direct way for testing:
./bin/godoctor-cli -server ./bin/godoctor fmt Println
```

## Development

This project follows the standard Go project layout.

*   `cmd/godoctor`: The source code for the MCP server.
*   `cmd/godoctor-cli`: The source code for the command-line client.
*   `internal/tool`: The implementation of the `code_review` and `go-doc` tools.

To run the test suite:

```bash
make test
```

To run the integration test, which verifies the end-to-end functionality of the CLI and server:
```bash
make integration-test
```