package config

import (
	"fmt"
	"sync"
	"testing"
)

const (
	disableFlag = "--disable"
	reviewCode  = "review_code"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantDisabled []string
	}{
		{
			name: "default",
			args: []string{},
		},
		{
			name:         "disable single tool",
			args:         []string{disableFlag, reviewCode},
			wantDisabled: []string{reviewCode},
		},
		{
			name:         "disable multiple tools",
			args:         []string{disableFlag, reviewCode + ",write, edit_code"},
			wantDisabled: []string{reviewCode, "write", "edit_code"},
		},
		{
			name:         "disable empty",
			args:         []string{disableFlag, ""},
			wantDisabled: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(tt.args)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if len(tt.wantDisabled) != len(cfg.DisabledTools) {
				t.Errorf("Load().DisabledTools len = %v, want %v", len(cfg.DisabledTools), len(tt.wantDisabled))
			}
			for _, d := range tt.wantDisabled {
				if !cfg.DisabledTools[d] {
					t.Errorf("Load().DisabledTools[%q] not found", d)
				}
			}
		})
	}
}

func TestIsToolEnabled(t *testing.T) {
	cfg := &Config{
		AllowedTools: map[string]bool{
			"tool1": true,
			"tool2": true,
		},
		DisabledTools: map[string]bool{
			"tool2": true,
		},
	}

	if !cfg.IsToolEnabled("tool1") {
		t.Errorf("expected tool1 to be enabled")
	}
	if cfg.IsToolEnabled("tool2") {
		t.Errorf("expected tool2 to be disabled via DisabledTools")
	}
	if cfg.IsToolEnabled("tool3") {
		t.Errorf("expected tool3 to be disabled (not in whitelist)")
	}
}

func TestConfig_ConcurrentAccess(t *testing.T) {
	cfg := &Config{
		DisabledTools: make(map[string]bool),
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			toolName := fmt.Sprintf("tool_%d", id%10)
			_ = cfg.IsToolEnabled(toolName)
		}(i)
	}

	// Concurrent readers accessing DisabledTools
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			toolName := fmt.Sprintf("tool_%d", id%10)
			_ = cfg.IsToolEnabled(toolName)
		}(i)
	}

	wg.Wait()
}

func TestIsToolEnabled_DefaultBehavior(t *testing.T) {
	// Test when AllowedTools is empty (default mode)
	cfg := &Config{}
	if !cfg.IsToolEnabled("any_tool") {
		t.Errorf("expected any_tool to be enabled when AllowedTools is empty")
	}
}

func TestLoad_Flags(t *testing.T) {
	cfg, err := Load([]string{"-version", "-list-tools", "-listen", "127.0.0.1:8080", "-allow", "toolA,toolB"})
	if err != nil {
		t.Fatalf("Load() unexpected error = %v", err)
	}
	if !cfg.Version {
		t.Errorf("expected Version to be true")
	}
	if !cfg.ListTools {
		t.Errorf("expected ListTools to be true")
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr = %q, want 127.0.0.1:8080", cfg.ListenAddr)
	}
	if !cfg.AllowedTools["toolA"] || !cfg.AllowedTools["toolB"] {
		t.Errorf("AllowedTools missing toolA or toolB: %v", cfg.AllowedTools)
	}
}


