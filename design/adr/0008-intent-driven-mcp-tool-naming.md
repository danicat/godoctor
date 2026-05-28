# ADR-0008: Intent-Driven MCP Tool Naming

- **Status:** Approved
- **Date:** 2026-01-23
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Gemini CLI

## 1. Context
Early GoDoctor tools used metaphorical, technology-focused names (`scalpel` for surgical edits, `scribble` for writing files, `endoscope` for web requests). While creative, these names did not align with how modern LLMs select tools. LLMs rely on semantic relevance matching between their prompt goals and tool descriptions/names.

We needed a tool naming scheme that reduced LLM cognitive load and maximized tool-selection accuracy.

## 2. Decision
We decided to completely deprecate the metaphorical names in favor of an **Intent-Driven** naming scheme:
- `scalpel` / `edit_code` -> **`smart_edit`**
- `godoc` / `read_code` -> **`smart_read`**
- `go-build` / `go-test` -> **`smart_build`**
- `go-get` -> **`add_dependency`**
- `go-init` -> **`project_init`**

## 3. Consequences
- **Positive:** Major reduction in LLM tool selection failures. By using clear verbs and "smart" intent markers, the LLM maps tool capabilities instantly.
- **Negative:** Required rewriting all tool registration blocks and updating all reference documentation (`README.md`, `GODOCTOR.md`).
- **Neutral:** Established the modern, unified tool registry mapping defined in `internal/toolnames/registry.go`.
