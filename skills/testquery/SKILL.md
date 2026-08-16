---
name: testquery
description: SQLite-driven test analytics and coverage analysis using TestQuery. Activate when inspecting test logs, analyzing slow test runs, and querying coverage gaps with SQL.
---

# TestQuery SQL test analytics guide

[TestQuery](https://github.com/danicat/testquery) records Go test execution logs and statement coverage data into a local SQLite database (`testquery.db`). This allows developers to query test results and coverage metrics with standard SQL.

## Database schema

The `testquery.db` database contains five tables:

### `all_tests` table
Stores test execution records and outputs.

| Column | Type | Description |
| :--- | :--- | :--- |
| `time` | `TEXT` | Timestamp of test execution |
| `action` | `TEXT` | Action/status: `run`, `pause`, `cont`, `pass`, `fail`, `output`, `skip` |
| `package` | `TEXT` | Go package import path |
| `test` | `TEXT` | Test function name |
| `elapsed` | `REAL` | Elapsed time in seconds |
| `output` | `TEXT` | Standard output / error log lines |

### `all_coverage` table
Stores statement-level execution counts across files.

| Column | Type | Description |
| :--- | :--- | :--- |
| `package` | `TEXT` | Go package import path |
| `file` | `TEXT` | File path relative to package |
| `start_line` | `INTEGER` | First line of statement block |
| `start_col` | `INTEGER` | First column of statement block |
| `end_line` | `INTEGER` | Last line of statement block |
| `end_col` | `INTEGER` | Last column of statement block |
| `stmt_num` | `INTEGER` | Number of Go statements in block |
| `count` | `INTEGER` | Execution count (`0` means uncovered) |
| `function_name` | `TEXT` | Enclosing function name |

### `test_coverage` table
Maps specific individual tests to statement coverage blocks.

| Column | Type | Description |
| :--- | :--- | :--- |
| `test_name` | `TEXT` | Individual test function |
| `package` | `TEXT` | Go package import path |
| `file` | `TEXT` | File path |
| `start_line` | `INTEGER` | First line of statement block |
| `start_col` | `INTEGER` | First column of statement block |
| `end_line` | `INTEGER` | Last line of statement block |
| `end_col` | `INTEGER` | Last column of statement block |
| `stmt_num` | `INTEGER` | Number of Go statements in block |
| `count` | `INTEGER` | Execution count for this test |
| `function_name` | `TEXT` | Enclosing function name |

### `all_code` table
Contains source code lines for join-based coverage inspections.

| Column | Type | Description |
| :--- | :--- | :--- |
| `package` | `TEXT` | Go package import path |
| `file` | `TEXT` | File path |
| `line_number` | `INTEGER` | 1-indexed source line number |
| `content` | `TEXT` | Source code line content |

### `metadata` table
Key-value metadata about the indexing run (e.g., git commit, go version).

---

## Running queries

Follow this execution hierarchy depending on your current environment:

### 1. If `testquery` is installed in PATH:
Execute queries directly via the `testquery` CLI tool:
```bash
testquery query --db testquery.db --format table "SELECT package, test, elapsed FROM all_tests WHERE action = 'fail'"
```

### 2. If `testquery` is not installed, but `godoctor` CLI is available:
Invoke queries via the `godoctor call` CLI subcommand with `tq` using a JSON arguments object (requires an absolute directory path):
```bash
godoctor call tq '{"dir": "/path/to/workspace", "query": "SELECT package, test, elapsed FROM all_tests WHERE action = '\''fail'\''"}'
```

### 3. If `godoctor` is running in MCP mode:
Call the `test_query` MCP tool with a SQL query string and the target absolute directory path (relative paths are rejected):
```json
{
  "dir": "/path/to/workspace",
  "query": "SELECT package, test, elapsed FROM all_tests WHERE action = 'fail';"
}
```

---

## Common SQL queries

### Listing recent test failures
```sql
SELECT package, test, elapsed, output
FROM all_tests
WHERE action = 'fail'
ORDER BY time DESC;
```

### Finding uncovered code blocks
```sql
SELECT package, file, function_name, start_line, end_line, stmt_num
FROM all_coverage
WHERE count = 0
ORDER BY package, file, start_line;
```

### Calculating coverage percentage by package
```sql
SELECT 
    package,
    SUM(CASE WHEN count > 0 THEN stmt_num ELSE 0 END) AS covered_statements,
    SUM(stmt_num) AS total_statements,
    ROUND(100.0 * SUM(CASE WHEN count > 0 THEN stmt_num ELSE 0 END) / SUM(stmt_num), 2) AS coverage_pct
FROM all_coverage
GROUP BY package
ORDER BY coverage_pct ASC;
```

### Finding slow tests (> 0.5s)
```sql
SELECT package, test, elapsed
FROM all_tests
WHERE action = 'pass' AND elapsed > 0.5
ORDER BY elapsed DESC;
```

### Inspecting source lines of uncovered code
```sql
SELECT c.file, c.line_number, c.content
FROM all_code c
JOIN all_coverage cov 
  ON c.file = cov.file 
 AND c.line_number BETWEEN cov.start_line AND cov.end_line
WHERE cov.count = 0
ORDER BY c.file, c.line_number;
```
