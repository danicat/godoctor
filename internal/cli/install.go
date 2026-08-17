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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/danicat/godoctor/skills"
)

// InstallOptions holds parsed options for the install command.
type InstallOptions struct {
	InstallAll    bool
	InstallMCP    bool
	InstallSkills bool
	Workspace     bool
	Global        bool
	Force         bool
	Quiet         bool
	ConfigPath    string
	SkillsDir     string
}

// resolveInstallPaths resolves target paths for installation.
func resolveInstallPaths(opts InstallOptions) (string, string, string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to determine user home directory: %w", err)
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

	return scopeName, targetRoot, mcpConfigFile, skillsTargetDir, nil
}

// ExecuteInstall performs the installation according to the given options.
func ExecuteInstall(ctx context.Context, opts InstallOptions, stdout, stderr io.Writer) error {
	_ = stderr
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	scopeName, targetRoot, mcpConfigFile, skillsTargetDir, err := resolveInstallPaths(opts)
	if err != nil {
		return err
	}

	binCommand := resolveBinaryCommand()
	var mcpConfigUpdated bool
	var installedSkills []string

	if opts.InstallMCP {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if err := configureMCPServer(mcpConfigFile, binCommand); err != nil {
			return fmt.Errorf("failed to configure MCP server in %s: %w", mcpConfigFile, err)
		}
		mcpConfigUpdated = true
	}

	if opts.InstallSkills {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		installed, err := unpackSkills(skillsTargetDir, opts.Force)
		if err != nil {
			return fmt.Errorf("failed to unpack skills to %s: %w", skillsTargetDir, err)
		}
		installedSkills = installed
	}

	if !opts.Workspace && opts.ConfigPath == "" {
		cleanupLegacyArtifacts(targetRoot)
	}

	if !opts.Quiet {
		printInstallSummary(
			stdout, scopeName, targetRoot, mcpConfigFile,
			skillsTargetDir, binCommand, mcpConfigUpdated, installedSkills,
		)
	}

	return nil
}

// resolveBinaryCommand determines whether to use "godoctor" (if in PATH) or absolute path.
func resolveBinaryCommand() string {
	execPath, err := os.Executable()
	if err != nil {
		return AppName
	}
	realPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realPath
	}

	// Check if godoctor is available in PATH and resolves to this binary
	if lookPath, err := exec.LookPath(AppName); err == nil {
		if realLookPath, err := filepath.EvalSymlinks(lookPath); err == nil {
			if realLookPath == execPath {
				return AppName
			}
		}
	}

	return execPath
}

// configureMCPServer updates or creates mcp_config.json safely.
func configureMCPServer(configPath, binCommand string) error {
	cleanConfigPath := filepath.Clean(configPath)
	configDir := filepath.Clean(filepath.Dir(cleanConfigPath))
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", configDir, err)
	}

	var root map[string]any
	data, err := os.ReadFile(cleanConfigPath)
	if err == nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			// Backup corrupted config
			backupConfigPath := filepath.Clean(configPath + ".bak")
			_ = os.WriteFile(backupConfigPath, data, 0600)
			root = make(map[string]any)
		}
	} else {
		root = make(map[string]any)
	}

	var servers map[string]any
	if existingServers, ok := root["mcpServers"].(map[string]any); ok {
		servers = existingServers
	} else {
		servers = make(map[string]any)
		root["mcpServers"] = servers
	}

	servers[AppName] = map[string]any{
		"command": binCommand,
		"args":    []string{CommandMCP},
	}

	formatted, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize mcp configuration: %w", err)
	}
	formatted = append(formatted, '\n')

	return os.WriteFile(cleanConfigPath, formatted, 0600)
}

// unpackSkills writes embedded skills from skills.FS into targetDir.
func unpackSkills(targetDir string, force bool) ([]string, error) {
	skillName := AppName
	destDir := filepath.Clean(filepath.Join(targetDir, skillName))

	var allUpToDate = true
	var hadModified = false
	var wroteFiles = false

	err := fs.WalkDir(skills.FS, skillName, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(skillName, path)
		if err != nil {
			return err
		}
		destPath := filepath.Clean(filepath.Join(destDir, relPath))
		if d.IsDir() {
			return os.MkdirAll(destPath, 0750)
		}
		content, err := fs.ReadFile(skills.FS, path)
		if err != nil {
			return err
		}
		if existing, err := os.ReadFile(destPath); err == nil {
			if bytes.Equal(existing, content) {
				return nil
			}
			if !force {
				allUpToDate = false
				hadModified = true
				return nil
			}
		}
		allUpToDate = false
		wroteFiles = true
		return os.WriteFile(destPath, content, 0600)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unpack skill %s: %w", skillName, err)
	}

	var installed []string
	if allUpToDate {
		installed = append(installed, skillName+" (up-to-date)")
	} else if hadModified && !wroteFiles {
		installed = append(installed, skillName+" (modified, skipped)")
	} else {
		installed = append(installed, skillName)
	}

	return installed, nil
}

// cleanupLegacyArtifacts removes obsolete plugins or agent definitions from target root.
func cleanupLegacyArtifacts(targetRoot string) {
	legacyPlugin := filepath.Join(targetRoot, "plugins", "godoctor")
	_ = os.RemoveAll(legacyPlugin)

	legacyAgent := filepath.Join(targetRoot, "agents", "godoctor.md")
	_ = os.Remove(legacyAgent)
}

// printInstallSummary displays the human-readable installation status.
func printInstallSummary(
	w io.Writer,
	scope, targetRoot, mcpPath, skillsDir, binCommand string,
	mcpUpdated bool,
	installedSkills []string,
) {
	_, _ = fmt.Fprintf(w, `=============================================================
             🩺 GoDoctor Installation Complete               
=============================================================
Scope:       %s (%s)
Binary:      %s

Surfaces Initialized:
`, scope, targetRoot, binCommand)

	if mcpUpdated {
		_, _ = fmt.Fprintf(w, `  ✓ MCP Server:  Registered in %s
                 Tools: smart_build, smart_test, smart_edit,
                        read_docs, selene, test_query
`, mcpPath)
	}

	if len(installedSkills) > 0 {
		_, _ = fmt.Fprintf(w, "  ✓ Skills:      Installed in %s\n", skillsDir)
		for _, s := range installedSkills {
			_, _ = fmt.Fprintf(w, "                 • @%s\n", s)
		}
	}

	_, _ = fmt.Fprintf(w, `  ✓ CLI Mode:    Ready for direct execution
                 Usage: godoctor call {edit|build|test|docs|selene|tq}

Quick Verification:
  • Test CLI: godoctor call test '{"level": "fast"}'
  • Test MCP: godoctor mcp
`)
}
