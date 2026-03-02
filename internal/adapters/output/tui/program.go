// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package tui provides a Bubble Tea-based terminal UI program for streaming
// LLM output and displaying rich progress chrome.
//
// # Block log model
//
// The UI state is a sequential log of typed blocks rendered top-to-bottom:
//
//   - LogBlock   — a finished static line (step header, status, divider, etc.)
//     Appended by LogLineMsg; rendered as-is.
//   - StreamBlock — a live streaming container that accumulates raw LLM tokens.
//     Created by the first TokenDeltaMsg after the previous stream
//     completed. Sealed by StreamDoneMsg: the accumulated text is
//     rendered as Markdown and the block is frozen in place.
//
// This means a multi-step workflow (stream → tool call → stream → …) produces
// a natural append-only log: each completed stream block stays visible above
// the next one, and ui.step / ui.success lines slot in between as LogBlocks.
//
// The live area is always the last block when it is an open StreamBlock.
// All other blocks are already frozen and their rendered text never changes.
package tui

import (
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/retran/meowg1k/internal/ui"
)

// ---------------------------------------------------------------------------
// Block types
// ---------------------------------------------------------------------------

type blockKind int

const (
	logBlock    blockKind = iota // static, already rendered
	streamBlock                  // accumulating tokens; may be sealed
)

// block is one entry in the UI log.
type block struct {
	kind blockKind

	// logBlock: the final rendered string (may contain ANSI).
	text string

	// streamBlock: raw accumulated tokens while open.
	raw strings.Builder

	// streamBlock: rendered Markdown after sealing.
	rendered string

	// sealed is true once StreamDoneMsg has been processed for this block.
	sealed bool
}

func newLogBlock(text string) *block { return &block{kind: logBlock, text: text} }
func newStreamBlock() *block         { return &block{kind: streamBlock} }

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// LogLineMsg appends a finished static line to the block log.
type LogLineMsg struct{ Line string }

// TokenDeltaMsg delivers one token delta from the LLM.
// If there is no open StreamBlock at the tail of the log, a new one is created.
type TokenDeltaMsg struct{ Text string }

// StreamDoneMsg seals the current StreamBlock: tokens are rendered as Markdown
// and the block is frozen. The next TokenDeltaMsg will open a fresh StreamBlock.
type StreamDoneMsg struct{}

// quitMsg stops the BubbleTea program.
type quitMsg struct{}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	blocks  []*block
	width   int
	noColor bool
}

func initialModel(noColor bool) model {
	return model{width: 120, noColor: noColor}
}

// tailStream returns the last block if it is an open (unsealed) StreamBlock,
// or nil otherwise.
func (m *model) tailStream() *block {
	if len(m.blocks) == 0 {
		return nil
	}
	b := m.blocks[len(m.blocks)-1]
	if b.kind == streamBlock && !b.sealed {
		return b
	}
	return nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case LogLineMsg:
		if msg.Line != "" {
			m.blocks = append(m.blocks, newLogBlock(msg.Line))
		}

	case TokenDeltaMsg:
		b := m.tailStream()
		if b == nil {
			b = newStreamBlock()
			m.blocks = append(m.blocks, b)
		}
		b.raw.WriteString(msg.Text)

	case StreamDoneMsg:
		b := m.tailStream()
		if b == nil {
			// No open stream — nothing to seal.
			break
		}
		raw := b.raw.String()
		if raw != "" {
			rendered, err := ui.RenderMarkdown(raw, ui.TerminalWidth(m.width), m.noColor)
			if err != nil {
				rendered = raw
			}
			b.rendered = rendered
		}
		b.raw.Reset()
		b.sealed = true

	case quitMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	for _, blk := range m.blocks {
		switch blk.kind {
		case logBlock:
			b.WriteString(blk.text)
			if !strings.HasSuffix(blk.text, "\n") {
				b.WriteByte('\n')
			}

		case streamBlock:
			if blk.sealed {
				b.WriteString(blk.rendered)
				if !strings.HasSuffix(blk.rendered, "\n") {
					b.WriteByte('\n')
				}
			} else {
				// Live: show raw tokens as they arrive.
				b.WriteString(blk.raw.String())
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Program wrapper
// ---------------------------------------------------------------------------

// Program owns a running BubbleTea program and exposes Send/Stop.
type Program struct {
	p    *tea.Program
	done chan struct{}
	once sync.Once
}

// NewInline starts a BubbleTea program that renders inline (no alt-screen),
// keeping output visible in the terminal scroll-back after exit.
func NewInline(noColor bool) *Program {
	p := tea.NewProgram(
		initialModel(noColor),
		tea.WithoutSignalHandler(),
	)
	prog := &Program{p: p, done: make(chan struct{})}
	go func() {
		defer close(prog.done)
		_, _ = p.Run()
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
