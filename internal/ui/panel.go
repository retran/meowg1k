// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderPanel renders a bordered panel with optional title and style.
// In plain mode (pipe output), renders without borders and padding.
func RenderPanel(content, title, style string, theme Theme, opts RenderOptions) string {
	if theme.Text == "" {
		theme = DefaultTheme()
	}

	if opts.Plain || !opts.Terminal || opts.NoBorders {
		if title != "" {
			return title + "\n" + content
		}
		return content
	}

	panelStyle := theme.PanelStyle.Copy()
	color := resolvePanelColor(style, theme)
	if color != "" && !opts.NoColor {
		panelStyle = panelStyle.BorderForeground(color)
	}

	if title != "" {
		titleStyle := theme.PanelTitleStyle
		if opts.NoColor {
			titleStyle = lipgloss.NewStyle()
		}
		titleText := titleStyle.Render(title)
		return titleText + "\n" + panelStyle.Render(content)
	}

	return panelStyle.Render(content)
}

// RenderPanelLegacy maintains backward compatibility - auto-detects terminal.
func RenderPanelLegacy(content, title, style string, theme Theme) string {
	return RenderPanel(content, title, style, theme, NewRenderOptions())
}

func resolvePanelColor(style string, theme Theme) lipgloss.Color {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "green", "success":
		return theme.Success
	case "red", "error":
		return theme.Error
	case "yellow", "warn", "warning":
		return theme.Warn
	case "blue", "info":
		return theme.Info
	case "accent":
		return theme.Accent
	default:
		return theme.Border
	}
}
