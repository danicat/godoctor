---
name: go-troubleshooting
description: "Highly actionable step-by-step checklist for diagnosing and resolving Go compilation errors, type errors, build/test failures, and runtime issues. Activate on any build or execution failure."
---

# Go Code Troubleshooting & Diagnostic Checklist

This skill outlines a rigorous, compiler-backed troubleshooting protocol for isolating, understanding, and correcting Go build and runtime errors.

## 1. Trigger Conditions
Activate this skill whenever the workspace encounters:
- Compilation errors or syntax errors.
- Type errors (e.g., mismatched types, undefined methods/fields).
- Build or test suite failures.
- Runtime panics, memory leaks, or data races.
- Any Go-related command failure.

## 2. Step-by-Step Diagnostic Checklist

Follow this systematic sequence to resolve issues, utilizing GoDoctor's compiler-gated tools to ensure precision:

### Step 1: Analyze Compilation Diagnostics
Examine the detailed error output from `smart_build` or `smart_edit`. 
- **Identify file coordinates:** Note the exact filename, line, and column of the failure (e.g., `main.go:34:12`).
- **Parse the error message:** Distinguish between a syntax issue (e.g., missing parenthesis), import issue (missing module), or strict type mismatch.

### Step 2: Leverage Code & Symbol Queries
Do not guess the shape of the types or functions causing the failure. Use GoDoctor's structural search and reference tools:
- **Read code with type annotations (`smart_read`):** Read the offending lines. The output will automatically include a `<types>` block outlining full structures, methods, and field declarations of any custom types referenced in those lines.
- **Inspect definition and references (`describe_symbol`):** Query the exact symbol coordinates (line and column) where the error is reported. This will instantly show:
  1. The exact symbol definition, its expected package, and its signature.
  2. All call-sites across the entire workspace to verify if other places are calling it correctly or differently.

### Step 3: Check Spelling and Field Matching
If the compiler reports `undeclared name`, `undefined`, or `no field or method`:
- **Utilize spelling recommendations:** When `smart_edit` fails, it automatically runs Levenshtein distance calculations against all symbols in the target package (via `gopls symbols`). Review the **💡 Suggestions:** section in the error report.
- **Verify field casing:** In Go, fields and methods must be capitalized (e.g., `Name` vs `name`) to be exported and visible outside their declaring package.

### Step 4: Investigate Runtime & Test Gaps
If compilation succeeds but tests are failing or runtime behaviors are unexpected:
- **Query coverage and execution results (`test_query`):** Run SQL queries on the test query SQLite database (`testquery.db`). This allows you to inspect:
  - Exact lines/branches with zero coverage count (`SELECT * FROM all_coverage WHERE count = 0`).
  - History of test failures and panics.
- **Run Mutation Tests (`mutation_test`):** To identify weak assertions or untested edge cases, execute mutation tests. Review surviving mutants to see where the test suite fails to detect behavioral changes.

### Step 5: Implement Safe, Atomic Correction
- **Edit atomically (`smart_edit`):** Apply edits to all target files in a single transaction. Do not use direct, unverified file-writing tools.
- **Verify compiler gate:** If the edit is successful, it means the changes were fully formatted and compiled successfully by `gopls check ./...`. If it fails, the workspace is automatically kept in its original healthy state with zero on-disk corruption.
