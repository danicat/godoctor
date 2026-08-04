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
// It checks the command name and arguments to block dynamic shell injection and execution features.
func CommandContext(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	if err := Validate(name); err != nil {
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

// Validate checks a string argument or command name for injection indicators (e.g. pipes, redirects, operators, control characters).
func Validate(val string) error {
	// Disallowed shell operator and control characters:
	// | (pipe), & (backgrounding/chaining), ; (command separator), < > (redirection), ` $ (expansion/subshell), \n, \r, \x00
	disallowed := []string{"|", "&", ";", "<", ">", "`", "$", "\n", "\r", "\x00"}
	for _, char := range disallowed {
		if strings.Contains(val, char) {
			return fmt.Errorf("value contains disallowed shell operator character %q", char)
		}
	}
	return nil
}
