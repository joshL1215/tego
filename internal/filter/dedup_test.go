package filter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/joshL1215/tego/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.OpenAt(dbPath)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- matchRuleBlock tests ---

func TestMatchRuleBlockMatches(t *testing.T) {
	rules := []compiledDedup{
		{re: mustCompile(`^\s*added\s+\d+\s+package`), category: "npm-install", minLines: 3},
	}
	lines := []string{
		"added 1 package",
		"added 2 packages",
		"added 3 packages",
		"added 4 packages",
		"some other line",
	}

	matched, end, category := matchRuleBlock(lines, 0, rules)
	if !matched {
		t.Fatal("expected match")
	}
	if end != 4 {
		t.Errorf("expected end=4, got %d", end)
	}
	if category != "npm-install" {
		t.Errorf("expected category npm-install, got %q", category)
	}
}

func TestMatchRuleBlockBelowThreshold(t *testing.T) {
	rules := []compiledDedup{
		{re: mustCompile(`^\s*added\s+\d+\s+package`), category: "npm-install", minLines: 5},
	}
	lines := []string{
		"added 1 package",
		"added 2 packages",
		"some other line",
	}

	matched, _, _ := matchRuleBlock(lines, 0, rules)
	if matched {
		t.Error("should not match below threshold")
	}
}

func TestMatchRuleBlockNoMatch(t *testing.T) {
	rules := []compiledDedup{
		{re: mustCompile(`^\s*added\s+\d+\s+package`), category: "npm-install", minLines: 3},
	}
	lines := []string{"unrelated line", "another line"}

	matched, _, _ := matchRuleBlock(lines, 0, rules)
	if matched {
		t.Error("should not match unrelated lines")
	}
}

func TestMatchRuleBlockMultipleRules(t *testing.T) {
	rules := []compiledDedup{
		{re: mustCompile(`^\s*added\s+\d+\s+package`), category: "npm-install", minLines: 5},
		{re: mustCompile(`^\s*Collecting\s`), category: "pip-install", minLines: 2},
	}
	lines := []string{
		"Collecting requests",
		"Collecting urllib3",
		"Collecting certifi",
		"done",
	}

	matched, end, category := matchRuleBlock(lines, 0, rules)
	if !matched {
		t.Fatal("expected second rule to match")
	}
	if end != 3 {
		t.Errorf("expected end=3, got %d", end)
	}
	if category != "pip-install" {
		t.Errorf("expected pip-install, got %q", category)
	}
}

// --- matchPrefixCluster tests ---

func TestMatchPrefixClusterMatches(t *testing.T) {
	lines := []string{
		"0123456789 line A",
		"0123456789 line B",
		"0123456789 line C",
		"0123456789 line D",
		"different prefix line",
	}

	end := matchPrefixCluster(lines, 0, 3)
	if end != 4 {
		t.Errorf("expected end=4, got %d", end)
	}
}

func TestMatchPrefixClusterBelowThreshold(t *testing.T) {
	lines := []string{
		"0123456789 line A",
		"0123456789 line B",
		"different line",
	}

	end := matchPrefixCluster(lines, 0, 3)
	if end != 0 {
		t.Errorf("expected no cluster (end=0), got %d", end)
	}
}

func TestMatchPrefixClusterShortLine(t *testing.T) {
	lines := []string{"short", "short", "short"}

	end := matchPrefixCluster(lines, 0, 2)
	if end != 0 {
		t.Errorf("expected no cluster for short lines, got %d", end)
	}
}

func TestMatchPrefixClusterMidSlice(t *testing.T) {
	lines := []string{
		"unrelated line here",
		"prefix1234 line A",
		"prefix1234 line B",
		"prefix1234 line C",
		"other stuff",
	}

	end := matchPrefixCluster(lines, 1, 3)
	if end != 4 {
		t.Errorf("expected end=4 starting from pos=1, got %d", end)
	}
}

// --- deduplicateBlocks tests ---

func TestDeduplicateBlocksNilStore(t *testing.T) {
	input := "line 1\nline 2\nline 3"
	rules := []DeduplicationRule{
		{Pattern: "^line", Category: "test", MinLines: 2},
	}

	got, blocks := deduplicateBlocks(input, rules, 0, nil)
	if got != input {
		t.Errorf("expected unchanged text with nil store, got %q", got)
	}
	if blocks != 0 {
		t.Errorf("expected 0 blocks with nil store, got %d", blocks)
	}
}

func TestDeduplicateBlocksRuleBased(t *testing.T) {
	s := openTestStore(t)

	npmLines := make([]string, 8)
	for i := range npmLines {
		npmLines[i] = fmt.Sprintf("added %d packages in 1s", i+1)
	}
	lines := append([]string{"header"}, npmLines...)
	lines = append(lines, "footer")
	input := strings.Join(lines, "\n")

	rules := []DeduplicationRule{
		{Pattern: `^\s*added\s+\d+\s+package`, Category: "npm-install", MinLines: 5},
	}

	got, blocks := deduplicateBlocks(input, rules, 0, s)
	if blocks != 1 {
		t.Errorf("expected 1 deduplicated block, got %d", blocks)
	}
	if !strings.Contains(got, "[8 lines of npm-install omitted") {
		t.Errorf("expected omission summary, got: %s", got)
	}
	if !strings.Contains(got, "tego retrieve") {
		t.Errorf("expected retrieve command in summary, got: %s", got)
	}
	if !strings.Contains(got, "header") {
		t.Error("header should be preserved")
	}
	if !strings.Contains(got, "footer") {
		t.Error("footer should be preserved")
	}

	// Verify stored content is retrievable
	// Summary format: [N lines of CATEGORY omitted — run `tego retrieve HASH` to view]
	for _, line := range strings.Split(got, "\n") {
		if idx := strings.Index(line, "tego retrieve "); idx >= 0 {
			rest := line[idx+len("tego retrieve "):]
			hash := strings.SplitN(rest, "`", 2)[0]
			content, err := s.Retrieve(hash)
			if err != nil {
				t.Fatalf("failed to retrieve stored block: %v", err)
			}
			if !strings.Contains(content, "added 1 packages") {
				t.Errorf("stored content should contain original lines, got: %s", content)
			}
			break
		}
	}
}

func TestDeduplicateBlocksHeuristicFallback(t *testing.T) {
	s := openTestStore(t)

	// Lines with a shared 10+ char prefix that won't match any rule
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("AAAAAAAAAA item %d here", i)
	}
	lines = append([]string{"before"}, lines...)
	lines = append(lines, "after")
	input := strings.Join(lines, "\n")

	got, blocks := deduplicateBlocks(input, nil, 10, s)
	if blocks != 1 {
		t.Errorf("expected 1 heuristic block, got %d", blocks)
	}
	if !strings.Contains(got, "[12 similar lines omitted") {
		t.Errorf("expected heuristic summary, got: %s", got)
	}
	if !strings.Contains(got, "before") {
		t.Error("before should be preserved")
	}
	if !strings.Contains(got, "after") {
		t.Error("after should be preserved")
	}
}

func TestDeduplicateBlocksHeuristicDisabled(t *testing.T) {
	s := openTestStore(t)

	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("AAAAAAAAAA item %d here", i)
	}
	input := strings.Join(lines, "\n")

	got, blocks := deduplicateBlocks(input, nil, 0, s)
	if blocks != 0 {
		t.Errorf("expected 0 blocks with heuristic disabled, got %d", blocks)
	}
	if got != input {
		t.Errorf("expected unchanged text with heuristic disabled")
	}
}

func TestDeduplicateBlocksMultipleBlocks(t *testing.T) {
	s := openTestStore(t)

	var lines []string
	lines = append(lines, "header")
	// First block: npm installs
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("added %d packages", i+1))
	}
	lines = append(lines, "middle")
	// Second block: pip installs
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("Collecting package%d", i))
	}
	lines = append(lines, "footer")
	input := strings.Join(lines, "\n")

	rules := []DeduplicationRule{
		{Pattern: `^\s*added\s+\d+\s+package`, Category: "npm-install", MinLines: 5},
		{Pattern: `^\s*Collecting\s`, Category: "pip-install", MinLines: 5},
	}

	got, blocks := deduplicateBlocks(input, rules, 0, s)
	if blocks != 2 {
		t.Errorf("expected 2 deduplicated blocks, got %d", blocks)
	}
	if !strings.Contains(got, "npm-install") {
		t.Error("expected npm-install category in output")
	}
	if !strings.Contains(got, "pip-install") {
		t.Error("expected pip-install category in output")
	}
	if !strings.Contains(got, "header") || !strings.Contains(got, "middle") || !strings.Contains(got, "footer") {
		t.Error("non-matching lines should be preserved")
	}
}

func TestDeduplicateBlocksRuleTakesPriorityOverHeuristic(t *testing.T) {
	s := openTestStore(t)

	// Lines that match both a rule AND share a prefix
	lines := make([]string, 6)
	for i := range lines {
		lines[i] = fmt.Sprintf("added %d packages in 1s", i+1)
	}
	input := strings.Join(lines, "\n")

	rules := []DeduplicationRule{
		{Pattern: `^\s*added\s+\d+\s+package`, Category: "npm-install", MinLines: 5},
	}

	got, blocks := deduplicateBlocks(input, rules, 5, s)
	if blocks != 1 {
		t.Errorf("expected 1 block, got %d", blocks)
	}
	// Should use the rule category, not the generic heuristic message
	if !strings.Contains(got, "npm-install") {
		t.Errorf("expected rule-based summary (npm-install), got: %s", got)
	}
	if strings.Contains(got, "similar lines") {
		t.Error("should not use heuristic summary when rule matches")
	}
}

func TestDeduplicateBlocksInvalidRegex(t *testing.T) {
	s := openTestStore(t)

	input := "line1\nline2\nline3"
	rules := []DeduplicationRule{
		{Pattern: "[invalid", Category: "bad", MinLines: 1},
	}

	got, blocks := deduplicateBlocks(input, rules, 0, s)
	if blocks != 0 {
		t.Errorf("invalid regex should not match, got %d blocks", blocks)
	}
	if got != input {
		t.Errorf("text should be unchanged with invalid regex")
	}
}

// --- Engine integration with dedup ---

func TestEngineWithDedup(t *testing.T) {
	s := openTestStore(t)

	config := &Config{
		StripANSI:             false,
		CollapseBlankLines:    0,
		CollapseRepeatedLines: 0,
		CollapsePassingTests:  0,
		DeduplicationMinLines: 10,
		Deduplication: []DeduplicationRule{
			{Pattern: `^\s*added\s+\d+\s+package`, Category: "npm-install", MinLines: 5},
		},
	}
	engine := NewEngine(config)
	engine.SetStore(s)

	var lines []string
	lines = append(lines, "build output")
	for i := 0; i < 7; i++ {
		lines = append(lines, fmt.Sprintf("added %d packages", i+1))
	}
	lines = append(lines, "build complete")
	input := strings.Join(lines, "\n")

	got, stats := engine.Filter(input)
	if stats.DeduplicatedBlocks != 1 {
		t.Errorf("expected 1 deduplicated block, got %d", stats.DeduplicatedBlocks)
	}
	if !strings.Contains(got, "build output") {
		t.Error("non-matching content should be preserved")
	}
	if !strings.Contains(got, "build complete") {
		t.Error("non-matching content should be preserved")
	}
	if strings.Contains(got, "added 1 packages") {
		t.Error("deduplicated lines should be removed from output")
	}
	if stats.FilteredBytes >= stats.OriginalBytes {
		t.Errorf("filtered should be smaller: %d >= %d", stats.FilteredBytes, stats.OriginalBytes)
	}
}

func TestEngineWithoutStore(t *testing.T) {
	config := &Config{
		DeduplicationMinLines: 10,
		Deduplication: []DeduplicationRule{
			{Pattern: `^\s*added\s+\d+\s+package`, Category: "npm-install", MinLines: 5},
		},
	}
	engine := NewEngine(config)

	var lines []string
	for i := 0; i < 7; i++ {
		lines = append(lines, fmt.Sprintf("added %d packages", i+1))
	}
	input := strings.Join(lines, "\n")

	got, stats := engine.Filter(input)
	if stats.DeduplicatedBlocks != 0 {
		t.Errorf("expected 0 blocks without store, got %d", stats.DeduplicatedBlocks)
	}
	if got != input {
		t.Error("text should be unchanged without store")
	}
}

func mustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
