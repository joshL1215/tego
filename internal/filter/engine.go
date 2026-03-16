package filter

import "fmt"

// FilterStats tracks what was stripped from a single piece of text.
type FilterStats struct {
	ANSISequences     int
	BlankLines        int
	RepeatedLines     int
	DroppedLines      int
	CollapsedTests    int
	ProgressBars      int
	OriginalBytes     int
	FilteredBytes     int
	DroppedCategories map[string]int // category name -> count
}

// Summary returns a human-readable summary of what was stripped.
func (s *FilterStats) Summary() string {
	parts := []string{}
	if s.ANSISequences > 0 {
		parts = append(parts, fmt.Sprintf("%d ANSI sequences", s.ANSISequences))
	}
	if s.BlankLines > 0 {
		parts = append(parts, fmt.Sprintf("%d blank lines", s.BlankLines))
	}
	if s.RepeatedLines > 0 {
		parts = append(parts, fmt.Sprintf("%d repeated lines", s.RepeatedLines))
	}
	if s.ProgressBars > 0 {
		parts = append(parts, fmt.Sprintf("%d progress bars", s.ProgressBars))
	}
	if s.CollapsedTests > 0 {
		parts = append(parts, fmt.Sprintf("%d passing-test lines collapsed", s.CollapsedTests))
	}
	for cat, count := range s.DroppedCategories {
		parts = append(parts, fmt.Sprintf("%d %s lines", count, cat))
	}
	if len(parts) == 0 {
		return "no changes"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}

// Engine applies filter rules to text content.
type Engine struct {
	config *Config
}

// NewEngine creates a filter engine with the given config.
func NewEngine(config *Config) *Engine {
	return &Engine{config: config}
}

// Filter applies all filtering rules to the input text and returns the filtered text and stats.
func (e *Engine) Filter(input string) (string, *FilterStats) {
	stats := &FilterStats{
		OriginalBytes:     len(input),
		DroppedCategories: make(map[string]int),
	}

	text := input

	// 1. Strip ANSI escape codes
	if e.config.StripANSI {
		text, stats.ANSISequences = stripANSI(text)
	}

	// 2-6. Drop matching lines (progress bars, npm/pip/cargo noise, git, docker, make)
	text, stats.ProgressBars, stats.DroppedCategories = dropMatchingLines(text, e.config.DropLines)

	// 7. Collapse passing tests
	if e.config.CollapsePassingTests > 0 {
		text, stats.CollapsedTests = collapsePassingTests(text, e.config.CollapsePassingTests)
	}

	// 8. Collapse blank lines
	if e.config.CollapseBlankLines > 0 {
		text, stats.BlankLines = collapseBlankLines(text, e.config.CollapseBlankLines)
	}

	// 9. Collapse repeated lines
	if e.config.CollapseRepeatedLines > 0 {
		text, stats.RepeatedLines = collapseRepeatedLines(text, e.config.CollapseRepeatedLines)
	}

	stats.FilteredBytes = len(text)
	return text, stats
}
