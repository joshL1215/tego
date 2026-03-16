package filter

import (
	"fmt"
	"regexp"
	"strings"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes all ANSI escape sequences from text.
func stripANSI(text string) (string, int) {
	matches := ansiPattern.FindAllStringIndex(text, -1)
	count := len(matches)
	if count == 0 {
		return text, 0
	}
	return ansiPattern.ReplaceAllString(text, ""), count
}

// dropMatchingLines removes lines matching any drop rule patterns.
// Returns filtered text, progress bar count, and a map of category -> count.
func dropMatchingLines(text string, rules []DropLineRule) (string, int, map[string]int) {
	if len(rules) == 0 {
		return text, 0, map[string]int{}
	}

	type compiledRule struct {
		re       *regexp.Regexp
		category string
	}

	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, compiledRule{re: re, category: r.Category})
	}

	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	progressBars := 0
	categories := map[string]int{}

	for _, line := range lines {
		dropped := false
		for _, rule := range compiled {
			if rule.re.MatchString(line) {
				if rule.category == "progress-bar" {
					progressBars++
				}
				categories[rule.category]++
				dropped = true
				break
			}
		}
		if !dropped {
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "\n"), progressBars, categories
}

var passingTestPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(PASS|PASSED|ok\s)`),
	regexp.MustCompile(`^\s*✓\s`),
	regexp.MustCompile(`^\s*✔\s`),
	regexp.MustCompile(`^\s*\.+$`), // jest-style dots
}

// collapsePassingTests replaces sequences of N+ passing test lines with a summary.
func collapsePassingTests(text string, threshold int) (string, int) {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	totalCollapsed := 0

	run := []string{}
	flushRun := func() {
		if len(run) >= threshold {
			result = append(result, fmt.Sprintf("[%d passing tests omitted]", len(run)))
			totalCollapsed += len(run)
		} else {
			result = append(result, run...)
		}
		run = run[:0]
	}

	for _, line := range lines {
		isPass := false
		for _, p := range passingTestPatterns {
			if p.MatchString(line) {
				isPass = true
				break
			}
		}
		if isPass {
			run = append(run, line)
		} else {
			if len(run) > 0 {
				flushRun()
			}
			result = append(result, line)
		}
	}
	if len(run) > 0 {
		flushRun()
	}

	return strings.Join(result, "\n"), totalCollapsed
}

// collapseBlankLines replaces sequences of N+ blank lines with a single blank line.
func collapseBlankLines(text string, threshold int) (string, int) {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	blankCount := 0
	totalRemoved := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount < threshold {
				result = append(result, line)
			} else {
				totalRemoved++
			}
		} else {
			blankCount = 0
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), totalRemoved
}

// collapseRepeatedLines replaces sequences of N+ identical lines with a summary.
func collapseRepeatedLines(text string, threshold int) (string, int) {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	totalCollapsed := 0

	i := 0
	for i < len(lines) {
		// Count consecutive identical lines
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		count := j - i
		if count >= threshold {
			result = append(result, fmt.Sprintf("%s (repeated %d times)", lines[i], count))
			totalCollapsed += count - 1
		} else {
			for k := i; k < j; k++ {
				result = append(result, lines[k])
			}
		}
		i = j
	}

	return strings.Join(result, "\n"), totalCollapsed
}
