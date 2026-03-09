// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"
)

// Simple ASCII banner without external dependencies.
// For more elaborate banners, consider adding github.com/common-nighthawk/go-figure

// RenderBanner creates a simple banner with title and optional subtext.
func RenderBanner(title, subtext string, theme Theme, opts RenderOptions) string { //nolint:gocritic // hugeParam: Theme passed by value for immutability
	if opts.Plain || !opts.Terminal {
		result := fmt.Sprintf("=== %s ===\n", title)
		if subtext != "" {
			result += subtext + "\n"
		}
		return result
	}

	var result strings.Builder

	width := len(title) + 4
	if len(subtext) > len(title) {
		width = len(subtext) + 4
	}

	borderChar := "═"
	if !opts.SupportsUnicode {
		borderChar = "="
	}

	border := strings.Repeat(borderChar, width)

	titleStyled := theme.SystemStyle.Bold(true).Render(title)

	result.WriteString(border + "\n")
	result.WriteString(theme.SystemStyle.Render("  ") + titleStyled + "\n")

	if subtext != "" {
		subtextStyled := theme.SystemStyle.Faint(true).Render(subtext)
		result.WriteString(theme.SystemStyle.Render("  ") + subtextStyled + "\n")
	}

	result.WriteString(border)

	return result.String()
}
