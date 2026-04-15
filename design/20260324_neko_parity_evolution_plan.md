# GoDoctor Evolution Plan: Achieving Parity with Neko

## 1. Overview
`neko` originated as a polyglot spin-off of `godoctor` but has since dramatically outpaced it in terms of architectural sophistication, structural awareness, and AI-agent workflows. This plan outlines the strategy to backport the Go-specific advancements from `neko` to `godoctor`, elevating it from a simple code modifier into a true **Semantic Operating System for Go development**.

## 2. Phase 1: Core Architecture & LSP Integration
The most significant gap is Neko's stateful, LSP-driven architecture. GoDoctor currently operates mostly statelessly, missing out on real-time compiler feedback.

*   **Project Lifecycle**: Introduce the two-phase lifecycle (Lobby vs. Project Open) by implementing `open_project` and `close_project`.
*   **`gopls` Client Integration**: Port Neko's `internal/lsp` package to GoDoctor. This will establish a persistent connection to the Go Language Server.
*   **Synchronous Diagnostics**: Modify the editing pipeline to capture LSP diagnostics synchronously after every edit, enabling "Full Disclosure" reporting to the agent.
*   **RAG Engine (Optional but Recommended)**: Port `internal/core/rag` to enable local AST/Embedding-based `semantic_search` within Go projects.

## 3. Phase 2: Tool Upgrades & Semantic Awareness
GoDoctor's file modification tools need to be upgraded to match Neko's safety and precision.

*   **Read & Discovery Enhancements**:
    *   Port `multi_read` to drastically reduce token overhead when exploring large Go codebases.
    *   Enhance the existing `outline` mode to use AST-powered parsing that includes structural docstrings (struct fields, interface methods).
    *   Implement Virtual Semantic Annotations (e.g., `<GODOCTOR>` tags) to inject type signatures directly into read outputs.
*   **Surgical Modifiers**:
    *   Port `line_edit` for absolute, line-range specific replacements.
    *   Port `multi_edit` for transactional, cross-file atomic changes.
*   **Language Intelligence Tools**:
    *   Port `rename_symbol` to enforce deterministic, project-wide renames via `gopls`.
    *   Enhance `find_references` to categorize usages cleanly into `[SOURCE]` and `[TESTS]`.
    *   Add `describe` tool for contextual `hover` type analysis.

## 4. Phase 3: Security & Interaction Hooks
Neko forces agents into its "Quality Gate" by deliberately blocking generic CLI tools. GoDoctor must adopt this defensive posture.

*   **Hook System**: Port `neko/hooks` directory to `godoctor/hooks`.
*   **Extension Configuration**: Update GoDoctor's `gemini-extension.json` to block standard `write_file`, `replace`, and generic shell file operations.
*   **Enforcement**: Funnel all agent actions through GoDoctor's LSP-aware `edit_file`, `multi_edit`, and `build(auto_fix=true)` tools to ensure semantic integrity.

## 5. Phase 4: Agent Skills Alignment
GoDoctor's skills are fragmented compared to Neko's centralized, high-quality workflows.

*   **`godoctor-development-flow`**: Create a unified master skill by porting `neko-development-flow`. This will absorb the old `go-project-setup` and include the newly consolidated Go references, assets, and project templates.
*   **`test-quality-optimizer`**: Port Neko's upgraded testing skill (which now includes advanced techniques like mutation analysis, fuzzing, and benchmark strategies) over to GoDoctor, replacing the basic `go-test` skill.
*   **Remove Redundancy**: Delete `go-project-setup` and `go-test` as standalone skills to prevent confusing the agent.

## 6. Phase 5: Documentation & System Prompts
The core instructions must shift from "how to edit files" to "how to safely engineer Go systems."

*   **Rewrite `GODOCTOR.md`**: Update the core instructions to mirror Neko's philosophy.
    *   Mandate the `Target Loop` (Edit -> Review Diagnostics -> Build -> Test).
    *   Emphasize the `Boy Scout Rule` for continuous improvement.
    *   Explicitly instruct the agent to use `rename_symbol` over `edit_file` for structural changes.
*   **Update `README.md`**: Reflect the new architectural capabilities (LSP, RAG) for end-users.

## 7. Execution Strategy
1.  **Prep**: Merge this design document.
2.  **Foundation (PR 1)**: Port `internal/lsp` and project lifecycle tools.
3.  **Tools (PR 2)**: Overhaul the toolset (`multi_read`, `multi_edit`, `rename_symbol`).
4.  **Skills & Hooks (PR 3)**: Move the Neko skills/hooks and block generic Gemini tools.
5.  **Docs (PR 4)**: Finalize `GODOCTOR.md` and release.