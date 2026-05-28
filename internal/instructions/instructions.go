// Package instructions generates dynamic system instructions for the AI agent.
// It tailors the guidance provided to the LLM based on the currently enabled tools and configuration,
// ensuring the agent is aware of its capabilities and how to use them effectively.
package instructions

import (
	"strings"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/toolnames"
)

// Get returns the agent instructions for the server based on enabled tools.
func Get(cfg *config.Config) string {
	var sb strings.Builder

	// Helper to check if a tool is enabled. Logic is centralized in config.
	isEnabled := func(tool string) bool {
		return cfg.IsToolEnabled(tool)
	}

	// 1. Persona
	sb.WriteString("# Go Smart Tooling Guide\n\n")
	sb.WriteString("⚠️ **CRITICAL: MULTI-ROOT WORKSPACE ENVIRONMENT**\n")
	sb.WriteString("This environment has multiple project roots registered. To ensure " +
		"that your requests are performed on the correct target project (and do not " +
		"fallback to the GoDoctor project folder), **YOU MUST ALWAYS USE ABSOLUTE PATHS** " +
		"for all file, directory, or path parameters. Never pass relative paths " +
		"(e.g., '.', '', or relative paths like 'pkg/main.go'). Always pass the " +
		"absolute path of the target workspace root or files.\n\n")

	// 2. Navigation
	sb.WriteString("### 🔍 Navigation: Save Tokens & Context\n")
	if isEnabled("smart_read") {
		sb.WriteString(toolnames.Registry["smart_read"].Instruction + "\n")
	}
	if isEnabled("list_files") {
		sb.WriteString(toolnames.Registry["list_files"].Instruction + "\n")
	}
	if isEnabled("describe_symbol") {
		sb.WriteString(toolnames.Registry["describe_symbol"].Instruction + "\n")
	}
	sb.WriteString("\n")

	// 3. Editing
	sb.WriteString("### ✏️ Editing: Ensure Safety\n")
	if isEnabled("smart_edit") {
		sb.WriteString(toolnames.Registry["smart_edit"].Instruction + "\n")
	}
	sb.WriteString("\n")

	// 4. Utilities
	sb.WriteString("### 🛠️ Utilities\n")
	if isEnabled("smart_build") {
		sb.WriteString(toolnames.Registry["smart_build"].Instruction + "\n")
	}
	if isEnabled("read_docs") {
		sb.WriteString(toolnames.Registry["read_docs"].Instruction + "\n")
	}
	if isEnabled("add_dependency") {
		sb.WriteString(toolnames.Registry["add_dependency"].Instruction + "\n")
	}
	if isEnabled("project_init") {
		sb.WriteString(toolnames.Registry["project_init"].Instruction + "\n")
	}
	sb.WriteString("\n")

	// 5. Testing
	sb.WriteString("### 🧪 Testing\n")
	if isEnabled("mutation_test") {
		sb.WriteString(toolnames.Registry["mutation_test"].Instruction + "\n")
	}
	if isEnabled("test_query") {
		sb.WriteString(toolnames.Registry["test_query"].Instruction + "\n")
	}

	return sb.String()
}
