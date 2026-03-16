-- Example testquery (tq) SQL queries for Go test analysis
-- Use with: test_query(query="...")

-- Find all uncovered lines
SELECT * FROM all_coverage WHERE count = 0;

-- Find uncovered functions (no line in function was hit)
SELECT function_name, file, start_line, end_line
FROM all_coverage
GROUP BY function_name, file, start_line, end_line
HAVING MAX(count) = 0;

-- Show failing tests
SELECT package, test, elapsed, output
FROM all_tests
WHERE action = 'fail';

-- Show slowest tests (top 10)
SELECT package, test, elapsed
FROM all_tests
WHERE action = 'pass'
ORDER BY elapsed DESC
LIMIT 10;

-- Coverage summary per file
SELECT file,
       COUNT(*) AS total_stmts,
       SUM(CASE WHEN count > 0 THEN 1 ELSE 0 END) AS covered,
       ROUND(100.0 * SUM(CASE WHEN count > 0 THEN 1 ELSE 0 END) / COUNT(*), 1) AS pct
FROM all_coverage
GROUP BY file
ORDER BY pct ASC;

-- Find tests that don't cover any code (test runs but hits nothing)
SELECT DISTINCT t.test_name
FROM test_coverage t
GROUP BY t.test_name
HAVING MAX(t.count) = 0;

-- Show which tests cover a specific file
SELECT DISTINCT test_name, SUM(count) as hits
FROM test_coverage
WHERE file LIKE '%handler%'
GROUP BY test_name
ORDER BY hits DESC;

-- Find code lines that are never tested
SELECT c.file, c.line_number, c.content
FROM all_code c
JOIN all_coverage cov ON c.file = cov.file
  AND c.line_number BETWEEN cov.start_line AND cov.end_line
WHERE cov.count = 0
ORDER BY c.file, c.line_number;
