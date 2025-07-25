package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errContains string
	}{
		{
			name:        "version flag",
			args:        []string{"--version"},
			expectError: false,
		},
		{
			name:        "bad flag",
			args:        []string{"--bad-flag"},
			expectError: true,
			errContains: "flag provided but not defined: -bad-flag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
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
		})
	}
}