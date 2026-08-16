---
name: testquery
description: Activate this skill whenever analyzing Go test execution results, inspecting failed test output logs, finding slow tests, or querying statement-level coverage gaps using TestQuery and SQLite. Trigger when the user asks which tests failed, show slow tests, find uncovered lines/functions, or calculate test coverage by package.
---

# TestQuery SQL Test Analytics Guide

[TestQuery](https://github.com/danicat/testquery) records Go test execution logs and statement coverage into a local SQLite database (`testquery.db`), enabling SQL-driven test analytics and coverage queries.

## Database Schema Reference

| Table | Purpose | Key Columns |
| :--- | :--- | :--- |
| `all_tests` | Test outcomes, execution times, and log output lines. | `time`, `action` (`pass`/`fail`), `package`, `test`, `elapsed`, `output` |
| `all_coverage` | Statement execution counts by block. | `package`, `file`, `function_name`, `start_line`, `end_line`, `stmt_num`, `count` |
| `test_coverage` | Mapping of individual tests to statement blocks. | `test_name`, `package`, `file`, `start_line`, `end_line`, `stmt_num`, `count` |
| `all_code` | Source code lines for join-based inspection. | `package`, `file`, `line_number`, `content` |

## Querying Test Analytics

### 1. Via GoDoctor CLI (`godoctor call tq`)
Always specify an absolute directory path:
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

## Common SQL Queries

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

## Notes

- **Auto-generation**: `testquery.db` is updated automatically whenever `godoctor call test` runs. If `testquery.db` does not exist when `godoctor call tq` is invoked, GoDoctor builds it first.
- **Absolute directory required**: The `dir` parameter must be an absolute path.
