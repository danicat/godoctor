# ADR-0014: Decouple GoDoctor from gopls

- **Status:** Approved
- **Date:** 2026-08-05
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6, Antigravity
- **Supersedes:** [ADR-0013](file:///Users/petruzalek/projects/godoctor/design/adr/0013-persistent-gopls-lsp-connection.md)

## 1. Context
GoDoctor originally relied on `gopls` (the official Go Language Server) to resolve AST coordinates, type definitions, symbol locations, and documentation. Initially, GoDoctor spawned short-lived CLI calls to `gopls` (e.g. `gopls definition`), which suffered from multi-second startup latency per invocation. In [ADR-0013](file:///Users/petruzalek/projects/godoctor/design/adr/0013-persistent-gopls-lsp-connection.md), GoDoctor transitioned to maintaining a persistent, stateful `gopls serve` JSON-RPC daemon per workspace session.

While the persistent LSP connection reduced query latency to milliseconds, maintaining a long-running background daemon introduced substantial operational complexity and fragility:
- **Process Lifecycle Overhead:** Managing background daemon startup, process tracking, IPC health monitoring, and graceful teardown across multiple workspace sessions required significant infrastructure logic in `internal/lsp`.
- **Daemon Instability & Crash Risk:** Background `gopls` processes could fail to launch, hang during intensive file mutations, consume excess memory, or crash unexpectedly when encountering broken code states during active editing.
- **External Binary Dependency:** Requiring an installed, compatible version of `gopls` on the host system created an external runtime requirement and potential version mismatch issues for calling agents and users.

GoDoctor needed a solution that eliminated stateful daemon process management entirely while retaining instantaneous, zero-latency static code analysis and documentation retrieval.

## 2. Decision
We decided to completely remove `gopls` and all LSP/JSON-RPC daemon logic from GoDoctor. 

Instead, GoDoctor transitions fully to native Go standard library AST parsing (`go/ast`, `go/parser`, `go/doc`, `go/token`) and standalone package documentation enrichment (`godoc`).

Specifically:
- **Removal of `internal/lsp`:** Deleted the background daemon manager (`manager.go`), stateful client (`client.go`), and JSON-RPC IPC infrastructure.
- **In-Process Stateless Parsing:** Standard tools like `smart_read` and `describe_symbol` now use native `go/parser` and `go/ast` to inspect syntax trees and symbol definitions directly in memory within the server process.
- **`godoc` Package Documentation Integration:** Native `go/doc` package parsing and standalone `godoc` integration handle package and symbol documentation extraction cleanly without external daemon processes.

## 3. Consequences
- **Positive:**
  - **Zero Background Process Lifecycle:** Completely eliminates daemon management, health checking, signal handling, and background process crash recovery logic.
  - **Zero External Dependencies:** GoDoctor no longer requires `gopls` to be installed on the host system, ensuring predictable execution in any standard Go environment.
  - **Instantaneous Sub-Millisecond Analysis:** In-memory standard library AST parsing executes synchronously within the GoDoctor server process with negligible CPU and memory overhead.
  - **Improved Reliability:** Stateless parsing avoids IPC connection drops, process hangs, and stale index states during rapid workspace edits.
- **Negative:**
  - **None:** The Go standard library (`go/ast`, `go/parser`, `go/doc`) satisfies all GoDoctor symbol inspection and documentation retrieval needs with higher reliability and simpler maintenance.
