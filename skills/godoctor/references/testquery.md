# TestQuery SQL Test Analytics Guide

[TestQuery](https://github.com/danicat/testquery) records Go test execution logs and statement coverage into a local SQLite database (`testquery.db`), enabling fast SQL-driven test analytics and coverage queries.

## Key Features & Safeguards

- **Zero-Fallback & Binary Discovery**: TestQuery executes directly from `$PATH` or as configured in `.godoctor.yaml`. Dynamic `go run` compilation fallbacks are banned to eliminate execution delays.
- **SQLite WAL Mode & Concurrency**: Operates with SQLite Write-Ahead Logging (`PRAGMA journal_mode=WAL;`) and busy timeouts (`PRAGMA busy_timeout=5000;`) to eliminate database locks during concurrent test runs and queries.
- **Unrestricted SQL Queries**: GoDoctor SafeShell supports standard SQL comparison operators (`<`, `>`, `<=`, `>=`), multiline queries (`\n`), string concatenation (`||`), and semicolons.
- **Auto-Synchronization**: Running `godoctor call test` automatically indexes test execution results and statement coverage into `testquery.db`.

---

## Database Schema Reference

| Table | Purpose | Key Columns |
| :--- | :--- | :--- |
| `all_tests` | Test outcomes, execution times, and log output lines. | `time`, `action` (`pass`/`fail`), `package`, `test`, `elapsed`, `output` |
| `all_coverage` | Statement execution counts by block. | `package`, `file`, `function_name`, `start_line`, `end_line`, `stmt_num`, `count` |
| `test_coverage` | Mapping of individual tests to statement blocks. | `test_name`, `package`, `file`, `start_line`, `end_line`, `stmt_num`, `count` |
| `all_code` | Source code lines for join-based inspection. | `package`, `file`, `line_number`, `content` |
| `selene` | AST mutation records and statuses. | `id`, `mutator`, `file`, `line`, `col`, `status` (`'killed'`, `'survived'`, `'uncovered'`, `'timeout'`, `'excluded'`), `killed_by` |
| `selene_tests` | Test effectiveness and mutation kill counts. | `test_name`, `package`, `status`, `mutations_killed`, `killed_mutant_ids` |

### Predefined Views
- `failed_tests`: Lists all failed test cases with execution time.
- `passed_tests`: Lists passing test cases.
- `missing_coverage`: Uncovered statements (`count = 0`).
- `code_coverage`: Percentage coverage aggregated by package and file.
- `selene_survived`: Surviving mutants that escaped existing test assertions.
- `selene_zero_kill_tests`: Tests that executed without catching any AST mutants.
- `selene_summary`: High-level mutation statistics and test suite score.
- `selene_excluded`: Mutants excluded for filesystem or process safety.

---

## Installation & Environment Verification

Verify installed tools using GoDoctor:
```bash
godoctor check
```

Install or upgrade TestQuery via Go toolchain:
```bash
go install github.com/danicat/testquery@latest
```

---

## Configuration via `.godoctor.yaml`

Configure TestQuery parameters in `.godoctor.yaml`:

```yaml
tools:
  testquery:
    command: "testquery"
    recommended_version: "latest"
    pkg: "github.com/danicat/testquery@latest"
    config: ".godoctor/testquery.db"
    timeout: "2m"
    disabled: false

testquery:
  db_path: ".godoctor/testquery.db"
  format: "table"

features:
  testquery_sync: true
```

---

## Querying Test Analytics

### 1. Via GoDoctor CLI (`godoctor call tq`)
Always specify an absolute directory path for `dir`:
```bash
godoctor call tq '{"dir": "/absolute/path/to/project", "query": "SELECT package, test, elapsed FROM all_tests WHERE action = '\''fail'\''"}'
```

### 2. In MCP Mode (`test_query`)
```json
{
  "dir": "/absolute/path/to/project",
  "query": "SELECT package, test, elapsed FROM all_tests WHERE action = 'fail';"
}
```

### 3. Direct CLI Tool (if in PATH)
```bash
testquery query --db testquery.db "SELECT * FROM all_tests WHERE action = 'fail'"
```

---

## Common SQL Query Recipes

### 1. Show Recent Test Failures with Outputs
```sql
SELECT package, test, elapsed, output
FROM all_tests
WHERE action = 'fail'
ORDER BY time DESC;
```

### 2. Identify Packages with Lowest Statement Coverage
```sql
SELECT 
    package,
    SUM(CASE WHEN count > 0 THEN stmt_num ELSE 0 END) AS covered_stmts,
    SUM(stmt_num) AS total_stmts,
    ROUND(100.0 * SUM(CASE WHEN count > 0 THEN stmt_num ELSE 0 END) / SUM(stmt_num), 2) AS coverage_pct
FROM all_coverage
GROUP BY package
ORDER BY coverage_pct ASC;
```

### 3. Find Uncovered Code Blocks in a Package
```sql
SELECT file, function_name, start_line, end_line, stmt_num
FROM all_coverage
WHERE count = 0 AND package LIKE '%auth%'
ORDER BY file, start_line;
```

### 4. Locate Exact Source Lines of Uncovered Statements
```sql
SELECT c.file, c.line_number, c.content
FROM all_code c
JOIN all_coverage cov 
  ON c.file = cov.file 
 AND c.line_number BETWEEN cov.start_line AND cov.end_line
WHERE cov.count = 0
ORDER BY c.file, c.line_number;
```

### 5. Find Slowest Passing Tests (> 0.25s)
```sql
SELECT package, test, elapsed
FROM all_tests
WHERE action = 'pass' AND elapsed > 0.25
ORDER BY elapsed DESC;
```

### 6. Inspect Test Coverage for a Specific Function
```sql
SELECT file, start_line, end_line, count
FROM all_coverage
WHERE function_name = 'ValidateToken'
ORDER BY start_line;
```

---

## Notes & Best Practices

- **Auto-Generation**: `testquery.db` is updated automatically whenever `godoctor call test` runs. If `testquery.db` does not exist when `godoctor call tq` is invoked, GoDoctor builds it first.
- **Absolute Directory Required**: The `dir` parameter is strictly required and must be an absolute path.
- **SQL String Literal Syntax**: In SQLite, string literals must use single quotes `'...'` (e.g. `WHERE status = 'killed'`). In JSON payloads, escape single quotes appropriately (`'\''` in bash single-quoted strings or standard JSON strings).
- **Concurrency Safety**: WAL mode allows concurrent queries while tests are writing. Avoid launching concurrent full test runs while mutation testing is in flight to prevent database lock contention.
