# TestQuery SQLite Database (`testquery.db`) Reference

The `test_query` tool executes SQL queries against the `testquery.db` SQLite database to analyze historic test runs, line-by-line coverage, and source code patterns.

---

## Database Schemas & Example Queries

### 1. `all_tests` Table
Contains execution history for individual unit and integration tests.

- **Schema**:
  ```sql
  CREATE TABLE all_tests (
      time TIMESTAMP,
      action TEXT,      -- 'run', 'pass', 'fail', 'skip'
      package TEXT,     -- e.g. 'github.com/danicat/godoctor/internal/config'
      test TEXT,        -- Test function name (e.g. 'TestLoadConfig')
      elapsed NUMERIC,  -- Duration in seconds
      output TEXT       -- Test execution output/logs
  );
  ```
- **Example Queries**:
  ```sql
  -- Find all failing tests and their output
  SELECT test, package, output FROM all_tests WHERE action = 'fail';

  -- List slowest tests (> 1 second)
  SELECT test, package, elapsed FROM all_tests WHERE action = 'pass' AND elapsed > 1.0 ORDER BY elapsed DESC;
  ```

---

### 2. `all_coverage` Table
Contains overall function and statement coverage statistics across all package files.

- **Schema**:
  ```sql
  CREATE TABLE all_coverage (
      package TEXT,
      file TEXT,
      start_line INT,
      start_col INT,
      end_line INT,
      end_col INT,
      stmt_num INT,
      count INT,          -- Execution count (0 = uncovered statement)
      function_name TEXT
  );
  ```
- **Example Queries**:
  ```sql
  -- Find uncovered code paths (count = 0)
  SELECT file, function_name, start_line, end_line FROM all_coverage WHERE count = 0 ORDER BY file;

  -- Calculate total covered vs uncovered statements
  SELECT package, SUM(CASE WHEN count > 0 THEN stmt_num ELSE 0 END) AS covered_stmts, SUM(stmt_num) AS total_stmts FROM all_coverage GROUP BY package;
  ```

---

### 3. `test_coverage` Table
Per-test granular statement coverage mapping.

- **Schema**:
  ```sql
  CREATE TABLE test_coverage (
      test_name TEXT,
      package TEXT,
      file TEXT,
      start_line INT,
      start_col INT,
      end_line INT,
      end_col INT,
      stmt_num INT,
      count INT,
      function_name TEXT
  );
  ```
- **Example Queries**:
  ```sql
  -- Find tests that cover a specific file
  SELECT DISTINCT test_name FROM test_coverage WHERE file LIKE '%config.go' AND count > 0;
  ```

---

### 4. `all_code` Table
Line-by-line indexed source code repository.

- **Schema**:
  ```sql
  CREATE TABLE all_code (
      package TEXT,
      file TEXT,
      line_number INT,
      content TEXT
  );
  ```
- **Example Queries**:
  ```sql
  -- Search for panics or TODOs in source code
  SELECT file, line_number, content FROM all_code WHERE content LIKE '%panic%' OR content LIKE '%TODO%';
  ```

---

### 5. `metadata` Table
Key-value metadata for the test database generation run.

- **Schema**:
  ```sql
  CREATE TABLE metadata (
      key TEXT PRIMARY KEY,
      value TEXT
  );
  ```
