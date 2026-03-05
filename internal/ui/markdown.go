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

// markdownWrapWidth is the maximum line width used when word-wrapping rendered
// Markdown output.
const markdownWrapWidth = 120

// RenderMarkdown renders Markdown content to a terminal-friendly string.
func RenderMarkdown(content string, width int, noColor bool) (string, error) {
	if width <= 0 {
		width = 80
	}
	if width > markdownWrapWidth {
		width = markdownWrapWidth
	}

	key := rendererKey{width: width, noColor: noColor}

	rendererMu.Lock()
	renderer := rendererCache[key]
	rendererMu.Unlock()

	if renderer == nil {
		var err error

		// Select the base style based on terminal color capabilities so that
		// syntax highlighting and colors are preserved in the rendered output.
		var styleConfig ansi.StyleConfig
		if noColor {
			styleConfig = styles.NoTTYStyleConfig
		} else if lipgloss.HasDarkBackground() {
			styleConfig = styles.DarkStyleConfig
		} else {
			styleConfig = styles.LightStyleConfig
		}

		// Zero out all margins for clean, copy-paste-friendly output while
		// keeping the default indentation intact.
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
