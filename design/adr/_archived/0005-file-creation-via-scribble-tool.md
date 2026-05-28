# ADR-0005: File Creation via Scribble Tool

- **Status:** Superseded by ADR-0011
- **Date:** 2025-11-20
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6

## 1. Context
To expand GoDoctor's capabilities from a read-only documentation tool to an active codebase modifier, the AI agent needed the ability to initialize new Go files, bootstrap packages, and write code. 

We needed a tool to write new files to disk.

## 2. Decision
We decided to implement a tool named `scribble` that accepted an absolute `file_path` and a raw `content` string. It initialized the target file and wrote the content directly to disk, creating any missing parent directories.

## 3. Consequences
- **Positive:** Enabled the agent to create new files, scaffold Go packages, and write new Go source code.
- **Negative:** Completely stateless and dangerous. It wrote files directly to the filesystem without running Go formatting (`gofmt`), import tidying, or checking if the newly introduced code compiled. If the agent wrote buggy boilerplate, the workspace broke immediately without warning.
- **Neutral:** Superseded by `file_create` (initially) and ultimately **ADR-0011** (Unified File Creation under `smart_edit`), which implements transactional compile-gated file creation.
