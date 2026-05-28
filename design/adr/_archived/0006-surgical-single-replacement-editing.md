# ADR-0006: Surgical Single-Replacement Editing

- **Status:** Superseded by ADR-0009
- **Date:** 2025-12-05
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek

## 1. Context
After introducing file creation, the next requirement was to edit existing files safely. Standard file writers completely overwrote target files, which was slow and risky for large files. We needed a precise, targeted editing mechanism to apply small changes (like fixing a line of logic or adding a single method).

## 2. Decision
We decided to implement a "surgical" single-replacement editor tool named `scalpel`. The tool took a `file_path`, a precise `old_string` block, and a `new_string` block. It located the exact match of `old_string` in the file and replaced it with `new_string`.

## 3. Consequences
- **Positive:** Enabled surgical edits on existing source files without overwriting the entire file. Highly efficient for small logic changes.
- **Negative:** Highly intolerant of variations. If there was a single space, tab, or newline discrepancy between the agent's `old_string` parameter and the file's actual content on disk, the string match failed, throwing a target matching error. This led to high agent friction and repeated edit failures.
- **Neutral:** Superseded by **ADR-0009** which introduced `smart_edit` with transactional multi-file coordinates and Levenshtein-based fuzzy matching.
