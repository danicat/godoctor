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
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// UninstallOptions holds parsed options for the uninstall command.
type UninstallOptions struct {
	UninstallMCP    bool
	UninstallSkills bool
	Workspace       bool
	Global          bool
	Quiet           bool
	ConfigPath      string
	SkillsDir       string
}

// runUninstall parses arguments and executes the godoctor uninstall command using Cobra.
func runUninstall(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cmd := newUninstallCmd(stdout, stderr)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd.ExecuteContext(ctx)
}

// ExecuteUninstall performs the uninstallation according to the given options.
func ExecuteUninstall(opts UninstallOptions, stdout, stderr io.Writer) error {
	_ = stderr
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine user home directory: %w", err)
	}

	var targetRoot string
	scopeName := "global"

	if opts.Workspace {
		scopeName = "workspace"
		targetRoot = filepath.Join(".", ".agents")
	} else {
		geminiConfig := os.Getenv("GEMINI_CONFIG_DIR")
		if geminiConfig != "" {
			targetRoot = geminiConfig
		} else {
			targetRoot = filepath.Join(homeDir, ".gemini", "config")
		}
	}

	mcpConfigFile := opts.ConfigPath
	if mcpConfigFile == "" {
		mcpConfigFile = filepath.Join(targetRoot, "mcp_config.json")
	}

	skillsTargetDir := opts.SkillsDir
	if skillsTargetDir == "" {
		skillsTargetDir = filepath.Join(targetRoot, "skills")
	}

	var mcpRemoved bool
	var removedSkills []string

	// 1. Remove MCP Server registration
	if opts.UninstallMCP {
		removed, err := removeMCPServer(mcpConfigFile)
		if err != nil {
			return fmt.Errorf("failed to remove MCP server from %s: %w", mcpConfigFile, err)
		}
		mcpRemoved = removed
	}

	// 2. Remove Skills
	if opts.UninstallSkills {
		removed := removeSkills(skillsTargetDir)
		removedSkills = removed
	}

	// 3. Summary Output
	if !opts.Quiet {
		printUninstallSummary(stdout, scopeName, targetRoot, mcpConfigFile, skillsTargetDir, mcpRemoved, removedSkills)
	}

	return nil
}

// removeMCPServer deletes the "godoctor" entry from mcp_config.json.
func removeMCPServer(configPath string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return false, nil
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		return false, nil
	}

	if _, exists := servers["godoctor"]; !exists {
		return false, nil
	}

	delete(servers, "godoctor")

	formatted, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("failed to serialize updated mcp configuration: %w", err)
	}
	formatted = append(formatted, '\n')

	if err := os.WriteFile(configPath, formatted, 0644); err != nil {
		return false, err
	}

	return true, nil
}

// removeSkills deletes GoDoctor skill directories.
func removeSkills(targetDir string) []string {
	skillsList := []string{"godoctor", "selene", "testquery"}
	var removed []string

	for _, skillName := range skillsList {
		skillDir := filepath.Join(targetDir, skillName)
		if _, err := os.Stat(skillDir); err == nil {
			_ = os.RemoveAll(skillDir)
			removed = append(removed, "@"+skillName)
		}
	}

	return removed
}

// printUninstallSummary displays the human-readable uninstallation status.
func printUninstallSummary(
	w io.Writer,
	scope, targetRoot, mcpPath, skillsDir string,
	mcpRemoved bool,
	removedSkills []string,
) {
	fmt.Fprintf(w, `=============================================================
             🗑️  GoDoctor Uninstallation Complete             
=============================================================
Scope:       %s (%s)

Actions Performed:
`, scope, targetRoot)

	if mcpRemoved {
		fmt.Fprintf(w, "  ✓ MCP Server:  Removed 'godoctor' from %s\n", mcpPath)
	} else {
		fmt.Fprintf(w, "  ℹ MCP Server:  Not found in %s\n", mcpPath)
	}

	if len(removedSkills) > 0 {
		fmt.Fprintf(w, "  ✓ Skills:      Removed from %s\n", skillsDir)
		for _, s := range removedSkills {
			fmt.Fprintf(w, "                 • %s\n", s)
		}
	} else {
		fmt.Fprintf(w, "  ℹ Skills:      None found in %s\n", skillsDir)
	}

	fmt.Fprintln(w, "")
}
