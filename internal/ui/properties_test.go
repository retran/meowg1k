// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
	"testing"
)

func TestRenderProperties_Basic(t *testing.T) {
	data := map[string]string{
		"Model":   "gpt-4",
		"Tokens":  "1500",
		"Cost":    "$0.03",
		"Latency": "2.5s",
	}

	theme := DefaultTheme()
	opts := RenderOptions{
		Terminal:        true,
		SupportsUnicode: true,
	}

	result := RenderProperties(data, "API Call Info", theme, opts)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	if !strings.Contains(result, "API Call Info") {
		t.Error("Expected title in output")
	}

	if !strings.Contains(result, "Model") {
		t.Error("Expected 'Model' key in output")
	}

	if !strings.Contains(result, "gpt-4") {
		t.Error("Expected 'gpt-4' value in output")
	}
}

func TestRenderProperties_Plain(t *testing.T) {
	data := map[string]string{
		"Status":  "OK",
		"Version": "1.0",
	}

	theme := DefaultTheme()
	opts := RenderOptions{
		Plain:    true,
		Terminal: false,
	}

	result := RenderProperties(data, "Config", theme, opts)

	// Plain mode should be simple
	if strings.Contains(result, "\x1b[") {
		t.Error("Plain mode should not contain ANSI codes")
	}

	if !strings.Contains(result, "Status: OK") {
		t.Error("Expected plain key: value format")
	}
}

func TestRenderProperties_Empty(t *testing.T) {
	data := map[string]string{}
	theme := DefaultTheme()
	opts := RenderOptions{Terminal: true}

	result := RenderProperties(data, "", theme, opts)

	if result != "" {
		t.Error("Expected empty string for empty data")
	}
}

func TestRenderProperties_Alignment(t *testing.T) {
	data := map[string]string{
		"A":       "value1",
		"LongKey": "value2",
		"B":       "value3",
	}

	theme := DefaultTheme()
	opts := RenderOptions{
		Plain:    true,
		Terminal: false,
	}

	result := RenderProperties(data, "", theme, opts)
	lines := strings.Split(result, "\n")

	// In plain mode, keys should still be aligned (with spaces)
	// This is a basic check - actual alignment depends on implementation
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
}
