package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joshL1215/tego/internal/message"
)

// LogEntry represents a single proxied request in the log file.
type LogEntry struct {
	Timestamp     string          `json:"ts"`
	Path          string          `json:"path"`
	OriginalBytes int             `json:"original_bytes"`
	FilteredBytes int             `json:"filtered_bytes"`
	SavingsPercent int            `json:"savings_pct"`
	DurationMs    int64           `json:"duration_ms"`
	ToolResults   []ToolResultLog `json:"tool_results,omitempty"`
}

// ToolResultLog is per-tool-result detail.
type ToolResultLog struct {
	Index         int    `json:"index"`
	OriginalBytes int    `json:"original_bytes"`
	FilteredBytes int    `json:"filtered_bytes"`
	Stripped      string `json:"stripped"`
}

type requestLogger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

func newRequestLogger(logPath string) (*requestLogger, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &requestLogger{
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

func (l *requestLogger) Log(path string, originalSize, filteredSize int, elapsed time.Duration, results []message.ToolResultInfo) {
	savings := 0
	if originalSize > 0 {
		savings = 100 - (filteredSize*100)/originalSize
	}

	entry := LogEntry{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Path:           path,
		OriginalBytes:  originalSize,
		FilteredBytes:  filteredSize,
		SavingsPercent: savings,
		DurationMs:     elapsed.Milliseconds(),
	}

	for _, tr := range results {
		entry.ToolResults = append(entry.ToolResults, ToolResultLog{
			Index:         tr.Index,
			OriginalBytes: tr.OriginalBytes,
			FilteredBytes: tr.FilteredBytes,
			Stripped:      tr.Stats.Summary(),
		})
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.enc.Encode(entry)
}

func (l *requestLogger) Close() error {
	return l.file.Close()
}
