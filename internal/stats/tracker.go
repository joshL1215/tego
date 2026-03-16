package stats

import "sync"

// Tracker accumulates token/byte savings across requests.
type Tracker struct {
	mu             sync.Mutex
	TotalRequests  int
	FilteredRequests int
	OriginalBytes  int64
	FilteredBytes  int64
}

// Global tracker instance.
var Global = &Tracker{}

// Record adds a request's stats to the tracker.
func (t *Tracker) Record(original, filtered int, wasFiltered bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalRequests++
	t.OriginalBytes += int64(original)
	t.FilteredBytes += int64(filtered)
	if wasFiltered {
		t.FilteredRequests++
	}
}

// Savings returns the percentage of bytes saved.
func (t *Tracker) Savings() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.OriginalBytes == 0 {
		return 0
	}
	return float64(t.OriginalBytes-t.FilteredBytes) / float64(t.OriginalBytes) * 100
}
