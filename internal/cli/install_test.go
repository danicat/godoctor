// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInstall_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runInstall(context.Background(), []string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error for --help: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage in stdout, got %q", stdout.String())
	}
}

func TestRunInstall_DefaultAll(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	var stdout, stderr bytes.Buffer
	args := []string{"--config", configFile, "--skills-dir", skillsDir}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}

	// 1. Verify MCP config created and valid
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read mcp_config.json: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid json in mcp_config.json: %v", err)
	}

	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers map in config")
	}

	godoctorEntry, ok := servers["godoctor"].(map[string]any)
	if !ok {
		t.Fatalf("missing godoctor server entry")
	}

	if godoctorEntry["command"] == "" {
		t.Errorf("command is empty")
	}

	argsList, ok := godoctorEntry["args"].([]any)
	if !ok || len(argsList) != 1 || argsList[0] != "mcp" {
		t.Errorf("expected args ['mcp'], got %v", godoctorEntry["args"])
	}

	// 2. Verify all 3 skills unpacked
	for _, skill := range []string{"godoctor", "selene", "testquery"} {
		skillPath := filepath.Join(skillsDir, skill, "SKILL.md")
		skillData, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("expected skill file %s to exist: %v", skillPath, err)
		}
		if len(skillData) == 0 {
			t.Errorf("skill file %s is empty", skillPath)
		}
	}

	// 3. Verify summary output
	outStr := stdout.String()
	if !strings.Contains(outStr, "GoDoctor Installation Complete") {
		t.Errorf("expected summary banner in output, got %q", outStr)
	}
	if !strings.Contains(outStr, "MCP Server:  Registered") {
		t.Errorf("expected MCP registration in output, got %q", outStr)
	}
}

func TestRunInstall_MCPOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	var stdout, stderr bytes.Buffer
	args := []string{"--mcp", "--config", configFile, "--skills-dir", skillsDir}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}

	// MCP config must exist
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("mcp config file not created: %v", err)
	}

	// Skills dir must NOT exist
	if _, err := os.Stat(skillsDir); !os.IsNotExist(err) {
		t.Fatalf("skills directory should not be created for --mcp only")
	}
}

func TestRunInstall_SkillsOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	var stdout, stderr bytes.Buffer
	args := []string{"--skills", "--config", configFile, "--skills-dir", skillsDir}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}

	// MCP config must NOT exist
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Fatalf("mcp config file should not be created for --skills only")
	}

	// Skills must exist
	for _, skill := range []string{"godoctor", "selene", "testquery"} {
		skillPath := filepath.Join(skillsDir, skill, "SKILL.md")
		if _, err := os.ReadFile(skillPath); err != nil {
			t.Fatalf("expected skill %s: %v", skillPath, err)
		}
	}
}

func TestRunInstall_WorkspaceScope(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	var stdout, stderr bytes.Buffer
	args := []string{"-w"}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall -w failed: %v", err)
	}

	expectedConfig := filepath.Join(tmpDir, ".agents", "mcp_config.json")
	if _, err := os.Stat(expectedConfig); err != nil {
		t.Fatalf("expected workspace config %s to exist: %v", expectedConfig, err)
	}

	expectedSkill := filepath.Join(tmpDir, ".agents", "skills", "godoctor", "SKILL.md")
	if _, err := os.Stat(expectedSkill); err != nil {
		t.Fatalf("expected workspace skill %s to exist: %v", expectedSkill, err)
	}
}

func TestRunInstall_CorruptedMCPConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	_ = os.WriteFile(configFile, []byte("{corrupted json"), 0644)

	var stdout, stderr bytes.Buffer
	args := []string{"--mcp", "--config", configFile}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed on corrupted config: %v", err)
	}

	// Backup file must exist
	backupFile := configFile + ".bak"
	if _, err := os.Stat(backupFile); err != nil {
		t.Fatalf("expected backup file %s: %v", backupFile, err)
	}

	// New config must be valid
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("recreated config is not valid JSON: %v", err)
	}
}

func TestRunInstall_ForceOverwriteSkills(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	skillPath := filepath.Join(skillsDir, "godoctor", "SKILL.md")
	_ = os.MkdirAll(filepath.Dir(skillPath), 0755)
	_ = os.WriteFile(skillPath, []byte("custom content"), 0644)

	// Run without --force -> should preserve
	var stdout, stderr bytes.Buffer
	args := []string{"--skills", "--skills-dir", skillsDir}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}
	content, _ := os.ReadFile(skillPath)
	if string(content) != "custom content" {
		t.Errorf("expected skill content to be preserved without force")
	}

	// Run with --force -> should overwrite
	stdout.Reset()
	args = []string{"--skills", "--skills-dir", skillsDir, "--force"}
	err = runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed with force: %v", err)
	}
	content, _ = os.ReadFile(skillPath)
	if string(content) == "custom content" {
		t.Errorf("expected skill content to be overwritten with force")
	}
}

func TestRunInstall_QuietMode(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	var stdout, stderr bytes.Buffer
	args := []string{"--quiet", "--config", configFile, "--skills-dir", skillsDir}
	err := runInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed in quiet mode: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected quiet mode to produce no stdout, got %q", stdout.String())
	}
}
