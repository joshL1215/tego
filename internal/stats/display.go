package stats

import "fmt"

// Display prints cumulative stats to stdout.
func Display(t *Tracker) {
	t.mu.Lock()
	defer t.mu.Unlock()

	fmt.Println("tego — cumulative token savings")
	fmt.Println("────────────────────────────────")
	fmt.Printf("Total requests:    %d\n", t.TotalRequests)
	fmt.Printf("Filtered requests: %d\n", t.FilteredRequests)
	fmt.Printf("Original bytes:    %d\n", t.OriginalBytes)
	fmt.Printf("Filtered bytes:    %d\n", t.FilteredBytes)
	fmt.Printf("Bytes saved:       %d\n", t.OriginalBytes-t.FilteredBytes)
	if t.OriginalBytes > 0 {
		pct := float64(t.OriginalBytes-t.FilteredBytes) / float64(t.OriginalBytes) * 100
		fmt.Printf("Savings:           %.1f%%\n", pct)
	}
}
