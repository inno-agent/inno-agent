package tokensource

import (
	"testing"
	"time"
)

func TestFreshnessThresholdExceedsLongestRun(t *testing.T) {
	// The agentic codegen run is bounded at 900s (agent) / 1000s (Go client), so
	// the freshness threshold must exceed that or the delegated token can expire
	// mid-run. Kept above the Go client timeout with margin.
	if freshnessThreshold < 20*time.Minute {
		t.Errorf("freshnessThreshold = %v, want >= 20m (must exceed the ~1000s run budget)", freshnessThreshold)
	}
}
