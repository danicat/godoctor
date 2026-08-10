package instructions_test

import (
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/instructions"
)

func TestGet(t *testing.T) {
	instr := instructions.Get()
	if !strings.Contains(instr, "MULTI-ROOT WORKSPACE ENVIRONMENT") {
		t.Errorf("instructions.Get() missing header banner, got:\n%s", instr)
	}

	expectedTools := []string{
		"smart_read",
		"smart_edit",
		"smart_multi_edit",
		"smart_build",
		"read_docs",
		"test_query",
		"add_dependencies",
		"list_files",
		"mutation_test",
	}

	for _, tool := range expectedTools {
		if !strings.Contains(instr, tool) {
			t.Errorf("instructions.Get() missing %s instruction", tool)
		}
	}

	removedTools := []string{
		"describe_symbol",
		"project_init",
	}

	for _, tool := range removedTools {
		if strings.Contains(instr, tool) {
			t.Errorf("instructions.Get() should not contain removed tool %s", tool)
		}
	}
}
