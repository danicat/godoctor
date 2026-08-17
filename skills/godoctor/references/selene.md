# Selene Mutation Testing Guide

[Selene](https://github.com/danicat/selene) evaluates Go unit test suites by introducing syntactic defects into the Abstract Syntax Tree (AST) and verifying whether existing tests catch them.

## Key Features & Safeguards

- **Zero-Fallback & Binary Discovery**: Selene executes directly from `$PATH` or as configured in `.godoctor.yaml`. Dynamic `go run` compilation fallbacks are banned to eliminate startup latency.
- **Targeted Fast Mode**: Integrates with TestQuery (`testquery.db`) to run only the tests that cover mutated lines using precise `-run` patterns.
- **In-Memory Compilation**: Uses Go's `-overlay` flag to build mutated code without altering files on disk.
- **Safety Exclusions**: Automatically ignores dangerous operations (`os.RemoveAll`, `exec.Command`, `syscall.*`) and their conditional guards.
- **Worker Pool**: Runs mutations across parallel workers (`-workers N`) with process-level timeout isolation.
- **Test Scoring**: Identifies tests that killed mutants vs tests that ran but never caught a bug (zero-kill tests).

---

## Mutant Status Reference

| Status | Meaning | Action |
| :--- | :--- | :--- |
| `KILLED` | A test caught the mutation and failed. | None. Test suite behaves as expected. |
| `SURVIVED` | All tests passed despite modified code. | Add assertions for the mutated logic, return value, or boundary. |
| `UNCOVERED` | No test executed across the mutated line. | Add unit test coverage for this branch. |
| `TIMEOUT` | Mutation caused an infinite loop or hang. | Treated as killed. |
| `EXCLUDED` | Mutation was skipped for safety reasons. | None. Skipped automatically. |

---

## Installation & Environment Verification

Verify installed tools using GoDoctor:
```bash
godoctor check
```

Install or upgrade Selene via Go toolchain:
```bash
go install github.com/danicat/selene/cmd/selene@latest
```

Or via the prebuilt binary installation script:
```bash
curl -fsSL https://raw.githubusercontent.com/danicat/selene/main/install.sh | bash
```

---

## Configuration via `.godoctor.yaml`

Configure Selene execution parameters in your repository's `.godoctor.yaml`:

```yaml
tools:
  selene:
    command: "selene"
    recommended_version: "latest"
    pkg: "github.com/danicat/selene/cmd/selene@latest"
    timeout: "3m"
    workers: 0                    # 0 defaults to runtime.GOMAXPROCS workers
    testquery_compat: true        # Enable TestQuery SQLite DB integration
    db_path: ".godoctor/testquery.db"
    disabled: false

features:
  mutation_testing: true
  testquery_compat: true
```

---

## Running Selene

### 1. Via GoDoctor CLI (Recommended)
```bash
# Direct mutation test run on repository
godoctor call selene '{"dir": "/absolute/path/to/project"}'

# Run complete test suite with Selene mutation testing enabled
godoctor call test '{"dir": "/absolute/path/to/project", "level": "complete"}'
```

### 2. Standalone Targeted Run with TestQuery
Generate coverage data first, then run Selene against the database:
```bash
tq build ./...
selene --db testquery.db -workers 8 -v ./...
```

### 3. Untargeted Run
```bash
selene -workers 8 ./...
```

### 4. JSON Output for CI Pipelines
```bash
selene --db testquery.db -json ./...
```

---

## Querying Results in `testquery.db`

Selene records mutation evaluations and test effectiveness metrics in `testquery.db`. The primary table is `selene` (with status values stored in lowercase: `'killed'`, `'survived'`, `'uncovered'`, `'timeout'`, `'excluded'`), along with several predefined views:

### 1. Surviving Mutants (Assertion Gaps)
```sql
SELECT id, mutator, file, line, col 
FROM selene_survived 
ORDER BY file, line;
```

### 2. Breakdown by Mutator
```sql
SELECT mutator, 
       COUNT(*) AS total, 
       SUM(CASE WHEN status = 'killed' THEN 1 ELSE 0 END) AS killed,
       SUM(CASE WHEN status = 'survived' THEN 1 ELSE 0 END) AS survived,
       SUM(CASE WHEN status = 'uncovered' THEN 1 ELSE 0 END) AS uncovered,
       SUM(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END) AS timeouts
FROM selene 
GROUP BY mutator 
ORDER BY total DESC;
```

### 3. Zero-Kill Tests (Tests That Caught Zero Mutants)
```sql
SELECT test_name, package 
FROM selene_zero_kill_tests;
```

### 4. Safety-Excluded Mutations
```sql
SELECT id, mutator, file, line, col, reason 
FROM selene_excluded;
```

### 5. Mutation Testing Summary Metrics
```sql
SELECT * FROM selene_summary;
```

### 6. Top 10 Most Effective Tests
```sql
SELECT test_name, package, mutations_killed, killed_mutant_ids 
FROM selene_tests 
WHERE mutations_killed > 0 
ORDER BY mutations_killed DESC 
LIMIT 10;
```

---

## Zero-Kill Test Remediation

A **Zero-Kill Test** is a test that executed during mutation testing but never failed when any AST mutation was applied. Common causes and remediation tactics:

| Root Cause | Pattern | Remediation |
| :--- | :--- | :--- |
| **Shallow Assertions** | Test only checks `err == nil` or exit code 0. | Add semantic assertions checking stdout, stderr, or parsed JSON payload fields. |
| **Redundant Subtest** | Trivial subtest superseded by a broader parent test case. | Consolidate duplicate table-driven test entries or assert on unique boundary conditions. |
| **Tautological Check** | Constructor / getter tests with no logic branches. | Combine with behavior tests or eliminate redundant unit tests. |
| **Empty Input Handlers** | Tests passing empty string / slice that early-returns. | Keep if guarding public API invariants; otherwise combine with table tests. |

---

## Fixing Surviving Mutants

When mutants survive, the test suite usually lacks specific boundary, error, or return value assertions:

### 1. Boundary Mutations (e.g. `>=` changed to `>`)
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

### 2. Return Value Mutations (e.g. `return token, nil` changed to `return "", nil`)
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

### 3. Inverted Conditionals (e.g. `if ok` changed to `if !ok`)
Ensure test suites cover both `true` and `false` execution branches.

---

## Notes & Best Practices

- **Select the Right Level**: Use `level: "fast"` during iterative coding. Reserve `level: "complete"` or standalone `selene` runs for quality audits and release validation.
- **Resource Management**: Selene spawns parallel worker processes (`-workers N`). Do not launch background test/build tasks concurrently with complete mutation runs to prevent CPU starvation and SQLite database lock contention.
- **Baseline Tests Must Pass**: If existing tests are failing, fix them before running mutation tests.
- **Refresh Coverage for Targeted Mode**: Always run `tq build` or `godoctor call test` before running targeted Selene tests so the coverage index matches the latest code.
- **Absolute Paths**: When using `godoctor call selene`, always supply an absolute path for `dir`.
