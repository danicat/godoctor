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

package safeshell_test

import (
	"context"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/safeshell"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid inputs
		{"Simple command", "go", false},
		{"Subcommand", "version", false},
		{"Flag", "-f", false},
		{"Format string without dollar/pipe", "{{.Dir}}", false},
		{"Import path", "github.com/google/uuid", false},
		{"Path with dots and slashes", "./foo/bar/baz.go", false},
		{"Alphanumeric with dashes and underscores", "my-arg_123", false},

		// Disallowed shell operators
		{"Pipe", "cat|sh", true},
		{"Backgrounding", "echo&", true},
		{"Semicolon", "echo;rm -rf /", true},
		{"Redirect input", "cat<file", true},
		{"Redirect output", "echo>file", true},
		{"Backticks", "`whoami`", true},
		{"Dollar variable", "$PATH", true},
		{"Subshell expansion", "$(id)", true},

		// Control characters (newline, carriage return, null byte)
		{"Newline", "echo\nhello", true},
		{"Carriage return", "echo\rhello", true},
		{"Null byte", "echo\x00hello", true},
		{"Trailing newline", "go\n", true},
		{"Trailing carriage return", "go\r", true},
		{"Trailing null byte", "go\x00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := safeshell.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCommandContext(t *testing.T) {
	ctx := context.Background()

	testValidCommandContext(ctx, t)
	testInvalidCommandContext(ctx, t)
}

func testValidCommandContext(ctx context.Context, t *testing.T) {
	t.Run("Valid command and args", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go", "version")
		if err != nil {
			t.Fatalf("CommandContext() unexpected error: %v", err)
		}
		if cmd == nil {
			t.Fatal("CommandContext() returned nil cmd")
		}
		if len(cmd.Args) != 2 || cmd.Args[0] != "go" || cmd.Args[1] != "version" {
			t.Errorf("CommandContext() args = %v, want [go version]", cmd.Args)
		}
	})
}

func testInvalidCommandContext(ctx context.Context, t *testing.T) {
	t.Run("Invalid command name - shell operator", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go;sh", "version")
		if err == nil {
			t.Error("CommandContext() expected error for invalid command name, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid command name") {
			t.Errorf("CommandContext() error = %v, want containing 'invalid command name'", err)
		}
	})

	t.Run("Invalid command name - control character", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go\n", "version")
		if err == nil {
			t.Error("CommandContext() expected error for newline in command name, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
	})

	t.Run("Invalid argument - shell operator", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go", "list;rm -rf /")
		if err == nil {
			t.Error("CommandContext() expected error for invalid argument, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid argument") {
			t.Errorf("CommandContext() error = %v, want containing 'invalid argument'", err)
		}
	})

	t.Run("Invalid argument - newline", func(t *testing.T) {
		_, err := safeshell.CommandContext(ctx, "go", "list\n")
		if err == nil {
			t.Error("CommandContext() expected error for newline in argument, got nil")
		}
	})

	t.Run("Invalid argument - carriage return", func(t *testing.T) {
		_, err := safeshell.CommandContext(ctx, "go", "list\r")
		if err == nil {
			t.Error("CommandContext() expected error for carriage return in argument, got nil")
		}
	})

	t.Run("Invalid argument - null byte", func(t *testing.T) {
		_, err := safeshell.CommandContext(ctx, "go", "list\x00")
		if err == nil {
			t.Error("CommandContext() expected error for null byte in argument, got nil")
		}
	})
}
