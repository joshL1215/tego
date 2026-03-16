package message

import (
	"encoding/json"
	"testing"

	"github.com/joshL1215/tego/internal/filter"
)

func testEngine() *filter.Engine {
	return filter.NewEngine(&filter.Config{
		StripANSI:             true,
		CollapseBlankLines:    3,
		CollapseRepeatedLines: 3,
		CollapsePassingTests:  5,
		DropLines: []filter.DropLineRule{
			{Pattern: "^npm warn", Category: "npm-warn"},
		},
	})
}

func TestFilterToolResults_StringContent(t *testing.T) {
	request := map[string]any{
		"model": "claude-sonnet-4-20250514",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":       "tool_result",
						"tool_use_id": "test-1",
						"content":    "npm warn deprecated\nactual output\nnpm warn old",
					},
				},
			},
		},
	}

	body, _ := json.Marshal(request)
	filtered, results, err := FilterToolResults(body, testEngine())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Verify the npm warn lines were removed
	var parsed map[string]any
	json.Unmarshal(filtered, &parsed)

	messages := parsed["messages"].([]any)
	msg := messages[0].(map[string]any)
	content := msg["content"].([]any)
	block := content[0].(map[string]any)
	text := block["content"].(string)

	if text != "actual output" {
		t.Errorf("expected 'actual output', got %q", text)
	}
}

func TestFilterToolResults_ArrayContent(t *testing.T) {
	request := map[string]any{
		"model": "claude-sonnet-4-20250514",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":       "tool_result",
						"tool_use_id": "test-1",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": "npm warn deprecated\nactual output\nnpm warn old",
							},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(request)
	filtered, results, err := FilterToolResults(body, testEngine())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var parsed map[string]any
	json.Unmarshal(filtered, &parsed)

	messages := parsed["messages"].([]any)
	msg := messages[0].(map[string]any)
	contentArr := msg["content"].([]any)
	block := contentArr[0].(map[string]any)
	innerContent := block["content"].([]any)
	textBlock := innerContent[0].(map[string]any)
	text := textBlock["text"].(string)

	if text != "actual output" {
		t.Errorf("expected 'actual output', got %q", text)
	}
}

func TestFilterToolResults_NoToolResults(t *testing.T) {
	request := map[string]any{
		"model": "claude-sonnet-4-20250514",
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello, Claude!",
			},
		},
	}

	body, _ := json.Marshal(request)
	filtered, results, err := FilterToolResults(body, testEngine())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	// Body should be unchanged (nil results means original body returned)
	if string(filtered) != string(body) {
		t.Error("body should be unchanged when no tool_results found")
	}
}

func TestFilterToolResults_PreservesOtherFields(t *testing.T) {
	request := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":       "tool_result",
						"tool_use_id": "test-1",
						"content":    "npm warn foo\nreal data",
					},
				},
			},
		},
	}

	body, _ := json.Marshal(request)
	filtered, _, err := FilterToolResults(body, testEngine())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(filtered, &parsed)

	if parsed["model"] != "claude-sonnet-4-20250514" {
		t.Error("model field was lost")
	}
	if parsed["stream"] != true {
		t.Error("stream field was lost")
	}
	if parsed["max_tokens"] != float64(1024) {
		t.Error("max_tokens field was lost")
	}
}
