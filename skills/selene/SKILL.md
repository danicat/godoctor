---
name: selene
description: Activate this skill whenever auditing unit test effectiveness, identifying assertion gaps, hardening test suites, or performing mutation testing on Go codebases using Selene. Trigger when the user asks "how good are my unit tests?", "find untested edge cases", "check surviving mutants", "run mutation testing", or wants to verify that tests reliably fail when logic, comparison boundaries, or return values are altered.
---

# Selene Mutation Testing Guide

Mutation testing evaluates unit test effectiveness by introducing deliberate defects into Go Abstract Syntax Trees (AST) and checking if test suites detect them.

## Mutation Outcomes

| Outcome | Meaning | Action Required |
| :--- | :--- | :--- |
| `KILLED` | Test suite caught the defect (at least one test failed). | ✅ None. Ideal outcome. |
| `SURVIVED` | Tests passed despite corrupted logic. | ⚠️ **Assertion Gap**: Write assertions verifying this specific logic/return. |
| `UNCOVERED` | No test executed the mutated line. | ⚠️ **Coverage Gap**: Add tests that exercise this code branch. |
| `TIMEOUT` | Mutation induced an infinite loop or hang. | ✅ Treated as caught/killed. |

---

## Running Selene

### Primary: via GoDoctor CLI (`godoctor call selene`)
Always provide an **absolute path** to the target workspace or package:
```bash
godoctor call selene '{"dir": "/absolute/path/to/project"}'
```

### In MCP Mode:
```json
{
  "dir": "/absolute/path/to/project"
}
```

### Direct CLI Tool (if in PATH):
```bash
selene ./...
```

---

## 5-Step Mutation Resolution Loop

When surviving mutants are reported, systematically eliminate them:

```
┌────────────────────────┐
│ 1. Run Selene          │ ──► Identify SURVIVED mutants & file locations
└───────────┬────────────┘
            │
┌───────────▼────────────┐
│ 2. Classify Defect     │ ──► Boundary (>=), Payload (return ""), or Boolean (!ok)
└───────────┬────────────┘
            │
┌───────────▼────────────┐
│ 3. Add Targeted Test   │ ──► Add table-driven case asserting the exact condition
└───────────┬────────────┘
            │
┌───────────▼────────────┐
│ 4. Run Fast Tests      │ ──► godoctor call test '{"level": "fast"}'
└───────────┬────────────┘
            │
┌───────────▼────────────┐
│ 5. Re-run Selene       │ ──► Confirm mutant is now KILLED
└────────────────────────┘
```

---

## Mutation Elimination Recipes

### 1. Boundary Mutations (`>=` mutated to `>`)
Surviving comparison operators mean the test suite lacks boundary value checks.
- **Fix**: Add test cases for values immediately below, exactly on, and immediately above the threshold.
```go
tests := []struct {
    name     string
    val      int
    expected bool
}{
    {name: "below boundary", val: 17, expected: false},
    {name: "exact boundary", val: 18, expected: true},
    {name: "above boundary", val: 19, expected: true},
}
```

### 2. Return Value Mutations (`return token, nil` $\to$ `return "", nil`)
Surviving return values happen when tests verify `err == nil` but ignore the returned data payload.
- **Fix**: Assert payload contents, lengths, or non-zero struct fields:
```go
res, err := GenerateToken("user123")
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
if res == "" {
    t.Errorf("expected non-empty token")
}
```

### 3. Boolean Inversion Mutations (`if isReady` $\to$ `if !isReady`)
Surviving inverted booleans indicate missing test coverage for one branch of conditional logic.
- **Fix**: Provide dedicated subtests exercising both `true` and `false` states.

---

## Critical Gotchas

> [!IMPORTANT]
> 1. **Baseline Tests Must Pass**: If tests currently fail, mutation testing cannot determine kill status. Always fix failing unit tests before running Selene.
> 2. **Absolute Directory Required**: `dir` must always be an absolute path.
