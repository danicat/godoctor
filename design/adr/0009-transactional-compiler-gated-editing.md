# ADR-0009: Transactional and Compiler-Gated Editing

- **Status:** Approved
- **Date:** 2026-02-15
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Gemini CLI, Claude Opus 4.6

## 1. Context
Previous versions of `smart_edit` (or `edit_code`) applied single-string search-and-replace actions directly to a single file. Matching failed if there was any whitespace discrepancy. Furthermore, edits were not verified against the workspace compiler: if an edit compiled but introduced a type mismatch in another file, the codebase remained broken, requiring manual diagnostic loops.

We needed a transactional, multi-file editing mechanism with compile-gate validation.

## 2. Decision
We decided to overhaul `smart_edit` to support:
1. **Multi-File Atomic Transactions:** Accepts an array of edit objects (`filename`, `old_content`, `new_content`, line range limits) and applies them sequentially in-memory.
2. **Post-Edit Compiler Gate:** Writes the changes to disk temporarily and runs `gopls check` across the entire workspace Go files.
3. **Automatic Transactional Rollback:** If the compiler check fails, all edited files are immediately restored to their backup states.
4. **Levenshtein-Based Typo Hints:** If `gopls` reports "undeclared name" errors, the tool queries `gopls symbols` on the file, compares spelling distances, and appends a `Did you mean '<Symbol>'?` suggestion block to the error return.

## 3. Consequences
- **Positive:** Guarantees workspace compilation safety. Code edits that break compilation or introduce type-mismatches are never committed to disk. Provides instant typo corrections to the agent.
- **Negative:** Increased runtime latency due to running parallel `gopls` checks on file writes.
- **Neutral:** Completely eliminates the risk of an agent leaving a repository in an uncompilable state.
