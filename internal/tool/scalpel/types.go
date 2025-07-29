package scalpel

// Position represents a single point in a file.
// Either Offset or both Line and Column must be provided.
type Position struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// Range represents a range of text in a file.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Match represents a single match from a search operation.
type Match struct {
	Range  Range    `json:"range"`
	Groups []string `json:"groups"`
}

// ScalpelParams defines the input parameters for the scalpel tool.
type ScalpelParams struct {
	Operation   string    `json:"operation"`
	FilePath    string    `json:"file_path"`
	Start       *Position `json:"start,omitempty"`
	End         *Position `json:"end,omitempty"`
	Content     string    `json:"content,omitempty"`
	Search      string    `json:"search,omitempty"`
	Pattern     string    `json:"pattern,omitempty"`
	Replacement string    `json:"replacement,omitempty"`
}
