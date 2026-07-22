package tokensource

import (
	"testing"
	"time"
)

// The delegated token must be re-exchanged while a run may still be in flight.
// issue-consumer's Mastra client waits up to 450s (7.5m) and the review agent
// up to 5m, so a token with, say, 8 minutes left must NOT be served from cache
// or it can expire mid-run.
func TestFreshnessThresholdExceedsLongestRun(t *testing.T) {
	if freshnessThreshold < 10*time.Minute {
		t.Errorf("freshnessThreshold = %v, want >= 10m (must exceed the 7.5m codegen run plus margin)", freshnessThreshold)
	}
}
