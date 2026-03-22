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
			logDiff(tr.OriginalText, tr.FilteredText)
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

const (
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	dim    = "\x1b[2m"
	reset  = "\x1b[0m"
)

// logDiff prints a compact diff showing removed and added lines between
// original and filtered text. Lines present only in the original are shown
// with a red "−" prefix; lines only in the filtered output (e.g. tego
// summary lines) are shown with a green "+" prefix.
func logDiff(original, filtered string) {
	origLines := strings.Split(original, "\n")
	filtLines := strings.Split(filtered, "\n")

	// Build a set of filtered lines for quick lookup
	filtSet := make(map[string]int, len(filtLines))
	for _, l := range filtLines {
		filtSet[l]++
	}
	origSet := make(map[string]int, len(origLines))
	for _, l := range origLines {
		origSet[l]++
	}

	// Show removed lines (in original but not filtered)
	removed := 0
	for _, l := range origLines {
		if filtSet[l] > 0 {
			filtSet[l]--
			continue
		}
		if removed == 0 {
			fmt.Printf("             %s┌─ diff ─────────────────────────%s\n", dim, reset)
		}
		// Truncate long lines for readability
		display := l
		if len(display) > 120 {
			display = display[:117] + "..."
		}
		fmt.Printf("             %s│ %s− %s%s\n", dim, red, display, reset)
		removed++
	}

	// Show added lines (in filtered but not original) — these are tego summary lines
	// Rebuild origSet for this pass
	for k := range origSet {
		delete(origSet, k)
	}
	for _, l := range origLines {
		origSet[l]++
	}
	for _, l := range filtLines {
		if origSet[l] > 0 {
			origSet[l]--
			continue
		}
		if removed == 0 {
			fmt.Printf("             %s┌─ diff ─────────────────────────%s\n", dim, reset)
		}
		display := l
		if len(display) > 120 {
			display = display[:117] + "..."
		}
		fmt.Printf("             %s│ %s+ %s%s\n", dim, green, display, reset)
		removed++
	}

	if removed > 0 {
		fmt.Printf("             %s└────────────────────────────────%s\n", dim, reset)
	}
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
