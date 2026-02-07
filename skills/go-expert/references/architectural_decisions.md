# Architectural Decision Records (ADRs)

Documenting significant architectural decisions is critical for long-term project health. Use this template to record choices.

## ADR Template

```markdown
# [Short Title, e.g., Use Postgres for Metadata]

**Status:** [Proposed | Accepted | Deprecated | Superseded]
**Date:** [YYYY-MM-DD]
**Author:** [Name]

## Context
[Describe the problem or opportunity. What options were considered? Why is a decision needed now?]

## Decision
[Describe the decision clearly. "We will use X to do Y."]

## Consequences
**Positive:**
*   [Benefit 1]
*   [Benefit 2]

**Negative:**
*   [Drawback 1]
*   [Drawback 2]

**Risks:**
*   [Risk 1]
```

## When to write an ADR
*   Choosing a database or storage engine.
*   Selecting a major framework (e.g., Echo vs. Gin).
*   Defining a new API standard.
*   Introducing a new dependency that significantly impacts the build size or complexity.
