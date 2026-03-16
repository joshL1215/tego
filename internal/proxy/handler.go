package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/joshL1215/tego/internal/message"
)

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Read the full request body
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	originalSize := len(body)

	// Filter tool_result content
	filtered, results, err := message.FilterToolResults(body, s.engine)
	if err != nil {
		slog.Warn("failed to filter request, forwarding unmodified", "error", err)
		filtered = body
	}

	filteredSize := len(filtered)

	// Build the upstream request
	upstreamURL := s.upstream + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, strings.NewReader(string(filtered)))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}

	// Copy all headers
	for key, values := range r.Header {
		for _, v := range values {
			upReq.Header.Add(key, v)
		}
	}
	// Update Content-Length
	upReq.Header.Set("Content-Length", strconv.Itoa(len(filtered)))
	upReq.ContentLength = int64(len(filtered))

	// Forward to upstream
	client := &http.Client{
		// No timeout — streaming responses can be long-lived
		Timeout: 0,
	}
	resp, err := client.Do(upReq)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response back — flush as data arrives for SSE
	if flusher, ok := w.(http.Flusher); ok {
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				flusher.Flush()
			}
			if readErr != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}

	elapsed := time.Since(start)

	// Write to log file
	if s.logger != nil {
		s.logger.Log(r.URL.Path, originalSize, filteredSize, elapsed, results)
	}

	// Log
	if len(results) > 0 {
		savings := 0
		if originalSize > 0 {
			savings = 100 - (filteredSize*100)/originalSize
		}
		slog.Info(fmt.Sprintf("POST /v1/messages │ %d tool_results filtered │ %s → %s bytes (−%d%%) │ %s",
			len(results),
			formatNumber(originalSize),
			formatNumber(filteredSize),
			savings,
			formatDuration(elapsed),
		))
		for i, tr := range results {
			connector := "├─"
			if i == len(results)-1 {
				connector = "└─"
			}
			trSavings := 0
			if tr.OriginalBytes > 0 {
				trSavings = 100 - (tr.FilteredBytes*100)/tr.OriginalBytes
			}
			slog.Info(fmt.Sprintf("             %s tool_result[%d]: %s → %s bytes (−%d%%) │ stripped: %s",
				connector,
				tr.Index,
				formatNumber(tr.OriginalBytes),
				formatNumber(tr.FilteredBytes),
				trSavings,
				tr.Stats.Summary(),
			))
		}
	} else {
		slog.Info(fmt.Sprintf("POST /v1/messages │ no tool_results to filter │ %s", formatDuration(elapsed)))
	}
}

func (s *Server) handlePassthrough(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Build upstream URL
	upstreamURL := s.upstream + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, v := range values {
			upReq.Header.Add(key, v)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(upReq)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	elapsed := time.Since(start)
	slog.Info(fmt.Sprintf("%s %s │ passthrough │ %s", r.Method, r.URL.Path, formatDuration(elapsed)))
}

func formatNumber(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
