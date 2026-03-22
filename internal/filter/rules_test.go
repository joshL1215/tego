package filter

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		count int
	}{
		{"no ansi", "hello world", "hello world", 0},
		{"color codes", "\x1b[31mred\x1b[0m text", "red text", 2},
		{"bold", "\x1b[1mbold\x1b[0m", "bold", 2},
		{"multiple", "\x1b[32m\x1b[1mgreen bold\x1b[0m", "green bold", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, count := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI() text = %q, want %q", got, tt.want)
			}
			if count != tt.count {
				t.Errorf("stripANSI() count = %d, want %d", count, tt.count)
			}
		})
	}
}

func TestDropMatchingLines(t *testing.T) {
	rules := []DropLineRule{
		{Pattern: "^npm warn", Category: "npm-warn"},
		{Pattern: "^npm notice", Category: "npm-notice"},
		{Pattern: `^\s*Compiling\s`, Category: "cargo"},
		{Pattern: `^remote:\s*(Counting|Compressing)`, Category: "git-transfer"},
		{Pattern: `^make\[\d+\]: (Entering|Leaving) directory`, Category: "make"},
	}

	input := strings.Join([]string{
		"npm warn deprecated package",
		"actual output line",
		"npm notice some notice",
		"  Compiling mycrate v0.1.0",
		"another real line",
		"remote: Counting objects: 100",
		"make[1]: Entering directory '/foo'",
		"final line",
	}, "\n")

	got, _, categories := dropMatchingLines(input, rules)
	lines := strings.Split(got, "\n")

	// 3 kept lines + 5 summary lines (one per category)
	if len(lines) != 8 {
		t.Errorf("expected 8 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "actual output line" {
		t.Errorf("unexpected line[0]: %q", lines[0])
	}
	// Verify summary lines are present
	if !strings.Contains(got, "[tego: stripped") {
		t.Errorf("expected tego summary lines in output")
	}
	if categories["npm-warn"] != 1 {
		t.Errorf("expected 1 npm-warn, got %d", categories["npm-warn"])
	}
	if categories["cargo"] != 1 {
		t.Errorf("expected 1 cargo, got %d", categories["cargo"])
	}
}

func TestCollapsePassingTests(t *testing.T) {
	lines := []string{
		"Running tests...",
		"PASS test_one",
		"PASS test_two",
		"PASS test_three",
		"PASS test_four",
		"PASS test_five",
		"PASS test_six",
		"Some other output",
	}
	input := strings.Join(lines, "\n")

	got, collapsed := collapsePassingTests(input, 5)
	if collapsed != 6 {
		t.Errorf("expected 6 collapsed, got %d", collapsed)
	}
	if !strings.Contains(got, "[6 passing tests omitted]") {
		t.Errorf("expected collapse marker, got: %s", got)
	}
	if strings.Contains(got, "PASS test_one") {
		t.Errorf("should not contain individual PASS lines")
	}
}

func TestCollapsePassingTestsBelowThreshold(t *testing.T) {
	lines := []string{
		"PASS test_one",
		"PASS test_two",
		"Some other output",
	}
	input := strings.Join(lines, "\n")

	got, collapsed := collapsePassingTests(input, 5)
	if collapsed != 0 {
		t.Errorf("expected 0 collapsed, got %d", collapsed)
	}
	if got != input {
		t.Errorf("expected unchanged output")
	}
}

func TestCollapseBlankLines(t *testing.T) {
	// 4 blank lines between line1 and line2, threshold=3 means keep 2, drop 2
	input := "line1\n\n\n\n\nline2\n\nline3"

	got, removed := collapseBlankLines(input, 3)
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
	// Should have line1, 2 blanks, line2, 1 blank, line3 = 6 lines
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
	}
}

func TestCollapseRepeatedLines(t *testing.T) {
	input := "header\nfoo\nfoo\nfoo\nfoo\nfoo\ntrailer"

	got, collapsed := collapseRepeatedLines(input, 3)
	if collapsed != 4 {
		t.Errorf("expected 4 collapsed, got %d", collapsed)
	}
	if !strings.Contains(got, "foo (repeated 5 times)") {
		t.Errorf("expected repeated marker, got: %s", got)
	}
}

func TestCollapseRepeatedLinesBelowThreshold(t *testing.T) {
	input := "foo\nfoo\nbar"

	got, collapsed := collapseRepeatedLines(input, 3)
	if collapsed != 0 {
		t.Errorf("expected 0 collapsed, got %d", collapsed)
	}
	if got != input {
		t.Errorf("expected unchanged output")
	}
}

func TestEngineFullPipeline(t *testing.T) {
	config := &Config{
		StripANSI:            true,
		CollapseBlankLines:   3,
		CollapseRepeatedLines: 3,
		CollapsePassingTests: 5,
		DropLines: []DropLineRule{
			{Pattern: "^npm warn", Category: "npm-warn"},
		},
	}
	engine := NewEngine(config)

	input := "\x1b[32mnpm warn old dep\x1b[0m\nreal output\n\n\n\n\nmore output"

	got, stats := engine.Filter(input)

	if stats.ANSISequences != 2 {
		t.Errorf("expected 2 ANSI sequences, got %d", stats.ANSISequences)
	}
	if !strings.Contains(got, "real output") {
		t.Errorf("should contain real output")
	}
	if strings.Contains(got, "npm warn old dep") {
		t.Errorf("should not contain original npm warn line")
	}
	if !strings.Contains(got, "[tego: stripped 1 npm-warn lines]") {
		t.Errorf("should contain tego summary for stripped lines")
	}
}
