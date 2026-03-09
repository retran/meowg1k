// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package ui provides terminal UI components including themes, widgets, and rendering
// utilities for Bubble Tea-based interactive output.
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// RenderOptions controls how widgets are rendered.
type RenderOptions struct {
	Plain           bool // No borders, no colors, no padding
	NoColor         bool // No ANSI colors
	NoEmoji         bool // Use text instead of emoji
	NoIndent        bool // Skip indentation
	Terminal        bool // Output is going to a terminal
	NoBorders       bool // Skip borders even in terminal mode
	SupportsUnicode bool // Terminal supports Unicode characters
}

// NewRenderOptions creates RenderOptions with auto-detected terminal status.
func NewRenderOptions() RenderOptions {
	return RenderOptions{
		Terminal:        IsTerminal(os.Stdout.Fd()),
		SupportsUnicode: SupportsUnicode(),
		Plain:           false,
		NoColor:         false,
		NoEmoji:         false,
		NoIndent:        false,
		NoBorders:       false,
	}
}

// Theme defines colors and styles used by the UI layer.
type Theme struct {
	SelectHint      lipgloss.Style
	SelectSelected  lipgloss.Style
	StepError       lipgloss.Style
	StepSuccess     lipgloss.Style
	StepTitle       lipgloss.Style
	StepBorder      lipgloss.Style
	ThoughtStyle    lipgloss.Style
	ActionStyle     lipgloss.Style
	AgentStyle      lipgloss.Style
	SystemStyle     lipgloss.Style
	SelectPreview   lipgloss.Style
	PanelStyle      lipgloss.Style
	StatusError     lipgloss.Style
	StatusInfo      lipgloss.Style
	SelectInput     lipgloss.Style
	StatusWarn      lipgloss.Style
	SelectMatch     lipgloss.Style
	SelectCursor    lipgloss.Style
	TableBorder     lipgloss.Style
	TableAltRow     lipgloss.Style
	TableRow        lipgloss.Style
	StatusSuccess   lipgloss.Style
	TableHeader     lipgloss.Style
	PanelTitleStyle lipgloss.Style
	DiffDel         lipgloss.Color
	DiffHunk        lipgloss.Color
	System          lipgloss.Color
	Muted           lipgloss.Color
	InputPrompt     lipgloss.Color
	Spinner         lipgloss.Color
	Thought         lipgloss.Color
	Action          lipgloss.Color
	Agent           lipgloss.Color
	Text            lipgloss.Color
	DiffHeader      lipgloss.Color
	DiffAdd         lipgloss.Color
	Surface2        lipgloss.Color
	Surface1        lipgloss.Color
	Highlight       lipgloss.Color
	Border          lipgloss.Color
	Error           lipgloss.Color
	Warn            lipgloss.Color
	Success         lipgloss.Color
	Info            lipgloss.Color
	Accent          lipgloss.Color
}

// DefaultTheme returns the default UI theme.
func DefaultTheme() Theme {
	return DefaultThemeWithOptions(NewRenderOptions())
}

// DefaultThemeWithOptions returns a theme configured for the given render options.
func DefaultThemeWithOptions(opts RenderOptions) Theme {
	theme := Theme{
		// Base colors - Catppuccin Mocha palette
		Text:       lipgloss.Color("#CDD6F4"), // Catppuccin Text
		Muted:      lipgloss.Color("#9399B2"), // Catppuccin Overlay1
		Accent:     lipgloss.Color("#F5C2E7"), // Catppuccin Pink
		Info:       lipgloss.Color("#89B4FA"), // Catppuccin Blue
		Success:    lipgloss.Color("#A6E3A1"), // Catppuccin Green
		Warn:       lipgloss.Color("#F9E2AF"), // Catppuccin Yellow
		Error:      lipgloss.Color("#F38BA8"), // Catppuccin Red
		Border:     lipgloss.Color("#6C7086"), // Catppuccin Overlay0
		Highlight:  lipgloss.Color("#FAB387"), // Catppuccin Peach
		Surface1:   lipgloss.Color("#45475A"), // Catppuccin Surface1
		Surface2:   lipgloss.Color("#585B70"), // Catppuccin Surface2
		DiffAdd:    lipgloss.Color("#A6E3A1"), // Catppuccin Green
		DiffDel:    lipgloss.Color("#F38BA8"), // Catppuccin Red
		DiffHunk:   lipgloss.Color("#89DCEB"), // Catppuccin Sky
		DiffHeader: lipgloss.Color("#B4BEFE"), // Catppuccin Lavender

		// Flux Terminal semantic colors - Catppuccin themed
		System:      lipgloss.Color("#6C7086"), // Catppuccin Overlay0 - system messages
		Agent:       lipgloss.Color("#CBA6F7"), // Catppuccin Mauve - AI/agent identity
		Action:      lipgloss.Color("#CBA6F7"), // Catppuccin Mauve - tool/API calls
		Thought:     lipgloss.Color("#A6ADC8"), // Catppuccin Subtext0 - quiet, skippable
		Spinner:     lipgloss.Color("#94E2D5"), // Catppuccin Teal - activity spinner
		InputPrompt: lipgloss.Color("#F5C2E7"), // Catppuccin Pink - user prompts
	}

	// Status styles omit bold for a cleaner appearance.
	theme.StatusSuccess = lipgloss.NewStyle().Foreground(theme.Success)
	theme.StatusError = lipgloss.NewStyle().Foreground(theme.Error)
	theme.StatusWarn = lipgloss.NewStyle().Foreground(theme.Warn)
	theme.StatusInfo = lipgloss.NewStyle().Foreground(theme.Info)

	theme.SystemStyle = lipgloss.NewStyle().Foreground(theme.System)
	theme.AgentStyle = lipgloss.NewStyle().Foreground(theme.Agent)
	theme.ActionStyle = lipgloss.NewStyle().Foreground(theme.Action)
	theme.ThoughtStyle = lipgloss.NewStyle().Foreground(theme.Thought)

	// Step styles are calm and structural; success/error states use color only, no bold.
	theme.StepBorder = lipgloss.NewStyle().Foreground(theme.Surface2)
	theme.StepTitle = lipgloss.NewStyle().Foreground(theme.Text)
	theme.StepSuccess = lipgloss.NewStyle().Foreground(theme.Success)
	theme.StepError = lipgloss.NewStyle().Foreground(theme.Error)

	// Use rounded Unicode borders when the terminal supports them; fall back to ASCII.
	var border lipgloss.Border
	if opts.SupportsUnicode {
		border = lipgloss.RoundedBorder()
	} else {
		border = lipgloss.ASCIIBorder()
	}

	theme.PanelStyle = lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.Border).
		Padding(0, 1)
	theme.PanelTitleStyle = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	theme.TableHeader = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	theme.TableRow = lipgloss.NewStyle().Foreground(theme.Text)
	theme.TableAltRow = lipgloss.NewStyle().Foreground(theme.Text).Faint(true)
	theme.TableBorder = lipgloss.NewStyle().Foreground(theme.Border)
	theme.SelectCursor = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	theme.SelectMatch = lipgloss.NewStyle().Foreground(theme.Highlight).Bold(true)
	theme.SelectHint = lipgloss.NewStyle().Foreground(theme.Muted)
	theme.SelectInput = lipgloss.NewStyle().Foreground(theme.Text)
	theme.SelectSelected = lipgloss.NewStyle().Foreground(theme.Success).Bold(true)
	theme.SelectPreview = lipgloss.NewStyle().Foreground(theme.Muted)

	return theme
}
