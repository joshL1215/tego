package message

import (
	"encoding/json"

	"github.com/joshL1215/tego/internal/filter"
)

// ToolResultInfo holds info about a filtered tool_result for logging.
type ToolResultInfo struct {
	Index         int
	OriginalBytes int
	FilteredBytes int
	Stats         *filter.FilterStats
}

// FilterToolResults parses the messages array in a request body, filters
// tool_result text content, and returns the modified body and filter info.
func FilterToolResults(body []byte, engine *filter.Engine) ([]byte, []ToolResultInfo, error) {
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		return body, nil, err
	}

	messagesRaw, ok := request["messages"]
	if !ok {
		return body, nil, nil
	}

	messages, ok := messagesRaw.([]any)
	if !ok {
		return body, nil, nil
	}

	var results []ToolResultInfo
	toolIdx := 0

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}

		role, _ := msgMap["role"].(string)

		// Handle role:"tool" messages (legacy format)
		if role == "tool" {
			if content, ok := msgMap["content"].(string); ok {
				filtered, stats := engine.Filter(content)
				if stats.OriginalBytes != stats.FilteredBytes {
					msgMap["content"] = filtered
					results = append(results, ToolResultInfo{
						Index:         toolIdx,
						OriginalBytes: stats.OriginalBytes,
						FilteredBytes: stats.FilteredBytes,
						Stats:         stats,
					})
				}
				toolIdx++
			}
			continue
		}

		// Handle content array with tool_result blocks
		contentRaw, ok := msgMap["content"]
		if !ok {
			continue
		}

		contentArr, ok := contentRaw.([]any)
		if !ok {
			continue
		}

		for _, block := range contentArr {
			blockMap, ok := block.(map[string]any)
			if !ok {
				continue
			}

			blockType, _ := blockMap["type"].(string)
			if blockType != "tool_result" {
				continue
			}

			filterToolResultContent(blockMap, engine, &results, &toolIdx)
		}
	}

	if len(results) == 0 {
		return body, nil, nil
	}

	modified, err := json.Marshal(request)
	if err != nil {
		return body, nil, err
	}

	return modified, results, nil
}

func filterToolResultContent(blockMap map[string]any, engine *filter.Engine, results *[]ToolResultInfo, toolIdx *int) {
	contentRaw, ok := blockMap["content"]
	if !ok {
		return
	}

	switch content := contentRaw.(type) {
	case string:
		// Simple string content
		filtered, stats := engine.Filter(content)
		if stats.OriginalBytes != stats.FilteredBytes {
			blockMap["content"] = filtered
			*results = append(*results, ToolResultInfo{
				Index:         *toolIdx,
				OriginalBytes: stats.OriginalBytes,
				FilteredBytes: stats.FilteredBytes,
				Stats:         stats,
			})
		}
		*toolIdx++

	case []any:
		// Array of content blocks
		for _, item := range content {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := itemMap["type"].(string)
			if itemType != "text" {
				continue
			}
			text, ok := itemMap["text"].(string)
			if !ok {
				continue
			}
			filtered, stats := engine.Filter(text)
			if stats.OriginalBytes != stats.FilteredBytes {
				itemMap["text"] = filtered
				*results = append(*results, ToolResultInfo{
					Index:         *toolIdx,
					OriginalBytes: stats.OriginalBytes,
					FilteredBytes: stats.FilteredBytes,
					Stats:         stats,
				})
			}
			*toolIdx++
		}
	}
}
