---
name: go-expert
description: The definitive expert for ALL Go (Golang) programming tasks. Mandatory for writing, debugging, refactoring, testing, or explaining Go code. Provides idiomatic solutions, project structure advice, and official best practices.
---

# Go Developer Skill

This skill equips Gemini with expert knowledge of Go best practices, project layout, and design patterns.

## Definition of Done (Mandatory)

**No code edit is successful until:**
1.  **Builds**: `go build ./...` succeeds without errors.
2.  **Tests**: `go test ./...` passes.
3.  **Acceptance Criteria**: The specific user requirements are met.
    *   **MANDATORY**: If the acceptance criteria are missing, vague, or ambiguous, the agent **MUST** request explicit criteria from the user before starting the task.

## Modes of Operation

Identify the user's intent to select the appropriate mode. If unsure, ask.

### 1. Prototyping / Spike (Velocity Focus)
**Goal:** Validate ideas and collect data with minimal friction.
*   **Ambiguity**: Accepted for low-stakes parts, but core goals must be clear.
*   **Templates**: Prefer simple layouts (`cli-simple`, flat file structure).
*   **Testing**: Critical path only. Focus on "happy path" verification.
*   **Quality Gate**:
    *   **Build & Test**: `go build` and `go test` must pass.
    *   No panics (use error handling).
    *   Standard formatting (`go fmt`).

### 2. Meticulous (Robustness Focus)
**Goal:** Deliver production-ready, maintainable, and scalable solutions.
*   **Ambiguity**: Zero tolerance. Ask clarifying questions for any vague requirement or missing edge case.
*   **Templates**: Use structured layouts (`cli-cobra`, `webservice`) enforcing Package Oriented Design.
*   **Testing**: Comprehensive. 100% logic coverage, integration tests, fuzzing for parsers, benchmarks for hot paths.
*   **Quality Gate**:
    *   **Strict Review**: Pass the "Senior Review Checklist" (`references/senior_review_checklist.md`).
    *   **Architecture**: Major decisions documented in ADRs (`references/architectural_decisions.md`).
    *   **Documentation**: Full godoc comments for all exported symbols.
    *   **Dependencies**: Justify every new dependency.

## Capabilities

1.  **Deliver Production Services**: Bootstrap and harden HTTP/gRPC services.
2.  **Modernize Legacy Code**: Refactor codebases to align with modern Go idioms.
3.  **Ensure Code Quality**: Conduct deep code reviews focusing on concurrency, safety, and performance.
4.  **Architectural Leadership**: Guide high-level system design and decision recording.
5.  **Advanced Verification**: Implement fuzzing and benchmarking strategies.

## Workflows

### Bootstrap Project

**Instructions:**
1.  **Determine Mode**: Is this a quick spike or a long-term project?
2.  **Select Template**:
    *   **Simple CLI**: `assets/cli-simple` (Prototyping)
    *   **Cobra CLI**: `assets/cli-cobra` (Meticulous)
    *   **Web Service**: `assets/webservice` (Meticulous - Standard lib 1.24+, POD)
    *   **MCP Server**: `assets/mcp-server`
    *   **Library**: `assets/library`
    *   **Game**: `assets/game`
3.  **Setup**:
    *   Ask for module name.
    *   **Validation**: Check `go version`. Ensure `go.mod` matches the installed version.
    *   Copy template.
    *   Run `go mod tidy` and verify no unnecessary dependencies were added.

### Refactor Code

1.  **Audit**:
    *   Check for interface pollution (interfaces defined on producer side).
    *   Check for tight coupling.
2.  **Plan**:
    *   *Prototyping*: "Inline this logic for speed."
    *   *Meticulous*: "Decouple this package using a consumer-defined interface. Create an ADR if changing the storage layer."
3.  **Execute**: Apply changes incrementally.
4.  **Verify**: **MANDATORY**: Run `go build ./...` and `go test ./...` after every significant edit.

### Design & Architecture

1.  **Problem Statement**: Before writing code, articulate the problem clearly.
2.  **Context**: Ask for constraints (latency, throughput, team size).
3.  **Record**: Use `references/architectural_decisions.md` to document the "Why".

## References

- **Effective Go**: `references/effective_go.md`
- **Code Review Comments**: `references/code_review_comments.md`
- **Senior Review Checklist**: `references/senior_review_checklist.md`
- **Google Go Style Guide**: `references/google_style_guide.md`
- **Project Layout**: `references/project_layout.md`
- **Package Oriented Design**: `references/package_oriented_design.md`
- **Architectural Decisions**: `references/architectural_decisions.md`
- **Advanced Testing**: `references/advanced_testing.md`
- **Go Proverbs**: `references/go_proverbs.md`
- **HTTP Services**: `references/http_services.md`
- **Ebitengine Docs**: `references/ebitengine_docs.md`