# GoDoctor Coherence Refactor

**Date:** 2026-03-16
**Status:** Proposal
**Author:** Code review by Claude (Opus 4.6)

## Problem Statement

GoDoctor has evolved through 10+ naming eras and multiple architectural visions. The result is a tool suite that works but feels incoherent:

1. **Instructions are broken.** `internal/instructions/instructions.go` references dead tool names (`verify_build`, `verify_tests`, `go_docs`) that no longer exist in the registry. The `--agents` output is incomplete and wrong.
2. **GODOCTOR.md and instructions.go are diverged.** Two sources of truth for agent guidance, neither correct.
3. **Tool naming is inconsistent.** Some tools use `smart_` prefix (smart_read, smart_edit, smart_build), some use domain prefixes (file_create, list_files), some use verbs (add_dependency, modernize_code). There's no clear convention.
4. **Security boundaries are inconsistently applied.** `smart_edit` and `smart_read` validate paths via `roots.Validate()`, but `smart_build` and `modernize_code` do not.
5. **Code duplication.** `levenshtein()` is duplicated across two packages. `errorResult()` is duplicated across three packages.
6. **The "story" is unclear.** A new user can't quickly understand what GoDoctor does or how the tools relate to each other.

## Goals

- **G1:** Single source of truth for agent instructions (embed `GODOCTOR.md` in binary)
- **G2:** Consistent tool naming convention
- **G3:** Fix all security gaps (roots validation everywhere)
- **G4:** Eliminate code duplication
- **G5:** Make `--agents` output directly usable as a `CLAUDE.md` section
- **G6:** Assess every tool for value vs. complexity and prepare a pruning plan

## Non-Goals

- Adding new tools (that's Phase 2+)
- Changing the MCP protocol layer
- Changing the `godoc` engine internals

---

## Phase 1: Structural Fixes (This PR)

### 1.1 Embed GODOCTOR.md as the Single Source of Truth

**Problem:** `instructions.go` builds instructions dynamically from `toolnames.Registry[].Instruction` fields, but references stale tool names and diverges from `GODOCTOR.md`.

**Solution:**

```
internal/instructions/
    embed.go          # //go:embed GODOCTOR.md
    instructions.go   # Get() returns the embedded content
```

- Use `//go:embed` to embed `GODOCTOR.md` directly into the binary.
- `Get(cfg)` returns the embedded text as-is. If tools are disabled, append a "Disabled tools" note at the bottom rather than trying to dynamically generate partial instructions.
- `godoctor --agents` prints this embedded text exactly.
- Users copy this into their `CLAUDE.md` (or any other agent config) verbatim.
- Remove the `Instruction` field from `toolnames.ToolDef` — it's no longer needed.

**Why embed rather than dynamic generation?** Dynamic generation has proven fragile (stale references, diverged docs). A single markdown file is easier to review, test, and keep accurate. The tradeoff (disabled tools still mentioned in text) is minor — a footnote is sufficient.

### 1.2 Fix Security Gaps

Add `roots.Global.Validate()` to:

| Tool | File | Current | Fix |
|------|------|---------|-----|
| `smart_build` | `go/quality/build.go` | No validation on `dir` | Add `Validate(dir)` at handler entry |
| `modernize_code` | `go/modernize/modernize.go` | No validation on `dir` | Add `Validate(dir)` at handler entry |
| `add_dependency` | `go/get/get.go` | Needs audit | Validate working dir if configurable |
| `project_init` | `go/project/init.go` | Needs audit | Validate `path` param |

### 1.3 Fix Path Containment Check

In `roots.go:127`, replace:
```go
if strings.HasPrefix(absPath, root) {
```
with:
```go
if absPath == root || strings.HasPrefix(absPath, root+string(filepath.Separator)) {
```

This prevents `/home/user/project-other` from matching root `/home/user/project`.

### 1.4 Deduplicate Shared Code

**`levenshtein()`** — Extract to `internal/tools/shared/levenshtein.go`. Remove copies from `edit.go` and `godoc.go`.

**`errorResult()`** — Extract to `internal/tools/shared/result.go`. Remove copies from `edit.go`, `modernize.go`, `quality.go`.

### 1.5 Fix instructions.go

Even with embedding, update the stale references so the dynamic path isn't completely broken for anyone who forks:

- `verify_build` -> `smart_build`
- `verify_tests` -> (merged into `smart_build`)
- `go_docs` -> `read_docs`

Or better: delete the dynamic generation entirely and replace with the embed approach (1.1).

### 1.6 Minor Code Fixes

- Remove commented-out `fmt.Printf` debug lines in `edit.go:251,262`
- Fix `similarity()` to use rune length instead of byte length (`edit.go:323`)
- Fix duplicate word in comment: "context context" (`quality/build.go:211`)
- Fix coverage file cleanup to use `os.Remove()` instead of shelling out to `rm` (`quality/build.go:118`)
- Remove empty line at `main.go:66`

---

## Phase 2: Tool Coherence (Future PR)

### 2.1 Tool Usability Assessment

#### Design Philosophy

Every GoDoctor tool exists to prevent a specific **LLM coding failure mode**. The assessment framework is:

| Axis | Question |
|------|----------|
| **Failure Mode** | What specific LLM mistake does this tool prevent? |
| **Effectiveness** | How well does it prevent that failure mode today? |
| **Improvement** | What changes would make it more effective? |

#### Assessment Matrix

| Tool | LLM Failure Mode Prevented | Effectiveness | Improvement Opportunities |
|------|---------------------------|---------------|--------------------------|
| **smart_read** | **Blind coding.** LLMs edit files without understanding the full context — imported types, package APIs, function signatures. smart_read provides IDE-like "hover" functionality: static analysis findings, documentation for imported packages, and AST-based outlines. | High. The outline mode and import doc hints are valuable. | **P0: Restore imported symbol resolution.** The original spec included type/function definitions for imported symbols (stripped due to a bug). This is the "IDE hover" feature. **P1: Doc memoization.** Track which package docs have already been shown to the agent in this session and skip them on subsequent reads to save tokens. |
| **smart_edit** | **Typos and syntax corruption.** LLMs frequently produce edits with whitespace mismatches, indentation errors, and minor typos that cause exact-match editors to fail. smart_edit tolerates these via normalization + Levenshtein distance. Post-edit static analysis catches syntax corruption before it reaches disk. | Medium-High. The fuzzy matching works well for small edits. For larger blocks, the friction log shows it can misalign replacements (workaround: use line ranges). | **P1: Better error diagnostics.** When fuzzy match misaligns, show a diff of what the tool *tried* to do (not just "syntax error"). **P2: Confidence tiers.** Auto-apply at >0.98, warn-and-apply at 0.95-0.98, reject below 0.95 with suggestions. |
| **file_create** | **Invalid initial files.** LLMs create `.go` files with wrong imports, missing imports, or syntax errors. file_create runs `goimports` + `gofmt` + syntax verification, ensuring every new file is valid Go from the first write. | High. Simple and reliable. | **P1: Template hints.** Suggest standard file structure (package declaration, imports, main patterns) based on filename conventions (e.g., `_test.go` -> test boilerplate). |
| **list_files** | **Disorientation.** LLMs need to understand project structure before making changes. list_files provides a clean, Go-project-aware view filtered of noise (.git, node_modules, build artifacts). | Medium. Works but limited. | **P1: `.gitignore` support.** Currently uses hardcoded skip list. Either parse `.gitignore` or shell out to `git ls-files`. **P2: Go module awareness.** Annotate directories that contain `go.mod` or are Go packages vs. plain directories. |
| **read_docs** | **API hallucination.** LLMs frequently invent function signatures, struct fields, and package APIs that don't exist. read_docs provides ground-truth documentation from the actual Go source, including examples, even for packages not installed in the current project. Vanity import resolution prevents confusion with redirected module paths. | High. The fallback chain (local -> download -> walk up path) is robust. | **P1: Method listing for types.** When querying a type, include its method set. |
| **smart_build** | **Ship-and-pray.** LLMs make changes and declare success without verifying. smart_build enforces a full quality pipeline (tidy -> fmt -> build -> test -> lint) as one atomic step, catching regressions before the agent moves on. | High. The pipeline design is solid. | **P1: Incremental mode.** Only test packages affected by recent changes. |
| **add_dependency** | **API hallucination (at install time).** LLMs `go get` a package then immediately hallucinate its API. add_dependency prints the package documentation right after install, grounding the agent in the real API. | High. Simple and effective. | **P1: Version pinning guidance.** Warn when installing `@latest` in a production project. |
| **project_init** | **Scaffolding errors.** LLMs run multi-step init sequences (`mkdir`, `go mod init`, `go get`, ...) and often get the order wrong or forget steps. project_init bundles it atomically with the same doc-fetching anti-hallucination behavior as add_dependency. | Medium-High. Works well after v0.12 fixes. | **P1: Integration with `go-project-setup` skill.** Skill selects template, tool does the init. |
| **modernize_code** | **Stale patterns.** LLMs reproduce Go patterns from their training data, which may be outdated. modernize_code uses the official `golang.org/x/tools` analyzer to identify and auto-fix legacy patterns. | Medium. Useful but niche. | **P1: Integrate findings into smart_build.** Run modernize check as optional step in the quality pipeline. |
| **code_review** | **Self-review bias.** When the same model that wrote the code also reviews it, it's biased toward its own patterns and blind to its own mistakes. code_review solves this by delegating review to a separate model with a clean context (no implementation history, no sunk-cost bias). Also allows using a *different* model for review (e.g., Gemini reviewing Claude's code or vice versa), providing genuine diversity of perspective. | Medium. The review quality depends on the Gemini model and prompt. Requires separate API credentials. | **P1: Use system instruction properly.** Currently packs the system prompt as a user message — use `GenerateContentConfig.SystemInstruction` instead. **P1: Make credentials optional gracefully.** Already auto-disables when no keys found, but improve the UX (clearer message about what's lost). |

#### Summary

| Action | Tools |
|--------|-------|
| **Keep (all 10)** | `smart_read`, `smart_edit`, `file_create`, `list_files`, `read_docs`, `smart_build`, `add_dependency`, `project_init`, `modernize_code`, `code_review` |
| **Add (2)** | `mutation_test` (selene wrapper), `test_query` (tq wrapper) |
| **Keep all current names** | No renames. Current names are well-established and the `smart_` convention is working as intended. |

#### Priority Improvements

| Priority | Tool | Improvement |
|----------|------|-------------|
| **P0** | `smart_read` | Restore imported symbol type/function resolution (the "IDE hover" feature) |
| **P1** | `smart_read` | Doc memoization — don't repeat docs already shown this session |
| **P1** | `smart_edit` | Better error diagnostics — show what the fuzzy match tried to do |
| **P1** | `list_files` | `.gitignore` support |
| **P1** | `code_review` | Use `GenerateContentConfig.SystemInstruction` instead of packing system prompt as user message |

#### Completed (v0.12.0)

| Tool | Improvement |
|------|-------------|
| `project_init` | `roots.Validate()` security fix |
| `modernize_code` | `roots.Validate()` security fix |
| `smart_read` | Filter stdlib from outline imports (show third-party only) |
| `add_dependency` | Accept single `"package"` string as convenience alias |
| `file_create` | Richer success output confirming format/syntax pipeline |
| `modernize_code` | Strip `go: downloading` noise from output |
| `project_init` | Richer success output with absolute path and go.mod confirmation |

### 2.2 Revised GODOCTOR.md

Merge the best of both sources:
- **From GODOCTOR.md:** Core Philosophy section and workflow examples (these steer agent behavior)
- **From dynamic instructions:** Compact Usage/Outcome format with concrete call examples (these help tool invocation)
- **Add:** The two missing tools (`project_init`, `modernize_code`)

Then embed via `//go:embed` and delete the dynamic generation in `instructions.go`.

The new GODOCTOR.md should tell a coherent story organized around developer workflows:

> GoDoctor gives your AI agent **Go superpowers**: Go-aware file operations, atomic quality gates, and direct access to the Go documentation ecosystem.

Four sections mapping to how developers actually work:

1. **Navigate** (`smart_read`, `list_files`) — Explore and understand Go projects with AST-aware outlines, static analysis, and clean project views
2. **Write** (`smart_edit`, `file_create`) — Edit and create Go files with fuzzy matching, auto-formatting, auto-imports, and syntax verification baked in
3. **Verify** (`smart_build`, `code_review`) — Build + test + lint in one atomic step, with optional cross-model code review for unbiased feedback
4. **Discover** (`read_docs`, `add_dependency`, `project_init`, `modernize_code`) — Access documentation, manage dependencies, bootstrap projects, and modernize patterns

---

## Implementation Order

```
Phase 1 (v0.12.0 - structural fixes): ✅ DONE
  1. Extract shared Levenshtein to internal/textdist
  2. Fix security gaps (roots.Validate in smart_build, modernize_code, project_init)
  3. Fix path containment bug in roots.go
  4. Fix instructions.go stale tool references
  5. Minor code fixes (similarity rune len, rm -> os.Remove, debug prints, etc.)
  6. Tool usability improvements (add_dependency, file_create, modernize_code, project_init, smart_read)
  7. Claude Code support (README, MCP config, goreleaser workflow)

Phase 2 (next PR - coherence):
  1. Embed GODOCTOR.md as single source of truth (merge best of static + dynamic)
  2. Rewrite GODOCTOR.md with Navigate/Write/Verify/Discover narrative
  3. Add go-code-review MCP prompt (structured review checklist)
  4. Add mutation_test and test_query MCP tools (go run selene/tq)
  5. Add go-test skill (Gemini CLI) with example tq queries
  4. Improve list_files (.gitignore support)
  5. Improve smart_read (restore imported symbol resolution)
  6. Improve code_review (use SystemInstruction properly)

Phase 3 (future PR - skills):
  1. Replace go-expert with go-project-setup (from danicat/skills repo)
  2. Clean up dead reference files (ebitengine_docs.md, http_services.md)
  3. Delete reference material that duplicates LLM training data
  4. Add dual-format support (Gemini CLI + Claude Code)
```

## Risks

| Risk | Mitigation |
|------|------------|
| Embedding GODOCTOR.md loses dynamic tool filtering | Append "disabled tools" note instead. Acceptable tradeoff for correctness. |
| Removing reference material loses value for weaker models | Keep references in Gemini CLI skill for models that need them. MCP prompt provides structured guidance without bulk. |

---

## Phase 3: Skills & Prompts (Future PR)

### 3.1 Problem with the Uber-Skill

The current `skills/go-expert/` is a monolithic "God skill" that tries to be everything:
- Project bootstrapping (6 templates)
- Code review checklist
- Architecture guidance (ADRs)
- Testing patterns (fuzz, bench, golden files)
- HTTP service patterns
- Ebitengine game development (empty file!)
- Two operational modes ("Prototyping" vs "Meticulous")

**Problems:**

1. **Context overload.** The skill loads ~3,000 lines of reference material. Most of it is irrelevant to any specific task.
2. **Vague activation.** "The definitive expert for ALL Go programming tasks" means it triggers on everything Go-related, always loading the full payload.
3. **Training data duplication.** Most reference material (Effective Go, Code Review Comments, Google Style Guide, testing patterns) is well-known public content that LLMs already have in their training data. Loading 2,370 lines of Effective Go adds latency and token cost with near-zero marginal value.
4. **Dead references.** `ebitengine_docs.md` and `http_services.md` are empty files (0 lines). `go_proverbs.md` is 2 lines.
5. **Gemini-specific.** The `SKILL.md` format is Gemini CLI-specific. For Claude Code, skills need to be expressed differently.

### 3.2 Value-Add Analysis

| Content | Lines | In LLM Training Data? | Value-Add |
|---------|-------|----------------------|-----------|
| `effective_go.md` | 2,370 | Yes (canonical Go doc) | **None.** Every major LLM knows this. |
| `code_review_comments.md` | 216 | Yes (Go wiki) | **None.** Well-known public document. |
| `google_style_guide.md` | 64 | Yes (Google OSS) | **None.** |
| `advanced_testing.md` | 56 | Yes (standard patterns) | **None.** Fuzz, bench, golden files are well-documented. |
| `senior_review_checklist.md` | 20 | No (custom) | **Low-Medium.** Useful as a *structured prompt* to force systematic checking. Not domain knowledge the LLM lacks, but a prompt engineering technique. |
| `project_layout.md` | 150 | Partially | **Medium.** Opinionated decisions ("no pkg/ directory") and Go 1.24+ conventions add value. |
| `package_oriented_design.md` | 42 | Partially (Ardan Labs) | **Low.** |
| `architectural_decisions.md` | 37 | Yes (ADR format) | **None.** |
| Project templates (assets/) | ~260 | No (custom code) | **High.** Concrete starter code with correct patterns (graceful shutdown, signal handling, run functions). LLMs can't consistently produce these. |

**Conclusion:** Only the **project templates** and **project layout opinions** provide genuine value beyond what LLMs already know. The senior review checklist has value as a structured prompt, not as reference material.

### 3.3 Decision: One Skill + One MCP Prompt

#### Skill: `go-project-setup` (Gemini CLI)

Replace `go-expert` with `go-project-setup` based on the existing skill at `github.com/danicat/skills/go-project-setup`.

**What it keeps:**
- 6 project templates (the high-value assets)
- `project_layout.md` (opinionated layout guidance)
- Structured workflow: scope assessment → template selection → init → verify

**What it drops:**
- `effective_go.md` (2,370 lines — training data)
- `code_review_comments.md` (216 lines — training data)
- `google_style_guide.md` (64 lines — training data)
- `advanced_testing.md` (56 lines — training data)
- `architectural_decisions.md` (37 lines — training data)
- `go_proverbs.md` (2 lines — nearly empty)
- `ebitengine_docs.md` (0 lines — empty)
- `http_services.md` (0 lines — empty)
- `package_oriented_design.md` (42 lines — low value)
- "Prototyping vs Meticulous" modes (over-engineered)

**Context budget:** ~410 lines (down from ~3,200). **87% reduction.**

#### MCP Prompt: `go-code-review`

Expose the senior review checklist as an **MCP prompt** rather than a skill or reference file. This is the right abstraction: the checklist's value is as a structured prompt that forces systematic review, not as domain knowledge.

The prompt combines:
- The senior review checklist (consumer-defined interfaces, goroutine lifecycles, mutex copying, channel hygiene, error wrapping)
- Guidance to use GoDoctor tools (`smart_build` to verify, `modernize_code` for pattern upgrades, `code_review` for cross-model second opinion)

**Implementation:** Add `internal/prompts/code_review.go` alongside the existing `import_this.go`, following the same pattern. Register in `server.go`.

```go
// code_review.go
func GoCodeReview(namespace string) *mcp.Prompt {
    return &mcp.Prompt{
        Name:        namespace + ":go_code_review",
        Title:       "Go Code Review",
        Description: "Structured senior-level Go code review checklist with GoDoctor tool integration.",
        Arguments: []*mcp.PromptArgument{
            {Name: "focus", Description: "Optional area to focus the review on", Required: false},
        },
    }
}
```

The prompt content includes:
1. The senior review checklist (concurrency, interfaces, errors, API design)
2. Key items from Go Code Review Comments (the non-obvious ones: receiver names, in-band errors, goroutine lifetimes)
3. Tool integration: "Run `smart_build` to verify. Use `modernize_code` to catch stale patterns. Use `code_review` for a cross-model second opinion."
4. If a `focus` argument is provided, narrow the review to that area.

#### Why MCP prompt (not a skill) for code review:
- No reference files needed (the checklist fits in ~40 lines of prompt text)
- Works across all MCP clients (Claude Code, Gemini CLI, any MCP client)
- Lightweight — no context overhead until explicitly invoked
- The `code_review` tool already handles the heavy lifting (cross-model review); the prompt just structures the host agent's own review

### 3.4 Skill: `go-test` — Testing with selene and testquery

Testing gets a **skill** (not just a prompt) because it integrates two external tools that provide genuine superpowers LLMs don't have internally:

| Tool | What It Does | LLM Failure Mode Addressed |
|------|-------------|---------------------------|
| **[selene](https://github.com/danicat/selene)** | Mutation testing — mutates code and checks if tests catch it | "Tests look good but catch nothing" — proves test quality objectively |
| **[testquery (tq)](https://github.com/danicat/testquery)** | SQL queries over test results and coverage | "Wrote 50 tests but missed the critical path" — finds coverage gaps and redundant tests |

#### Integration: `go run` pattern (like modernize_code)

Both tools are integrated via `go run`, avoiding code duplication while keeping selene and tq as independent projects:

```go
// Mutation testing
exec.CommandContext(ctx, "go", "run", "github.com/danicat/selene@latest", "run", "./...")

// Test querying
exec.CommandContext(ctx, "go", "run", "github.com/danicat/testquery@latest", "query", sqlQuery)
```

This matches the existing `modernize_code` pattern. Benefits:
- Zero code duplication — logic lives in selene/tq repos
- No separate install step (auto-downloads)
- Isolated from API changes in selene/tq
- Can graduate to direct Go library import later when APIs stabilize

#### New MCP Tools

| Tool | Description | Params |
|------|------------|--------|
| `mutation_test` | Run selene mutation testing on a package | `dir`, `mutators` (optional filter) |
| `test_query` | Run a SQL query over test results and coverage | `dir`, `query`, `mode` (memory/file) |

#### Skill Workflow

The `go-test` skill (Gemini CLI) / prompt guidance (Claude Code) orchestrates a testing loop:

```
1. smart_read(outline=true) — understand the code under test
2. Write tests (table-driven, adversarial edge cases, error paths)
3. smart_build(run_tests=true) — verify tests pass
4. mutation_test(dir=".") — do the tests actually catch bugs?
5. Fix surviving mutants (mutations the tests didn't detect)
6. test_query(query="SELECT * FROM all_coverage WHERE coverage < 80") — find coverage gaps
7. test_query(query="SELECT * FROM test_coverage WHERE unique_coverage = 0") — find redundant tests
```

#### Example tq Queries (skill assets)

```sql
-- Find functions with no test coverage
SELECT * FROM all_coverage WHERE coverage = 0;

-- Find tests that cover nothing unique (candidates for removal)
SELECT * FROM test_coverage WHERE unique_coverage = 0;

-- Coverage summary by package
SELECT package, AVG(coverage) as avg_coverage FROM all_coverage GROUP BY package;

-- Find the most impactful function to test next
SELECT * FROM all_coverage WHERE coverage < 50 ORDER BY num_statements DESC LIMIT 10;
```

#### Why skill (not just a prompt):
- **Custom tools:** LLMs don't know selene or tq — they need concrete usage guidance
- **Workflow orchestration:** The write → mutate → fix → query loop is non-obvious
- **Assets:** Example tq queries are genuine value-add (not training data)
- **Two new MCP tools** to register and document

### 3.5 Reference Material Cleanup

| File | Lines | Action |
|------|-------|--------|
| `effective_go.md` | 2,370 | **Delete.** Training data duplication. |
| `code_review_comments.md` | 216 | **Delete.** Training data. Key items absorbed into `go-code-review` MCP prompt. |
| `project_layout.md` | 150 | **Keep.** Move to `go-project-setup`. |
| `google_style_guide.md` | 64 | **Delete.** Training data. |
| `advanced_testing.md` | 56 | **Delete.** Training data. |
| `package_oriented_design.md` | 42 | **Delete.** Low value-add. |
| `architectural_decisions.md` | 37 | **Delete.** ADR format is well-known. |
| `senior_review_checklist.md` | 20 | **Delete file, absorb into MCP prompt.** The checklist content lives on as the `go-code-review` prompt. |
| `go_proverbs.md` | 2 | **Delete.** Nearly empty. |
| `ebitengine_docs.md` | 0 | **Delete.** Empty. |
| `http_services.md` | 0 | **Delete.** Empty. |

### 3.6 Migration Path

```
Step 1: Add go-code-review MCP prompt (internal/prompts/code_review.go)
Step 2: Register prompt in server.go
Step 3: Add mutation_test tool (internal/tools/go/mutation/mutation.go) — wraps selene via go run
Step 4: Add test_query tool (internal/tools/go/testquery/testquery.go) — wraps tq via go run
Step 5: Register both tools in server.go and toolnames registry
Step 6: Replace go-expert/ with go-project-setup/ (from danicat/skills repo)
Step 7: Add go-test/ skill with SKILL.md + example tq queries as assets
Step 4: Keep only project_layout.md reference + 6 template assets
Step 5: Delete all other reference files
Step 6: Update GODOCTOR.md to reference the prompt and skill
Step 7: Update goreleaser to package go-project-setup instead of go-expert
```

---

## Open Questions

1. **Should `smart_read` keep outline mode as a standalone tool or merge it into `edit_file` as a dry-run/preview?** Outline mode is the only differentiating feature.
2. **Should we keep the HTTP transport?** It adds complexity and the Cloud Run deployment story. If the primary audience is CLI agents (Claude Code, Gemini CLI), stdio is sufficient.
3. **Should the `go-project-setup` skill keep the `game` template (Ebitengine)?** It's niche but popular in the Go community. Consider keeping it but removing the empty reference file.
4. **Should `import_this` prompt be kept, merged with `go-code-review`, or removed?** It asks the LLM to read external URLs and produce instructions — useful for bootstrapping but redundant once GODOCTOR.md is embedded.
