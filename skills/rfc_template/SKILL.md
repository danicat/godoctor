---
name: rfc_template
description: "Guidelines and templates for authoring Request for Comments (RFCs) to explore design alternatives, gather early feedback, and resolve architectural ambiguity before formalizing ADRs."
---

# Request for Comments (RFC) Framework

This skill establishes the standards, templates, and procedures for authoring **Request for Comments (RFCs)**. RFCs are collaborative, conversational design proposals used when an engineering problem is highly ambiguous, complex, or has multiple viable solutions. 

---

## 1. Trigger Conditions
Activate this skill whenever the agent or developer needs to:
- Propose a significant new feature or structural refactoring.
- Explore design alternatives when there is no obvious "correct" technical choice.
- Gather feedback and technical consensus from other team members (or agents).
- Flush out technical details for an ambiguous problem *before* formalizing a decision.

---

## 2. The Relationship Between RFCs and ADRs

According to modern architectural practices (like scaling architecture conversationally):
*   **RFC (Request for Comments):** Fluid, exploratory, collaborative, and optional. It is a tool for *discussion*. It presents multiple options, asks questions, and solicits opinions. It can be drafted, debated, modified, or rejected.
*   **ADR (Architecture Decision Record):** Rigid, immutable, and authoritative. It is a record of a *decision*. 
*   **The Pipeline:** A successful RFC often culminates in the creation of one or more formal ADRs. However, **not all RFCs lead to ADRs**. Some RFCs are rejected or postponed, which is still a valuable outcome to document.

---

## 3. Directory & Naming Conventions

All RFCs must be stored in the dedicated RFC directory:
```text
design/rfc/
  ├── 0001-use-structured-logging.md
  ├── 0002-migrate-to-sqlite-cache.md
  └── 0003-parallelize-type-enrichment.md
```

- **File Format:** Markdown (`.md`)
- **Naming Pattern:** `NNNN-short-descriptive-title.md` where `NNNN` is a sequential 4-digit number starting at `0001`.

---

## 4. Standard RFC Template

Use this standard template when creating a new RFC file in the `design/rfc/` directory:

```markdown
# RFC-[Number]: [Descriptive Proposal Title]

- **Status:** [Draft | Ready for Review | In Review | Approved | Rejected]
- **Date:** [YYYY-MM-DD]
- **Author(s):** [Names]
- **Deciders/Reviewers:** [Names of people or agents requested for feedback]
- **ADR Reference:** [Optional: Link to ADR-XXXX if approved]

## 1. Executive Summary
Provide a high-level, 2–3 sentence summary of the proposal, what problem it addresses, and the recommended solution. Think of this as the elevator pitch.

## 2. Context & Problem Statement
Detail the current situation, the forces at play, and the engineering pain points we are trying to solve. What are the constraints, user requirements, or performance bottlenecks?

## 3. Proposed Solution
Describe the recommended technical design in detail. Include structural layout, API changes, package splits, or new tool registrations. Use Mermaid diagrams or code blocks to illustrate the design.

## 4. Discarded Alternatives
List the other options that were considered but are not recommended. Be explicit about why they are inferior (e.g., "Option B was discarded because it requires an external Docker daemon, breaking our local-only execution constraint").

## 5. Supporting Materials & Prototypes
Document any spikes, benchmark results, or code prototypes. Link to any temporary code created in the `scratch/` directory during exploration.

## 6. Open Questions
List any unresolved issues, technical gaps, or design points where you are explicitly requesting community (or agent) feedback.

## 7. References
Provide links to official documentations, source code declarations, or industry articles that support the proposal.
```

---

## 5. Lifecycle States

An RFC progresses through these states:
1. **Draft:** The proposal is being written and is not yet complete.
2. **Ready for Review:** The author has completed the proposal and requests input.
3. **In Review:** Active discussions and comments are happening on the proposal.
4. **Approved:** Technical consensus is achieved. This RFC will now transition to one or more immutable ADRs.
5. **Rejected:** The proposal was found to be unviable or out of scope. The reasons for rejection should be documented in the **Approval/Rejection Notes** section for future reference.
