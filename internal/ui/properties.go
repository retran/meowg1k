// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"
)

// RenderProperties renders a key-value list with aligned keys.
func RenderProperties(data map[string]string, title string, theme Theme, opts RenderOptions) string {
	if len(data) == 0 {
		return ""
	}

	if opts.Plain || !opts.Terminal {
		var lines []string
		if title != "" {
			lines = append(lines, title)
		}
		for key, value := range data {
			lines = append(lines, fmt.Sprintf("%s: %s", key, value))
		}
		return strings.Join(lines, "\n")
	}

	maxKeyLen := 0
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}
	}

	var lines []string

	if title != "" {
		titleStyle := theme.PanelTitleStyle
		lines = append(lines, titleStyle.Render(title))
	}

	keyStyle := theme.Muted
	valueStyle := theme.Text

	for _, key := range keys {
		value := data[key]

		paddedKey := key + strings.Repeat(" ", maxKeyLen-len(key))

		keyPart := theme.StatusInfo.Foreground(keyStyle).Render(paddedKey)
		valuePart := theme.StatusSuccess.Foreground(valueStyle).Render(value)

		line := fmt.Sprintf("%s:  %s", keyPart, valuePart)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
