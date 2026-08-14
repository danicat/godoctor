---
name: testquery
description: SQLite-driven test analytics and coverage analysis using TestQuery. Activate when inspecting test logs, analyzing slow test runs, and querying coverage gaps with SQL.
---

# TestQuery SQL test analytics guide

[TestQuery](https://github.com/danicat/testquery) records Go test execution logs and statement coverage data into a local SQLite database (`testquery.db`). This allows developers to query test results and coverage metrics with standard SQL.

## Database schema

The `testquery.db` database contains two main tables:

### `tests` table
Stores execution details for each test function run.

| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `INTEGER PRIMARY KEY` | Unique identifier for the test record |
| `package` | `TEXT` | Go package import path |
| `name` | `TEXT` | Test function name |
| `status` | `TEXT` | Execution status: `PASS`, `FAIL`, or `SKIP` |
| `duration_ms` | `REAL` | Execution time in milliseconds |
| `output` | `TEXT` | Console output and failure trace |
| `run_at` | `TIMESTAMP` | Time when the test was run |

### `coverage` table
Stores statement-level execution data from Go coverage profiles.

| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `INTEGER PRIMARY KEY` | Unique identifier for the coverage record |
| `package` | `TEXT` | Go package import path |
| `file` | `TEXT` | Relative or absolute file path |
| `start_line` | `INTEGER` | First line of the code block |
| `end_line` | `INTEGER` | Last line of the code block |
| `num_stmt` | `INTEGER` | Number of Go statements in the block |
| `count` | `INTEGER` | Number of times the block executed (`0` means uncovered) |

## Running queries

### Using the MCP tool
Call `test_query` with a SQL query string:
```json
{
  "query": "SELECT package, name, duration_ms FROM tests WHERE status = 'FAIL' ORDER BY run_at DESC;"
}
```

### Running from the shell
Execute queries via the TestQuery CLI tool:
```bash
go run github.com/danicat/testquery@latest -query "SELECT * FROM tests WHERE status = 'FAIL'"
```

## Common SQL queries

### Listing recent test failures
```sql
SELECT package, name, duration_ms, output
FROM tests
WHERE status = 'FAIL'
ORDER BY run_at DESC
LIMIT 10;
```

### Finding uncovered code blocks
```sql
SELECT package, file, start_line, end_line, num_stmt
FROM coverage
WHERE count = 0
ORDER BY package, file, start_line;
```

### Calculating coverage percentage by package
```sql
SELECT 
    package,
    SUM(CASE WHEN count > 0 THEN num_stmt ELSE 0 END) AS covered_statements,
    SUM(num_stmt) AS total_statements,
    ROUND(100.0 * SUM(CASE WHEN count > 0 THEN num_stmt ELSE 0 END) / SUM(num_stmt), 2) AS coverage_pct
FROM coverage
GROUP BY package
ORDER BY coverage_pct ASC;
```

### Finding tests that take longer than 500ms
```sql
SELECT package, name, duration_ms
FROM tests
WHERE duration_ms > 500
ORDER BY duration_ms DESC;
```

### Detecting flaky tests with mixed pass and fail history
```sql
SELECT 
    package, 
    name, 
    COUNT(*) AS total_runs,
    SUM(CASE WHEN status = 'FAIL' THEN 1 ELSE 0 END) AS fail_count,
    SUM(CASE WHEN status = 'PASS' THEN 1 ELSE 0 END) AS pass_count
FROM tests
GROUP BY package, name
HAVING fail_count > 0 AND pass_count > 0
ORDER BY fail_count DESC;
```
