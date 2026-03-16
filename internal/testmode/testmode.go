package testmode

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/joshL1215/tego/internal/filter"
	"github.com/joshL1215/tego/internal/proxy"
)

// sampleRequest builds a realistic /v1/messages request with noisy tool_result content.
func sampleRequest() map[string]any {
	return map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Run the build and tests",
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type": "tool_use",
						"id":   "toolu_01A",
						"name": "bash",
						"input": map[string]any{
							"command": "npm install && npm test",
						},
					},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_01A",
						"content":     npmInstallOutput(),
					},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type": "tool_use",
						"id":   "toolu_01B",
						"name": "bash",
						"input": map[string]any{
							"command": "cargo build 2>&1",
						},
					},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_01B",
						"content":     cargoBuildOutput(),
					},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type": "tool_use",
						"id":   "toolu_01C",
						"name": "bash",
						"input": map[string]any{
							"command": "go test ./... 2>&1",
						},
					},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_01C",
						"content":     goTestOutput(),
					},
				},
			},
		},
	}
}

func npmInstallOutput() string {
	return "\x1b[33mnpm warn\x1b[0m deprecated inflight@1.0.6\n" +
		"\x1b[33mnpm warn\x1b[0m deprecated glob@7.2.3\n" +
		"\x1b[33mnpm warn\x1b[0m deprecated @humanwhocodes/object-schema@2.0.3\n" +
		"npm notice\n" +
		"npm notice New major version of npm available! 9.8.1 -> 10.2.4\n" +
		"npm notice Run `npm install -g npm@10.2.4` to update!\n" +
		"npm notice\n" +
		"\n" +
		"added 487 packages in 12s\n" +
		"\n" +
		"> myproject@1.0.0 test\n" +
		"> jest --coverage\n" +
		"\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/utils.test.js\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/parser.test.js\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/formatter.test.js\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/validator.test.js\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/handler.test.js\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/middleware.test.js\n" +
		"\x1b[1m\x1b[32mPASS\x1b[0m src/router.test.js\n" +
		"\n" +
		"Test Suites: 7 passed, 7 total\n" +
		"Tests:       42 passed, 42 total\n" +
		"Time:        3.14s\n"
}

func cargoBuildOutput() string {
	return "  Compiling proc-macro2 v1.0.78\n" +
		"  Compiling unicode-ident v1.0.12\n" +
		"  Compiling quote v1.0.35\n" +
		"  Compiling syn v2.0.48\n" +
		"  Compiling serde_derive v1.0.196\n" +
		"  Compiling serde v1.0.196\n" +
		"  Compiling serde_json v1.0.113\n" +
		"  Compiling tokio v1.36.0\n" +
		"  Compiling myproject v0.1.0 (/home/user/myproject)\n" +
		"\n" +
		"\x1b[1m\x1b[32m    Finished\x1b[0m dev [unoptimized + debuginfo] target(s) in 14.23s\n"
}

func goTestOutput() string {
	lines := []string{
		"ok  \tmyproject/pkg/auth\t0.012s",
		"ok  \tmyproject/pkg/handler\t0.034s",
		"ok  \tmyproject/pkg/middleware\t0.021s",
		"ok  \tmyproject/pkg/parser\t0.008s",
		"ok  \tmyproject/pkg/validator\t0.015s",
		"ok  \tmyproject/pkg/router\t0.028s",
		"ok  \tmyproject/pkg/service\t0.041s",
		"",
		"FAIL\tmyproject/pkg/database\t0.123s",
		"--- FAIL: TestDatabaseConnection (0.100s)",
		"    database_test.go:42: expected nil error, got: connection refused",
		"",
		"FAIL",
	}
	return strings.Join(lines, "\n")
}

// Run starts a mock upstream, sends a sample request through the proxy, and shows the results.
func Run(port int) error {
	// Start mock upstream that echoes back a simple SSE response
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		slog.Info(fmt.Sprintf("mock upstream received %d bytes", len(body)))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-20250514\"}}\n\n"))
		w.Write([]byte("data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		w.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"The build succeeded but there's a database test failure.\"}}\n\n"))
		w.Write([]byte("data: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer upstream.Close()

	config, err := filter.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load filter config: %w", err)
	}

	engine := filter.NewEngine(config)
	server := proxy.NewServer(port, engine)
	server.SetUpstream(upstream.URL)

	// Start proxy in background
	go server.Start()

	// Give it a moment to start
	fmt.Println()
	fmt.Println("tego test mode — sending sample request with noisy tool_result content")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Send sample request
	reqBody := sampleRequest()
	body, _ := json.Marshal(reqBody)

	originalSize := len(body)
	fmt.Printf("Original request size: %d bytes\n", originalSize)
	fmt.Println()

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/v1/messages", port),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("Response status: %d\n", resp.StatusCode)
	fmt.Printf("Response body:\n%s\n", string(respBody))
	fmt.Println()
	fmt.Println("Check the log lines above ↑ to see what was filtered.")

	return nil
}
