package skills_test

import (
	"io/fs"
	"testing"

	"github.com/danicat/godoctor/skills"
)

func TestEmbeddedSkills(t *testing.T) {
	requiredSkills := []string{
		"godoctor/SKILL.md",
		"selene/SKILL.md",
		"testquery/SKILL.md",
	}

	for _, relPath := range requiredSkills {
		data, err := fs.ReadFile(skills.FS, relPath)
		if err != nil {
			t.Fatalf("failed to read embedded skill %s: %v", relPath, err)
		}
		if len(data) == 0 {
			t.Errorf("embedded skill %s is empty", relPath)
		}
	}
}
