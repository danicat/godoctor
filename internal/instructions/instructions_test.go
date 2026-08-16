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
		"smart_edit",
		"smart_build",
		"read_docs",
		"test_query",
		"smart_test",
		"selene",
	}

	for _, tool := range expectedTools {
		if !strings.Contains(instr, tool) {
			t.Errorf("instructions.Get() missing %s instruction", tool)
		}
	}

	removedTools := []string{
		"describe_symbol",
		"project_init",
		"smart_read",
		"smart_multi_edit",
		"add_dependencies",
		"list_files",
	}

	for _, tool := range removedTools {
		if strings.Contains(instr, tool) {
			t.Errorf("instructions.Get() should not contain removed tool %s", tool)
		}
	}
}
