# ADR-0011: Unified File Creation (Deprecate `file_create`)

- **Status:** Approved
- **Date:** 2026-05-19
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6

## 1. Context
GoDoctor previously maintained two separate tools for file modifications: `file_create` (to create new files) and `smart_edit` (to edit existing files). This split-path design added significant cognitive load on AI agents. They frequently guessed incorrectly about file existence, trying to edit non-existent paths (causing "file not found" errors) or write to existing ones (causing "already exists" errors).

Furthermore, file creation via `file_create` was stateless and lacked the transactional compiler-gate validation present in `smart_edit`.

## 2. Decision
We decided to completely deprecate `file_create` and absorb its capabilities directly into `smart_edit`:
1. If `smart_edit` receives a target `filename` that does not exist, it marks it as `newlyCreated`, initializes it with empty bytes in-memory, and registers a `nil` backup state.
2. The new file is compiled, formatted, and validated under the **gopls compiler gate** alongside existing files.
3. If the newly created file causes compilation or syntax errors anywhere in the workspace, the transaction rolls back, and the new file is automatically deleted from disk, keeping the repository clean.

## 3. Consequences
- **Positive:** Unifies filesystem writes under a single, highly intuitive intent-driven tool. Reduces LLM cognitive overhead and eliminates split-path tool selection errors. Ensures that newly created files are subject to transactional safety, import organization, and compiler gates.
- **Negative:** None identified.
- **Neutral:** Required adjusting `internal/hooks/intercept.go` to redirect blocked raw `write_file` calls directly to `smart_edit` instead of `file_create`.
