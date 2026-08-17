package skills_test

import (
	"io/fs"
	"testing"

	"github.com/danicat/godoctor/skills"
)

func TestEmbeddedSkills(t *testing.T) {
	requiredFiles := []string{
		"godoctor/SKILL.md",
		"godoctor/references/selene.md",
		"godoctor/references/testquery.md",
	}

	for _, relPath := range requiredFiles {
		data, err := fs.ReadFile(skills.FS, relPath)
		if err != nil {
			t.Fatalf("failed to read embedded skill file %s: %v", relPath, err)
		}
		if len(data) == 0 {
			t.Errorf("embedded skill file %s is empty", relPath)
		}
	}
}
