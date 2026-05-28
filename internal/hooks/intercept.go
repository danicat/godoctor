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
		deny("Failed to read hook payload: "+err.Error(), "🛑 Error")
		return
	}

	var payload HookPayload
	if err := json.Unmarshal(input, &payload); err != nil {
		deny("Failed to parse hook payload: "+err.Error(), "🛑 Parse Error")
		return
	}

	switch payload.ToolName {
	case "replace":
		deny("Optimization Hook: The native `replace` tool is blocked. You MUST use GoDoctor's `smart_edit` tool for safe, fuzzy-matched, syntax-verified file modifications.", "🛑 Blocked raw replace")
		return
	case "read_file":
		deny("Optimization Hook: Raw reads are blocked. You MUST use GoDoctor's `smart_read` to inspect Go code. It provides structural outlines and context-aware scoping.", "🛑 Blocked raw read")
		return
	case "write_file":
		deny("Optimization Hook: Raw file creation is blocked. You MUST use GoDoctor's `smart_edit` tool which handles atomic file creation natively.", "🛑 Blocked raw write")
		return
	case "run_shell_command":
		handleShellCommand(payload.ToolInput)
		return
	}

	allow()
}

func handleShellCommand(input map[string]interface{}) {
	cmdInterface, ok := input["command"]
	if !ok {
		allow()
		return
	}

	cmdStr, ok := cmdInterface.(string)
	if !ok {
		allow()
		return
	}

	cmdStr = strings.TrimSpace(cmdStr)

	// 1. Build/Test Commands
	buildPatterns := []string{"go build", "go test", "go vet", "golangci-lint"}
	for _, p := range buildPatterns {
		if strings.Contains(cmdStr, p) {
			deny("Quality Gate Hook: Manual toolchains are blocked. You MUST use GoDoctor's `smart_build` tool to execute the quality gate pipeline (tidy -> modernize -> format -> test -> lint).", "🛑 Blocked manual build/test")
			return
		}
	}

	// 2. Dependency Commands
	if strings.HasPrefix(cmdStr, "go get") || strings.Contains(cmdStr, " go get ") {
		deny("Optimization Hook: Use `add_dependency` to install packages. It fetches the documentation automatically, saving you a context-gathering step.", "🛑 Blocked go get")
		return
	}

	// 3. File Writers
	writePatterns := []string{"sed -i", "echo ", "tee "}
	for _, p := range writePatterns {
		// Basic check for echo redirect
		if p == "echo " && strings.Contains(cmdStr, "echo ") && strings.Contains(cmdStr, ">") && !strings.Contains(cmdStr, "> /dev/null") && !strings.Contains(cmdStr, ">/dev/null") {
			deny("Optimization Hook: Shell file modifications are blocked. Use `smart_edit` to modify files safely.", "🛑 Blocked raw file write")
			return
		}
		if p != "echo " && strings.Contains(cmdStr, p) {
			deny("Optimization Hook: Shell file modifications are blocked. Use `smart_edit` to modify files safely.", "🛑 Blocked raw file edit")
			return
		}
	}

	// 4. File Readers
	if strings.HasPrefix(cmdStr, "cat ") && strings.HasSuffix(cmdStr, ".go") || strings.Contains(cmdStr, " cat ") && strings.HasSuffix(cmdStr, ".go") {
		deny("Optimization Hook: Raw shell reads are blocked. You MUST use GoDoctor's `smart_read` to inspect Go code.", "🛑 Blocked shell cat")
		return
	}
	if strings.Contains(cmdStr, "grep") && strings.HasSuffix(cmdStr, ".go") && !strings.Contains(cmdStr, "|") {
		deny("Optimization Hook: Raw shell reads are blocked. You MUST use GoDoctor's `smart_read` to inspect Go code.", "🛑 Blocked shell grep")
		return
	}

	allow()
}

func allow() {
	resp := HookResponse{Decision: "allow"}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
	os.Exit(0)
}

func deny(reason, systemMessage string) {
	resp := HookResponse{
		Decision:      "deny",
		Reason:        reason,
		SystemMessage: systemMessage,
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
	os.Exit(0)
}
