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

package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

type runTestCase struct {
	name        string
	args        []string
	expectError bool
	errContains string
}

func getSuccessRunCases() []runTestCase {
	return []runTestCase{
		{name: "no args prints help", args: []string{}},
		{name: "help flag", args: []string{"--help"}},
		{name: "version flag", args: []string{"--version"}},
		{name: "version subcommand", args: []string{"version"}},
		{name: "list subcommand", args: []string{"list"}},
		{name: "check subcommand", args: []string{"check"}},
	}
}

func getErrorRunCases() []runTestCase {
	return []runTestCase{
		{
			name:        "bad flag",
			args:        []string{"--bad-flag"},
			expectError: true,
			errContains: "unknown flag: --bad-flag",
		},
		{
			name:        "unknown command",
			args:        []string{"unknowncmd"},
			expectError: true,
			errContains: "unknown command",
		},
	}
}

func executeRunTest(t *testing.T, tc runTestCase) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := run(ctx, tc.args)
	if (err != nil) != tc.expectError {
		t.Errorf("run() error = %v, expectError %v", err, tc.expectError)
	}

	if err != nil && tc.errContains != "" {
		if !strings.Contains(err.Error(), tc.errContains) {
			t.Errorf("run() error = %q, want to contain %q", err.Error(), tc.errContains)
		}
	}
}

func TestRun_SuccessCases(t *testing.T) {
	for _, tc := range getSuccessRunCases() {
		t.Run(tc.name, func(t *testing.T) {
			executeRunTest(t, tc)
		})
	}
}

func TestRun_ErrorCases(t *testing.T) {
	for _, tc := range getErrorRunCases() {
		t.Run(tc.name, func(t *testing.T) {
			executeRunTest(t, tc)
		})
	}
}

func TestBuildInfoFallback(t *testing.T) {
	if version == "" {
		t.Errorf("expected version to be non-empty")
	}

	v := resolveVersionFromBuildInfo("fallback-v1")
	if v == "" {
		t.Errorf("expected non-empty version from resolveVersionFromBuildInfo")
	}
}
