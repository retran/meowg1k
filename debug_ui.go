package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	borderChar  = "─"
	topLeft     = "╭"
	topRight    = "╮"
	bottomLeft  = "╰"
	bottomRight = "╯"
)

type Theme struct {
	SystemStyle lipgloss.Style
}

func RenderCodeDebug(content, title string) string {

	lines := strings.Split(content, "\n")
	maxWidth := 0
	for _, line := range lines {
		visibleLen := lipgloss.Width(line)
		if visibleLen > maxWidth {
			maxWidth = visibleLen
		}
	}

	if maxWidth < 40 {
		maxWidth = 40
	}

	theme := Theme{SystemStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("205"))}
	var result strings.Builder

	// Top border
	if title != "" {
		titleStr := fmt.Sprintf(" %s ", title)
		// Recalculate maxWidth if title is longer than content
		if len(titleStr) > maxWidth {
			maxWidth = len(titleStr)
		}

		totalInnerWidth := maxWidth + 2

		leftLen := (totalInnerWidth - len(titleStr)) / 2
		rightLen := totalInnerWidth - len(titleStr) - leftLen

		if leftLen < 0 {
			leftLen = 0
		}
		if rightLen < 0 {
			rightLen = 0
		}

		fmt.Printf("DEBUG: maxWidth=%d, inner=%d, left=%d, right=%d, titleLen=%d\n",
			maxWidth, totalInnerWidth, leftLen, rightLen, len(titleStr))

		result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s%s%s",
			topLeft,
			strings.Repeat(borderChar, leftLen),
			titleStr,
			strings.Repeat(borderChar, rightLen),
			topRight,
		)) + "\n")
	} else {
		result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s",
			topLeft,
			strings.Repeat(borderChar, maxWidth+2),
			topRight,
		)) + "\n")
	}

	for _, line := range lines {
		visibleLen := lipgloss.Width(line)
		padding := maxWidth - visibleLen
		if padding < 0 {
			padding = 0
		}

		result.WriteString(theme.SystemStyle.Render("│ ") + line + strings.Repeat(" ", padding) + theme.SystemStyle.Render(" │") + "\n")
	}

	result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s",
		bottomLeft,
		strings.Repeat(borderChar, maxWidth+2),
		bottomRight,
	)))

	return result.String()
}

func main() {
	content := "This is a test line.\nAnother thicker line."
	title := "Test Box"
	fmt.Println(RenderCodeDebug(content, title))
	fmt.Println()
	fmt.Println(RenderCodeDebug(content, ""))
}
