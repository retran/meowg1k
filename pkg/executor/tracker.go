// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	feedbackChanSize = 128
	maxDetailsLines  = 15
	spinnerInterval  = 120 * time.Millisecond
)

// BubbleTeaTracker provides a styled TUI for tracking execution progress.
type BubbleTeaTracker struct {
	program    *tea.Program
	model      *bubbleModel
	feedbackCh chan *Feedback
	wg         sync.WaitGroup
	silent     bool
}

type runningActivity struct {
	name       string
	message    string
	lastUpdate time.Time
	status     Status
}

type logEntry struct {
	message string
	details string
	isError bool
}

// bubbleModel is the Bubbletea model for rendering the TUI.
type bubbleModel struct {
	logLines []logEntry
	running  map[string]runningActivity
	mu       *sync.RWMutex
	width    int
	height   int
	spinner  int
}

// Styles for the TUI.
var (
	styleRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	styleFailed  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleDetails = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

var spinnerFrames = []string{"-", "\\", "|", "/"}

// NewBubbleTeaTracker creates a new Bubbletea-based progress tracker.
func NewBubbleTeaTracker(silent bool) *BubbleTeaTracker {
	model := &bubbleModel{
		logLines: make([]logEntry, 0),
		running:  make(map[string]runningActivity),
		mu:       &sync.RWMutex{},
	}

	return &BubbleTeaTracker{
		feedbackCh: make(chan *Feedback, feedbackChanSize),
		silent:     silent,
		model:      model,
	}
}

// Start launches the Bubbletea program.
func (t *BubbleTeaTracker) Start() {
	if t == nil {
		return
	}

	t.wg.Add(1)
	go t.processFeedback()

	if t.silent {
		return
	}

	t.program = tea.NewProgram(t.model)
	t.wg.Go(func() {
		if _, err := t.program.Run(); err != nil {
			_ = err
		}
	})
}

// Stop stops the Bubbletea program.
func (t *BubbleTeaTracker) Stop() {
	if t == nil {
		return
	}

	if t.feedbackCh != nil {
		close(t.feedbackCh)
	}

	if t.program != nil {
		t.program.Quit()
	}

	t.wg.Wait()
}

// FeedbackHandler returns a handler for receiving feedback.
func (t *BubbleTeaTracker) FeedbackHandler() FeedbackHandler {
	return func(feedback *Feedback) {
		if t == nil || t.feedbackCh == nil || feedback == nil {
			return
		}
		select {
		case t.feedbackCh <- feedback:
		default:
		}
	}
}

// processFeedback processes feedback messages and updates the model.
func (t *BubbleTeaTracker) processFeedback() {
	defer t.wg.Done()

	for feedback := range t.feedbackCh {
		if t.program != nil {
			t.program.Send(feedback)
		} else if t.model != nil {
			t.model.handleFeedback(feedback)
		}
	}
}

// GetExecution returns a copy of an execution (for testing).
func (t *BubbleTeaTracker) GetExecution(name string) *Execution {
	if t == nil || t.model == nil {
		return nil
	}

	t.model.mu.RLock()
	defer t.model.mu.RUnlock()

	var exec *Execution

	if run, ok := t.model.running[name]; ok {
		exec = &Execution{
			Name:    run.name,
			Status:  run.status,
			Message: run.message,
		}
	} else {
		for i := len(t.model.logLines) - 1; i >= 0; i-- {
			entry := t.model.logLines[i]
			if entry.message == "" {
				continue
			}
			if entry.message == name {
				status := StatusCompleted
				if entry.isError {
					status = StatusFailed
				}
				exec = &Execution{
					Name:   name,
					Status: status,
					Result: entry.message,
				}
				break
			}
		}
	}

	if exec != nil {
		parts := strings.Split(exec.Name, "::")
		exec.Level = len(parts) - 1
		if len(parts) > 1 {
			exec.ParentName = strings.Join(parts[:len(parts)-1], "::")
		}
	}

	return exec
}

// GetExecutionCount returns the number of tracked executions (for testing).
func (t *BubbleTeaTracker) GetExecutionCount() int {
	if t == nil || t.model == nil {
		return 0
	}

	t.model.mu.RLock()
	defer t.model.mu.RUnlock()
	return len(t.model.running) + len(t.model.logLines)
}

type spinnerTickMsg struct{}

func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (m *bubbleModel) Init() tea.Cmd {
	return spinnerTick()
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case *Feedback:
		m.handleFeedback(msg)
		return m, nil
	case spinnerTickMsg:
		if len(m.running) > 0 {
			m.spinner = (m.spinner + 1) % len(spinnerFrames)
		}
		return m, spinnerTick()
	}

	return m, nil
}

func (m *bubbleModel) handleFeedback(msg *Feedback) {
	if m == nil || msg == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update running activities
	if msg.Status == StatusRunning {
		run := m.running[msg.ActivityName]
		run.name = msg.ActivityName
		run.message = strings.TrimSpace(msg.Message)
		run.status = msg.Status
		run.lastUpdate = msg.Timestamp
		m.running[msg.ActivityName] = run
	}

	// Remove completed/failed activities from running
	if msg.Status == StatusCompleted || msg.Status == StatusFailed {
		delete(m.running, msg.ActivityName)
	}

	// Append to log entries
	entry := logEntry{
		message: strings.TrimSpace(msg.Message),
		details: strings.TrimSpace(msg.Details),
		isError: msg.Status == StatusFailed,
	}
	m.logLines = append(m.logLines, entry)
}

func (m *bubbleModel) View() string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	for _, entry := range m.logLines {
		line := m.renderLogEntry(entry)
		if line == "" {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(m.running) > 0 {
		run := m.currentRunningLocked()
		if strings.TrimSpace(run.message) != "" {
			sb.WriteString("\n")
			sb.WriteString(m.renderRunningLine(run))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m *bubbleModel) currentRunningLocked() runningActivity {
	var current runningActivity
	for _, run := range m.running {
		if current.name == "" || run.lastUpdate.After(current.lastUpdate) {
			current = run
		}
	}
	return current
}

func (m *bubbleModel) renderLogEntry(entry logEntry) string {
	message := strings.TrimSpace(entry.message)
	details := strings.TrimSpace(entry.details)
	if message == "" && details == "" {
		return ""
	}

	var sb strings.Builder

	if message != "" {
		available := m.width
		lines := wrapText(message, available)
		if len(lines) == 0 {
			lines = []string{message}
		}

		for i, line := range lines {
			if i > 0 {
				sb.WriteString("\n")
			}
			rendered := line
			if entry.isError {
				rendered = styleFailed.Render(line)
			}
			if i == 0 {
				sb.WriteString(rendered)
				continue
			}
			sb.WriteString(rendered)
		}
	}

	if details != "" {
		sb.WriteString("\n")
		sb.WriteString(styleDetails.Render(formatDetailsBlock(details)))
	}
	return sb.String()
}

func (m *bubbleModel) renderRunningLine(run runningActivity) string {
	spinner := "-"
	if len(spinnerFrames) > 0 {
		spinner = spinnerFrames[m.spinner%len(spinnerFrames)]
	}
	prefixWidth := runewidth.StringWidth(spinner) + 1
	available := m.width - prefixWidth
	lines := wrapText(run.message, available)
	if len(lines) == 0 {
		lines = []string{run.message}
	}

	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		rendered := styleRunning.Render(line)
		if i == 0 {
			sb.WriteString(styleRunning.Render(spinner))
			sb.WriteString(" ")
			sb.WriteString(rendered)
			continue
		}
		sb.WriteString(strings.Repeat(" ", prefixWidth))
		sb.WriteString(rendered)
	}
	return sb.String()
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		if rawLine == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(rawLine)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		var current string
		currentWidth := 0
		for _, word := range words {
			parts := splitLongWord(word, width)
			for _, part := range parts {
				partWidth := runewidth.StringWidth(part)
				if current == "" {
					current = part
					currentWidth = partWidth
					continue
				}
				if currentWidth+1+partWidth <= width {
					current += " " + part
					currentWidth += 1 + partWidth
					continue
				}
				lines = append(lines, current)
				current = part
				currentWidth = partWidth
			}
		}
		lines = append(lines, current)
	}
	return lines
}

func splitLongWord(word string, width int) []string {
	if width <= 0 || runewidth.StringWidth(word) <= width {
		return []string{word}
	}

	parts := make([]string, 0, (len(word)/width)+1)
	var sb strings.Builder
	currentWidth := 0
	for _, r := range word {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width && sb.Len() > 0 {
			parts = append(parts, sb.String())
			sb.Reset()
			currentWidth = 0
		}
		sb.WriteRune(r)
		currentWidth += runeWidth
	}
	if sb.Len() > 0 {
		parts = append(parts, sb.String())
	}
	return parts
}

func formatDetailsBlock(details string) string {
	clean := strings.ReplaceAll(details, "\r\n", "\n")
	lines := strings.Split(clean, "\n")

	if len(lines) > maxDetailsLines {
		visible := maxDetailsLines - 1
		moreCount := len(lines) - visible
		lines = append(lines[:visible], fmt.Sprintf("[... and %d more lines]", moreCount))
	}

	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("    | ")
		sb.WriteString(line)
	}
	return sb.String()
}
