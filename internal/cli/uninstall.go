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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// UninstallOptions holds parsed options for the uninstall command.
type UninstallOptions struct {
	UninstallAll    bool
	UninstallMCP    bool
	UninstallSkills bool
	Workspace       bool
	Global          bool
	Quiet           bool
	ConfigPath      string
	SkillsDir       string
}

// resolveUninstallPaths determines target scopes and paths for uninstallation.
func resolveUninstallPaths(opts UninstallOptions) (string, string, string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to determine user home directory: %w", err)
	}

	var targetRoot string
	scopeName := "global"

	if opts.Workspace {
		scopeName = "workspace"
		targetRoot = filepath.Clean(".")
	} else {
		geminiConfig := os.Getenv("GEMINI_CONFIG_DIR")
		if geminiConfig != "" {
			targetRoot = filepath.Clean(geminiConfig)
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

	return scopeName, targetRoot, mcpConfigFile, skillsTargetDir, nil
}

// ExecuteUninstall performs the uninstallation according to the given options.
func ExecuteUninstall(ctx context.Context, opts UninstallOptions, stdout, stderr io.Writer) error {
	_ = stderr
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	scopeName, targetRoot, mcpConfigFile, skillsTargetDir, err := resolveUninstallPaths(opts)
	if err != nil {
		return err
	}

	var mcpRemoved bool
	var removedSkills []string

	// 1. Remove MCP Server registration
	if opts.UninstallMCP {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		removed, err := removeMCPServer(mcpConfigFile)
		if err != nil {
			return fmt.Errorf("failed to remove MCP server from %s: %w", mcpConfigFile, err)
		}
		mcpRemoved = removed
	}

	// 2. Remove Skills
	if opts.UninstallSkills {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		removedSkills = removeSkills(skillsTargetDir)
	}

	// 3. Summary Output
	if !opts.Quiet {
		printUninstallSummary(stdout, scopeName, targetRoot, mcpConfigFile, skillsTargetDir, mcpRemoved, removedSkills)
	}

	return nil
}

// removeMCPServer deletes the "godoctor" entry from mcp_config.json.
func removeMCPServer(configPath string) (bool, error) {
	cleanPath := filepath.Clean(configPath)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read %s: %w", cleanPath, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("failed to parse JSON from %s: %w", configPath, err)
	}

	mcpServers, ok := raw["mcpServers"].(map[string]any)
	if !ok || mcpServers == nil {
		return false, nil
	}

	if _, exists := mcpServers["godoctor"]; !exists {
		return false, nil
	}

	delete(mcpServers, "godoctor")
	raw["mcpServers"] = mcpServers

	updatedData, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return false, fmt.Errorf("failed to encode JSON for %s: %w", configPath, err)
	}

	if err := os.WriteFile(configPath, updatedData, 0600); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return true, nil
}

// removeSkills deletes GoDoctor skill directories.
func removeSkills(targetDir string) []string {
	skillsList := []string{AppName, ToolNameSelene, ToolNameTestQuery}
	var removed []string

	for _, skillName := range skillsList {
		skillDir := filepath.Clean(filepath.Join(targetDir, skillName))
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
	_, _ = fmt.Fprintf(w, `=============================================================
             🗑️  GoDoctor Uninstallation Complete             
=============================================================
Scope:       %s (%s)

Actions Performed:
`, scope, targetRoot)

	if mcpRemoved {
		_, _ = fmt.Fprintf(w, "  ✓ MCP Server:  Removed 'godoctor' from %s\n", mcpPath)
	} else {
		_, _ = fmt.Fprintf(w, "  ℹ MCP Server:  Not found in %s\n", mcpPath)
	}

	if len(removedSkills) > 0 {
		_, _ = fmt.Fprintf(w, "  ✓ Skills:      Removed from %s\n", skillsDir)
		for _, s := range removedSkills {
			_, _ = fmt.Fprintf(w, "                 • %s\n", s)
		}
	} else {
		_, _ = fmt.Fprintf(w, "  ℹ Skills:      None found in %s\n", skillsDir)
	}

	_, _ = fmt.Fprintln(w, "")
}
