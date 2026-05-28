# ADR-0004: Automated goimports Formatting

- **Status:** Approved
- **Date:** 2025-11-12
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek

## 1. Context
When AI agents created or modified Go files, they frequently left behind unformatted code, bad indentations, or unused/missing import declarations. This led to tedious compile-repair cycles where the agent had to execute manual formatting commands in the shell to clean up syntax.

We needed GoDoctor's writing tools to guarantee syntactically formatted and tidy Go code.

## 2. Decision
We decided to integrate Go's native `golang.org/x/tools/imports` package directly into the GoDoctor filesystem edit/creation pipeline. Every time a file modification is made by a tool:
1. The tool automatically runs `imports.Process` (which executes both `gofmt` and `goimports`) on the file's content in memory before committing to disk.
2. If `imports.Process` detects a syntax error, it immediately aborts, rolls back the changes, and returns the syntax error description to the agent.

## 3. Consequences
- **Positive:** Guarantees that code written by GoDoctor is always properly styled, has tidied imports, and is free of syntax errors. Eliminates manual formatting tool calls.
- **Negative:** Slightly increases the processing duration of write operations, as AST-parsing is run on the fly.
- **Neutral:** Shifts syntax verification from the post-compile phase directly into the write transaction phase.
