---
name: selene
description: Activate this skill to audit unit test effectiveness, find assertion gaps, and run mutation testing on Go codebases using Selene. Trigger when analyzing test suite quality, checking surviving mutants, identifying zero-kill tests, or hardening test suites.
---

# Selene Mutation Testing Guide

Selene evaluates Go unit test suites by introducing syntactic defects into the Abstract Syntax Tree (AST) and checking whether existing tests catch them.

## Features

- **Targeted fast mode**: Integrates with [TestQuery](skill:testquery) (`testquery.db`) to run only the tests that cover mutated lines, using precise `-run` patterns.
- **In-memory compilation**: Uses Go's `-overlay` flag to build mutated code without altering files on disk.
- **Safety exclusions**: Automatically ignores dangerous operations (`os.RemoveAll`, `exec.Command`, `syscall.*`) and their conditional guards.
- **Worker pool**: Runs mutations across parallel workers (`-workers N`) with process-level timeout isolation.
- **Test scoring**: Identifies tests that killed mutants vs tests that ran but never caught a bug (zero-kill tests).

## Mutant Statuses

| Status | Meaning | Action |
| :--- | :--- | :--- |
| `KILLED` | A test caught the mutation and failed. | None. Test behaves as expected. |
| `SURVIVED` | All tests passed despite modified code. | Add assertions for the mutated logic or return value. |
| `UNCOVERED` | No test ran across the mutated line. | Add test coverage for this branch. |
| `TIMEOUT` | Mutation caused an infinite loop or hang. | Treated as killed. |
| `EXCLUDED` | Mutation was skipped for safety reasons. | None. Skipped automatically. |

## Installation

Install via prebuilt binary:
```bash
curl -fsSL https://raw.githubusercontent.com/danicat/selene/main/install.sh | bash
```

Or via Go toolchain:
```bash
go install github.com/danicat/selene/cmd/selene@latest
```

## Running Selene

### Targeted run with TestQuery (Recommended)

Generate coverage data first, then run Selene against the database:
```bash
tq build ./...
selene --db testquery.db -workers 8 -v ./...
```

### Untargeted run
```bash
selene -workers 8 ./...
```

### JSON output for CI
```bash
selene --db testquery.db -json ./...
```

### Via GoDoctor CLI
```bash
godoctor call selene '{"dir": "/absolute/path/to/project"}'
```

## Querying Results

Selene stores mutation records and test metrics in `testquery.db`. Query them using `tq query` or the `sqlite3` CLI:

### Surviving mutants (assertion gaps)
```sql
SELECT id, mutator, file, line, col 
FROM selene_survived 
ORDER BY file, line;
```

### Zero-kill tests (tests that caught zero mutants)
```sql
SELECT test_name, package 
FROM selene_zero_kill_tests;
```

### Safety-excluded mutations
```sql
SELECT id, mutator, file, line, col, reason 
FROM selene_excluded;
```

### Summary metrics
```sql
SELECT * FROM selene_summary;
```

### Most effective tests
```sql
SELECT test_name, package, mutations_killed, killed_mutant_ids 
FROM selene_tests 
WHERE mutations_killed > 0 
ORDER BY mutations_killed DESC 
LIMIT 10;
```

## Fixing Surviving Mutants

When mutants survive, the test suite usually lacks specific assertions:

### Boundary mutations (e.g. `>=` changed to `>`)
Add test cases for values right at the boundary:
```go
tests := []struct {
    name     string
    val      int
    expected bool
}{
    {name: "below threshold", val: 17, expected: false},
    {name: "at threshold",    val: 18, expected: true},
    {name: "above threshold", val: 19, expected: true},
}
```

### Return value mutations (e.g. `return token, nil` changed to `return "", nil`)
Tests that only check `err == nil` miss payload bugs. Assert return values directly:
```go
res, err := GenerateToken("user123")
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
if res == "" {
    t.Errorf("expected non-empty token")
}
```

### Inverted conditionals (e.g. `if ok` changed to `if !ok`)
Ensure test suites cover both `true` and `false` execution branches.

## Notes

- **Baseline tests must pass**: If existing tests are failing, fix them before running mutation tests.
- **Refresh coverage for targeted mode**: Always run `tq build` before running `selene --db testquery.db` so the coverage index matches the latest code.
