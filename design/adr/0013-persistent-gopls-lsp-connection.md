# ADR-0013: Persistent gopls LSP Connection

- **Status:** Superseded (2026-08-05)
- **Superseded By:** [ADR-0014](file:///Users/petruzalek/projects/godoctor/design/adr/0014-decouple-godoctor-from-gopls.md)
- **Date:** 2026-06-01
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Claude Opus 4.6, Antigravity

> [!NOTE]
> **Superseded Warning / Notice:** This decision has been superseded as of 2026-08-05 by [ADR-0014](file:///Users/petruzalek/projects/godoctor/design/adr/0014-decouple-godoctor-from-gopls.md). The persistent `gopls` LSP connection has been removed in favor of native Go AST parsing (`go/ast`, `go/parser`, `go/doc`) and package documentation (`godoc`). See [Section 4: Amendment — Superseded Decision (2026-08-05)](#4-amendment--superseded-decision-2026-08-05) below for details.

## 1. Context
GoDoctor's code discovery and navigation features (such as type-enrichment in `smart_read` and declaration lookups in `describe_symbol`) previously relied on spawning short-lived, short-circuit external CLI commands (e.g., executing `gopls definition` or `gopls symbols`). 

Spawning external CLI processes for every single AST coordinate lookup introduced severe performance bottlenecks:
- Every execution incurred the full overhead of starting a brand-new `gopls` JVM/Go process.
- Each subprocess had to load, parse, and re-index the workspace files from scratch.
- Query latencies climbed past several seconds when resolving multiple coordinates concurrently.

To achieve fast, responsive tool feedback loops for calling agents, GoDoctor needed a way to execute coordinate type-lookups in milliseconds rather than seconds.

## 2. Decision
We decided to implement a stateful, persistent Language Server Protocol (LSP) connection within GoDoctor, transitioning fully away from individual CLI subprocesses. 

Specifically:
- We created a unified `internal/lsp` package containing a stateful client (`client.go`) and a background daemon process manager (`manager.go`).
- The manager spawns and maintains a single shared `gopls serve` daemon process per workspace session.
- Communication with `gopls` is conducted via multiplexed JSON-RPC over stdin/stdout, allowing parallel non-blocking requests.
- The `smart_read` and `describe_symbol` tools retrieve this shared background connection to resolve AST types, definitions, and references instantly.
- Added a graceful teardown handler in `internal/server/server.go` to stop the persistent daemon when the MCP server shuts down.

## 3. Consequences
- **Positive:** Reduces type resolution and definition coordinates lookup latency from seconds to milliseconds. Resolves multiple AST type specifications concurrently with negligible performance overhead.
- **Negative:** Introduces stateful background process lifecycle management. GoDoctor must correctly track process status and recover from any underlying `gopls` crash or connection issue.
- **Neutral:** Clean, mocked TDD client test coverage verifies connection protocols, handshakes, and shutdowns safely without requiring a real binary setup during local unit tests.

---

## 4. Amendment — Superseded Decision (2026-08-05)

### Context & Rationale
While the persistent `gopls` daemon improved type-resolution latency over short-lived CLI subprocesses, maintaining a long-running external `gopls` process introduced operational overhead and instability. 

The persistent gopls LSP connection has been **superseded and removed** in [ADR-0014](file:///Users/petruzalek/projects/godoctor/design/adr/0014-decouple-godoctor-from-gopls.md) in favor of native Go standard library AST parsing (`go/ast`, `go/parser`, `go/doc`) and package documentation enrichment (`godoc`).

Key drivers for this architectural shift include:
- **Eliminating Stateful Process Lifecycle Management:** Removed the necessity to manage daemon initialization, background health checks, connection multiplexing, and teardown logic.
- **Removing `gopls` Daemon Startup & Crash Risks:** Avoids edge cases where background `gopls` instances failed to start, hung, crashed during heavy edits, or suffered from IPC connection drops.
- **Instantaneous Zero-Dependency Static Analysis:** Native `go/ast` and `go/parser` operate directly in-process via standard library calls, delivering sub-millisecond static analysis and type/doc lookups without relying on an external `gopls` binary being installed on the user's host system.

### Consequences
- **Positive:**
  - **Simplified Server Lifecycle:** Server startup, runtime, and shutdown are purely deterministic and contained within single-process Go execution boundaries.
  - **Zero External Process Dependencies:** Eliminates runtime dependency on `gopls` executable availability, version mismatches, or background daemon management.
  - **Fast Memory & CPU Footprint:** In-memory parsing via standard Go stdlib (`go/ast`, `go/parser`, `go/doc`) consumes minimal memory and responds virtually instantaneously.
- **Negative:**
  - **None:** Native Go standard library packages satisfy all AST inspection, symbol description, and documentation extraction requirements cleanly without the burden of LSP connection state.
