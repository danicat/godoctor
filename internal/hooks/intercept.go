package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// HookPayload represents the JSON payload sent by Gemini CLI to the hook via stdin.
type HookPayload struct {
	ToolName  string                 `json:"tool_name"`
	ToolInput map[string]interface{} `json:"tool_input"`
}

// HookResponse represents the decision returned to Gemini CLI via stdout.
type HookResponse struct {
	Decision      string `json:"decision"`
	Reason        string `json:"reason,omitempty"`
	SystemMessage string `json:"systemMessage,omitempty"`
}

// Intercept reads the tool payload from standard input, evaluates it against the rules,
// and outputs the decision to standard output.
func Intercept() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeResponse(HookResponse{Decision: "deny", Reason: "Failed to read hook payload: " + err.Error(), SystemMessage: "🛑 Error"})
		os.Exit(0)
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		writeResponse(HookResponse{Decision: "deny", Reason: "Failed to parse hook payload: " + err.Error(), SystemMessage: "🛑 Parse Error"})
		os.Exit(0)
		return
	}

	resp := evaluate(payload)
	writeResponse(resp)
	os.Exit(0)
}

// evaluate applies the interception rules to a payload and returns the decision.
// It is pure (no I/O, no os.Exit) so that it can be unit-tested directly.
func evaluate(payload HookPayload) HookResponse {
	switch payload.ToolName {
	case "replace":
		if !isGoFile(payload.ToolInput) {
			return allow()
		}
		return deny(
			"Optimization Hook: The native `replace` tool is blocked for Go files. You MUST use GoDoctor's `smart_edit` tool for safe, fuzzy-matched, syntax-verified file modifications.",
			"🛑 Blocked raw replace",
		)
	case "read_file":
		if !isGoFile(payload.ToolInput) {
			return allow()
		}
		return deny(
			"Optimization Hook: Raw reads are blocked for Go files. You MUST use GoDoctor's `smart_read` to inspect Go code. It provides structural outlines and context-aware scoping.",
			"🛑 Blocked raw read",
		)
	case "write_file":
		if !isGoFile(payload.ToolInput) {
			return allow()
		}
		return deny(
			"Optimization Hook: Raw file creation is blocked for Go files. You MUST use GoDoctor's `smart_edit` tool which handles atomic file creation natively.",
			"🛑 Blocked raw write",
		)
	case "run_shell_command":
		return evaluateShellCommand(payload.ToolInput)
	}

	return allow()
}

// isGoFile reports whether the tool input's path targets a Go source file.
func isGoFile(input map[string]interface{}) bool {
	for _, key := range []string{"path", "file_path", "filename"} {
		if val, ok := input[key]; ok {
			if s, ok := val.(string); ok {
				return strings.HasSuffix(s, ".go")
			}
		}
	}
	return false
}

func evaluateShellCommand(input map[string]interface{}) HookResponse {
	cmdInterface, ok := input["command"]
	if !ok {
		return allow()
	}

	cmdStr, ok := cmdInterface.(string)
	if !ok {
		return allow()
	}

	cmdStr = strings.TrimSpace(cmdStr)

	// 1. Build/Test Commands — always blocked: these are Go-specific by definition.
	buildPatterns := []string{"go build", "go test", "go vet", "golangci-lint"}
	for _, p := range buildPatterns {
		if strings.Contains(cmdStr, p) {
			return deny(
				"Quality Gate Hook: Manual toolchains are blocked. You MUST use GoDoctor's `smart_build` tool to execute the quality gate pipeline (tidy -> modernize -> format -> test -> lint).",
				"🛑 Blocked manual build/test",
			)
		}
	}

	// 2. Dependency Commands — always blocked: go get is Go-specific.
	if strings.HasPrefix(cmdStr, "go get") || strings.Contains(cmdStr, " go get ") {
		return deny(
			"Optimization Hook: Use `add_dependency` to install packages. It fetches the documentation automatically, saving you a context-gathering step.",
			"🛑 Blocked go get",
		)
	}

	// 3. File Writers — only block when the command targets a .go file.
	if strings.Contains(cmdStr, ".go") {
		writePatterns := []string{"sed -i", "tee "}
		for _, p := range writePatterns {
			if strings.Contains(cmdStr, p) {
				return deny(
					"Optimization Hook: Shell file modifications are blocked for Go files. Use `smart_edit` to modify Go files safely.",
					"🛑 Blocked raw file edit",
				)
			}
		}
		// echo redirecting into a .go file
		if strings.Contains(cmdStr, "echo ") && strings.Contains(cmdStr, ">") &&
			!strings.Contains(cmdStr, "> /dev/null") && !strings.Contains(cmdStr, ">/dev/null") {
			return deny(
				"Optimization Hook: Shell file modifications are blocked for Go files. Use `smart_edit` to modify Go files safely.",
				"🛑 Blocked raw file write",
			)
		}
	}

	// 4. File Readers — only block when the command targets a .go file.
	if strings.HasPrefix(cmdStr, "cat ") && strings.Contains(cmdStr, ".go") ||
		strings.Contains(cmdStr, " cat ") && strings.Contains(cmdStr, ".go") {
		return deny(
			"Optimization Hook: Raw shell reads are blocked for Go files. You MUST use GoDoctor's `smart_read` to inspect Go code.",
			"🛑 Blocked shell cat",
		)
	}
	if strings.Contains(cmdStr, "grep") && strings.Contains(cmdStr, ".go") && !strings.Contains(cmdStr, "|") {
		return deny(
			"Optimization Hook: Raw shell reads are blocked for Go files. You MUST use GoDoctor's `smart_read` to inspect Go code.",
			"🛑 Blocked shell grep",
		)
	}

	return allow()
}

func allow() HookResponse {
	return HookResponse{Decision: "allow"}
}

func deny(reason, systemMessage string) HookResponse {
	return HookResponse{
		Decision:      "deny",
		Reason:        reason,
		SystemMessage: systemMessage,
	}
}

func writeResponse(resp HookResponse) {
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}

// handleShellCommand is kept for backward compatibility with direct callers.
func handleShellCommand(input map[string]interface{}) {
	resp := evaluateShellCommand(input)
	writeResponse(resp)
	os.Exit(0)
}
