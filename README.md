# GoDoctor

[![Go Report Card](https://goreportcard.com/badge/github.com/danicat/godoctor)](https://goreportcard.com/report/github.com/danicat/godoctor)

GoDoctor is an intelligent, AI-powered companion for the modern Go developer. It integrates seamlessly with AI-powered IDEs and other development tools through the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/), providing a suite of powerful features to enhance your workflow.

Whether you need instant access to documentation, automated code reviews, or intelligent code analysis, GoDoctor is designed to be your go-to assistant, helping you write better Go code, faster.

## Features

*   **MCP Compliant:** Built on the Model Context Protocol for broad compatibility.
*   **Documentation On-Demand:** Instantly retrieve documentation for any symbol in the Go standard library or your project's dependencies.

## Roadmap

GoDoctor is an evolving project. Here are some of the features planned for the near future:

*   **Automated Code Reviews:** Get instant feedback on your code, with suggestions for improvement based on Go best practices.
*   **Advanced Code Analysis:** Perform complex analysis of your codebase to identify potential issues, security vulnerabilities, and performance bottlenecks.
*   **Intelligent Refactoring:** Get smart suggestions for refactoring your code to improve its structure and readability.

## User Instructions

These instructions are for users who want to use GoDoctor with an MCP-compatible client, such as the Gemini CLI.

### Prerequisites

*   Go 1.18 or later.
*   `make`

### Installation and Configuration

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/danicat/godoctor.git
    cd godoctor
    ```

2.  **Build the server:**
    ```bash
    make build
    ```
    This will create the `godoctor` binary in the `bin/` directory.

3.  **Configure your MCP Client:**
    Configure your MCP client to use the `godoctor` executable located at `bin/godoctor`. The exact steps will depend on your client. For example, in the Gemini CLI, you would configure the path to the `godoctor` binary in your settings.

### Versioning

To check the version of the `godoctor` server or the `godoctor-cli` client, use the `-version` flag:

```bash
./bin/godoctor -version
./bin/godoctor-cli -version
```

## Developer Instructions

These instructions are for developers who want to contribute to GoDoctor.

### Project Structure

This project follows the standard Go project layout.

*   `cmd/godoctor`: The source code for the MCP server.
*   `cmd/godoctor-cli`: The source code for a simple command-line client used for testing and development.

### Getting Started

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/danicat/godoctor.git
    cd godoctor
    ```

2.  **Build the server and client:**
    ```bash
    make build
    ```

### Running Tests

To run the test suite, execute the following command from the root of the project:

```bash
make test
```

### Interacting with the Server

The `godoctor-cli` tool can be used to interact with the `godoctor` server for development and testing purposes. It takes a single argument: the fully qualified symbol name you want to look up.

*   **Get documentation for a standard library function:**
    ```bash
    ./bin/godoctor-cli fmt.Println
    ```

*   **Get documentation for a third-party package symbol:**
    ```bash
    ./bin/godoctor-cli github.com/google/uuid.New
    ```

*   **Get package-level documentation:**
    ```bash
    ./bin/godoctor-cli github.com/modelcontextprotocol/go-sdk/mcp
    ```

## License

This project is licensed under the [Apache License 2.0](LICENSE).