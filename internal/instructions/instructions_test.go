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

	// Verify accurate SQLite table name
	if !strings.Contains(instr, "all_coverage") {
		t.Errorf("instructions.Get() should reference all_coverage table, got:\n%s", instr)
	}
	if strings.Contains(instr, "FROM coverage WHERE") {
		t.Errorf("instructions.Get() should not contain non-existent 'FROM coverage', got:\n%s", instr)
	}

	// Verify accurate smart_edit diagnostics description
	if !strings.Contains(instr, "go vet") {
		t.Errorf("instructions.Get() missing go vet compiler diagnostics description for smart_edit")
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

func TestGetForTools(t *testing.T) {
	t.Run("Subset of tools", func(t *testing.T) {
		instr := instructions.GetForTools([]string{"smart_edit", "read_docs"})
		if !strings.Contains(instr, "smart_edit") {
			t.Errorf("expected smart_edit in instructions, got:\n%s", instr)
		}
		if !strings.Contains(instr, "read_docs") {
			t.Errorf("expected read_docs in instructions, got:\n%s", instr)
		}
		if strings.Contains(instr, "smart_build") {
			t.Errorf("unexpected smart_build in instructions, got:\n%s", instr)
		}
		if strings.Contains(instr, "smart_test") {
			t.Errorf("unexpected smart_test in instructions, got:\n%s", instr)
		}
		if strings.Contains(instr, "selene") {
			t.Errorf("unexpected selene in instructions, got:\n%s", instr)
		}
		if strings.Contains(instr, "test_query") {
			t.Errorf("unexpected test_query in instructions, got:\n%s", instr)
		}
		if strings.Contains(instr, "## Testing and analytics") {
			t.Errorf("unexpected Testing and analytics category in instructions, got:\n%s", instr)
		}
	})

	t.Run("Tool aliases", func(t *testing.T) {
		instr := instructions.GetForTools([]string{"edit", "docs", "tq"})
		if !strings.Contains(instr, "smart_edit") {
			t.Errorf("expected smart_edit via alias 'edit', got:\n%s", instr)
		}
		if !strings.Contains(instr, "read_docs") {
			t.Errorf("expected read_docs via alias 'docs', got:\n%s", instr)
		}
		if !strings.Contains(instr, "test_query") {
			t.Errorf("expected test_query via alias 'tq', got:\n%s", instr)
		}
		if strings.Contains(instr, "smart_test") {
			t.Errorf("unexpected smart_test in instructions, got:\n%s", instr)
		}
	})

	t.Run("Nil and empty tool list returns all", func(t *testing.T) {
		instrNil := instructions.GetForTools(nil)
		instrEmpty := instructions.GetForTools([]string{})
		instrAll := instructions.Get()

		if instrNil != instrAll {
			t.Errorf("GetForTools(nil) != Get()")
		}
		if instrEmpty != instrAll {
			t.Errorf("GetForTools([]) != Get()")
		}
	})
}
