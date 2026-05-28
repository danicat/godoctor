# ADR-0003: Core Instruction Set and Agent Guidance

- **Status:** Approved
- **Date:** 2025-11-05
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6

## 1. Context
As the complexity of the GoDoctor MCP server grew, AI agents struggled to select the correct tools in the correct order. Agents frequently fell back to basic shell commands (like `cat`, `sed`, `grep`, or raw `go build`) instead of utilizing GoDoctor's high-density, type-enriched, and compiler-gated tools.

We needed a centralized, authoritative mechanism to inject GoDoctor tool usage guidelines into the agent's core system prompt.

## 2. Decision
We decided to:
1. Create a dedicated instructions file in the workspace named `GODOCTOR.md` (and a corresponding `CLAUDE.md` generator).
2. Implement a CLI subcommand `godoctor --agents` that dynamically compiles and outputs the entire system instructions markdown payload.
3. Configure the MCP server session options to automatically advertise these instructions in the server's capabilities description block.

## 3. Consequences
- **Positive:** Standardizes agent behavior across different platforms (Claude Code, Gemini CLI, Cursor). Drastically reduces tool selection mistakes and forces agents to respect the GoDoctor workflow gate.
- **Negative:** Adds a small token overhead (approx. 500-1000 tokens) to the initial handshake/system prompt context.
- **Neutral:** Established the core philosophy of "Active Agent Guidance" which later enabled the creation of Specialist Skills under the Antigravity system.
