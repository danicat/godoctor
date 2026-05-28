---
name: task_template
description: "Guidelines and templates for structuring software development goals and ideas into actionable, bounded tasks using Context, To Do, Not To Do, and Acceptance Criteria, enforced by the Definition of Ready (DoR) gate."
---

# Actionable Task Scoping & Decomposition

This skill establishes a rigorous framework for transforming raw goals, ideas, or feature requests into high-precision, actionable, and bounded task files. It utilizes the standard four-section Agile ticket template and enforces a strict **Definition of Ready (DoR)** gate to ensure work is fully understood and scoped before any execution begins.

---

## 1. Trigger Conditions
Activate this skill whenever the agent or developer needs to:
- Scope a new user request or feature goal.
- Break down a large architecture plan (or RFC) into separate engineering tasks.
- Create a new ticket or a task file (like `task.md`) to guide codebase changes.
- Clarify ambiguous requirements by defining explicit work boundaries.

---

## 2. Core Philosophy: Extreme Clarity
> [!IMPORTANT]
> A poorly scoped task is the primary source of AI hallucinations and redundant execution loops. 
> An agent should be able to estimate effort and execute changes successfully **using the task description alone**, without needing to search for missing context.

---

## 3. The 4-Section Task Template

All scoped tasks must utilize this exact structural template:

```markdown
# Task: [Descriptive Action Title]

## Context
Provide a brief, factual description of:
1. **What we have today:** The current state of the codebase, files involved, and limitations.
2. **What we want to achieve:** The target state of the codebase once this task is complete.

Include direct links to relevant source files, documentation, or mockups.

## TO DO
A concise list of highly actionable, bulleted tasks. Every item must represent a clear engineering step.
- [ ] Implement [feature] in file [filename]
- [ ] Add unit test for [scenario]
- [ ] Format and verify syntax

## NOT TO DO
A list of explicit boundaries to restrict the scope of this task. Use this to keep tasks small, focused, and fast to complete.
- Do not modify [unrelated component]
- Do not backfill historical database schemas (deferred to future task ADR/RFC-XXXX)
- Do not add support for [advanced feature]

## Acceptance Criteria
The explicit, verifiable conditions that must be observed to prove the task is successfully completed.
- Running `go test ./...` passes without errors.
- Querying the [endpoint] returns a successful 2xx response with the new [field].
- Creating a file that violates compilation causes `smart_edit` to automatically roll back.
```

---

## 4. The "Definition of Ready" (DoR) Gate

Before starting work on *any* task, you must run the task description through this quality gate checklist. If any item is unchecked, the task is **Not Ready** and you must pause to gather context or refine boundaries first.

- [ ] **Described:** The **Context** section clearly describes both the *Current State* and the *Target State*.
- [ ] **Actionable:** Every item in the **TO DO** list starts with a clear, active verb (e.g., *Implement*, *Add*, *Refactor*, *Delete*) and specifies the target files or locations.
- [ ] **Bounded:** The **NOT TO DO** list defines at least one clear constraint, ensuring the task does not suffer from scope creep.
- [ ] **Verifiable:** The **Acceptance Criteria** list at least one test, command, or observable behavior that can objectively prove success.
- [ ] **Tidy:** Dependencies are identified, modules are tidied, and no external unknowns block the work.
