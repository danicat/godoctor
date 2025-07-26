# GoDoctor

GoDoctor is an intelligent, AI-powered companion for the modern Go developer. It integrates seamlessly with AI-powered IDEs and other development tools through the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/), providing a suite of powerful features to enhance your workflow.

This project was developed and refined through an iterative process of AI-driven self-review, where GoDoctor's own code review tool was used to improve its own source code.

## Features

*   **AI-Powered Code Review:** Get instant, context-aware feedback on your Go code. The `code_review` tool analyzes your code for quality, clarity, and adherence to Go best practices.
*   **On-Demand Documentation:** Instantly retrieve documentation for any symbol in the Go standard library or your project's dependencies using the `go-doc` tool.
*   **Flexible Transports:** Communicate with the `godoctor` server via standard I/O or over the network with a new HTTP mode.
*   **Powerful CLI:** A command-line interface (`godoctor-cli`) for direct interaction with the GoDoctor server, either by launching a local process or connecting to a remote one.
*   **MCP Compliant:** Built on the Model Context Protocol for broad compatibility with modern development tools.

## Installation

1.  **Prerequisites:**
    *   Go 1.18 or later
    *   `make`
    *   A Gemini API Key (for the code review tool).

2.  **Clone and Build:**
    ```bash
    git clone https://github.com/danicat/godoctor.git
    cd godoctor
    make build
    ```
    This will create the `godoctor` server and `godoctor-cli` client in the `bin/` directory.

## Usage

### API Key Configuration

The `code_review` tool requires a Gemini API Key to function. The server can be configured to find this key in two ways:

1.  **Default Environment Variable (Recommended)**: By default, the `godoctor` server will look for the API key in an environment variable named `GEMINI_API_KEY`.

    ```bash
    # Set the default environment variable
    export GEMINI_API_KEY="your-api-key"

    # Run the server (it will find the key automatically)
    ./bin/godoctor --listen :8080
    ```

2.  **Custom Environment Variable (Override)**: You can tell the server to look for the key in a different environment variable by using the `--api-key-env` flag. This is useful for environments where you might have multiple keys or need to avoid naming conflicts.

    ```bash
    # Set a custom environment variable
    export MY_APP_API_KEY="your-api-key"

    # Tell the server to use the custom variable
    ./bin/godoctor --listen :8080 --api-key-env MY_APP_API_KEY
    ```

### Server (`godoctor`)

The `godoctor` binary is the MCP server. It can be run in two modes.

*   **Standard I/O (default):** The server communicates over `stdin` and `stdout`. This is the mode used when the `godoctor-cli` starts it automatically.
*   **HTTP Mode:** The server can listen for connections on a network port.

```bash
# Run the server in HTTP mode, listening on port 8080
./bin/godoctor --listen :8080
```

#### Agent Instructions

The `godoctor` server includes a special `-instructions` flag designed to help configure AI agents. When used, this flag prints a detailed guide on when and how to use the `go-doc` and `code-review` tools and then exits. This output is ideal for inclusion in an agent's configuration file (e.g., `GEMINI.md`).

```bash
# Print the agent instructions
./bin/godoctor -instructions
```
This command takes precedence over all other flags.

### Client (`godoctor-cli`)

The `godoctor-cli` is the primary way to interact with GoDoctor from the command line.

By default, it finds and starts a `godoctor` server process for you. Alternatively, you can connect to a server that is already running in HTTP mode.

#### Code Review

```bash
# Review a file using a local, auto-started server
./bin/godoctor-cli -review cmd/godoctor/main.go

# Review a file by connecting to a running HTTP server
./bin/godoctor-cli --http-server http://localhost:8080 -review cmd/godoctor/main.go

# Review code from stdin with a hint
git diff --staged | ./bin/godoctor-cli -review - -hint "Focus on improving error handling"
```

#### Get Documentation

```bash
# Get package documentation for 'fmt' using an auto-started server
./bin/godoctor-cli fmt

# Get documentation for 'fmt.Println' from a running HTTP server
./bin/godoctor-cli --http-server http://localhost:8080 fmt Println
```

### Help

For a full list of commands and flags:

```bash
./bin/godoctor-cli -help
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