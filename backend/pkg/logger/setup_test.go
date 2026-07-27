package logger

import (
	"testing"
)

func TestNewReturnsNonNilLogger(t *testing.T) {
	log := New("test-service")
	if log == nil {
		t.Error("New() returned nil logger")
	}
}
