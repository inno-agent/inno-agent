package tokensource

import (
	"testing"
	"time"
)

func TestFreshnessThresholdExceedsLongestRun(t *testing.T) {
	if freshnessThreshold < 10*time.Minute {
		t.Errorf("freshnessThreshold = %v, want >= 10m", freshnessThreshold)
	}
}
