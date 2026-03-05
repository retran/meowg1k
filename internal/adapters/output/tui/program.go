// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package tui provides a Bubble Tea-based terminal UI program that renders a
// conversation log with user/assistant turn bubbles, live spinner steps, a
// pinned status bar, and streaming LLM output.
//
// # Architecture
//
// The UI state is a sequential log of typed blocks rendered top-to-bottom inside
// a viewport.  The viewport occupies (termHeight - 2) lines; the last two lines
// are a separator and the status bar which are always visible.
//
//   - headerBlock        — a one-line tool/command header printed at the top
//   - userTurnBlock      — a user turn bubble (plain text)
//   - assistantTurnBlock — sealed assistant turn (steps + stream, all frozen)
//   - liveTurnBlock      — the current live assistant turn (last block when active)
//
// The live turn holds an ordered list of liveStep entries and a streaming text
// accumulator.  Steps animate with a bubbles/spinner.  StreamDoneMsg seals the
// stream text as rendered Markdown.  EndTurnMsg seals the whole live turn into
// an assistantTurnBlock and clears liveTurn.
//
// Messages flow in from output.Service via Program.Send().
package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/retran/meowg1k/internal/ui"
)

// ---------------------------------------------------------------------------
// Block types
// ---------------------------------------------------------------------------

type blockKind int

const (
	headerBlock        blockKind = iota
	userTurnBlock                // frozen user message
	assistantTurnBlock           // frozen completed assistant turn
	liveTurnBlock                // current live turn (at most one)
)

// liveStep is one step inside the live assistant turn.
type liveStep struct {
	id        string
	text      string
	info      []string
	done      bool
	ok        bool
	startTime time.Time
	elapsed   time.Duration
}

// liveSubTurn is a nested group inside a liveTurn.
// It has its own steps and stream, rendered indented below the parent.
type liveSubTurn struct {
	label      string
	steps      []*liveStep
	streamBuf  strings.Builder
	streamDone bool
	streamText string
	done       bool
}

// liveTurnItem is one ordered entry in a liveTurn — either a top-level step
// or a subturn.  Exactly one of step/subturn is non-nil.
type liveTurnItem struct {
	step    *liveStep
	subturn *liveSubTurn
}

// liveTurn holds the mutable state of the current (last, live) assistant turn.
type liveTurn struct {
	items      []*liveTurnItem // ordered: steps and subturns interleaved
	streamBuf  strings.Builder // raw accumulated tokens
	streamDone bool
	streamText string // rendered markdown after sealing
}

// activeSubTurn returns the last subturn if it is still open, otherwise nil.
// Step and stream messages route into this when non-nil.
func (lt *liveTurn) activeSubTurn() *liveSubTurn {
	for i := len(lt.items) - 1; i >= 0; i-- {
		if lt.items[i].subturn != nil {
			st := lt.items[i].subturn
			if !st.done {
				return st
			}
			return nil
		}
	}
	return nil
}

// block is one entry in the conversation log.
type block struct {
	kind blockKind

	// headerBlock / userTurnBlock / assistantTurnBlock:
	// rendered text (may contain ANSI).
	text string

	// liveTurnBlock:
	live *liveTurn
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// HeaderMsg emits a one-line header at the top of the log.
type HeaderMsg struct{ Text string }

// BeginUserTurnMsg opens a user-turn bubble.
type BeginUserTurnMsg struct{ Text string }

// BeginAssistantTurnMsg opens a new live assistant turn.
type BeginAssistantTurnMsg struct{}

// OpenStepMsg creates a new step in the current live turn.
type OpenStepMsg struct{ ID, Text string }

// UpdateStepMsg changes a step label.
type UpdateStepMsg struct{ ID, Text string }

// AddStepInfoMsg appends an info line to a step.
type AddStepInfoMsg struct{ ID, Text string }

// CloseStepMsg marks a step done or failed.
type CloseStepMsg struct {
	ID      string
	OK      bool
	Summary string
}

// BeginSubTurnMsg opens a nested subturn inside the current live turn.
type BeginSubTurnMsg struct{ Label string }

// EndSubTurnMsg closes the active subturn.
type EndSubTurnMsg struct{}

type TokenDeltaMsg struct{ Text string }

// StreamDoneMsg seals the stream block of the current live turn.
type StreamDoneMsg struct{}

// EndTurnMsg seals the current live turn.
type EndTurnMsg struct{ Summary string }

// SetStatusMsg updates the pinned status bar.
type SetStatusMsg struct{ Text string }

// LogLineMsg is kept for compatibility with legacy output paths.
type LogLineMsg struct{ Line string }

// quitMsg stops the BubbleTea program.
type quitMsg struct{}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	blocks    []*block
	width     int
	height    int
	noColor   bool
	theme     ui.Theme
	viewport  viewport.Model
	spinner   spinner.Model
	status    string
	cancelled bool // set to true when user presses q / Ctrl+C
}

func initialModel(theme ui.Theme, noColor bool) model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Spinner)

	vp := viewport.New(120, 20)

	return model{
		width:    120,
		height:   24,
		noColor:  noColor,
		theme:    theme,
		viewport: vp,
		spinner:  sp,
	}
}

// findLiveTurn returns the live turn block, or nil.
func (m *model) findLiveTurn() *liveTurn {
	if len(m.blocks) == 0 {
		return nil
	}
	b := m.blocks[len(m.blocks)-1]
	if b.kind == liveTurnBlock {
		return b.live
	}
	return nil
}

// findStep returns the liveStep with the given id, searching the active
// subturn first, then the parent turn's own steps.
func (lt *liveTurn) findStep(id string) *liveStep {
	if st := lt.activeSubTurn(); st != nil {
		for _, s := range st.steps {
			if s.id == id {
				return s
			}
		}
	}
	for _, item := range lt.items {
		if item.step != nil && item.step.id == id {
			return item.step
		}
	}
	return nil
}

// hasActiveStep returns true if any step in the live turn (or its active
// subturn) is still open, or if any subturn itself is still open.
func (lt *liveTurn) hasActiveStep() bool {
	for _, item := range lt.items {
		if item.subturn != nil {
			if !item.subturn.done {
				return true
			}
			for _, s := range item.subturn.steps {
				if !s.done {
					return true
				}
			}
		}
		if item.step != nil && !item.step.done {
			return true
		}
	}
	return false
}

// sealLiveTurn converts the last liveTurnBlock to a frozen assistantTurnBlock.
func (m *model) sealLiveTurn(summary string) {
	if len(m.blocks) == 0 {
		return
	}
	b := m.blocks[len(m.blocks)-1]
	if b.kind != liveTurnBlock {
		return
	}
	lt := b.live
	// Seal any open subturn first.
	if st := lt.activeSubTurn(); st != nil {
		sealSubTurn(st)
	}
	rendered := m.renderLiveTurn(lt)
	if summary != "" {
		summaryLine := m.theme.StatusInfo.Render(summary)
		rendered = rendered + summaryLine + "\n"
	}
	b.kind = assistantTurnBlock
	b.text = rendered
	b.live = nil
}

// sealSubTurn freezes the stream buffer of a subturn into rendered text.
// Called from sealLiveTurn when the parent turn is being sealed.
// Note: cannot call RenderMarkdown here because we have no width/noColor
// context; leave streamBuf populated and streamDone=false so that
// renderSubTurn will render it live on the next pass.
func sealSubTurn(st *liveSubTurn) {
	st.done = true
}

// renderLiveTurn converts a liveTurn to a frozen string.
func (m *model) renderLiveTurn(lt *liveTurn) string {
	var sb strings.Builder
	for _, item := range lt.items {
		if item.step != nil {
			sb.WriteString(m.renderStep(item.step))
		} else if item.subturn != nil {
			sb.WriteString(m.renderSubTurn(item.subturn))
		}
	}
	if lt.streamDone && lt.streamText != "" {
		sb.WriteString(lt.streamText)
	} else if !lt.streamDone && lt.streamBuf.Len() > 0 {
		if rendered, err := ui.RenderMarkdown(lt.streamBuf.String(), ui.TerminalWidth(m.width), m.noColor); err == nil {
			sb.WriteString(rendered)
		} else {
			sb.WriteString(lt.streamBuf.String())
		}
	}
	return sb.String()
}

// renderSubTurn renders a nested subturn, indented with a label header.
func (m *model) renderSubTurn(st *liveSubTurn) string {
	var sb strings.Builder

	// Label header line — show spinner while the subturn is still open.
	labelStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
	var prefix string
	if st.done {
		prefix = "  " + m.theme.StatusSuccess.Render("✓") + " "
	} else {
		prefix = "  " + m.spinner.View() + " "
	}
	sb.WriteString(prefix + labelStyle.Render(st.label) + "\n")

	// Steps inside the subturn — indent by 2 spaces relative to parent.
	for _, s := range st.steps {
		sb.WriteString("    " + m.renderStep(s))
	}

	// Stream content inside the subturn — indented by 4 spaces.
	const streamIndent = "    "
	var streamContent string
	if st.streamDone {
		streamContent = st.streamText
	} else if st.streamBuf.Len() > 0 {
		if rendered, err := ui.RenderMarkdown(st.streamBuf.String(), ui.TerminalWidth(m.width), m.noColor); err == nil {
			streamContent = rendered
		} else {
			streamContent = st.streamBuf.String()
		}
	}
	if streamContent != "" {
		for _, line := range strings.Split(strings.TrimRight(streamContent, "\n"), "\n") {
			sb.WriteString(streamIndent + line + "\n")
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

// renderStep renders a single step to a string.
func (m *model) renderStep(s *liveStep) string {
	var icon string
	if s.done {
		if s.ok {
			icon = m.theme.StatusSuccess.Render("✓")
		} else {
			icon = m.theme.StatusError.Render("✗")
		}
	} else {
		icon = m.spinner.View()
	}
	elapsed := ""
	if s.done {
		elapsed = fmt.Sprintf(" [%s]", s.elapsed.Round(time.Millisecond))
	}
	line := fmt.Sprintf("%s %s%s\n", icon, s.text, elapsed)
	var sb strings.Builder
	sb.WriteString(line)
	for _, info := range s.info {
		sb.WriteString("  " + m.theme.StatusInfo.Render("· "+info) + "\n")
	}
	if s.done && s.ok && s.text != "" {
		// no-op, summary was already embedded in text
	} else if s.done && !s.ok {
		// fail is already in text
	}
	return sb.String()
}

// rebuildViewportContent rebuilds the viewport content from all blocks.
func (m *model) rebuildViewportContent() {
	var sb strings.Builder
	for _, b := range m.blocks {
		switch b.kind {
		case headerBlock:
			sb.WriteString(b.text + "\n")
		case userTurnBlock:
			sb.WriteString(b.text + "\n")
		case assistantTurnBlock:
			sb.WriteString(b.text)
			if !strings.HasSuffix(b.text, "\n") {
				sb.WriteByte('\n')
			}
		case liveTurnBlock:
			sb.WriteString(m.renderLiveTurn(b.live))
		}
	}
	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(sb.String())
	if atBottom {
		m.viewport.GotoBottom()
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

//nolint:cyclop,funlen // Message switch necessarily large
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// viewport gets all lines except separator (1) + status bar (1)
		vpHeight := m.height - 2
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport.Width = m.width
		m.viewport.Height = vpHeight
		m.rebuildViewportContent()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// Only animate when there is an active live turn with open steps.
		if lt := m.findLiveTurn(); lt != nil && lt.hasActiveStep() {
			m.rebuildViewportContent()
			return m, cmd
		}
		return m, cmd

	case HeaderMsg:
		header := m.theme.AgentStyle.Bold(true).Render("▶ " + msg.Text)
		m.blocks = append(m.blocks, &block{kind: headerBlock, text: header})
		m.rebuildViewportContent()

	case BeginUserTurnMsg:
		text := msg.Text
		m.blocks = append(m.blocks, &block{kind: userTurnBlock, text: text})
		m.rebuildViewportContent()

	case BeginAssistantTurnMsg:
		// Seal any existing live turn first (shouldn't normally happen).
		m.sealLiveTurn("")
		lt := &liveTurn{}
		m.blocks = append(m.blocks, &block{kind: liveTurnBlock, live: lt})
		m.rebuildViewportContent()

	case OpenStepMsg:
		lt := m.findLiveTurn()
		if lt == nil {
			// Auto-create live turn if missing.
			lt = &liveTurn{}
			m.blocks = append(m.blocks, &block{kind: liveTurnBlock, live: lt})
		}
		newStep := &liveStep{
			id:        msg.ID,
			text:      msg.Text,
			startTime: time.Now(),
		}
		if st := lt.activeSubTurn(); st != nil {
			st.steps = append(st.steps, newStep)
		} else {
			lt.items = append(lt.items, &liveTurnItem{step: newStep})
		}
		m.rebuildViewportContent()

	case UpdateStepMsg:
		if lt := m.findLiveTurn(); lt != nil {
			if s := lt.findStep(msg.ID); s != nil {
				s.text = msg.Text
				m.rebuildViewportContent()
			}
		}

	case AddStepInfoMsg:
		if lt := m.findLiveTurn(); lt != nil {
			if s := lt.findStep(msg.ID); s != nil {
				s.info = append(s.info, msg.Text)
				m.rebuildViewportContent()
			}
		}

	case CloseStepMsg:
		if lt := m.findLiveTurn(); lt != nil {
			if s := lt.findStep(msg.ID); s != nil {
				s.done = true
				s.ok = msg.OK
				s.elapsed = time.Since(s.startTime)
				if msg.Summary != "" {
					s.text = msg.Summary
				}
				m.rebuildViewportContent()
			}
		}

	case BeginSubTurnMsg:
		lt := m.findLiveTurn()
		if lt == nil {
			lt = &liveTurn{}
			m.blocks = append(m.blocks, &block{kind: liveTurnBlock, live: lt})
		}
		lt.items = append(lt.items, &liveTurnItem{subturn: &liveSubTurn{label: msg.Label}})
		m.rebuildViewportContent()

	case EndSubTurnMsg:
		if lt := m.findLiveTurn(); lt != nil {
			if st := lt.activeSubTurn(); st != nil {
				// Seal stream text before marking done.
				if !st.streamDone {
					raw := st.streamBuf.String()
					if raw != "" {
						rendered, err := ui.RenderMarkdown(raw, ui.TerminalWidth(m.width), m.noColor)
						if err != nil {
							rendered = raw
						}
						st.streamText = rendered
					}
					st.streamBuf.Reset()
					st.streamDone = true
				}
				st.done = true
				m.rebuildViewportContent()
			}
		}

	case TokenDeltaMsg:
		lt := m.findLiveTurn()
		if lt == nil {
			lt = &liveTurn{}
			m.blocks = append(m.blocks, &block{kind: liveTurnBlock, live: lt})
		}
		if st := lt.activeSubTurn(); st != nil {
			st.streamBuf.WriteString(msg.Text)
		} else {
			lt.streamBuf.WriteString(msg.Text)
		}
		m.rebuildViewportContent()

	case StreamDoneMsg:
		if lt := m.findLiveTurn(); lt != nil {
			if st := lt.activeSubTurn(); st != nil {
				// Seal the subturn's stream.
				if !st.streamDone {
					raw := st.streamBuf.String()
					if raw != "" {
						rendered, err := ui.RenderMarkdown(raw, ui.TerminalWidth(m.width), m.noColor)
						if err != nil {
							rendered = raw
						}
						st.streamText = rendered
					}
					st.streamBuf.Reset()
					st.streamDone = true
				}
			} else if !lt.streamDone {
				raw := lt.streamBuf.String()
				if raw != "" {
					rendered, err := ui.RenderMarkdown(raw, ui.TerminalWidth(m.width), m.noColor)
					if err != nil {
						rendered = raw
					}
					lt.streamText = rendered
				}
				lt.streamBuf.Reset()
				lt.streamDone = true
			}
			m.rebuildViewportContent()
		}

	case EndTurnMsg:
		m.sealLiveTurn(msg.Summary)
		m.rebuildViewportContent()

	case SetStatusMsg:
		m.status = msg.Text

	case LogLineMsg:
		// Legacy path: wrap in a headerBlock.
		if msg.Line != "" {
			m.blocks = append(m.blocks, &block{kind: headerBlock, text: msg.Line})
			m.rebuildViewportContent()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			m.status = "Cancelling…"
			return m, tea.Quit
		}

	case quitMsg:
		return m, tea.Quit
	}

	// Forward unhandled messages to the viewport so mouse wheel and keyboard
	// navigation (arrow keys, PgUp/PgDn) work.
	// Note: holding Shift while dragging bypasses mouse capture in most
	// terminal emulators and allows text selection even with mouse mode on.
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) View() string {
	// Status bar style
	statusStyle := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Width(m.width)
	separatorStyle := lipgloss.NewStyle().
		Foreground(m.theme.Border).
		Width(m.width)

	statusText := m.status
	if statusText == "" {
		statusText = "q / Ctrl+C to cancel"
	}
	separator := separatorStyle.Render(strings.Repeat("─", m.width))
	statusBar := statusStyle.Render(statusText)

	return m.viewport.View() + "\n" + separator + "\n" + statusBar
}

// ---------------------------------------------------------------------------
// Program wrapper
// ---------------------------------------------------------------------------

// Program owns a running BubbleTea program and exposes Send/Stop.
type Program struct {
	p      *tea.Program
	done   chan struct{}
	once   sync.Once
	cancel func() // called after program exits (Ctrl+C / q path)
}

// New starts a BubbleTea program using the alternate screen buffer.
// The TUI occupies the full terminal while running; on exit the terminal
// switches back to the normal buffer and the TUI disappears cleanly.
// theme and noColor control visual rendering.
// cancel, if non-nil, is called after the program exits when the user pressed
// Ctrl+C or q (so the terminal is fully restored before cancellation occurs).
func New(theme ui.Theme, noColor bool, cancel func()) *Program {
	m := initialModel(theme, noColor)
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutSignalHandler(),
	)
	prog := &Program{p: p, done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(prog.done)
		finalModel, _ := p.Run()
		// If the user quit via key press, call cancel after the terminal is restored.
		if fm, ok := finalModel.(model); ok && fm.cancelled && prog.cancel != nil {
			prog.cancel()
		}
	}()
	return prog
}

// Send delivers a message to the running program. Safe from any goroutine.
func (pr *Program) Send(msg tea.Msg) { pr.p.Send(msg) }

// Stop asks the program to quit and blocks until it has exited. Idempotent.
func (pr *Program) Stop() {
	pr.once.Do(func() {
		pr.p.Send(quitMsg{})
		<-pr.done
	})
}
