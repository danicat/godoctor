---
name: selene
description: Mutation testing workflows for Go using Selene. Activate when checking unit test effectiveness, reviewing surviving or uncovered mutants, and writing targeted assertions.
---

# Selene mutation testing guide

Mutation testing measures unit test effectiveness by modifying the syntax tree of your Go code and running tests against each change. [Selene](https://github.com/danicat/selene) modifies operators such as arithmetic signs (`+` to `-`), comparison boundaries (`>=` to `>`), boolean checks, and return values.

## Core concepts

When Selene introduces an Abstract Syntax Tree (AST) mutation, it records one of four outcomes:

| Outcome | Description | Resolution |
| :--- | :--- | :--- |
| `KILLED` | At least one test failed on mutated code. | Expected outcome. The test suite detects the logic defect. |
| `SURVIVED` | All tests passed despite mutated logic. | Assertion gap. Add tests or assertions covering the specific logic. |
| `UNCOVERED` | No test executed the mutated block. | Coverage gap. Write test cases that reach this code path. |
| `TIMEOUT` | Mutation caused an infinite loop or hang. | Treated as killed. |

The overall mutation score measures the percentage of introduced defects caught by tests:

$$\text{Mutation Score} = \frac{\text{Killed Mutants}}{\text{Total Mutants}} \times 100\%$$

## Running mutation tests

Follow this execution hierarchy depending on your current environment:

### 1. If `selene` is installed in PATH:
Run `selene` directly from the shell:
```bash
selene ./...
# Or for a specific package:
selene ./internal/auth
```

### 2. If `selene` is not installed, but `godoctor` CLI is available:
Invoke Selene via the `godoctor call` CLI subcommand with a JSON arguments object (requires an absolute directory path):
```bash
godoctor call selene '{"dir": "/path/to/workspace"}'
```

### 3. If `godoctor` is running in MCP mode:
Call the `selene` MCP tool with the target absolute directory path (relative paths are rejected):
```json
{
  "dir": "/path/to/workspace"
}
```

### 4. Direct Go toolchain fallback:
If neither CLI binary is installed:
```bash
go run github.com/danicat/selene/cmd/selene@latest ./...
```

## Reviewing output and fixing surviving mutants

### Sample report
```text
Mutation testing results:

Total mutations: 12
Killed:          10
Timeouts:         0
Survived:         2
Uncovered:        0

Mutation Score:  83.33% (killed/total mutations)

Surviving Mutants:
1. ./internal/auth/auth.go:42:15
   Mutated: 'if user.Age >= 18' -> 'if user.Age > 18'
   Status: SURVIVED (Tests passed when condition was mutated)

2. ./internal/auth/auth.go:58:8
   Mutated: 'return token, nil' -> 'return "", nil'
   Status: SURVIVED (Tests passed when token was cleared)
```

### Fixing boundary mutations

Surviving comparison mutations such as `>=` changed to `>` mean the test suite lacks boundary value checks.

Add table-driven test cases covering boundary values:
```go
func TestIsAdult(t *testing.T) {
    tests := []struct {
        name     string
        user     User
        expected bool
    }{
        {name: "below boundary", user: User{Age: 17}, expected: false},
        {name: "exact boundary", user: User{Age: 18}, expected: true},
        {name: "above boundary", user: User{Age: 19}, expected: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := IsAdult(tt.user); got != tt.expected {
                t.Errorf("IsAdult() = %v, want %v", got, tt.expected)
            }
        })
    }
}
```

### Fixing return value mutations

Surviving return value mutations happen when unit tests check `err == nil` but ignore the returned data payload.

Assert the returned values in addition to error checks:
```go
func TestGenerateToken(t *testing.T) {
    token, err := GenerateToken("user123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if token == "" {
        t.Fatal("expected non-empty token")
    }
    if !strings.HasPrefix(token, "ey") {
        t.Errorf("token = %q, want JWT prefix ey", token)
    }
}
```

### Fixing boolean inversion mutations

When an inverted boolean condition survives, add test cases that explicitly exercise both `true` and `false` execution paths.
