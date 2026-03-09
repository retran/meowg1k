// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pagerModel is a Bubble Tea model that wraps bubbles/viewport for paged display.
type pagerModel struct {
	theme       Theme
	content     string
	title       string
	viewport    viewport.Model
	lineNumbers bool
	ready       bool
}

func (m pagerModel) Init() tea.Cmd { //nolint:gocritic // hugeParam: Bubble Tea requires value receiver for model
	return nil
}

func (m pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocritic // hugeParam: Bubble Tea requires value receiver for model
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 0
		if m.title != "" {
			headerHeight = 1
		}
		footerHeight := 1
		verticalMargin := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargin)
			m.viewport.SetContent(m.buildContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargin
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m pagerModel) View() string { //nolint:gocritic // hugeParam: Bubble Tea requires value receiver for model
	if !m.ready {
		return "\n  Loading..."
	}

	var b strings.Builder

	if m.title != "" {
		titleStyle := lipgloss.NewStyle().Bold(true)
		b.WriteString(titleStyle.Render("=== "+m.title+" ===") + "\n")
	}

	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	percent := int(m.viewport.ScrollPercent() * 100)
	footerStyle := lipgloss.NewStyle().Faint(true)
	b.WriteString(footerStyle.Render(fmt.Sprintf("  %d%%  ↑/↓ scroll  q quit", percent)))

	return b.String()
}

func (m pagerModel) buildContent() string { //nolint:gocritic // hugeParam: Bubble Tea requires value receiver for model
	lines := strings.Split(m.content, "\n")
	if !m.lineNumbers {
		return m.content
	}
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%4d  %s\n", i+1, line)
	}
	return b.String()
}

// RenderWithPager displays content in an interactive viewport pager if the
// content is long enough and the output is a terminal. Falls back to direct
// output otherwise.
func RenderWithPager(content, title string, lineNumbers bool, opts RenderOptions) error {
	lines := strings.Split(content, "\n")

	if opts.Plain || !opts.Terminal || len(lines) <= 30 {
		if title != "" {
			fmt.Fprintf(os.Stderr, "=== %s ===\n", title)
		}
		if lineNumbers {
			for i, line := range lines {
				fmt.Fprintf(os.Stderr, "%4d  %s\n", i+1, line)
			}
		} else {
			fmt.Fprintln(os.Stderr, content)
		}
		return nil
	}

	model := pagerModel{
		content:     content,
		title:       title,
		lineNumbers: lineNumbers,
		theme:       DefaultTheme(),
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run pager: %w", err)
	}
	return nil
}

// TruncateContent truncates content to maxLines with an indicator.
func TruncateContent(content string, maxLines int, opts RenderOptions) (string, bool) {
	if maxLines <= 0 {
		return content, false
	}

	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content, false
	}

	truncated := strings.Join(lines[:maxLines], "\n")

	var indicator string
	if opts.SupportsUnicode {
		indicator = fmt.Sprintf("\n⋮ [%d more lines]", len(lines)-maxLines)
	} else {
		indicator = fmt.Sprintf("\n... [%d more lines]", len(lines)-maxLines)
	}

	return truncated + indicator, true
}
