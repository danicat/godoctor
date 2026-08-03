# Selene Mutation Testing Reference (`mutation_test`)

Selene is GoDoctor's high-performance mutation testing engine. It objectively evaluates the assertion strength and behavioral coverage of Go test suites by injecting subtle syntax and AST mutations into source code and verifying whether existing unit/integration tests detect and kill them.

---

## How Mutation Testing Works

1. **Baseline Verification**: Selene runs the target package's test suite to ensure all tests pass initially.
2. **Mutant Generation**: Selene parses the Go AST and systematically generates code mutations.
3. **Execution & Evaluation**:
   - For each mutant, Selene re-runs the package test suite.
   - **Killed Mutant (Pass)**: At least one test failed due to the mutation. The test suite correctly detected the behavior change.
   - **Surviving Mutant (Fail/Warning)**: All tests passed despite the code mutation. Indicates a gap in test assertions or untested boundary conditions.

---

## Supported AST Mutation Operators

| Mutation Operator | Original Code | Mutated Code | Description |
| :--- | :--- | :--- | :--- |
| **Condition Inversion** | `if err != nil` | `if err == nil` | Inverts conditional logic |
| **Relational Operator Swap** | `if count > 0` | `if count >= 0` or `if count < 0` | Modifies boundary checks |
| **Arithmetic Operator Swap** | `total = a + b` | `total = a - b` | Alters math operations |
| **Boolean Logic Inversion** | `if isValid && active` | `if isValid \|\| active` | Flips AND/OR logic |
| **Return Value Mutation** | `return err` | `return nil` | Alters function return values |
| **Statement Removal** | `log.Println(...)` | `/* deleted */` | Removes side-effect statements |

---

## Interpreting `mutation_test` Output

- **Mutation Score**:
  $$\text{Mutation Score} = \frac{\text{Killed Mutants}}{\text{Total Mutants}} \times 100\%$$
- **Target Threshold**: Aim for a **> 80% mutation score** on critical domain logic.

---

## Action Plan for Surviving Mutants

When `mutation_test` reports surviving mutants:

1. **Locate the Mutation**: Identify the file path, line number, and original vs mutated AST node from the `mutation_test` report.
2. **Analyze the Coverage Gap**:
   - If line coverage already exists, the issue is **weak assertions** (e.g., tests call the function but do not assert on exact return values or side effects).
   - If line coverage is zero, the issue is **untested code paths**.
3. **Write Targeted Boundary Tests**: Add a table-driven unit test case that specifically asserts the exact behavior at that boundary condition.
4. **Re-run `mutation_test`**: Confirm that the new test kills the mutant.
