---
name: go-test
description: >
  Advanced Go testing skill with mutation testing and coverage analysis.
  Activate when writing tests, improving test quality, or analyzing coverage.
  Uses Selene for mutation testing and testquery for SQL-based coverage analysis.
---

# Go Testing Skill

This skill guides writing effective Go tests and objectively measuring their quality using mutation testing and coverage analysis.

## Core Principle

**Tests exist to catch bugs, not to hit coverage numbers.** Use mutation testing to prove your tests actually detect real defects.

## LLM Testing Failure Modes

Before writing tests, be aware of these common mistakes:

1. **Happy path tunnel vision**: Test failure cases, not just success
2. **Testing implementation, not behavior**: Tests should survive refactors
3. **Weak assertions**: Don't just check `err == nil` — verify the result
4. **Mock addiction**: Prefer real implementations over mocks when feasible
5. **No concurrency testing**: Always run with `-race`, test concurrent access
6. **No adversarial thinking**: Ask "what input would break this?"

## Workflow

### 1. Understand the Code
```
smart_read(filename="target.go", outline=true)
```

### 2. Write Tests
- Default to **table-driven tests** with `t.Run()`
- Use `t.Parallel()` for independent subtests
- Test these categories systematically:
  - **Boundaries**: zero, nil, empty, max values, unicode
  - **Error paths**: every error return, error types, wrapping
  - **State transitions**: Init twice, Close then Write, etc.
  - **Concurrency**: concurrent access with `-race`

### 3. Verify Tests Pass
```
smart_build(run_tests=true)
```

### 4. Mutation Test (Prove Quality)
```
mutation_test(dir=".")
```
Review surviving mutants — each one is a real defect your tests missed. Fix the tests to catch them.

### 5. Analyze Coverage
Use SQL queries to find gaps:
```
test_query(query="SELECT * FROM all_coverage WHERE count = 0")
```

### 6. Iterate
Repeat steps 2-5 until mutation score is acceptable.

## Example Queries

See `assets/queries.sql` for useful testquery SQL patterns.

## References
- `references/tq_schema.md`: TestQuery database schema and common SQL patterns.

## Tools
- `smart_read`: Understand the code under test
- `smart_build`: Run tests and verify
- `mutation_test`: Measure test quality with mutation testing (Selene)
- `test_query`: Query test results and coverage with SQL (testquery/tq)
