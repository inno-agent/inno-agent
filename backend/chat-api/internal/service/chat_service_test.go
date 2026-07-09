package service

import (
	"testing"
)

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"trim spaces", "  hello  ", "hello"},
		{"trim newlines", "hello\nworld", "hello"},
		{"trim quotes", `"hello"`, "hello"},
		{"trim backticks", "`hello`", "hello"},
		{"trim dots", "hello.", "hello"},
		{"trim asterisks", "*hello*", "hello"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"only quotes", `""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanTitle(tt.input)
			if result != tt.expected {
				t.Errorf("cleanTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello"},
		{"empty string", "", 5, ""},
		{"multibyte", "Привет мир", 6, "Привет"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}
