---
name: ready-for-release-check
description: "Pre-release checklist and quality gate to verify codebase health, documentation accuracy, dependency compliance, security boundaries, and release artifact validity before interacting with Git."
---

# Pre-Release Checklist & Quality Gate

This skill establishes the authoritative pre-release quality gate checklist. It ensures that before any release candidate is packaged, tagged, or committed, the workspace is fully verified against compiling standards, security practices, and documentation completeness.

---

## 1. Trigger Conditions
Activate this skill whenever:
- Preparing to tag, publish, or release a new version of the software.
- Concluding a major milestone, refactoring, or feature implementation.
- Running final verification gates on a pull request before merging.
- Staging code for a production or release-candidate deployment.

---

## 2. The Golden Rule of Releases
> [!CAUTION]
> **YOU ARE ONLY ALLOWED TO TOUCH GIT (stage, commit, tag, push, or release) AFTER THIS CHECKLIST IS 100% COMPLETED AND VERIFIED.**
> This absolute boundary ensures that we never publish invalid, uncompilable, unsecured, or poorly documented releases to the community or package registries.

---

## 3. The 15-Point Pre-Release Checklist

Before performing any Git operations to finalize a release, you **must** run and check off every single item on this list:

### Codebase Health & Quality
- [ ] **1. Compilation Check:** Does the code compile successfully without warnings or errors? (Run GoDoctor's `smart_build` or `go build ./...`).
- [ ] **2. Test Verification:** Do all unit, integration, and workspace tests pass successfully? (Run GoDoctor's `smart_build` or `go test ./...`).
- [ ] **3. Linter Compliance:** Are all linting errors, style warnings, cognitive complexity alerts, and static check findings fully addressed? (Check `golangci-lint` or running compiler gates).
- [ ] **4. Regression Assessment:** Did you explicitly check for regressions? Verify that new changes do not break legacy features, old command flags, or backwards-compatible setups.
- [ ] **5. Functional Correctness:** Does the software work exactly as expected? Manually verify standard user stories, happy path scenarios, and failure inputs to confirm the logic is sound.

### Versioning & Assets
- [ ] **6. Version Bumps:** Are versions bumped correctly across all manifests, config files, and registry files (e.g. `gemini-extension.json`, `go.mod`, `plugin.json` or equivalent)?
- [ ] **7. Release Files Alignment:** Are all release-specific packaging files (e.g., `.goreleaser.yaml` or build script lists) fully updated to include all newly introduced files and exclude deleted ones?
- [ ] **8. Software & Model Versions:** Are all external software, library dependencies, and LLM model identifiers verified to be up to date and correct in the real world (using the `latest-version` skill)?
- [ ] **9. CI/CD & Release Workflow Audit:** Did you check the CI/CD workflow configurations (such as `.github/workflows/` or build pipeline scripts) to verify automated release triggers? Ensure the correct Git release tags are prepared, created, and pushed to the remote repository so the packaging run does not fail.

### Documentation & Decisions
- [ ] **10. README Precision:** Is the `README.md` file completely up to date, mapping the exact current list of features, setup requirements, installation details, and CLI options?
- [ ] **11. Context-Sensitive Files:** Are all context-sensitive system prompt guides (such as `GODOCTOR.md` or CLAUDE.md) updated to reflect the absolute latest tool registries and boundaries?
- [ ] **12. General Documentation:** Are all other developer guides, operational runbooks, and documentation files fully aligned and clear?
- [ ] **13. Decision Records:** Are all architectural decisions fully documented? Ensure all key choices have active, numbered ADRs under `design/adr/` (with superseded decisions cleanly archived).

### Security & Compliance
- [ ] **14. Security Review:** Have you conducted a security review for common attack vectors? (e.g., input sanitization, safe command execution, access control restrictions on paths, session separation).
- [ ] **15. Credential Leak Check:** Have you checked that absolutely no credentials, API keys, private keys, or passwords are leaked, either in environment files (`.env`), temporary files, or inline within the source code?

---

## 4. Execution Protocol

If **any** checklist item fails, you must **HALT the release process immediately**. 
1. Fix the underlying issue (e.g. correct the broken test, fix the linter warning, update the doc).
2. Rerun the failed check.
3. Once all 15 boxes are checked `[x]`, you are formally authorized to proceed with the Git commit, version tagging, and publishing sequence.
