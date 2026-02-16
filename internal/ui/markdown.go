// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	rendererMu    sync.Mutex
	rendererCache = map[rendererKey]*glamour.TermRenderer{}
)

type rendererKey struct {
	width   int
	noColor bool
}

// RenderMarkdown renders Markdown content to a terminal-friendly string.
func RenderMarkdown(content string, width int, noColor bool) (string, error) {
	if width <= 0 {
		width = 80
	}

	key := rendererKey{width: width, noColor: noColor}

	rendererMu.Lock()
	renderer := rendererCache[key]
	rendererMu.Unlock()

	if renderer == nil {
		var err error

		// Determine base style based on environment to ensure colors/syntax highlighting are preserved
		var styleConfig ansi.StyleConfig
		if noColor {
			styleConfig = styles.NoTTYStyleConfig
		} else if lipgloss.HasDarkBackground() {
			styleConfig = styles.DarkStyleConfig
		} else {
			styleConfig = styles.LightStyleConfig
		}

		// Apply zero margins for clean output (copy-paste friendly), but keep default indents
		zero := uint(0)
		styleConfig.Document.Margin = &zero
		// styleConfig.Document.Indent = &zero // Keep default indent
		styleConfig.Paragraph.Margin = &zero
		// styleConfig.Paragraph.Indent = &zero // Keep default indent
		styleConfig.CodeBlock.Margin = &zero
		// styleConfig.CodeBlock.Indent = &zero // Keep default indent
		styleConfig.H1.Margin = &zero
		styleConfig.H2.Margin = &zero
		styleConfig.H3.Margin = &zero
		styleConfig.H4.Margin = &zero
		styleConfig.List.Margin = &zero
		// styleConfig.List.Indent = &zero // Keep default indent

		options := []glamour.TermRendererOption{
			glamour.WithWordWrap(width),
			glamour.WithStyles(styleConfig),
		}

		renderer, err = glamour.NewTermRenderer(options...)
		if err != nil {
			return content, err
		}

		rendererMu.Lock()
		rendererCache[key] = renderer
		rendererMu.Unlock()
	}

	return renderer.Render(content)
}
