---
name: setup-hooks
description: Setup and configure GoDoctor hooks and environment. Run this skill when the user requests to setup or install GoDoctor hooks or enforcement.
---

# GoDoctor Hook Setup

This skill configures GoDoctor hooks in the workspace or globally. The setup script (`scripts/setup-hooks.sh`) installs a `PreToolUse` hook configuration into the agent's `hooks.json` file. This hook enforces the usage of proper MCP tools (`smart_build`, `project_init`, `add_dependency`, `smart_edit`, `smart_read`) by intercepting and rejecting raw shell commands and low-level file edits targeting Go files.

## Usage

To install globally (default):
```bash
./skills/setup-hooks/scripts/setup-hooks.sh --global
```

To install for the current workspace:
```bash
./skills/setup-hooks/scripts/setup-hooks.sh --workspace
```

To uninstall:
```bash
./skills/setup-hooks/scripts/setup-hooks.sh --uninstall
```
