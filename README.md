This is not an officially supported Google product.

# GoDoctor

<p align="center">
  <img src="logo.png" alt="GoDoctor Logo" width="200"/>
</p>

GoDoctor is an intelligent, AI-powered companion for the modern Go developer. It integrates seamlessly with AI-powered IDEs and other development tools through the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/), providing a suite of powerful features to enhance your workflow.

This project was developed and refined through an iterative process of AI-driven self-review, where GoDoctor's own code review tool was used to improve its own source code.

## Features

*   **AI-Powered Code Review:** Get instant, context-aware feedback on your Go code. The `review_code` tool analyzes your code for quality, clarity, and adherence to Go best practices.
*   **On-Demand Documentation:** Instantly retrieve documentation for any symbol in the Go standard library or your project's dependencies using the `get_documentation` tool.
*   **Code Manipulation:** The `write_code` and `edit_code` tools allow you to create and edit Go source files.
*   **Flexible Transports:** Communicate with the `godoctor` server via standard I/O or over the network with a new HTTP mode.
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
    This will create the `godoctor` server in the `bin/` directory. You can also run `make install` to install the binary in your `$GOPATH/bin` directory.

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

*   **Standard I/O (default):** The server communicates over `stdin` and `stdout`.
*   **HTTP Mode:** The server can listen for connections on a network port.

```bash
# Run the server in HTTP mode, listening on port 8080
./bin/godoctor --listen :8080
```



## Development

This project follows the standard Go project layout.

*   `cmd/godoctor`: The source code for the MCP server.
*   `internal/tool`: The implementation of the `review_code`, `get_documentation`, `write_code`, and `edit_code` tools.

To run the test suite:

```bash
make test
```

### Manual Tests

You can send the JSON-RPC payloads directly to the server process if it is running in stdio mode. Example:

```sh
(
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}';
  echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}';
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}';
) | ./bin/godoctor
```

And for tool calls:

```sh
(
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}';
  echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}';
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_documentation", "arguments":{"package":"fmt"}}}';
) | ./bin/godoctor
```