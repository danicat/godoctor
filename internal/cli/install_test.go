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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeInstall(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cmd := newInstallCmd(stdout, stderr)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd.ExecuteContext(ctx)
}

func TestRunInstall_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := executeInstall(context.Background(), []string{"--help"}, &stdout, &stderr)
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
	err := executeInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}

	// 1. Verify MCP config created and valid
	data, err := os.ReadFile(filepath.Clean(configFile))
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

	// 2. Verify godoctor skill and references unpacked
	expectedFiles := []string{
		"godoctor/SKILL.md",
		"godoctor/references/selene.md",
		"godoctor/references/testquery.md",
	}
	for _, relFile := range expectedFiles {
		filePath := filepath.Clean(filepath.Join(skillsDir, relFile))
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("expected file %s to exist: %v", filePath, err)
		}
		if len(fileData) == 0 {
			t.Errorf("file %s is empty", filePath)
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

func TestRunInstall_AllFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	var stdout, stderr bytes.Buffer
	args := []string{"--all", "--config", configFile, "--skills-dir", skillsDir}
	err := executeInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall --all failed: %v", err)
	}

	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("expected mcp config file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "godoctor", "SKILL.md")); err != nil {
		t.Fatalf("expected skill file to exist: %v", err)
	}
}

func TestRunInstall_MCPOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	var stdout, stderr bytes.Buffer
	args := []string{"--mcp", "--config", configFile, "--skills-dir", skillsDir}
	err := executeInstall(context.Background(), args, &stdout, &stderr)
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
	err := executeInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}

	// MCP config must NOT exist
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Fatalf("mcp config file should not be created for --skills only")
	}

	// Skills and references must exist
	expectedFiles := []string{
		"godoctor/SKILL.md",
		"godoctor/references/selene.md",
		"godoctor/references/testquery.md",
	}
	for _, relFile := range expectedFiles {
		filePath := filepath.Clean(filepath.Join(skillsDir, relFile))
		if _, err := os.ReadFile(filePath); err != nil {
			t.Fatalf("expected file %s: %v", filePath, err)
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
	err := executeInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall -w failed: %v", err)
	}

	expectedConfig := filepath.Clean(filepath.Join(tmpDir, ".agents", "mcp_config.json"))
	if _, err := os.Stat(expectedConfig); err != nil {
		t.Fatalf("expected workspace config %s to exist: %v", expectedConfig, err)
	}

	expectedSkill := filepath.Clean(filepath.Join(tmpDir, ".agents", "skills", "godoctor", "SKILL.md"))
	if _, err := os.Stat(expectedSkill); err != nil {
		t.Fatalf("expected workspace skill %s to exist: %v", expectedSkill, err)
	}
}

func TestRunInstall_CorruptedMCPConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Clean(filepath.Join(tmpDir, "mcp_config.json"))
	_ = os.WriteFile(configFile, []byte("{corrupted json"), 0600)

	var stdout, stderr bytes.Buffer
	args := []string{"--mcp", "--config", configFile}
	err := executeInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed on corrupted config: %v", err)
	}

	// Backup file must exist
	backupFile := filepath.Clean(configFile + ".bak")
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
	skillsDir := filepath.Clean(filepath.Join(tmpDir, "skills"))
	skillPath := filepath.Clean(filepath.Join(skillsDir, "godoctor", "SKILL.md"))
	_ = os.MkdirAll(filepath.Clean(filepath.Dir(skillPath)), 0750)
	_ = os.WriteFile(skillPath, []byte("custom content"), 0600)

	// Run without --force -> should preserve
	var stdout, stderr bytes.Buffer
	args := []string{"--skills", "--skills-dir", skillsDir}
	err := executeInstall(context.Background(), args, &stdout, &stderr)
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
	err = executeInstall(context.Background(), args, &stdout, &stderr)
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
	err := executeInstall(context.Background(), args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstall failed in quiet mode: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected quiet mode to produce no stdout, got %q", stdout.String())
	}
}
