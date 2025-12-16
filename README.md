This is not an officially supported Google product.

# GoDoctor

<p align="center">
  <img src="logo.png" alt="GoDoctor Logo" width="200"/>
</p>

GoDoctor is an intelligent, AI-powered companion for the modern Go developer. It integrates seamlessly with AI-powered IDEs and other development tools through the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/), providing a suite of powerful features to enhance your workflow.

This project was developed and refined through an iterative process of AI-driven self-review, where GoDoctor's own code review tool was used to improve its own source code.

## Features

*   **AI-Powered Code Review:** Get instant, context-aware feedback on your Go code. The `code_review` tool analyzes your code for quality, clarity, and adherence to Go best practices.
*   **On-Demand Documentation:** Instantly retrieve documentation for any symbol in the Go standard library or your project's dependencies using the `get_docs` tool.
*   **Flexible Transports:** Communicate with the `godoctor` server via standard I/O or over the network with a new HTTP mode.
*   **MCP Compliant:** Built on the Model Context Protocol for broad compatibility with modern development tools.

## Installation

1.  **Prerequisites:**
    *   Go 1.24 or later
    *   `make`
    *   A Gemini API Key (for the code review tool) OR Google Cloud Vertex AI credentials.

2.  **Clone and Build:**
    ```bash
    git clone https://github.com/danicat/godoctor.git
    cd godoctor
    make build
    ```
    This will create the `godoctor` server in the `bin/` directory. You can also run `make install` to install the binary in your `$GOPATH/bin` directory.

## Usage

### Authentication

The `code_review` tool uses the Google Gen AI SDK. You can authenticate in one of two ways:

1.  **Gemini API (Recommended for Personal Use):**
    Set the `GOOGLE_API_KEY` (or `GEMINI_API_KEY`) environment variable.
    ```bash
    export GOOGLE_API_KEY="your-api-key"
    ```

2.  **Vertex AI (Recommended for Enterprise):**
    Set `GOOGLE_GENAI_USE_VERTEXAI=true` and provide your Google Cloud Project ID and Location. The SDK will then use Application Default Credentials (ADC) for authentication.
    ```bash
    export GOOGLE_GENAI_USE_VERTEXAI=true
    export GOOGLE_CLOUD_PROJECT="your-project-id"
    export GOOGLE_CLOUD_LOCATION="us-central1"
    gcloud auth application-default login
    ```

### Configuration

You can configure the server using command-line flags:

*   `--listen`: Address to listen on for HTTP transport (e.g., `:8080`). If omitted, uses Standard I/O.
*   `--model`: Default Gemini model to use (default: `gemini-2.5-pro`).
*   `--version`: Print the version and exit.

### Running the Server

**Standard I/O (Default):**
```bash
export GOOGLE_API_KEY="your-api-key"
./bin/godoctor
```

**HTTP Mode:**
```bash
export GOOGLE_API_KEY="your-api-key"
./bin/godoctor --listen :8080
```

## Development

This project follows the standard Go project layout.

*   `cmd/godoctor`: The source code for the MCP server.
*   `internal/tools`: The implementation of the available tools:
    *   `codereview`: AI-powered code analysis using Gemini/Vertex AI.
    *   `getdocs`: Documentation retrieval wrapper for `go doc`.
*   `internal/server`: The core MCP server implementation.
*   `internal/config`: Configuration handling.

To run the test suite:

```bash
make test
```

To run lint checks:

```bash
golangci-lint run
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
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_docs", "arguments":{"package_path":"fmt"}}}';
) | ./bin/godoctor
```