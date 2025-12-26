// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	feedbackChanSize = 128
	maxMessageLength = 100
)

// batchProgressTracker tracks progress for batch operations like "Fetching 36 files".
type batchProgressTracker struct {
	activity  string
	total     int
	completed int
}

// BubbleTeaTracker provides a rich TUI experience for tracking execution progress.
type BubbleTeaTracker struct {
	program    *tea.Program
	model      *bubbleModel
	feedbackCh chan *Feedback
	executions map[string]*Execution
	order      []string
	mu         sync.RWMutex
	wg         sync.WaitGroup
	silent     bool
}

// bubbleModel is the Bubbletea model for rendering the TUI.
type bubbleModel struct {
	executions    map[string]*Execution
	order         []string
	batchProgress map[string]*batchProgressTracker
	toolCallCount map[string]int
	mu            *sync.RWMutex
	spinner       spinner.Model
	progress      progress.Model
	width         int
	height        int
}

type tickMsg time.Time

// Styles for the TUI.
var (
	styleSubtle    = lipgloss.NewStyle().Faint(true)
	styleRunning   = lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))
	styleCompleted = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
	styleFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("red"))
	styleDuration  = lipgloss.NewStyle().Faint(true)
	styleLLMBlock  = lipgloss.NewStyle().Faint(true).PaddingLeft(1).BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
)

// NewBubbleTeaTracker creates a new Bubbletea-based progress tracker.
func NewBubbleTeaTracker(silent bool) *BubbleTeaTracker {
	tracker := &BubbleTeaTracker{
		feedbackCh: make(chan *Feedback, feedbackChanSize),
		executions: make(map[string]*Execution),
		order:      make([]string, 0),
		silent:     silent,
	}

	if silent {
		return tracker
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	p := progress.New(progress.WithDefaultGradient())

	model := &bubbleModel{
		spinner:       s,
		progress:      p,
		executions:    make(map[string]*Execution),
		order:         make([]string, 0),
		batchProgress: make(map[string]*batchProgressTracker),
		toolCallCount: make(map[string]int),
		mu:            &sync.RWMutex{},
	}

	tracker.model = model

	return tracker
}

// Start launches the Bubbletea program.
func (t *BubbleTeaTracker) Start() {
	if t == nil {
		return
	}

	// Always start feedback processor
	t.wg.Add(1)
	go t.processFeedback()

	// Only start TUI if not in silent mode
	if !t.silent {
		t.program = tea.NewProgram(t.model, tea.WithOutput(os.Stderr))
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			if _, err := t.program.Run(); err != nil {
				// Log error but don't crash
				_ = err
			}
		}()
	}
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
		t.program.Send(tea.Quit())
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
		t.mu.Lock()
		t.updateExecution(feedback)
		t.mu.Unlock()

		if t.program != nil {
			// Update the model
			t.model.mu.Lock()
			t.model.executions = copyExecutions(t.executions)
			t.model.order = append([]string{}, t.order...)
			t.model.mu.Unlock()

			t.program.Send(feedback)
		}
	}
}

// updateExecution updates execution state based on feedback.
func (t *BubbleTeaTracker) updateExecution(feedback *Feedback) {
	if feedback == nil || feedback.ActivityName == "" {
		return
	}

	exec := t.getOrCreateExecution(feedback)
	t.updateExecutionState(exec, feedback)
	t.updateBatchProgress(exec, feedback)
}

// getOrCreateExecution gets existing execution or creates a new one.
func (t *BubbleTeaTracker) getOrCreateExecution(feedback *Feedback) *Execution {
	exec, exists := t.executions[feedback.ActivityName]
	if exists {
		return exec
	}

	parentName, level := parseActivityHierarchy(feedback.ActivityName)
	exec = &Execution{
		Name:       feedback.ActivityName,
		StartTime:  feedback.Timestamp,
		Metadata:   make(map[string]any),
		ParentName: parentName,
		Children:   make([]string, 0),
		Level:      level,
	}
	t.executions[feedback.ActivityName] = exec
	t.order = append(t.order, feedback.ActivityName)

	if parent, ok := t.executions[parentName]; ok && parent != nil {
		parent.Children = append(parent.Children, feedback.ActivityName)
	}

	return exec
}

// updateExecutionState updates the state fields of an execution.
func (t *BubbleTeaTracker) updateExecutionState(exec *Execution, feedback *Feedback) {
	exec.Status = feedback.Status
	exec.Error = feedback.Error
	exec.Message = sanitizeDescription(feedback.Message)

	// Update metadata if provided
	if feedback.Metadata != nil {
		for k, v := range feedback.Metadata {
			exec.Metadata[k] = v
		}
	}

	if feedback.Status == StatusCompleted || feedback.Status == StatusFailed {
		exec.Result = sanitizeDescription(feedback.Message)
		endTime := feedback.Timestamp
		exec.EndTime = &endTime
	}
}

// updateBatchProgress updates batch progress tracking.
func (t *BubbleTeaTracker) updateBatchProgress(exec *Execution, feedback *Feedback) {
	if feedback.Status == StatusRunning {
		t.initBatchProgress(exec)
	}
	if feedback.Status == StatusCompleted && exec.ParentName != "" && t.model != nil {
		if batchProg, exists := t.model.batchProgress[exec.ParentName]; exists {
			batchProg.completed++
		}
	}
}

// initBatchProgress initializes batch progress tracking.
func (t *BubbleTeaTracker) initBatchProgress(exec *Execution) {
	if exec == nil || exec.Message == "" || t.model == nil {
		return
	}

	msg := exec.Message
	var total int

	if n, err := fmt.Sscanf(msg, "Fetching staged diffs for %d files", &total); err == nil && n == 1 {
		t.model.batchProgress[exec.Name] = &batchProgressTracker{total: total, activity: "Fetching"}
	} else if n, err := fmt.Sscanf(msg, "Summarizing changes in %d files", &total); err == nil && n == 1 {
		t.model.batchProgress[exec.Name] = &batchProgressTracker{total: total, activity: "Summarizing"}
	} else if n, err := fmt.Sscanf(msg, "Fetching branch diffs for %d files", &total); err == nil && n == 1 {
		t.model.batchProgress[exec.Name] = &batchProgressTracker{total: total, activity: "Fetching"}
	}
}

// GetExecution returns a copy of an execution (for testing).
func (t *BubbleTeaTracker) GetExecution(name string) *Execution {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	exec, exists := t.executions[name]
	if !exists || exec == nil {
		return nil
	}

	copyExec := *exec
	return &copyExec
}

// GetExecutionCount returns the number of tracked executions (for testing).
func (t *BubbleTeaTracker) GetExecutionCount() int {
	if t == nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.executions)
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
		m.progress.Width = msg.Width - 20
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tickCmd()

	case *Feedback:
		// Model is updated externally by processFeedback
		return m, nil
	}

	return m, nil
}

func (m *bubbleModel) View() string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	m.renderCompletedExecutions(&sb)
	m.renderDeepestRunningExecution(&sb)

	return sb.String()
}

func (m *bubbleModel) renderCompletedExecutions(sb *strings.Builder) {
	for _, name := range m.order {
		exec, exists := m.executions[name]
		if !exists || exec == nil || exec.Status == StatusRunning {
			continue
		}

		line := m.renderExecution(exec)
		if line != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
}

func (m *bubbleModel) renderDeepestRunningExecution(sb *strings.Builder) {
	var deepestRunning *Execution
	maxLevel := -1
	for _, name := range m.order {
		exec, exists := m.executions[name]
		if !exists || exec == nil || exec.Status != StatusRunning {
			continue
		}
		if exec.Level > maxLevel {
			maxLevel = exec.Level
			deepestRunning = exec
		}
	}

	if deepestRunning != nil {
		line := m.renderExecution(deepestRunning)
		if line != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
}

func (m *bubbleModel) renderExecution(exec *Execution) string {
	if exec == nil {
		return ""
	}

	var result strings.Builder

	// Get human-readable name and icon
	readableName := getHumanReadableName(exec)
	icon := m.getIcon(exec)
	timestamp := exec.StartTime.Format("15:04:05")
	duration := m.getDuration(exec)

	// For running activities, show spinner and action
	if exec.Status == StatusRunning {
		result.WriteString(fmt.Sprintf("%s %s %s...",
			styleSubtle.Render(timestamp), icon, styleRunning.Render(readableName)))
		return result.String()
	}

	// For completed/failed, show result
	if exec.Status == StatusFailed && exec.Error != nil {
		errorMsg := styleFailed.Render(exec.Error.Error())
		result.WriteString(fmt.Sprintf("%s %s %s%s — %s",
			styleSubtle.Render(timestamp), icon, styleFailed.Render(readableName), duration, errorMsg))
		return result.String()
	}

	// Completed successfully
	result.WriteString(fmt.Sprintf("%s %s %s%s",
		styleSubtle.Render(timestamp), icon, styleCompleted.Render(readableName), duration))

	// If this execution has an LLM response, show it as a formatted block
	if exec.Status == StatusCompleted {
		if llmResponse, ok := exec.Metadata["llm_response"].(string); ok && llmResponse != "" {
			result.WriteString("\n")
			result.WriteString(m.renderLLMResponse(llmResponse))
		}
	}

	return result.String()
}

// renderLLMResponse formats LLM response with proper markdown-like styling.
func (m *bubbleModel) renderLLMResponse(response string) string {
	var result strings.Builder
	maxWidth := m.width - 6 // Account for border and padding
	if maxWidth <= 0 {
		maxWidth = 74
	}

	// Split into lines
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			result.WriteString("\n")
			continue
		}

		// Word wrap and render each line
		wrappedLines := wordWrap(line, maxWidth)
		for _, wrappedLine := range wrappedLines {
			result.WriteString(styleLLMBlock.Render(wrappedLine))
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m *bubbleModel) getIcon(exec *Execution) string {
	switch exec.Status {
	case StatusRunning:
		return styleRunning.Render(m.spinner.View())
	case StatusCompleted:
		return styleCompleted.Render("✓")
	case StatusFailed:
		return styleFailed.Render("✗")
	case StatusPending:
		return styleSubtle.Render("…")
	default:
		return styleSubtle.Render("→")
	}
}

// getHumanReadableName converts technical activity names to human-readable descriptions.
func getHumanReadableName(exec *Execution) string {
	if exec == nil {
		return ""
	}

	// Always prefer the message from the activity itself
	if exec.Message != "" {
		return exec.Message
	}

	// Fall back to result if available
	if exec.Result != "" {
		return exec.Result
	}

	// Last resort: convert the activity name
	parts := strings.Split(exec.Name, "::")
	if len(parts) == 0 {
		return "Working"
	}

	lastPart := parts[len(parts)-1]
	return convertCamelToReadable(lastPart)
}

func (m *bubbleModel) getDuration(exec *Execution) string {
	duration := exec.getDurationString()
	if duration == "" {
		return ""
	}
	return styleDuration.Render(fmt.Sprintf(" (%s)", duration))
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func copyExecutions(src map[string]*Execution) map[string]*Execution {
	dst := make(map[string]*Execution, len(src))
	for k, v := range src {
		if v != nil {
			execCopy := *v
			dst[k] = &execCopy
		}
	}
	return dst
}

// Helper functions moved from old tracker

func sanitizeDescription(description string) string {
	if description == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(description))
	for _, r := range description {
		if unicode.IsPrint(r) && r != '\x1b' {
			b.WriteRune(r)
		}
	}

	return truncateString(b.String(), maxMessageLength)
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func convertCamelToReadable(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s) + 5)
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune(' ')
		}
		if i == 0 {
			result.WriteRune(unicode.ToUpper(r))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

func parseActivityHierarchy(activityName string) (parentName string, level int) {
	parts := strings.Split(activityName, "::")
	level = len(parts) - 1
	if level > 0 {
		parentName = strings.Join(parts[:level], "::")
	}
	return parentName, level
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
