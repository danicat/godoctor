package instructions_test

import (
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/instructions"
)

func TestGet(t *testing.T) {
	cfg, err := config.Load([]string{})
	if err != nil {
		t.Fatalf("config.Load() unexpected error = %v", err)
	}

	instr := instructions.Get(cfg)
	if !strings.Contains(instr, "MULTI-ROOT WORKSPACE ENVIRONMENT") {
		t.Errorf("instructions.Get() missing header banner, got:\n%s", instr)
	}
	if !strings.Contains(instr, "smart_edit") {
		t.Errorf("instructions.Get() missing smart_edit instruction")
	}

	// Test with disabled tools
	disabledCfg, _ := config.Load([]string{"--disable", "smart_edit"})
	disabledInstr := instructions.Get(disabledCfg)
	if strings.Contains(disabledInstr, "smart_edit") {
		t.Errorf("instructions.Get() should not contain disabled tool smart_edit")
	}
}
