# ADR-0001: Symbol-Based Go Documentation Retrieval

- **Status:** Superseded by ADR-0002
- **Date:** 2025-10-15
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Gemini CLI

## 1. Context
At the inception of the GoDoctor project, the primary goal was to provide an AI agent with basic visibility into Go packages in a workspace. AI models had no direct way to access package comments or function signatures within local or external Go modules, resulting in package API assumptions and syntax hallucinations.

We needed a lightweight mechanism to retrieve simple documentation from Go codebases using standard toolchains.

## 2. Decision
We decided to implement a minimal Stdio MCP tool named `getDoc` that executed basic symbol-based documentation lookups. The tool accepted a symbol name string and performed a regex search/plain-text extraction across the workspace's Go files.

## 3. Consequences
- **Positive:** Established the initial proof of concept of GoDoctor as an MCP server. Provided basic symbol inspection.
- **Negative:** Highly fragile. Simple string-based regex parsing had no package-path awareness, failed on external module references, and was easily confused by identically-named symbols in different packages.
- **Neutral:** Superseded by **ADR-0002** (Standalone GoDoc Integration) to utilize Go's native toolchain for robust, package-aware queries.
