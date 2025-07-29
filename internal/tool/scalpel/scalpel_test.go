package scalpel_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/tool/scalpel"
	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	// Create a temporary file for testing.
	tmpfile, err := os.CreateTemp("", "scalpel_test")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name()) // clean up

	initialContent := `line 1
line 2
`

	tests := []struct {
		name         string
		params       *scalpel.ScalpelParams
		want         func(string) string
		wantErr      bool
		finalContent string
	}{
		{
			name:    "unknown Operation",
			params:  &scalpel.ScalpelParams{Operation: "foo"},
			wantErr: true,
		},
		{
			name: "insert at beginning",
			params: &scalpel.ScalpelParams{
				Operation: "insert",
				FilePath:  tmpfile.Name(),
				Content:   "inserted ",
				Start:     &scalpel.Position{Line: 1, Column: 1},
			},
			finalContent: `inserted line 1
line 2
`,
		},
		{
			name: "insert at end",
			params: &scalpel.ScalpelParams{
				Operation: "insert",
				FilePath:  tmpfile.Name(),
				Content:   " inserted",
				Start:     &scalpel.Position{Line: 2, Column: 7},
			},
			finalContent: `line 1
line 2 inserted
`,
		},
		{
			name: "insert in middle",
			params: &scalpel.ScalpelParams{
				Operation: "insert",
				FilePath:  tmpfile.Name(),
				Content:   "inserted ",
				Start:     &scalpel.Position{Line: 1, Column: 4},
			},
			finalContent: `lininserted e 1
line 2
`,
		},
		{
			name: "insert multiple lines",
			params: &scalpel.ScalpelParams{
				Operation: "insert",
				FilePath:  tmpfile.Name(),
				Content:   "inserted\nlines\n",
				Start:     &scalpel.Position{Line: 1, Column: 1},
			},
			finalContent: `inserted
lines
line 1
line 2
`,
		},
		{
			name:    "insert invalid file path",
			params:  &scalpel.ScalpelParams{Operation: "insert", FilePath: "non-existent-file", Content: "foo", Start: &scalpel.Position{Line: 1, Column: 1}},
			wantErr: true,
		},
		{
			name:    "insert invalid position",
			params:  &scalpel.ScalpelParams{Operation: "insert", FilePath: tmpfile.Name(), Content: "foo", Start: &scalpel.Position{Line: 10, Column: 1}},
			wantErr: true,
		},
		{
			name: "delete at beginning",
			params: &scalpel.ScalpelParams{
				Operation: "delete",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 1, Column: 4},
			},
			finalContent: `e 1
line 2
`,
		},
		{
			name: "delete at end",
			params: &scalpel.ScalpelParams{
				Operation: "delete",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 2, Column: 4},
				End:       &scalpel.Position{Line: 2, Column: 7},
			},
			finalContent: `line 1
lin
`,
		},
		{
			name: "delete whole line",
			params: &scalpel.ScalpelParams{
				Operation: "delete",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 2, Column: 1},
			},
			finalContent: `line 2
`,
		},
		{
			name: "delete multiple lines",
			params: &scalpel.ScalpelParams{
				Operation: "delete",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 2, Column: 7},
			},
			finalContent: `
`,
		},
		{
			name:    "delete invalid file path",
			params:  &scalpel.ScalpelParams{Operation: "delete", FilePath: "non-existent-file", Start: &scalpel.Position{Line: 1, Column: 1}, End: &scalpel.Position{Line: 1, Column: 2}},
			wantErr: true,
		},
		{
			name:    "delete invalid range",
			params:  &scalpel.ScalpelParams{Operation: "delete", FilePath: tmpfile.Name(), Start: &scalpel.Position{Line: 10, Column: 1}, End: &scalpel.Position{Line: 10, Column: 2}},
			wantErr: true,
		},
		{
			name: "replace at beginning",
			params: &scalpel.ScalpelParams{
				Operation: "replace",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 1, Column: 4},
				Content:   "replaced",
			},
			finalContent: `replacede 1
line 2
`,
		},
		{
			name: "replace at end",
			params: &scalpel.ScalpelParams{
				Operation: "replace",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 2, Column: 4},
				End:       &scalpel.Position{Line: 2, Column: 7},
				Content:   "replaced",
			},
			finalContent: `line 1
linreplaced
`,
		},
		{
			name: "replace whole line",
			params: &scalpel.ScalpelParams{
				Operation: "replace",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 2, Column: 1},
				Content:   "replaced",
			},
			finalContent: `replacedline 2
`,
		},
		{
			name: "replace multiple lines",
			params: &scalpel.ScalpelParams{
				Operation: "replace",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 2, Column: 7},
				Content:   "replaced",
			},
			finalContent: `replaced
`,
		},
		{
			name: "replace with empty content",
			params: &scalpel.ScalpelParams{
				Operation: "replace",
				FilePath:  tmpfile.Name(),
				Start:     &scalpel.Position{Line: 1, Column: 1},
				End:       &scalpel.Position{Line: 1, Column: 6},
				Content:   "",
			},
			finalContent: `1
line 2
`,
		},
		{
			name:    "replace invalid file path",
			params:  &scalpel.ScalpelParams{Operation: "replace", FilePath: "non-existent-file", Start: &scalpel.Position{Line: 1, Column: 1}, End: &scalpel.Position{Line: 1, Column: 2}, Content: "foo"},
			wantErr: true,
		},
		{
			name:    "replace invalid range",
			params:  &scalpel.ScalpelParams{Operation: "replace", FilePath: tmpfile.Name(), Start: &scalpel.Position{Line: 10, Column: 1}, End: &scalpel.Position{Line: 10, Column: 2}, Content: "foo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Because the scalpel Operation modifies the file, we need to reset the file content before each test.
			f, err := os.Create(tmpfile.Name())
			require.NoError(t, err)
			_, err = f.WriteString(initialContent)
			require.NoError(t, err)
			f.Close()

			got, err := scalpel.Execute(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// The new 'want' is the final content with line numbers.
			var builder strings.Builder
			lines := strings.Split(tt.finalContent, "\n")
			for i, line := range lines {
				if i == len(lines)-1 && len(line) == 0 {
					continue
				}
				builder.WriteString(fmt.Sprintf("%d: %s\n", i+1, line))
			}
			want := builder.String()

			require.Equal(t, want, got)

			if tt.finalContent != "" {
				content, err := os.ReadFile(tmpfile.Name())
				require.NoError(t, err)
				require.Equal(t, tt.finalContent, string(content))
			}
		})
	}
}