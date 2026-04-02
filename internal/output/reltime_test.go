package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRelativeTime_Public(t *testing.T) {
	// Test that RelativeTime (the public function) returns a non-empty string
	// for a recent time. We can't test the exact value since it depends on
	// the current time, but we can verify it runs without error.
	recent := time.Now().Add(-5 * time.Minute)
	result := RelativeTime(recent)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "m ago")

	// Test with a very old date
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	result = RelativeTime(old)
	assert.NotEmpty(t, result)
	assert.Equal(t, "2020-01-01", result)
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 4, 2, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "just now",
			input:    now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "minutes ago",
			input:    now.Add(-15 * time.Minute),
			expected: "15m ago",
		},
		{
			name:     "hours ago",
			input:    now.Add(-3 * time.Hour),
			expected: "3h ago",
		},
		{
			name:     "yesterday with day name",
			input:    now.Add(-30 * time.Hour),
			expected: "Wed 09:30",
		},
		{
			name:     "this year",
			input:    time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC),
			expected: "Feb 14",
		},
		{
			name:     "last year",
			input:    time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC),
			expected: "2025-12-25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relativeTimeFrom(tt.input, now)
			assert.Equal(t, tt.expected, result)
		})
	}
}
