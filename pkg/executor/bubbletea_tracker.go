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
	styleSubtle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleRunning   = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	styleCompleted = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleAgentStep = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleDuration  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleIndent    = "  "
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
		t.program = tea.NewProgram(t.model, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
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

	// Render executions in order
	for _, name := range m.order {
		exec, exists := m.executions[name]
		if !exists || exec == nil {
			continue
		}

		// Skip children of batch operations
		if m.isChildOfBatchOp(exec) {
			continue
		}

		// Skip aggregated tool calls
		if m.shouldAggregateToolCall(exec) {
			continue
		}

		line := m.renderExecution(exec)
		if line != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m *bubbleModel) renderExecution(exec *Execution) string {
	if exec == nil {
		return ""
	}

	indent := strings.Repeat(styleIndent, exec.Level)
	icon := m.getIcon(exec)
	name := m.getDisplayName(exec)
	duration := m.getDuration(exec)

	// Add batch progress if applicable
	if batchProg, exists := m.batchProgress[exec.Name]; exists && exec.Status == StatusRunning {
		percent := float64(batchProg.completed) / float64(batchProg.total)
		bar := m.progress.ViewAs(percent)
		return fmt.Sprintf("%s%s %s [%d/%d]\n%s%s",
			indent, icon, name, batchProg.completed, batchProg.total,
			indent+styleIndent, bar)
	}

	// Add error message if failed
	errorMsg := ""
	if exec.Status == StatusFailed && exec.Error != nil {
		errorMsg = styleSubtle.Render(" — " + exec.Error.Error())
	}

	return fmt.Sprintf("%s%s %s%s%s", indent, icon, name, duration, errorMsg)
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

func (m *bubbleModel) getDisplayName(exec *Execution) string {
	name := buildDisplayName(exec)

	// Add emoji for agent steps
	if isAgentStep(exec.Name) {
		name = formatAgentStepName(name)
		return styleAgentStep.Render(name)
	}

	// Tool calls
	if isToolCall(exec.Name) {
		return styleSubtle.Render("⚙️  " + name)
	}

	// Regular execution
	switch exec.Status {
	case StatusCompleted:
		return styleCompleted.Render(name)
	case StatusFailed:
		return styleFailed.Render(name)
	case StatusPending, StatusRunning:
		return name
	default:
		return name
	}
}

func (m *bubbleModel) getDuration(exec *Execution) string {
	duration := exec.getDurationString()
	if duration == "" {
		return ""
	}
	return styleDuration.Render(fmt.Sprintf(" (%s)", duration))
}

func (m *bubbleModel) isChildOfBatchOp(exec *Execution) bool {
	if exec == nil || exec.ParentName == "" {
		return false
	}
	_, exists := m.batchProgress[exec.ParentName]
	return exists
}

func (m *bubbleModel) shouldAggregateToolCall(exec *Execution) bool {
	if exec == nil || exec.ParentName == "" {
		return false
	}

	name := exec.Name
	if strings.Contains(name, "::Tool::") ||
		strings.Contains(name, "memory.") ||
		strings.Contains(name, "workspace.") ||
		strings.Contains(name, "plan.") {
		if exec.Status == StatusCompleted {
			m.toolCallCount[exec.ParentName]++
		}
		return true
	}

	return false
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

func buildDisplayName(exec *Execution) string {
	if exec == nil {
		return ""
	}

	if exec.Result != "" {
		return truncateString(exec.Result, maxMessageLength)
	}
	if exec.Message != "" {
		return truncateString(exec.Message, maxMessageLength)
	}

	parts := strings.Split(exec.Name, "::")
	if len(parts) == 0 {
		return ""
	}
	return convertCamelToReadable(parts[len(parts)-1])
}

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

func isAgentStep(name string) bool {
	return strings.Contains(name, "AgentStep") ||
		strings.Contains(strings.ToLower(name), "agent step")
}

func isToolCall(name string) bool {
	return strings.Contains(name, "::Tool::") ||
		strings.Contains(name, "memory.") ||
		strings.Contains(name, "workspace.") ||
		strings.Contains(name, "plan.")
}

func formatAgentStepName(name string) string {
	// Extract step type from messages like "Agent step: research"
	if strings.Contains(name, "Agent step:") {
		parts := strings.Split(name, ":")
		if len(parts) >= 2 {
			stepType := strings.TrimSpace(parts[1])
			switch strings.ToLower(stepType) {
			case "research":
				return "🧠 Researching..."
			case "plan":
				return "📝 Planning..."
			case "execute":
				return "🚀 Executing..."
			case "verify":
				return "✅ Verifying..."
			default:
				return "🤖 " + strings.ToUpper(stepType[:1]) + stepType[1:] + "..."
			}
		}
	}

	// Handle completion messages
	if strings.Contains(name, "completed:") || strings.Contains(name, "Completed:") {
		parts := strings.Split(name, ":")
		if len(parts) >= 2 {
			stepType := strings.TrimSpace(parts[1])
			return "Agent step completed: " + stepType
		}
	}

	return name
}
