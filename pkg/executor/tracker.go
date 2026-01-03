// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/gosuri/uilive"
	"github.com/mattn/go-runewidth"
)

const (
	feedbackChanSize = 128
	spinnerInterval  = 120 * time.Millisecond
	widthCacheTTL    = 500 * time.Millisecond
)

// BubbleTeaTracker provides a styled TUI-like log with a single live status line.
type BubbleTeaTracker struct {
	bypass       io.Writer
	feedbackDone chan struct{}
	running      map[string]runningActivity
	liveWriter   *uilive.Writer
	spinnerStop  chan struct{}
	feedbackCh   chan *Feedback
	wg           sync.WaitGroup
	logCount     int
	spinner      int
	mu           sync.RWMutex
	spinnerOnce  sync.Once
	stopOnce     sync.Once
	stopped      atomic.Bool
	silent       bool
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

// Styles for the TUI.
var (
	styleTitle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	styleRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleFailed  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleDetails = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	cachedTerminalWidth atomic.Int64
	cachedWidthAt       atomic.Int64
)

// NewBubbleTeaTracker creates a new progress tracker.
func NewBubbleTeaTracker(silent bool) *BubbleTeaTracker {
	return &BubbleTeaTracker{
		feedbackCh: make(chan *Feedback, feedbackChanSize),
		silent:     silent,
		running:    make(map[string]runningActivity),
	}
}

// Start launches the live status writer.
func (t *BubbleTeaTracker) Start() {
	if t == nil {
		return
	}

	t.feedbackDone = make(chan struct{})

	t.wg.Add(1)
	go t.processFeedback()

	if t.silent {
		return
	}

	t.liveWriter = uilive.New()
	t.liveWriter.Out = os.Stderr
	t.liveWriter.Start()
	t.bypass = t.liveWriter.Bypass()

	t.spinnerStop = make(chan struct{})
	t.wg.Add(1)
	go t.spinnerLoop()
}

// Stop stops the tracker.
func (t *BubbleTeaTracker) Stop() {
	if t == nil {
		return
	}

	t.stopped.Store(true)
	t.stopOnce.Do(func() {
		if t.feedbackCh != nil {
			close(t.feedbackCh)
		}
	})

	if t.feedbackDone != nil {
		<-t.feedbackDone
	}

	t.stopSpinner()

	t.wg.Wait()

	if t.liveWriter != nil {
		t.liveWriter.Stop()
	}
}

// FeedbackHandler returns a handler for receiving feedback.
func (t *BubbleTeaTracker) FeedbackHandler() FeedbackHandler {
	return func(feedback *Feedback) {
		if t == nil || t.feedbackCh == nil || feedback == nil {
			return
		}
		if t.stopped.Load() {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				// Silently ignore panic from closed channel
				_ = r
			}
		}()
		select {
		case t.feedbackCh <- feedback:
		default:
		}
	}
}

// processFeedback processes feedback messages.
func (t *BubbleTeaTracker) processFeedback() {
	defer t.wg.Done()
	defer func() {
		if t.feedbackDone != nil {
			close(t.feedbackDone)
		}
	}()

	for feedback := range t.feedbackCh {
		t.handleFeedback(feedback)

		if t.silent {
			continue
		}

		if feedback.Status != StatusCompleted && feedback.Status != StatusFailed && feedback.Status != StatusProgress {
			continue
		}

		entry := logEntry{
			message: strings.TrimSpace(feedback.Message),
			details: strings.TrimSpace(feedback.Details),
			isError: feedback.Status == StatusFailed,
		}
		rendered := renderLogEntry(entry, terminalWidth())
		if rendered == "" {
			continue
		}
		t.writeLogLine(rendered)
	}
}

func (t *BubbleTeaTracker) handleFeedback(msg *Feedback) {
	if t == nil || msg == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.logCount++

	message := strings.TrimSpace(msg.Message)
	if msg.Status == StatusRunning && message != "" {
		run := t.running[msg.ActivityName]
		run.name = msg.ActivityName
		run.message = message
		run.status = msg.Status
		run.lastUpdate = msg.Timestamp
		t.running[msg.ActivityName] = run
	}

	if msg.Status == StatusCompleted || msg.Status == StatusFailed {
		if _, ok := t.running[msg.ActivityName]; !ok {
			delete(t.running, msg.ActivityName)
		}
	}
}

func (t *BubbleTeaTracker) spinnerLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.spinnerStop:
			t.renderStatus("")
			return
		case <-ticker.C:
			run := t.currentRunning()
			if strings.TrimSpace(run.message) == "" {
				t.renderStatus("")
				continue
			}

			t.spinner = (t.spinner + 1) % len(spinnerFrames)
			line := renderRunningLine(run, t.spinner, terminalWidth())
			t.renderStatus(line)
		}
	}
}

func (t *BubbleTeaTracker) stopSpinner() {
	t.spinnerOnce.Do(func() {
		if t.spinnerStop != nil {
			close(t.spinnerStop)
		}
	})
}

func (t *BubbleTeaTracker) currentRunning() runningActivity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var current runningActivity
	for _, run := range t.running {
		if current.name == "" || run.lastUpdate.After(current.lastUpdate) {
			current = run
		}
	}
	return current
}

func (t *BubbleTeaTracker) renderStatus(line string) {
	if t.liveWriter == nil {
		return
	}
	if strings.TrimSpace(line) == "" {
		_, err := fmt.Fprint(t.liveWriter, " \n")
		logWriteError(err)
		return
	}
	_, err := fmt.Fprintf(t.liveWriter, "%s\n", line)
	logWriteError(err)
}

func (t *BubbleTeaTracker) writeLogLine(line string) {
	if t.bypass == nil {
		return
	}
	_, err := fmt.Fprint(t.bypass, line)
	logWriteError(err)
	if !strings.HasSuffix(line, "\n") {
		_, err := fmt.Fprint(t.bypass, "\n")
		logWriteError(err)
	}
}

// GetExecution returns a copy of an execution (for testing).
func (t *BubbleTeaTracker) GetExecution(name string) *Execution {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var exec *Execution
	if run, ok := t.running[name]; ok {
		exec = &Execution{
			Name:    run.name,
			Status:  run.status,
			Message: run.message,
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
	if t == nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.running) + t.logCount
}

func renderLogEntry(entry logEntry, width int) string {
	message := strings.TrimSpace(entry.message)
	details := strings.TrimSpace(entry.details)
	if message == "" && details == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")

	if message != "" {
		renderMessageBlock(&sb, entry, width)
	}

	if details != "" {
		if message != "" {
			sb.WriteString("\n")
		}
		sb.WriteString(styleDetails.Render(formatDetailsBlock(details, width)))
	}

	return sb.String()
}

func renderMessageBlock(sb *strings.Builder, entry logEntry, width int) {
	bullet := "• "
	bulletWidth := runewidth.StringWidth(bullet)
	available := max(width-bulletWidth, 0)

	lines := wrapText(entry.message, available)
	if len(lines) == 0 {
		lines = []string{entry.message}
	}

	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		var renderedLine string
		if i == 0 {
			renderedLine = bullet + line
		} else {
			renderedLine = strings.Repeat(" ", bulletWidth) + line
		}
		rendered := styleTitle.Render(renderedLine)
		if entry.isError {
			rendered = styleFailed.Render(renderedLine)
		}
		sb.WriteString(rendered)
	}
}

func renderRunningLine(run runningActivity, spinnerIndex int, width int) string {
	spinner := "-"
	if len(spinnerFrames) > 0 {
		spinner = spinnerFrames[spinnerIndex%len(spinnerFrames)]
	}
	prefix := fmt.Sprintf("%s ", spinner)
	prefixWidth := runewidth.StringWidth(prefix)
	available := max(width-prefixWidth, 0)

	message := strings.TrimSpace(run.message)
	if available > 0 {
		message = truncateToWidth(message, available)
	} else {
		message = ""
	}

	renderedPrefix := styleRunning.Render(prefix)
	renderedMessage := styleRunning.Render(message)
	if message == "" {
		return renderedPrefix
	}
	return fmt.Sprintf("%s%s", renderedPrefix, renderedMessage)
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}

	text = strings.TrimSpace(text)
	if runewidth.StringWidth(text) <= width {
		return text
	}

	var sb strings.Builder
	currentWidth := 0
	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width {
			break
		}
		sb.WriteRune(r)
		currentWidth += runeWidth
	}
	return sb.String()
}

func terminalWidth() int {
	now := time.Now()
	if cached := int(cachedTerminalWidth.Load()); cached > 0 {
		last := time.Unix(0, cachedWidthAt.Load())
		if now.Sub(last) < widthCacheTTL {
			return cached
		}
	}

	if cols := os.Getenv("COLUMNS"); cols != "" {
		if value, err := strconv.Atoi(cols); err == nil && value > 0 {
			if value > 72 {
				value = 72
			}
			cachedTerminalWidth.Store(int64(value))
			cachedWidthAt.Store(now.UnixNano())
			return value
		}
	}
	width, _, err := term.GetSize(os.Stderr.Fd())
	if err != nil || width <= 0 {
		cachedTerminalWidth.Store(72)
		cachedWidthAt.Store(now.UnixNano())
		return 72
	}
	cachedTerminalWidth.Store(int64(width))
	cachedWidthAt.Store(now.UnixNano())
	return width
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
		lines = append(lines, wrapLine(rawLine, width)...)
	}
	return lines
}

func wrapLine(line string, width int) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
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

func formatDetailsBlock(details string, width int) string {
	clean := strings.ReplaceAll(details, "\r\n", "\n")
	lines := strings.Split(clean, "\n")
	prefix := "  │ "
	prefixWidth := runewidth.StringWidth(prefix)
	available := max(width-prefixWidth, 0)

	var sb strings.Builder
	for i, line := range lines {
		wrapped := wrapTextPreserveSpaces(line, available)
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		for j, wrappedLine := range wrapped {
			if i > 0 || j > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(prefix)
			sb.WriteString(wrappedLine)
		}
	}
	return sb.String()
}

func wrapTextPreserveSpaces(text string, width int) []string {
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
		lines = append(lines, wrapLinePreserveSpaces(rawLine, width)...)
	}
	return lines
}

func wrapLinePreserveSpaces(line string, width int) []string {
	tokens := splitTokensPreserveSpaces(line)
	var lines []string
	var current strings.Builder
	currentWidth := 0
	for _, token := range tokens {
		tokenWidth := runewidth.StringWidth(token)
		if tokenWidth == 0 {
			continue
		}

		if currentWidth+tokenWidth <= width {
			current.WriteString(token)
			currentWidth += tokenWidth
			continue
		}

		if tokenWidth > width {
			lines, currentWidth = handleLongToken(token, width, currentWidth, &current, &lines)
			continue
		}

		if currentWidth > 0 {
			lines = append(lines, current.String())
			current.Reset()
		}
		current.WriteString(token)
		currentWidth = tokenWidth
	}
	if currentWidth > 0 || current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func handleLongToken(token string, width, currentWidth int, current *strings.Builder, lines *[]string) (resultLines []string, newWidth int) {
	if currentWidth > 0 {
		*lines = append(*lines, current.String())
		current.Reset()
	}
	parts := splitLongWord(token, width)
	newWidth = 0
	for i, part := range parts {
		if runewidth.StringWidth(part) == 0 {
			continue
		}
		if i < len(parts)-1 {
			*lines = append(*lines, part)
		} else {
			current.WriteString(part)
			newWidth = runewidth.StringWidth(part)
		}
	}
	return *lines, newWidth
}

func splitTokensPreserveSpaces(line string) []string {
	tokens := make([]string, 0, len(line))
	var sb strings.Builder
	var inSpace bool
	for _, r := range line {
		isSpace := r == ' ' || r == '\t'
		if sb.Len() == 0 {
			inSpace = isSpace
			sb.WriteRune(r)
			continue
		}
		if isSpace == inSpace {
			sb.WriteRune(r)
			continue
		}
		tokens = append(tokens, sb.String())
		sb.Reset()
		inSpace = isSpace
		sb.WriteRune(r)
	}
	if sb.Len() > 0 {
		tokens = append(tokens, sb.String())
	}
	return tokens
}

func logWriteError(err error) {
	if err == nil {
		return
	}
	log.Printf("tracker output error: %v", err)
}
