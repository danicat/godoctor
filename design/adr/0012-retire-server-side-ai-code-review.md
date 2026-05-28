# ADR-0012: Retire Server-Side AI Code Review

- **Status:** Approved
- **Date:** 2026-05-19
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6

## 1. Context
GoDoctor previously provided a server-side AI-based tool named `code_review` to analyze source files and provide architectural and concurrency critiques. While helpful in isolated workflows, this tool was a heavy server dependency:
- It required server-side configuration of Generative AI API credentials (`GOOGLE_API_KEY`, Vertex AI configs).
- It introduced significant token overhead and latency for large files.
- It duplicated features that the calling AI client or IDE agent could execute much better directly within its own native prompt cycle, using its own cross-file context.

We needed to simplify the server's external dependencies and focus on local, compiler-driven tools.

## 2. Decision
We decided to retire the `code_review` tool completely from the GoDoctor MCP registry. The codebase packages (`internal/tools/agent/review/...`) were deleted, and registration logic was removed from `internal/server/server.go`.

## 3. Consequences
- **Positive:** Major reduction in server complexity. Eliminates the requirement to maintain and secure AI API keys at the MCP server level. Lowers token usage.
- **Negative:** AI clients must now perform code critiques using their own internal prompts rather than delegating them to a custom GoDoctor-curated model endpoint.
- **Neutral:** Completely clarifies GoDoctor's scope as a local, compiler-gated software engineering assistant rather than a general-purpose AI chat server.
