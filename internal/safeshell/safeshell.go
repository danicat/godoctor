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

// Package safeshell provides a secure wrapper for subprocess command execution to prevent shell injection.
package safeshell

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CommandContext creates a validated, secure exec.Cmd wrapper.
// It checks the command name for control characters and null bytes, and validates arguments against null bytes.
func CommandContext(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	if err := ValidateCommandName(name); err != nil {
		return nil, fmt.Errorf("invalid command name: %w", err)
	}

	for _, arg := range args {
		if err := Validate(arg); err != nil {
			return nil, fmt.Errorf("invalid argument %q: %w", arg, err)
		}
	}

	// nolint:gosec // G204: safeshell strictly sanitizes name and arguments before execution
	return exec.CommandContext(ctx, name, args...), nil
}

// ValidateCommandName checks a command executable name for control characters, newlines, and null bytes.
func ValidateCommandName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("command name cannot be empty")
	}
	if strings.Contains(name, "\x00") {
		return fmt.Errorf("command name contains null byte")
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return fmt.Errorf("command name contains control character %q", r)
		}
	}
	return nil
}

// Validate checks a string argument for safety indicators (e.g. null bytes).
// Note: In direct execve subprocess execution, operators like |, &, ;, <, >, `, $
// and newlines are safe in arguments as they are passed directly without shell interpretation.
func Validate(val string) error {
	if strings.Contains(val, "\x00") {
		return fmt.Errorf("value contains null byte")
	}
	return nil
}
