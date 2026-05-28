---
name: adr_template
description: "Guidelines, best practices, and standard templates for authoring and maintaining Architecture Decision Records (ADRs) to document context, decisions, and consequences."
---

# Architecture Decision Records (ADRs)

This skill provides the guidance and standard templates for writing and maintaining **Architecture Decision Records (ADRs)**. Based on Martin Fowler's bliki and the conversational scaling of software architecture, ADRs are lightweight, plain-text documents that capture significant architectural choices, the context behind them, and their consequences.

---

## 1. Trigger Conditions
Activate this skill whenever the user or agent needs to:
- Propose a major architectural change or technical direction.
- Resolve technical debates or choose between competing design alternatives.
- Document the rationale behind significant codebase refactorings.
- Create a permanent historical log of technical decisions for future maintainers.

---

## 2. Core Philosophy: Why Write ADRs?

According to Martin Fowler and standard industry practice, architecture is not about rigid blueprints but about **key decisions that are hard to change**. ADRs solve two major engineering problems:

1. **Context Loss (The "Chesterton's Fence" Problem):** Developers often look at code and ask, *"Why did they build it this way? That looks stupid. Let's delete it."* Only after deleting it do they discover the hidden constraint. ADRs preserve the "Why."
2. **Conversational Scaling:** ADRs act as a snapshot of a conversation. Instead of documenting every detail, they capture the *conclusion* and the *forces* that shaped it.

---

## 3. The 3 Immutable Rules of ADRs

> [!IMPORTANT]
> **Rule 1: Keep It Lightweight**
> An ADR should be short (1–2 pages max). If it takes more than 15 minutes to read, it is too long. It is a record of a decision, not a full design specification.
> 
> **Rule 2: ADRs are Immutable Logs**
> Once an ADR is approved and committed, **NEVER edit its decision.** If a decision is changed or reverted in the future, do not overwrite the old file. Instead, write a **new** ADR (e.g., `0005-use-http-instead-of-grpc.md`) and mark the old one's status as `Superseded by ADR-0005`.
> 
> **Rule 3: Focus on Consequences & Tradeoffs**
> Every architectural decision has a cost. A senior-level ADR is judged by the honesty of its **Consequences** section. Document the downsides, technical debt, and new constraints introduced by the choice.

---

## 4. Directory & Naming Conventions

All ADRs should be stored in the project's documentation folder:
```text
design/adr/
  ├── 0001-record-architecture-decisions.md
  ├── 0002-use-stdio-mcp-transport.md
  └── 0003-transactional-compiler-gated-editing.md
```

- **File Format:** Markdown (`.md`)
- **Naming Pattern:** `NNNN-short-descriptive-title.md` where `NNNN` is a sequential 4-digit number starting at `0001`.

---

## 5. Standard ADR Template

Use this standard template when creating a new ADR file in the `design/adr/` directory:

```markdown
# ADR-[Number]: [Active Verb Title]

- **Status:** [Proposed | Approved | Superseded by ADR-XXXX | Rejected]
- **Date:** [YYYY-MM-DD]
- **Author(s):** [Names]
- **Deciders:** [Names of people involved in the decision]

## 1. Context
Describe the current situation, background context, and the problem we are solving. What are the technological, organizational, or operational forces at play? What constraints must we respect? 

Keep this section factual. State the facts as they are.

## 2. Decision
State the chosen architectural path clearly and concisely. Explain **why** this path was selected over alternatives. 

List the alternative options that were seriously considered and why they were rejected (e.g., "Alternative A was rejected because it introduces a dependency on external Python runtimes").

## 3. Consequences
What is the impact of making this decision? Every decision has tradeoffs. Be honest and explicit about:
- **Positive Consequences (What we gain):** Improved performance, reduced cognitive load, stronger safety gates.
- **Negative Consequences (What we lose or must accept):** Added complexity, performance overhead, temporary backwards-compatibility friction, learning curve.
- **Neutral Consequences:** Structural shifts, directory reorganizations.

## 4. Compliance & Verification
How will we verify that this decision is being respected and implemented correctly? (e.g., "Verified by running unit tests under the go test pipeline" or "Enforced by hooks preventing direct file modifications").
```

---

## 6. Conversational Design Checklist

Before final approval, verify that the ADR answers these questions conversationally:
- [ ] **What are the forces?** (e.g., speed, safety, token limits, ecosystem standards).
- [ ] **What alternatives did we discard?** (This prevents future developers from suggesting the same discarded alternatives).
- [ ] **Is the tone objective?** Avoid words like "perfect," "flawless," or "elegant." State technical impacts factually.
