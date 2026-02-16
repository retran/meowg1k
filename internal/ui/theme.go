// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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
	// Base colors
	Text       lipgloss.Color
	Muted      lipgloss.Color
	Accent     lipgloss.Color
	Info       lipgloss.Color
	Success    lipgloss.Color
	Warn       lipgloss.Color
	Error      lipgloss.Color
	Border     lipgloss.Color
	Highlight  lipgloss.Color
	Surface1   lipgloss.Color // Progress bar empty blocks
	Surface2   lipgloss.Color // Subtle UI elements
	DiffAdd    lipgloss.Color
	DiffDel    lipgloss.Color
	DiffHunk   lipgloss.Color
	DiffHeader lipgloss.Color

	// Flux Terminal semantic colors
	System      lipgloss.Color // SlateGray - system/infrastructure messages
	Agent       lipgloss.Color // Magenta - AI/LLM operations
	Action      lipgloss.Color // Cyan - tool calls, external actions
	Thought     lipgloss.Color // Dimmed gray - agent reasoning
	Spinner     lipgloss.Color // Teal - activity spinner
	InputPrompt lipgloss.Color // White bold - user input prompts

	// Base styles
	StatusSuccess   lipgloss.Style
	StatusError     lipgloss.Style
	StatusWarn      lipgloss.Style
	StatusInfo      lipgloss.Style
	PanelStyle      lipgloss.Style
	PanelTitleStyle lipgloss.Style
	TableHeader     lipgloss.Style
	TableRow        lipgloss.Style
	TableAltRow     lipgloss.Style
	TableBorder     lipgloss.Style
	SelectCursor    lipgloss.Style
	SelectMatch     lipgloss.Style
	SelectHint      lipgloss.Style
	SelectInput     lipgloss.Style
	SelectSelected  lipgloss.Style
	SelectPreview   lipgloss.Style

	// Flux Terminal semantic styles
	SystemStyle  lipgloss.Style // For system-level messages
	AgentStyle   lipgloss.Style // For AI/agent thoughts
	ActionStyle  lipgloss.Style // For tool/action calls
	ThoughtStyle lipgloss.Style // For reasoning/thinking (dimmed, italic)

	// Step context styles
	StepBorder  lipgloss.Style // Border for step containers
	StepTitle   lipgloss.Style // Step titles
	StepSuccess lipgloss.Style // Step completion (success)
	StepError   lipgloss.Style // Step completion (error)
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

	// Base status styles - no bold for cleaner appearance
	theme.StatusSuccess = lipgloss.NewStyle().Foreground(theme.Success)
	theme.StatusError = lipgloss.NewStyle().Foreground(theme.Error)
	theme.StatusWarn = lipgloss.NewStyle().Foreground(theme.Warn)
	theme.StatusInfo = lipgloss.NewStyle().Foreground(theme.Info)

	// Flux Terminal semantic styles
	theme.SystemStyle = lipgloss.NewStyle().Foreground(theme.System)
	theme.AgentStyle = lipgloss.NewStyle().Foreground(theme.Agent)
	theme.ActionStyle = lipgloss.NewStyle().Foreground(theme.Action)
	theme.ThoughtStyle = lipgloss.NewStyle().Foreground(theme.Thought)

	// Step context styles - calm, structural, not status
	theme.StepBorder = lipgloss.NewStyle().Foreground(theme.Surface2)
	theme.StepTitle = lipgloss.NewStyle().Foreground(theme.Text)
	theme.StepSuccess = lipgloss.NewStyle().Foreground(theme.Success)
	theme.StepError = lipgloss.NewStyle().Foreground(theme.Error)

	// Choose border style based on Unicode support
	var border lipgloss.Border
	if opts.SupportsUnicode {
		// Use beautiful rounded borders for Unicode terminals
		border = lipgloss.RoundedBorder()
	} else {
		// Fall back to ASCII borders for compatibility
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
