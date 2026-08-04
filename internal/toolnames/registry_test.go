package toolnames_test

import (
	"testing"

	"github.com/danicat/godoctor/internal/toolnames"
)

func TestRegistryCompleteness(t *testing.T) {
	expectedTools := []string{
		"smart_edit",
		"smart_read",
		"list_files",
		"read_docs",
		"smart_build",
		"add_dependency",
		"project_init",
		"mutation_test",
		"test_query",
		"describe_symbol",
	}

	for _, name := range expectedTools {
		def, exists := toolnames.Registry[name]
		if !exists {
			t.Errorf("Registry missing tool definition for %q", name)
			continue
		}
		if def.Name != name {
			t.Errorf("ToolDef name mismatch: got %q, want %q", def.Name, name)
		}
		if def.Title == "" {
			t.Errorf("ToolDef %q missing Title", name)
		}
		if def.Description == "" {
			t.Errorf("ToolDef %q missing Description", name)
		}
		if def.Instruction == "" {
			t.Errorf("ToolDef %q missing Instruction", name)
		}
	}
}
