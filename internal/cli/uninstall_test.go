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

func TestRunUninstall_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runUninstall(context.Background(), []string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error for --help: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage in stdout, got %q", stdout.String())
	}
}

func TestRunUninstall_DefaultAll(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	// First initialize
	initArgs := []string{"--config", configFile, "--skills-dir", skillsDir}
	var stdout, stderr bytes.Buffer
	if err := runInstall(context.Background(), initArgs, &stdout, &stderr); err != nil {
		t.Fatalf("runInstall failed: %v", err)
	}

	// Verify both existed
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("config file was not created by init")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "godoctor")); err != nil {
		t.Fatalf("skill godoctor was not created by init")
	}

	// Now uninstall
	stdout.Reset()
	uninstallArgs := []string{"--config", configFile, "--skills-dir", skillsDir}
	if err := runUninstall(context.Background(), uninstallArgs, &stdout, &stderr); err != nil {
		t.Fatalf("runUninstall failed: %v", err)
	}

	// Verify MCP config removed godoctor
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	servers := root["mcpServers"].(map[string]any)
	if _, exists := servers["godoctor"]; exists {
		t.Errorf("godoctor was not removed from mcpServers")
	}

	// Verify skills deleted
	for _, skill := range []string{"godoctor", "selene", "testquery"} {
		skillDir := filepath.Join(skillsDir, skill)
		if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
			t.Errorf("skill dir %s was not deleted", skillDir)
		}
	}
}

func TestRunUninstall_MCPOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	// Initialize all
	var stdout, stderr bytes.Buffer
	_ = runInstall(context.Background(), []string{"--config", configFile, "--skills-dir", skillsDir}, &stdout, &stderr)

	// Uninstall MCP only
	stdout.Reset()
	mcpArgs := []string{"--mcp", "--config", configFile, "--skills-dir", skillsDir}
	err := runUninstall(context.Background(), mcpArgs, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runUninstall --mcp failed: %v", err)
	}

	// Skills must still exist
	if _, err := os.Stat(filepath.Join(skillsDir, "godoctor")); err != nil {
		t.Errorf("expected skill godoctor to remain after --mcp uninstall")
	}
}

func TestRunUninstall_SkillsOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "mcp_config.json")
	skillsDir := filepath.Join(tmpDir, "skills")

	// Initialize all
	var stdout, stderr bytes.Buffer
	_ = runInstall(context.Background(), []string{"--config", configFile, "--skills-dir", skillsDir}, &stdout, &stderr)

	// Uninstall Skills only
	stdout.Reset()
	skillsArgs := []string{"--skills", "--config", configFile, "--skills-dir", skillsDir}
	err := runUninstall(context.Background(), skillsArgs, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runUninstall --skills failed: %v", err)
	}

	// Skills must be deleted
	if _, err := os.Stat(filepath.Join(skillsDir, "godoctor")); !os.IsNotExist(err) {
		t.Errorf("expected skill godoctor to be deleted")
	}

	// MCP config must still contain godoctor
	data, _ := os.ReadFile(configFile)
	var root map[string]any
	_ = json.Unmarshal(data, &root)
	servers := root["mcpServers"].(map[string]any)
	if _, exists := servers["godoctor"]; !exists {
		t.Errorf("expected godoctor to remain in mcpServers")
	}
}
