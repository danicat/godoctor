# ADR-0010: Session-Based Workspace Roots Mapping

- **Status:** Approved
- **Date:** 2026-03-16
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek

## 1. Context
Previously, GoDoctor kept a single, global list of project directories/roots under `internal/roots/roots.go`. When an MCP client connected and declared its roots, they were appended globally. 

This architecture broke down in multi-root workspaces or multi-session client scenarios (e.g. multiple editor windows or different agents connecting concurrently). Roots from one session leaked into another, creating race conditions, path collisions, and security directory leaks.

## 2. Decision
We decided to completely refactor `internal/roots/roots.go` to store project directories mapped explicitly to their active server session (`*mcp.ServerSession`). 
- Roots are added, retrieved, and deleted on a per-session basis.
- Path validation checks (`Validate`) now take the active session parameter and verify that the target filepath lies strictly within the directories registered for that specific session.
- We also integrated strict validation to deny access if the session's active workspace directory resolves to the system root `/`.

## 3. Consequences
- **Positive:** Guarantees absolute isolation of workspace paths between different concurrent client sessions. Restricts filesystem manipulation strictly to directories registered by the client, solving path security leakage.
- **Negative:** Required modifying signatures of all filesystem-accessing tools (`smart_edit`, `smart_read`, `list_files`, `describe_symbol`) to receive and pass the active `req.Session` parameter.
- **Neutral:** Cleanly handles initialization hook setups and list roots updates.
