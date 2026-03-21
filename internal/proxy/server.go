package proxy

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/joshL1215/tego/internal/filter"
	"github.com/joshL1215/tego/internal/store"
)

const defaultUpstream = "https://api.anthropic.com"

// Server is the tego proxy server.
type Server struct {
	Port     int
	engine   *filter.Engine
	upstream string
}

// NewServer creates a new proxy server. If a store is provided, it is attached
// to the filter engine for deduplication.
func NewServer(port int, engine *filter.Engine, s *store.Store) *Server {
	if s != nil {
		engine.SetStore(s)
	}
	return &Server{
		Port:     port,
		engine:   engine,
		upstream: defaultUpstream,
	}
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
