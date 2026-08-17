// Package skills embeds the GoDoctor agent skills for distribution.
package skills

import (
	"embed"
)

// FS contains all embedded GoDoctor skills and reference documents.
//
//go:embed godoctor/*
var FS embed.FS
