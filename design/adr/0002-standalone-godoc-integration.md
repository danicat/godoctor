# ADR-0002: Standalone GoDoc Toolchain Integration

- **Status:** Approved
- **Date:** 2025-10-25
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Gemini CLI

## 1. Context
Following the limitations of basic symbol lookups (ADR-0001), we needed a reliable, standard-compliant way for AI agents to query external package documentation and local struct specifications. In Go, the official and standard way to retrieve documentation is via the `go doc` command.

We needed a tool to interface natively with the Go toolchain to fetch authoritative documentation.

## 2. Decision
We decided to replace the symbol-based regex lookup with a robust tool called `read_docs` (originally `godoc` / `go-doc`). The tool accepts a target `import_path` (e.g. `net/http`) and an optional `symbol` name. It runs `go doc` natively in the local environment and streams the structured package declaration, exported types, function signatures, and package-level comments back to the agent.

## 3. Consequences
- **Positive:** Leverages Go's authoritative, built-in toolchain. Guarantees 100% accurate API specifications and eliminates docstring hallucinations.
- **Negative:** Dependent on a local Go SDK installation on the server host. Requires the target packages to be fully cached or resolved in the `go.mod` module cache to load.
- **Neutral:** Established the toolchain-integration pattern that represents GoDoctor's core architecture.
