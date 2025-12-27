// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BubbleTeaTracker provides a rich TUI experience for tracking execution progress.
type BubbleTeaTracker struct {
	program    *tea.Program
	model      *bubbleModel
	feedbackCh chan *Feedback
	mu         sync.RWMutex
	wg         sync.WaitGroup
	silent     bool
}

type runningActivity struct {
	name      string
	message   string
	startTime time.Time
	metadata  map[string]any
	status    Status
}

type logEntry struct {
	timestamp    time.Time
	activityName string
	result       string
	details      string
	isError      bool
	duration     time.Duration
	err          error
}

// bubbleModel is the Bubbletea model for rendering the TUI.
type bubbleModel struct {
	logLines []logEntry
	running  map[string]runningActivity
	mu       *sync.RWMutex
	spinner  spinner.Model
	width    int
	height   int
}

type tickMsg time.Time

// Styles for the TUI.
var (
	styleSubtle    = lipgloss.NewStyle().Faint(true)
	styleRunning   = lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))
	styleCompleted = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
	styleFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("red"))
	styleDuration  = lipgloss.NewStyle().Faint(true)
	styleDetails   = lipgloss.NewStyle().Faint(true).PaddingLeft(1).BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
)

// NewBubbleTeaTracker creates a new Bubbletea-based progress tracker.
func NewBubbleTeaTracker(silent bool) *BubbleTeaTracker {
	tracker := &BubbleTeaTracker{
		feedbackCh: make(chan *Feedback, feedbackChanSize),
		silent:     silent,
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	model := &bubbleModel{
		spinner:  s,
		logLines: make([]logEntry, 0),
		running:  make(map[string]runningActivity),
		mu:       &sync.RWMutex{},
	}

	tracker.model = model

	return tracker
}

// Start launches the Bubbletea program.
func (t *BubbleTeaTracker) Start() {
	if t == nil {
		return
	}

	// Start feedback processor
	t.wg.Add(1)
	go t.processFeedback()

	if t.silent {
		return
	}

	t.program = tea.NewProgram(t.model)

	// Start UI
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		if _, err := t.program.Run(); err != nil {
			// Log error but don't crash
			_ = err
		}
	}()
}

// Stop stops the Bubbletea program.
func (t *BubbleTeaTracker) Stop() {
	if t == nil {
		return
	}

	if t.feedbackCh != nil {
		close(t.feedbackCh)
	}

	// Wait a moment for the feedback processor to finish processing any pending messages
	time.Sleep(100 * time.Millisecond)

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
			// Drop feedback if channel is full
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

	// Check running
	if run, ok := t.model.running[name]; ok {
		exec = &Execution{
			Name:      run.name,
			Status:    run.status,
			Message:   run.message,
			StartTime: run.startTime,
			Metadata:  run.metadata,
		}
	} else {
		// Check logLines (reverse order to get latest)
		for i := len(t.model.logLines) - 1; i >= 0; i-- {
			entry := t.model.logLines[i]
			if entry.activityName == name {
				status := StatusCompleted
				if entry.isError {
					status = StatusFailed
				}
				endTime := entry.timestamp
				startTime := endTime.Add(-entry.duration)

				exec = &Execution{
					Name:      entry.activityName,
					Status:    status,
					Result:    entry.result,
					StartTime: startTime,
					EndTime:   &endTime,
					Error:     entry.err,
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

// Bubbletea model methods

func (m *bubbleModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tickCmd()

	case *Feedback:
		m.handleFeedback(msg)
		return m, nil
	}

	return m, nil
}

func (m *bubbleModel) handleFeedback(msg *Feedback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg.Status == StatusRunning || msg.Status == StatusPending {
		run := m.running[msg.ActivityName]
		run.name = msg.ActivityName
		run.message = msg.Message
		run.status = msg.Status
		if run.startTime.IsZero() {
			run.startTime = msg.Timestamp
		}
		if msg.Metadata != nil {
			run.metadata = msg.Metadata
		}
		m.running[msg.ActivityName] = run
	} else if msg.Status == StatusCompleted || msg.Status == StatusFailed {
		// Remove from running
		start := time.Time{}
		if run, ok := m.running[msg.ActivityName]; ok {
			start = run.startTime
			delete(m.running, msg.ActivityName)
		}

		// Create log entry
		duration := time.Duration(0)
		if !start.IsZero() {
			duration = msg.Timestamp.Sub(start)
		}

		result := msg.Message
		if strings.TrimSpace(result) == "" {
			result = compactActivityName(msg.ActivityName)
		}

		details := ""
		if msg.Metadata != nil {
			if s, ok := msg.Metadata["details"].(string); ok {
				details = strings.TrimSpace(s)
			}
		}

		entry := logEntry{
			timestamp:    msg.Timestamp,
			activityName: msg.ActivityName,
			result:       result,
			details:      details,
			isError:      msg.Status == StatusFailed,
			duration:     duration,
			err:          msg.Error,
		}
		m.logLines = append(m.logLines, entry)
	}
}

func (m *bubbleModel) View() string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	// Render log lines
	for _, entry := range m.logLines {
		sb.WriteString(m.renderLogEntry(entry))
		sb.WriteString("\n")
	}

	// Render running activities
	if len(m.running) > 0 {
		sb.WriteString("\n")
		sb.WriteString(styleSubtle.Render("Running:"))
		sb.WriteString("\n")
		for _, run := range m.running {
			display := run.message
			if strings.TrimSpace(display) == "" {
				display = compactActivityName(run.name)
			}

			indicator := m.spinner.View()
			if run.status == StatusPending {
				indicator = styleSubtle.Render("...")
			}

			sb.WriteString(fmt.Sprintf(" %s %s\n", indicator, styleRunning.Render(display)))
		}
	}

	return sb.String()
}

func (m *bubbleModel) renderLogEntry(entry logEntry) string {
	timestamp := entry.timestamp.Format("15:04:05")
	duration := styleDuration.Render(fmt.Sprintf("(%s)", entry.duration.Round(time.Millisecond)))

	prefix := ""
	if entry.isError {
		prefix = styleFailed.Render("FAILED") + " "
	}

	line := fmt.Sprintf("%s %s%s %s", styleSubtle.Render(timestamp), prefix, entry.result, duration)

	if entry.isError && entry.err != nil {
		line += " — " + styleFailed.Render(entry.err.Error())
	}
	if entry.details != "" {
		line += "\n" + m.renderBlock(entry.details, &styleDetails)
	}

	return line
}

func (m *bubbleModel) renderBlock(content string, style *lipgloss.Style) string {
	var result strings.Builder
	maxWidth := m.width - 6
	if maxWidth <= 0 {
		maxWidth = 74
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result.WriteString("\n")
			continue
		}
		wrappedLines := wordWrap(line, maxWidth)
		for _, wrappedLine := range wrappedLines {
			result.WriteString(style.Render(wrappedLine))
			result.WriteString("\n")
		}
	}
	return result.String()
}

func compactActivityName(activityName string) string {
	if strings.TrimSpace(activityName) == "" {
		return ""
	}
	parts := strings.Split(activityName, "::")
	return strings.TrimSpace(parts[len(parts)-1])
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// wordWrap wraps text to fit within the specified width.
func wordWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var currentLine strings.Builder
	for i, word := range words {
		// If adding this word would exceed width, start a new line
		if currentLine.Len() > 0 && currentLine.Len()+1+len(word) > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}

		// Add space before word (except for first word in line)
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)

		// If this is the last word, add the current line
		if i == len(words)-1 {
			lines = append(lines, currentLine.String())
		}
	}

	if len(lines) == 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}
