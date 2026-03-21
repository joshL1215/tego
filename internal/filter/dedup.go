package filter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/joshL1215/tego/internal/store"
)

type compiledDedup struct {
	re       *regexp.Regexp
	category string
	minLines int
}

// deduplicateBlocks detects collapsible blocks using rule-based patterns and
// heuristic prefix clustering. Matched blocks are stored in the SQLite store
// and replaced with a summary line containing the retrieval command.
func deduplicateBlocks(text string, rules []DeduplicationRule, heuristicMin int, s *store.Store) (string, int) {
	if s == nil {
		return text, 0
	}

	compiled := make([]compiledDedup, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, compiledDedup{
			re:       re,
			category: r.Category,
			minLines: r.MinLines,
		})
	}

	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	totalBlocks := 0
	i := 0

	for i < len(lines) {
		// Try rule-based matching first
		matched, end, category := matchRuleBlock(lines, i, compiled)
		if matched {
			block := strings.Join(lines[i:end], "\n")
			count := end - i
			id, err := s.Store(block)
			if err == nil {
				result = append(result, fmt.Sprintf("[%d lines of %s omitted — run `tego retrieve %s` to view]", count, category, id))
				totalBlocks++
			} else {
				result = append(result, lines[i:end]...)
			}
			i = end
			continue
		}

		// Try heuristic prefix clustering
		if heuristicMin > 0 {
			end := matchPrefixCluster(lines, i, heuristicMin)
			if end > i {
				block := strings.Join(lines[i:end], "\n")
				count := end - i
				id, err := s.Store(block)
				if err == nil {
					result = append(result, fmt.Sprintf("[%d similar lines omitted — run `tego retrieve %s` to view]", count, id))
					totalBlocks++
				} else {
					result = append(result, lines[i:end]...)
				}
				i = end
				continue
			}
		}

		result = append(result, lines[i])
		i++
	}

	return strings.Join(result, "\n"), totalBlocks
}

// matchRuleBlock checks if a run of consecutive lines starting at pos matches
// any dedup rule. Returns whether a match was found, the end index, and the category.
func matchRuleBlock(lines []string, pos int, rules []compiledDedup) (bool, int, string) {
	for _, rule := range rules {
		if !rule.re.MatchString(lines[pos]) {
			continue
		}
		// Count consecutive matching lines
		end := pos + 1
		for end < len(lines) && rule.re.MatchString(lines[end]) {
			end++
		}
		if end-pos >= rule.minLines {
			return true, end, rule.category
		}
	}
	return false, 0, ""
}

// matchPrefixCluster detects a block of consecutive lines sharing a common prefix
// of at least 10 characters. Returns the end index if a block >= threshold is found,
// or the start index if no cluster was detected.
func matchPrefixCluster(lines []string, pos int, threshold int) int {
	if pos >= len(lines) || len(lines[pos]) < 10 {
		return pos
	}

	prefix := lines[pos][:10]
	end := pos + 1
	for end < len(lines) && len(lines[end]) >= 10 && lines[end][:10] == prefix {
		end++
	}

	if end-pos >= threshold {
		return end
	}
	return pos
}
