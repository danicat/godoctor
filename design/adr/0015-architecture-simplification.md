# ADR-0015: Architecture Simplification

- **Status:** Approved
- **Date:** 2026-08-10
- **Author(s):** Lead Documentation & Skills Engineer, Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6, Antigravity

## 1. Context

Over time, GoDoctor accumulated several layers of excess abstraction and accidental complexity. These included complex dynamic configuration loading mechanisms, elaborate prompt template management systems, catch-all generic utility packages (e.g., `textutil`, `util`, `shared`, `common`, `helpers`), and redundant or overlapping tool definitions (such as `describe_symbol` alongside `smart_read`, and `project_init`).

This architectural sprawl created several issues:
- **High Cognitive Overhead:** Developers and AI agents had to navigate multiple abstraction layers and dynamic loading mechanisms to locate core tool implementations and configuration logic.
- **Blurred Domain Boundaries:** Generic utility packages became dumping grounds for unrelated helper functions, obscuring package dependencies and violating single-responsibility principles.
- **Redundant Tool Interfaces:** Overlapping tools diluted the tool surface area, causing ambiguity in AI agent tool selection and increasing maintenance overhead.
- **Indirection and Fragility:** Dynamic runtime configuration loading and prompt template injection introduced unnecessary failure modes and made system behavior harder to reason about and debug.

To restore simplicity, maintainability, and clarity, GoDoctor underwent a comprehensive architecture simplification effort.

## 2. Decision

We decided to streamline GoDoctor by removing extraneous abstraction layers, enforcing strict domain packaging rules, consolidating redundant tool definitions, and standardizing system instructions and agent skill definitions.

 Specifically, we adopted the following architectural simplification decisions:

1. **Configuration Removal:** Hardcoding tool definitions in system instructions instead of relying on complex runtime configuration.
2. **Tool Standardization:** Placing all tool implementations strictly in dedicated packages at `./internal/tools/<tool-name>`.
3. **Strict Prohibition of Generic Utility Packages:** Complete removal and strict prohibition of `textutil`, `util`, `shared`, `common`, `helpers`, or similar generic packages. Helpers now live directly within domain/tool packages.
4. **Removal of Prompt Management:** Retiring prompt templates and dynamic prompt injection.
5. **Removal of Obsolete Tools:** Retiring legacy and redundant tools:
   - `describe_symbol`: Retired because its functionality is superseded by `smart_read` type enrichment.
   - `project_init`: Consolidated directly into `add_dependencies`.
6. **Tool Renaming & Splitting:**
   - Renaming `add_dependency` to `add_dependencies` and incorporating auto `go mod init`.
   - Splitting `smart_edit` into two focused tools: `smart_edit` (for single file edits) and `smart_multi_edit` (for atomic batch multi-file edits).
7. **System Instructions Standardization:** Hardcoding tool definitions directly in system instructions.
8. **Skill Creation:** Creating the canonical agent skill at `skills/godoctor/SKILL.md`.

## 3. Consequences

### Positive
- **Reduced Cognitive Overhead:** Developers and AI agents can easily discover, navigate, and reason about tool implementations within predictable directory structures (`./internal/tools/<tool-name>`).
- **Clean Domain Boundaries:** Enforcing the strict prohibition of generic utility packages (`util`, `shared`, `helpers`, etc.) ensures tight package cohesion, explicit dependency graphs, and no hidden cross-package coupling.
- **Faster Builds and Execution:** Removing dynamic configuration parsing, prompt template compilation, and dynamic injection reduces runtime startup overhead and speeds up compilation.
- **Simpler Maintenance and Debugging:** A consolidated tool set with hardcoded tool definitions in system instructions reduces bug surface area and simplifies agent orchestration.

### Negative
- **Breaking Changes for Legacy Tool Names:** Retiring `describe_symbol` and `project_init`, and renaming `add_dependency` to `add_dependencies`, introduces breaking changes for legacy clients or workflows relying on the old tool signatures.
