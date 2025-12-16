# Gap Analysis: `edit_code` Prototype vs. Design Specification

## Overview
This document analyzes the gap between the `edit_code` design specification in `proposal_smart_edit.md` and the initial prototype implementation.

## Feature Status

| Feature | Status | Notes |
| :--- | :--- | :--- |
| **Strategy: `single_match`** | ✅ Implemented | Works for single block replacements. |
| **Strategy: `overwrite_file`** | ✅ Implemented | Works for full file rewrites. |
| **Strategy: `replace_all`** | ❌ Missing | Logic for multiple replacements is stubbed/implied but not fully implemented or tested. |
| **Fuzzy Matching: Line-based** | ⚠️ Partial | Implemented a basic sliding window with line containment/equality. |
| **Fuzzy Matching: Levenshtein** | ❌ Missing | The proposal specified Levenshtein/Diff-based scoring. The prototype uses a simpler containment check which is less robust against typos. |
| **Fuzzy Matching: Threshold** | ⚠️ Partial | Threshold logic exists but relies on the simpler scoring mechanism. |
| **Ambiguity Detection** | ✅ Implemented | Detects multiple matches and rejects them for `single_match`. |
| **Feedback: "Best Match"** | ❌ Missing | The proposal required returning the "Best Match" diff when no match is found. The prototype returns a generic error message. |
| **Strict Syntax Validation** | ✅ Implemented | Uses `go/parser` to reject invalid ASTs before writing. |
| **Auto-Correction: `goimports`** | ✅ Implemented | Runs `imports.Process` to fix imports and formatting. |
| **Auto-Correction: Typos** | ❌ Missing | Proposal mentioned auto-fixing single-char typos in context. |
| **Soft Validation: `go build`** | ❌ Missing | The "Build/Test Check (Soft)" using a temp file was omitted for simplicity. No build warnings are returned. |
| **Input Schema** | ⚠️ Partial | Basic struct tags used. Detailed JSON schema descriptions/enums from the proposal are not fully reflected in the code (though functional). |

## Critical Gaps to Address
1.  **"Best Match" Feedback:** This is the most critical missing feature for Agent UX. Without it, the agent gets a blind "No match" error and cannot self-correct.
2.  **Robust Fuzzy Scoring:** The current `strings.Contains` check is too weak. It needs true Levenshtein distance to handle minor typos in the code itself, not just whitespace.
3.  **Soft Validation:** Implementing the `go build` check on a temp file is needed to warn agents about breaking changes (e.g., undefined variables).

## Next Steps
1.  Implement Levenshtein distance for scoring (re-add `sahilm/fuzzy` or similar).
2.  Implement the "Best Match" finder and diff generator for error messages.
3.  Implement `replace_all` strategy.
