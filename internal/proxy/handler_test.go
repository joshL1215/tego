package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHandleMessages_FiltersAndForwards(t *testing.T) {
	// Mock upstream that captures what it receives
	var receivedBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte("data: {\"type\":\"message_start\"}\n\n"))
	}))
	defer upstream.Close()

	server := &Server{
		Port:     0,
		engine:   testEngine(),
		upstream: upstream.URL,
	}

	request := map[string]any{
		"model":  "claude-sonnet-4-20250514",
		"stream": true,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":       "tool_result",
						"tool_use_id": "test-1",
						"content":    "npm warn deprecated pkg\nactual important output\nnpm warn peer dep",
					},
				},
			},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test-key")

	rec := httptest.NewRecorder()
	server.handleMessages(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify the forwarded request was filtered
	var forwarded map[string]any
	if err := json.Unmarshal(receivedBody, &forwarded); err != nil {
		t.Fatalf("failed to parse forwarded body: %v", err)
	}

	messages := forwarded["messages"].([]any)
	msg := messages[0].(map[string]any)
	content := msg["content"].([]any)
	block := content[0].(map[string]any)
	text := block["content"].(string)

	if strings.Contains(text, "npm warn") {
		t.Errorf("forwarded body should not contain npm warn lines, got: %q", text)
	}
	if !strings.Contains(text, "actual important output") {
		t.Errorf("forwarded body should contain actual output, got: %q", text)
	}
}

func TestHandleMessages_PreservesAuthHeader(t *testing.T) {
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	server := &Server{
		Port:     0,
		engine:   testEngine(),
		upstream: upstream.URL,
	}

	request := map[string]any{
		"model":    "claude-sonnet-4-20250514",
		"messages": []any{},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer sk-ant-my-key")

	rec := httptest.NewRecorder()
	server.handleMessages(rec, req)

	if receivedAuth != "Bearer sk-ant-my-key" {
		t.Errorf("expected auth header to be forwarded, got %q", receivedAuth)
	}
}

func TestHandlePassthrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()

	server := &Server{
		Port:     0,
		engine:   testEngine(),
		upstream: upstream.URL,
	}

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	server.handlePassthrough(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != `{"models":[]}` {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{42, "42"},
		{999, "999"},
		{1000, "1,000"},
		{14280, "14,280"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
