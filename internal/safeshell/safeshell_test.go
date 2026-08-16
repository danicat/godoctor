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
		// Valid inputs (direct execve passes arguments directly without shell evaluation)
		{"Simple command", "go", false},
		{"Subcommand", "version", false},
		{"Flag", "-f", false},
		{"Format string without dollar/pipe", "{{.Dir}}", false},
		{"Import path", "github.com/google/uuid", false},
		{"Path with dots and slashes", "./foo/bar/baz.go", false},
		{"Alphanumeric with dashes and underscores", "my-arg_123", false},

		// Regex filters with dollar signs and anchors
		{"Regex test filter", "-run=^TestAuth$", false},
		{"Benchmark regex filter", "-bench=^BenchmarkHash$", false},

		// SQL queries with operators (<, >, ;, newlines, carriage returns)
		{"SQL query with comparisons", "SELECT * FROM all_coverage WHERE count < 5 AND count > 0\nORDER BY count", false},
		{"SQL query with semicolon", "SELECT * FROM all_tests WHERE action = 'fail';", false},

		// Arguments with pipes, redirects, dollar signs, backticks
		{"Pipe in argument", "cat|sh", false},
		{"Backgrounding in argument", "echo&", false},
		{"Semicolon in argument", "echo;rm -rf /", false},
		{"Redirect input in argument", "cat<file", false},
		{"Redirect output in argument", "echo>file", false},
		{"Backticks in argument", "`whoami`", false},
		{"Dollar variable in argument", "$PATH", false},
		{"Subshell expansion in argument", "$(id)", false},
		{"Newline in argument", "echo\nhello", false},
		{"Carriage return in argument", "echo\rhello", false},

		// Disallowed null bytes
		{"Null byte", "echo\x00hello", true},
		{"Trailing null byte", "go\x00", true},
		{"Leading null byte", "\x00go", true},
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

func TestValidateCommandName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid command", "go", false},
		{"Valid binary with path", "/usr/local/bin/go", false},
		{"Valid binary with dashes", "golangci-lint", false},
		{"Empty command name", "", true},
		{"Whitespace only command name", "   ", true},
		{"Command name with newline", "go\n", true},
		{"Command name with carriage return", "go\r", true},
		{"Command name with null byte", "go\x00", true},
		{"Command name with tab", "go\t", true},
		{"Command name with escape", "go\x1b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := safeshell.ValidateCommandName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommandName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
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

	t.Run("Valid command with regex and operators", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go", "test", "-run=^TestAuth$", "SELECT * FROM all_coverage WHERE count < 5")
		if err != nil {
			t.Fatalf("CommandContext() unexpected error: %v", err)
		}
		if cmd == nil {
			t.Fatal("CommandContext() returned nil cmd")
		}
		if len(cmd.Args) != 4 {
			t.Errorf("CommandContext() args length = %d, want 4", len(cmd.Args))
		}
	})
}

func testInvalidCommandContext(ctx context.Context, t *testing.T) {
	t.Run("Invalid command name - empty", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "", "version")
		if err == nil {
			t.Error("CommandContext() expected error for empty command name, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid command name") {
			t.Errorf("CommandContext() error = %v, want containing 'invalid command name'", err)
		}
	})

	t.Run("Invalid command name - newline", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go\n", "version")
		if err == nil {
			t.Error("CommandContext() expected error for newline in command name, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
	})

	t.Run("Invalid command name - null byte", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go\x00", "version")
		if err == nil {
			t.Error("CommandContext() expected error for null byte in command name, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
	})

	t.Run("Invalid argument - null byte", func(t *testing.T) {
		cmd, err := safeshell.CommandContext(ctx, "go", "list\x00")
		if err == nil {
			t.Error("CommandContext() expected error for null byte in argument, got nil")
		}
		if cmd != nil {
			t.Errorf("CommandContext() expected nil cmd, got %v", cmd)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid argument") {
			t.Errorf("CommandContext() error = %v, want containing 'invalid argument'", err)
		}
	})
}
