---
name: go-latest-version
description: "Guidelines and procedures for querying, verifying, and upgrading Go module dependencies and toolchains to their latest stable releases using proxy.golang.org, go list, or package registries."
---

# Go Dependency & Toolchain Version Verification

This skill establishes a rigorous process for verifying, querying, and updating Go modules, libraries, and toolchains to their absolute latest stable releases. It prevents "version hallucination" by leveraging live Go proxies and standard tool commands instead of relying on stale offline training weights.

## 1. Trigger Conditions
Activate this skill whenever the user or agent needs to:
- Add a new third-party Go package dependency.
- Upgrade or audit existing dependencies in `go.mod`.
- Determine the latest stable Go runtime toolchain version (e.g., Go 1.24+).
- Address vulnerabilities or security notices in dependencies.
- Fix compilation failures stemming from mismatched module versions.

---

## 2. Core Mandate: NEVER GUESS
> [!IMPORTANT]
> Go libraries evolve constantly. Internal database cutoffs are guaranteed to be outdated. Always fetch the real-time source of truth from the live registry.
> **Trust live data over static assumptions.**

---

## 3. Querying the Go Proxy (Real-Time Truth)

The official Go Module Proxy (`proxy.golang.org`) is the authoritative source for all versioned Go modules. You can query it directly using simple shell commands, Go CLI, or HTTP endpoints.

### Method A: Using `go list` (Recommended for Local Workspaces)
If you are inside a Go workspace, the fastest way to check available versions is the `go list` command:

```bash
# List all tagged versions of a module
go list -m -versions github.com/modelcontextprotocol/go-sdk

# Find the latest resolved stable version
go list -m github.com/modelcontextprotocol/go-sdk@latest
```

### Method B: Querying proxy.golang.org (HTTP API)
If the Go CLI is unavailable or you need to bypass local module caches, use the proxy's public JSON/Text endpoints. Replace the module path with lowercase, and convert dots to slashes as per Go Proxy specifications:

```bash
# Format: https://proxy.golang.org/<escaped-module-path>/@v/list
# Example for github.com/modelcontextprotocol/go-sdk:
curl -s https://proxy.golang.org/github.com/modelcontextprotocol/go-sdk/@v/list

# Retrieve the latest version details
curl -s https://proxy.golang.org/github.com/modelcontextprotocol/go-sdk/@v/latest
```

---

## 4. Go SDK Toolchain Version Verification

To ensure compatibility with modern Go language features (such as Go 1.24's generic improvements or alias rules), always verify the latest Go compiler release.

Query the official Go website's JSON API to find active Go compiler releases:
```bash
curl -s "https://go.dev/dl/?mode=json" | grep -o '"version": "[^"]*"' | head -n 5
```
This returns a list of the latest stable active releases (e.g., `go1.24.3`, `go1.23.6`).

---

## 5. Safe Dependency Upgrade Workflow

When upgrading a module within `godoctor`, always follow this sequentially gated pipeline to preserve workspace health and compile-safety:

### Step 1: Query the Latest Stable Version
Retrieve the exact version using the `latest-software-versions` skill or live Go Proxy checks. Avoid pre-releases (`-rc`, `-alpha`, `-beta`) unless explicitly requested.

### Step 2: Fetch and Update Module
Use GoDoctor's `add_dependency` tool, or run `go get` combined with `go mod tidy` to resolve the package:
```bash
go get <module-path>@<version>
go mod tidy
```

### Step 3: Atomic Compilation Gate
Verify that the package upgrade did not introduce breaking changes or type mismatches by compiling the entire workspace:
```bash
# Run GoDoctor's strict quality gate
godoctor smart_build
```

### Step 4: Handle Major Version Upgrades (Go v2+ Rules)
> [!WARNING]
> Remember Go's Import Path Rule: Go modules at version `v2` or higher must include the major version suffix in their import paths (e.g., `/v2`, `/v3`), unless they are in `gopkg.in`.
>
> **Correct:** `go get github.com/foo/bar/v2@v2.1.0`  
> **Incorrect:** `go get github.com/foo/bar@v2.1.0` (will fail or fetch legacy v1)

---

## 6. Common Ecosystem Dependency Versions (As of May 2026)
Keep this live list of standard MCP and Go tools as high-quality reference baselines:
*   **MCP Go SDK:** `github.com/modelcontextprotocol/go-sdk @ v1.6.0`
*   **Golangci-lint:** `github.com/golangci/golangci-lint @ v1.64.5`
*   **Gopls (Go Language Server):** `golang.org/x/tools/gopls @ v0.18.1`
*   **Structured Logging (Slog):** Native in `log/slog` (Go 1.21+)
*   **Database PGX Driver:** `github.com/jackc/pgx/v5 @ v5.7.2`
