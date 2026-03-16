package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joshL1215/tego/internal/filter"
)

const defaultUpstream = "https://api.anthropic.com"

// Server is the tego proxy server.
type Server struct {
	Port     int
	engine   *filter.Engine
	upstream string
	logger   *requestLogger
}

// NewServer creates a new proxy server.
func NewServer(port int, engine *filter.Engine) *Server {
	logPath := defaultLogPath()
	logger, err := newRequestLogger(logPath)
	if err != nil {
		slog.Warn("failed to open log file, logging disabled", "path", logPath, "error", err)
	} else {
		slog.Info("request log: " + logPath)
	}
	return &Server{
		Port:     port,
		engine:   engine,
		upstream: defaultUpstream,
		logger:   logger,
	}
}

func defaultLogPath() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, ".tego", "requests.jsonl")
}

// SetUpstream overrides the upstream URL (useful for testing).
func (s *Server) SetUpstream(url string) {
	s.upstream = url
}

// Start begins listening and serving.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handlePassthrough)
	mux.HandleFunc("POST /v1/messages", s.handleMessages)

	addr := fmt.Sprintf(":%d", s.Port)
	slog.Info(fmt.Sprintf("tego proxy listening on %s → api.anthropic.com", addr))

	return http.ListenAndServe(addr, mux)
}
