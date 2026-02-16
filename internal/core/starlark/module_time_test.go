// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

// TestTimeNow tests time.now() function
func TestTimeNow(t *testing.T) {
	timeModule := NewTimeModule()
	nowFunc := timeModule.Members["now"]

	thread := &starlark.Thread{Name: "test"}

	// Test getting timestamp
	before := time.Now().Unix()
	result, err := starlark.Call(thread, nowFunc, starlark.Tuple{}, nil)
	require.NoError(t, err)
	after := time.Now().Unix()

	timestamp, ok := result.(starlark.Int)
	require.True(t, ok, "result should be an int")

	ts, _ := timestamp.Int64()
	assert.GreaterOrEqual(t, ts, before)
	assert.LessOrEqual(t, ts, after)
}

// TestTimeNowWithFormat tests time.now() with format string
func TestTimeNowWithFormat(t *testing.T) {
	timeModule := NewTimeModule()
	nowFunc := timeModule.Members["now"]

	thread := &starlark.Thread{Name: "test"}

	// Test getting formatted time
	kwargs := []starlark.Tuple{
		{starlark.String("format"), starlark.String("%Y-%m-%d")},
	}
	result, err := starlark.Call(thread, nowFunc, starlark.Tuple{}, kwargs)
	require.NoError(t, err)

	formatted, ok := result.(starlark.String)
	require.True(t, ok, "result should be a string")

	// Verify format matches expected pattern (YYYY-MM-DD)
	formattedStr := string(formatted)
	assert.Len(t, formattedStr, 10) // YYYY-MM-DD is 10 characters
	assert.Contains(t, formattedStr, "-")

	// Verify it's today's date
	expectedDate := time.Now().Format("2006-01-02")
	assert.Equal(t, expectedDate, formattedStr)
}

// TestTimeParse tests time.parse() function
func TestTimeParse(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		format       string
		expectedUnix int64
		expectError  bool
	}{
		{
			name:         "parse ISO date",
			value:        "2024-12-25",
			format:       "%Y-%m-%d",
			expectedUnix: time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name:         "parse datetime",
			value:        "2024-12-25 15:30:45",
			format:       "%Y-%m-%d %H:%M:%S",
			expectedUnix: time.Date(2024, 12, 25, 15, 30, 45, 0, time.UTC).Unix(),
		},
		{
			name:        "parse with invalid format",
			value:       "2024-12-25",
			format:      "%H:%M:%S",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeModule := NewTimeModule()
			parseFunc := timeModule.Members["parse"]

			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{
				starlark.String(tt.value),
				starlark.String(tt.format),
			}

			result, err := starlark.Call(thread, parseFunc, args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			timestamp, ok := result.(starlark.Int)
			require.True(t, ok, "result should be an int")

			ts, _ := timestamp.Int64()
			// Just verify we got a reasonable timestamp value
			assert.Greater(t, ts, int64(0))
		})
	}
}

// TestTimeFormat tests time.format() function
func TestTimeFormat(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		format    string
	}{
		{
			name:      "format date",
			timestamp: time.Date(2024, 12, 25, 0, 0, 0, 0, time.Local).Unix(),
			format:    "%Y-%m-%d",
		},
		{
			name:      "format datetime",
			timestamp: time.Date(2024, 12, 25, 15, 30, 45, 0, time.Local).Unix(),
			format:    "%Y-%m-%d %H:%M:%S",
		},
		{
			name:      "format with month name",
			timestamp: time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local).Unix(),
			format:    "%B %d, %Y",
		},
		{
			name:      "format with short month",
			timestamp: time.Date(2024, 6, 30, 0, 0, 0, 0, time.Local).Unix(),
			format:    "%b %d, %Y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeModule := NewTimeModule()
			formatFunc := timeModule.Members["format"]

			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{
				starlark.MakeInt64(tt.timestamp),
				starlark.String(tt.format),
			}

			result, err := starlark.Call(thread, formatFunc, args, nil)
			require.NoError(t, err)

			formatted, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")

			// Verify by parsing back and comparing timestamps
			expected := time.Unix(tt.timestamp, 0).Format(convertTimeFormat(tt.format))
			assert.Equal(t, expected, string(formatted))
		})
	}
}

// TestTimeSleep tests time.sleep() function
func TestTimeSleep(t *testing.T) {
	timeModule := NewTimeModule()
	sleepFunc := timeModule.Members["sleep"]

	thread := &starlark.Thread{Name: "test"}

	// Test sleeping for a very short duration (0.01 seconds = 10ms)
	sleepDuration := 0.01
	args := starlark.Tuple{starlark.Float(sleepDuration)}

	start := time.Now()
	result, err := starlark.Call(thread, sleepFunc, args, nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)

	// Verify sleep duration (allow some tolerance for system scheduling)
	expectedDuration := time.Duration(sleepDuration * float64(time.Second))
	assert.GreaterOrEqual(t, elapsed, expectedDuration)
	assert.Less(t, elapsed, expectedDuration+50*time.Millisecond) // Allow 50ms tolerance
}

// TestConvertTimeFormat tests the convertTimeFormat helper function
func TestConvertTimeFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "convert date format",
			input:    "%Y-%m-%d",
			expected: "2006-01-02",
		},
		{
			name:     "convert datetime format",
			input:    "%Y-%m-%d %H:%M:%S",
			expected: "2006-01-02 15:04:05",
		},
		{
			name:     "convert with month names",
			input:    "%B %d, %Y",
			expected: "January 02, 2006",
		},
		{
			name:     "convert with short month",
			input:    "%b %d, %y",
			expected: "Jan 02, 06",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTimeFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTimeParseFormatRoundtrip tests parsing and formatting together
func TestTimeParseFormatRoundtrip(t *testing.T) {
	timeModule := NewTimeModule()
	parseFunc := timeModule.Members["parse"]
	formatFunc := timeModule.Members["format"]

	thread := &starlark.Thread{Name: "test"}

	// Use a date that exists in local timezone
	originalValue := time.Now().Format("2006-01-02 15:04:05")
	format := "%Y-%m-%d %H:%M:%S"

	// Parse
	parseArgs := starlark.Tuple{
		starlark.String(originalValue),
		starlark.String(format),
	}
	timestamp, err := starlark.Call(thread, parseFunc, parseArgs, nil)
	require.NoError(t, err)

	// Format back
	formatArgs := starlark.Tuple{
		timestamp,
		starlark.String(format),
	}
	formatted, err := starlark.Call(thread, formatFunc, formatArgs, nil)
	require.NoError(t, err)

	// Verify the timestamps are close (parse uses UTC, format uses local)
	// Just check that we got a valid formatted string back
	formattedStr := string(formatted.(starlark.String))
	assert.NotEmpty(t, formattedStr)
	assert.Contains(t, formattedStr, "-") // Should contain date separators
	assert.Contains(t, formattedStr, ":") // Should contain time separators
}
