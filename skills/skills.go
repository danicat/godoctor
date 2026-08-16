// Package skills embeds the GoDoctor agent skills for distribution.
package skills

import (
	"embed"
)

// FS contains all embedded GoDoctor skills (godoctor, selene, testquery).
//
//go:embed godoctor/SKILL.md selene/SKILL.md testquery/SKILL.md
var FS embed.FS
