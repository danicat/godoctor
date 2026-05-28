# GoDoctor

**GoDoctor** is an intelligent, AI-powered Model Context Protocol (MCP) server designed to assist Go developers. It provides a comprehensive suite of tools for navigating, editing, analyzing, and modernizing Go codebases.

## Features

GoDoctor organizes its capabilities into domain-specific tools to streamline development workflows.

### 🔍 Navigation & Discovery
*   **`list_files`**: Explore the project hierarchy recursively to understand the architecture.
*   **`smart_read`**: **The Universal Reader.** Inspects file content, structure (outline), or specific snippets. Parses AST and appends definitions of referenced types.
*   **`describe_symbol`**: Deep semantic analysis of declarations, comments, and references.

### ✏️ Smart Editing
*   **`smart_edit`**: Perform targeted, atomic code modifications across multiple files. Natively handles file creation, formatting, and syntax validation under a transactional compiler gate.

### 🛠️ Go Toolchain & Quality Gate
*   **`smart_build`**: **The Universal Quality Gate.** A complete pipeline that tidies modules, automatically modernizes legacy Go patterns, formats, compiles, runs tests, and lints in a single atomic step.
*   **`add_dependency`**: Manage module dependencies and immediately fetch documentation for the new package.
*   **`read_docs`**: Query documentation for any package or symbol in the Go ecosystem.

### 🧪 Testing
*   **`mutation_test`**: Assess test suite coverage quality by injecting artificial mutations.
*   **`test_query`**: Execute SQL queries over test coverage results and execution timelines.


## Installation

### For Claude Code Users

1.  **Install the binary:**
    ```bash
    go install github.com/danicat/godoctor/cmd/godoctor@latest
    ```

2.  **Add GoDoctor as an MCP server:**
    ```bash
    claude mcp add --transport stdio --scope user godoctor -- godoctor
    ```

3.  **(Optional) Add agent instructions to your project:**
    ```bash
    godoctor --agents >> CLAUDE.md
    ```
    This appends tool usage guidance to your `CLAUDE.md` so Claude knows how to use each tool effectively.

### For Gemini CLI Users

If you use the [Gemini CLI](https://github.com/google/gemini-cli), you can install GoDoctor as an extension:

```bash
gemini extensions install https://github.com/danicat/godoctor
```

### From Source

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/danicat/godoctor.git
    cd godoctor
    ```
2.  **Build and install:**
    ```bash
    make install
    ```

## Configuration

### Command-Line Flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--model` | Default Gemini model to use for AI tasks. | `gemini-2.5-pro` |
| `--allow` | Comma-separated list of tools to explicitly **enable** (whitelist mode). | `""` |
| `--disable` | Comma-separated list of tools to explicitly **disable**. | `""` |
| `--listen` | Address to listen on for HTTP transport (e.g., `:8080`). | `""` (Stdio) |
| `--list-tools`| List all available tools and their descriptions, then exit. | `false` |
| `--agents` | Print LLM agent instructions and exit. | `false` |
| `--version` | Print version and exit. | `false` |

## Agent Integration

To get the optimal system prompt for your AI agent:

```bash
godoctor --agents
```

To see which tools are currently active:

```bash
godoctor --list-tools
```

## License

Apache 2.0

### 🛡️ Hook System (Gemini CLI)
When installed in Gemini CLI, GoDoctor features a built-in intelligent hook system that intercepts raw, inefficient shell operations (like `go build`, `sed`, `cat`) or native tools (like `replace`, `read_file`) and forces the agent to use GoDoctor's context-aware, robust tools (`smart_build`, `smart_edit`, `smart_read`). This ensures the quality gate pipeline is respected and cognitive drift is minimized.
