# Scenario C: The Migration

## Background
The `io/ioutil` package has been deprecated since Go 1.16. Its functions have moved to `os` and `io`.

## Task
Migrate the codebase to replace all usages of `ioutil`:
*   `ioutil.ReadFile` -> `os.ReadFile`
*   `ioutil.ReadDir` -> `os.ReadDir` (Note: returns `[]os.DirEntry` instead of `[]fs.FileInfo`, might need small adjustment if used, but here just printing name is compatible mostly)
*   `ioutil.ReadAll` -> `io.ReadAll`

## Instructions for Agent
*   Find all usages of `ioutil`.
*   Replace them with the modern equivalents.
*   Update imports (remove `io/ioutil`, add `os` or `io`).
*   Ensure the code still compiles.
