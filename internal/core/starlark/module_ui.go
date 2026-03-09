// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/internal/ui"
)

const (
	stepAttr = "step"
	doneAttr = "done"
	failAttr = "fail"
)

var (
	defaultRenderOptions = ui.NewRenderOptions()
	defaultTheme         = ui.DefaultThemeWithOptions(defaultRenderOptions)
)

// noopBuiltin returns a Starlark builtin that accepts any arguments and returns None.
func noopBuiltin(name string) *starlark.Builtin {
	return starlark.NewBuiltin(name, func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})
}

// noopTurnFunc returns a function that creates a no-op TurnHandle.
func noopTurnFunc() func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return &TurnHandle{writer: nil}, nil
	}
}

// noopUIModule returns a ui module where every function is a no-op.
// Used when stdout is not a TTY so UI chrome is suppressed entirely.
func noopUIModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "ui",
		Members: starlark.StringDict{
			"user_turn":      starlark.NewBuiltin("ui.user_turn", noopTurnFunc()),
			"assistant_turn": starlark.NewBuiltin("ui.assistant_turn", noopTurnFunc()),
			"prompt":         starlark.NewBuiltin("ui.prompt", uiPrompt),   // interactive — keep
			"confirm":        starlark.NewBuiltin("ui.confirm", uiConfirm), // interactive — keep
			"progress_bar":   starlark.NewBuiltin("ui.progress_bar", makeProgressBarFunc("", os.Stdout)),
			"markdown":       noopBuiltin("ui.markdown"),
			"table":          noopBuiltin("ui.table"),
			"panel":          noopBuiltin("ui.panel"),
			"select":         starlark.NewBuiltin("ui.select", uiSelect), // interactive — keep
			"render":         noopBuiltin("ui.render"),
			"link":           starlark.NewBuiltin("ui.link", makeLinkFunc("")), // returns string — keep
			"pager":          noopBuiltin("ui.pager"),
			"code":           noopBuiltin("ui.code"),
			"diff":           noopBuiltin("ui.diff"),
			"tree":           noopBuiltin("ui.tree"),
			"banner":         noopBuiltin("ui.banner"),
			"progress":       noopBuiltin("ui.progress"),
		},
	}
}

// NewUIModule creates the ui module, auto-detecting whether stdout is a TTY
// and writing directly to os.Stdout.
func NewUIModule() *starlarkstruct.Module {
	return NewUIModuleWithUIWriter(0, nil)
}

// NewUIModuleWithUIWriter creates a UI module wired to a ports.UIWriter.
// When writer is nil or IsTTY() is false, all display functions become no-ops.
func NewUIModuleWithUIWriter(depth int, writer ports.UIWriter) *starlarkstruct.Module {
	isTTY := false
	if writer != nil {
		isTTY = writer.IsTTY()
	}
	if !isTTY {
		return noopUIModule()
	}

	indent := strings.Repeat("| ", depth)
	logW := writer.LogWriter()

	return &starlarkstruct.Module{
		Name: "ui",
		Members: starlark.StringDict{
			// Turn-based conversation model
			"user_turn":      starlark.NewBuiltin("ui.user_turn", makeUserTurnFunc(writer)),
			"assistant_turn": starlark.NewBuiltin("ui.assistant_turn", makeAssistantTurnFunc(writer)),

			// Rich display functions (kept from old API)
			"prompt":       starlark.NewBuiltin("ui.prompt", uiPrompt),
			"confirm":      starlark.NewBuiltin("ui.confirm", uiConfirm),
			"progress":     starlark.NewBuiltin("ui.progress", makeProgressFunc(indent, logW)),
			"progress_bar": starlark.NewBuiltin("ui.progress_bar", makeProgressBarFunc(indent, logW)),
			"markdown":     starlark.NewBuiltin("ui.markdown", makeMarkdownFunc(indent, logW)),
			"table":        starlark.NewBuiltin("ui.table", makeTableFunc(indent, logW)),
			"panel":        starlark.NewBuiltin("ui.panel", makePanelFunc(indent, logW)),
			"select":       starlark.NewBuiltin("ui.select", uiSelect),
			"render":       starlark.NewBuiltin("ui.render", makeRenderFunc(indent, logW)),
			"link":         starlark.NewBuiltin("ui.link", makeLinkFunc(indent)),
			"pager":        starlark.NewBuiltin("ui.pager", makePagerFunc(indent)),
			"code":         starlark.NewBuiltin("ui.code", makeCodeFunc(indent, logW)),
			"diff":         starlark.NewBuiltin("ui.diff", makeDiffFunc(indent, logW)),
			"tree":         starlark.NewBuiltin("ui.tree", makeTreeFunc(indent, logW)),
			"banner":       starlark.NewBuiltin("ui.banner", makeBannerFunc(indent, logW)),
		},
	}
}

// ---------------------------------------------------------------------------
// Turn factory functions
// ---------------------------------------------------------------------------

// openStepFromWriter opens a new step on the given writer, returning a StepHandle.
// It is shared by TurnHandle.step and SubTurnHandle.step.
func openStepFromWriter(funcName string, writer ports.TurnWriter, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackPositionalArgs(funcName, args, kwargs, 1, &text); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if writer == nil {
		return &StepHandle{id: "", writer: nil}, nil
	}
	id := writer.OpenStep(text)
	return &StepHandle{id: id, writer: writer}, nil
}

func makeUserTurnFunc(writer ports.TurnWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var text string
		if err := starlark.UnpackPositionalArgs("ui.user_turn", args, kwargs, 1, &text); err != nil {
			return nil, fmt.Errorf("ui.user_turn: %w", err)
		}
		writer.BeginUserTurn(text)
		return starlark.None, nil
	}
}

func makeAssistantTurnFunc(writer ports.TurnWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackPositionalArgs("ui.assistant_turn", args, kwargs, 0); err != nil {
			return nil, fmt.Errorf("ui.assistant_turn: %w", err)
		}
		writer.BeginAssistantTurn()
		return &TurnHandle{writer: writer}, nil
	}
}

// ---------------------------------------------------------------------------
// TurnHandle — Starlark value for an assistant turn
// ---------------------------------------------------------------------------

// TurnHandle is a Starlark value representing an active assistant turn.
// When writer is nil the handle is a no-op (used on non-TTY).
type TurnHandle struct {
	writer ports.TurnWriter
}

func (t *TurnHandle) String() string { return "<turn>" }

// Type returns the Starlark type name for TurnHandle.
func (t *TurnHandle) Type() string { return "turn" }

// Freeze is a no-op since TurnHandle holds no frozen-mutable state.
func (t *TurnHandle) Freeze() {}

// Truth returns True since a turn handle is always truthy.
func (t *TurnHandle) Truth() starlark.Bool { return starlark.True }

// Hash returns an error since turn handles are not hashable.
func (t *TurnHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: turn") }

// Attr implements starlark.HasAttrs for TurnHandle.
func (t *TurnHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case stepAttr:
		return starlark.NewBuiltin("turn.step", t.step), nil
	case "stream":
		return starlark.NewBuiltin("turn.stream", t.stream), nil
	case doneAttr:
		return starlark.NewBuiltin("turn.done", t.done), nil
	case failAttr:
		return starlark.NewBuiltin("turn.fail", t.fail), nil
	case "info":
		return starlark.NewBuiltin("turn.info", t.info), nil
	case "warn":
		return starlark.NewBuiltin("turn.warn", t.warn), nil
	case "subturn":
		return starlark.NewBuiltin("turn.subturn", t.subturn), nil
	default:
		return nil, nil
	}
}

// AttrNames implements starlark.HasAttrs for TurnHandle.
func (t *TurnHandle) AttrNames() []string {
	return []string{stepAttr, "stream", doneAttr, failAttr, "info", "warn", "subturn"}
}

func (t *TurnHandle) step(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return openStepFromWriter("turn.step", t.writer, args, kwargs)
}

func (t *TurnHandle) stream(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var delta string
	var done bool
	if err := starlark.UnpackArgs("turn.stream", args, kwargs, "delta", &delta, "done?", &done); err != nil {
		return nil, fmt.Errorf("turn.stream: %w", err)
	}
	if t.writer != nil {
		t.writer.StreamToken(delta, done)
	}
	return starlark.None, nil
}

func (t *TurnHandle) done(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	summary := ""
	if err := starlark.UnpackArgs("turn.done", args, kwargs, "summary?", &summary); err != nil {
		return nil, fmt.Errorf("turn.done: %w", err)
	}
	if t.writer != nil {
		t.writer.EndTurn(summary)
	}
	return starlark.None, nil
}

func (t *TurnHandle) fail(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	summary := ""
	if err := starlark.UnpackArgs("turn.fail", args, kwargs, "summary?", &summary); err != nil {
		return nil, fmt.Errorf("turn.fail: %w", err)
	}
	if t.writer != nil {
		t.writer.EndTurn(summary)
	}
	return starlark.None, nil
}

func (t *TurnHandle) info(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackPositionalArgs("turn.info", args, kwargs, 1, &text); err != nil {
		return nil, fmt.Errorf("turn.info: %w", err)
	}
	if t.writer != nil {
		t.writer.SetStatus(text)
	}
	return starlark.None, nil
}

func (t *TurnHandle) warn(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackPositionalArgs("turn.warn", args, kwargs, 1, &text); err != nil {
		return nil, fmt.Errorf("turn.warn: %w", err)
	}
	if t.writer != nil {
		t.writer.SetStatus("! " + text)
	}
	return starlark.None, nil
}

func (t *TurnHandle) subturn(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var label string
	if err := starlark.UnpackPositionalArgs("turn.subturn", args, kwargs, 1, &label); err != nil {
		return nil, fmt.Errorf("turn.subturn: %w", err)
	}
	if t.writer != nil {
		t.writer.BeginSubTurn(label)
	}
	return &SubTurnHandle{writer: t.writer}, nil
}

// ---------------------------------------------------------------------------
// SubTurnHandle — Starlark value for a nested subturn inside an assistant turn
// ---------------------------------------------------------------------------

// SubTurnHandle is a Starlark value for a nested subturn.
// It has the same step/stream/done interface as TurnHandle but routes into
// the subturn opened by BeginSubTurn. Calling done() sends EndSubTurn.
type SubTurnHandle struct {
	writer ports.TurnWriter
}

func (s *SubTurnHandle) String() string { return "<subturn>" }

// Type returns the Starlark type name for SubTurnHandle.
func (s *SubTurnHandle) Type() string { return "subturn" }

// Freeze is a no-op since SubTurnHandle holds no frozen-mutable state.
func (s *SubTurnHandle) Freeze() {}

// Truth returns True since a subturn handle is always truthy.
func (s *SubTurnHandle) Truth() starlark.Bool { return starlark.True }

// Hash returns an error since subturn handles are not hashable.
func (s *SubTurnHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: subturn") }

// Attr implements starlark.HasAttrs for SubTurnHandle.
func (s *SubTurnHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case stepAttr:
		return starlark.NewBuiltin("subturn.step", s.step), nil
	case "stream":
		return starlark.NewBuiltin("subturn.stream", s.stream), nil
	case doneAttr:
		return starlark.NewBuiltin("subturn.done", s.done), nil
	case failAttr:
		return starlark.NewBuiltin("subturn.fail", s.fail), nil
	default:
		return nil, nil
	}
}

// AttrNames implements starlark.HasAttrs for SubTurnHandle.
func (s *SubTurnHandle) AttrNames() []string {
	return []string{stepAttr, "stream", doneAttr, failAttr}
}

func (s *SubTurnHandle) step(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return openStepFromWriter("subturn.step", s.writer, args, kwargs)
}

func (s *SubTurnHandle) stream(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var delta string
	var done bool
	if err := starlark.UnpackArgs("subturn.stream", args, kwargs, "delta", &delta, "done?", &done); err != nil {
		return nil, fmt.Errorf("subturn.stream: %w", err)
	}
	if s.writer != nil {
		s.writer.StreamToken(delta, done)
	}
	return starlark.None, nil
}

func (s *SubTurnHandle) done(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("subturn.done", args, kwargs, 0); err != nil {
		return nil, fmt.Errorf("subturn.done: %w", err)
	}
	if s.writer != nil {
		s.writer.EndSubTurn()
	}
	return starlark.None, nil
}

func (s *SubTurnHandle) fail(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("subturn.fail", args, kwargs, 0); err != nil {
		return nil, fmt.Errorf("subturn.fail: %w", err)
	}
	if s.writer != nil {
		s.writer.EndSubTurn()
	}
	return starlark.None, nil
}

// ---------------------------------------------------------------------------
// StepHandle — Starlark value for a step inside an assistant turn
// ---------------------------------------------------------------------------

// StepHandle is a Starlark value for step operations inside an assistant turn.
// When writer is nil the handle is a no-op (used on non-TTY).
type StepHandle struct {
	writer ports.TurnWriter
	id     string
}

func (s *StepHandle) String() string { return "<step>" }

// Type returns the Starlark type name for StepHandle.
func (s *StepHandle) Type() string { return "step" }

// Freeze is a no-op since StepHandle holds no frozen-mutable state.
func (s *StepHandle) Freeze() {}

// Truth returns True since a step handle is always truthy.
func (s *StepHandle) Truth() starlark.Bool { return starlark.True }

// Hash returns an error since step handles are not hashable.
func (s *StepHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: step") }

// Attr implements starlark.HasAttrs for StepHandle.
func (s *StepHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case doneAttr:
		return starlark.NewBuiltin("step.done", s.done), nil
	case failAttr:
		return starlark.NewBuiltin("step.fail", s.fail), nil
	case "info":
		return starlark.NewBuiltin("step.info", s.info), nil
	case "update":
		return starlark.NewBuiltin("step.update", s.update), nil
	default:
		return nil, nil
	}
}

// AttrNames implements starlark.HasAttrs for StepHandle.
func (s *StepHandle) AttrNames() []string {
	return []string{doneAttr, failAttr, "info", "update"}
}

func (s *StepHandle) done(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	text := ""
	if err := starlark.UnpackArgs("step.done", args, kwargs, "text?", &text); err != nil {
		return nil, fmt.Errorf("step.done: %w", err)
	}
	if s.writer != nil && s.id != "" {
		s.writer.CloseStep(s.id, true, text)
	}
	return starlark.None, nil
}

func (s *StepHandle) fail(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	text := ""
	if err := starlark.UnpackArgs("step.fail", args, kwargs, "text?", &text); err != nil {
		return nil, fmt.Errorf("step.fail: %w", err)
	}
	if s.writer != nil && s.id != "" {
		s.writer.CloseStep(s.id, false, text)
	}
	return starlark.None, nil
}

func (s *StepHandle) info(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackPositionalArgs("step.info", args, kwargs, 1, &text); err != nil {
		return nil, fmt.Errorf("step.info: %w", err)
	}
	if s.writer != nil && s.id != "" {
		s.writer.AddStepInfo(s.id, text)
	}
	return starlark.None, nil
}

func (s *StepHandle) update(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackPositionalArgs("step.update", args, kwargs, 1, &text); err != nil {
		return nil, fmt.Errorf("step.update: %w", err)
	}
	if s.writer != nil && s.id != "" {
		s.writer.UpdateStep(s.id, text)
	}
	return starlark.None, nil
}

// ---------------------------------------------------------------------------
// ProgressBarHandle — kept for progress_bar widget
// ---------------------------------------------------------------------------

// ProgressBarHandle is a Starlark value for progress bar operations.
// When bar is nil the handle is a no-op (used on non-TTY).
type ProgressBarHandle struct {
	bar *ui.ProgressBar
}

func (p *ProgressBarHandle) String() string { return "<progress_bar>" }

// Type returns the Starlark type name for ProgressBarHandle.
func (p *ProgressBarHandle) Type() string { return "progress_bar" }

// Freeze is a no-op since ProgressBarHandle holds no frozen-mutable state.
func (p *ProgressBarHandle) Freeze() {}

// Truth returns True since a progress bar handle is always truthy.
func (p *ProgressBarHandle) Truth() starlark.Bool { return starlark.True }

// Hash returns an error since progress bar handles are not hashable.
func (p *ProgressBarHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: progress_bar") }

// Attr implements starlark.HasAttrs for ProgressBarHandle.
func (p *ProgressBarHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case "inc":
		return starlark.NewBuiltin("progress_bar.inc", p.inc), nil
	case "set":
		return starlark.NewBuiltin("progress_bar.set", p.set), nil
	case doneAttr:
		return starlark.NewBuiltin("progress_bar.done", p.done), nil
	default:
		return nil, nil
	}
}

// AttrNames implements starlark.HasAttrs for ProgressBarHandle.
func (p *ProgressBarHandle) AttrNames() []string {
	return []string{"inc", "set", doneAttr}
}

func (p *ProgressBarHandle) inc(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	amount := 1
	if err := starlark.UnpackArgs("progress_bar.inc", args, kwargs, "amount?", &amount); err != nil {
		return nil, fmt.Errorf("progress_bar.inc: %w", err)
	}
	if p.bar != nil {
		p.bar.Inc(amount)
	}
	return starlark.None, nil
}

func (p *ProgressBarHandle) set(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value int
	if err := starlark.UnpackPositionalArgs("progress_bar.set", args, kwargs, 1, &value); err != nil {
		return nil, fmt.Errorf("progress_bar.set: %w", err)
	}
	if p.bar != nil {
		p.bar.Set(value)
	}
	return starlark.None, nil
}

func (p *ProgressBarHandle) done(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	message := "Complete"
	if err := starlark.UnpackArgs("progress_bar.done", args, kwargs, "message?", &message); err != nil {
		return nil, fmt.Errorf("progress_bar.done: %w", err)
	}
	if p.bar != nil {
		p.bar.Done(message)
	}
	return starlark.None, nil
}

// ---------------------------------------------------------------------------
// Display functions (kept from old API)
// ---------------------------------------------------------------------------

// writeLine writes text followed by a newline to the given writer, returning any error.
func writeLine(w interface{ Write([]byte) (int, error) }, text string) error {
	_, err := fmt.Fprintln(w, text)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	return nil
}

func makeProgressFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			message string
			current int
			total   int
		)
		if err := starlark.UnpackArgs("ui.progress", args, kwargs, "message", &message, "current?", &current, "total?", &total); err != nil {
			return nil, fmt.Errorf("ui.progress: %w", err)
		}

		var text string
		if total > 0 {
			percentage := float64(current) / float64(total) * 100
			text = fmt.Sprintf("%s [%d/%d] %.0f%%", message, current, total, percentage)
		} else {
			text = message
		}
		if err := writeLine(writer, ui.IndentLines(text, indent)); err != nil {
			return nil, fmt.Errorf("ui.progress write failed: %w", err)
		}
		return starlark.None, nil
	}
}

func makeMarkdownFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		if err := starlark.UnpackPositionalArgs("ui.markdown", args, kwargs, 1, &content); err != nil {
			return nil, fmt.Errorf("ui.markdown: %w", err)
		}
		rendered, err := ui.RenderMarkdown(content, ui.TerminalWidth(120), false)
		if err != nil {
			rendered = content
		}
		if err := writeLine(writer, ui.IndentLines(rendered, indent)); err != nil {
			return nil, fmt.Errorf("ui.markdown write failed: %w", err)
		}
		return starlark.None, nil
	}
}

func makeTableFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var data starlark.Value
		var columnsVal starlark.Value
		var title string
		var query string
		if err := starlark.UnpackArgs("ui.table", args, kwargs, "data", &data, "columns", &columnsVal, "title?", &title, "query?", &query); err != nil {
			return nil, fmt.Errorf("ui.table: %w", err)
		}

		columns, err := parseStringList(columnsVal)
		if err != nil {
			return nil, err
		}
		rows, err := dataToRows(data, columns)
		if err != nil {
			return nil, err
		}

		rendered := ui.RenderTable(rows, columns, ui.TableOptions{
			Title:    title,
			Query:    query,
			MaxWidth: ui.TerminalWidth(120),
			Theme:    defaultTheme,
			Opts:     defaultRenderOptions,
		})
		if err := writeLine(writer, ui.IndentLines(rendered, indent)); err != nil {
			return nil, fmt.Errorf("ui.table write failed: %w", err)
		}
		return starlark.None, nil
	}
}

func makePanelFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		var title string
		var style string
		if err := starlark.UnpackArgs("ui.panel", args, kwargs, "content", &content, "title?", &title, "style?", &style); err != nil {
			return nil, fmt.Errorf("ui.panel: %w", err)
		}

		rendered := ui.RenderPanel(content, title, style, defaultTheme, defaultRenderOptions)
		if err := writeLine(writer, ui.IndentLines(rendered, indent)); err != nil {
			return nil, fmt.Errorf("ui.panel write failed: %w", err)
		}
		return starlark.None, nil
	}
}

// uiPrompt prompts the user for input.
func uiPrompt(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit // complexity inherent in interactive prompt with validation and sensitive input handling
	var message string
	var defaultValue string
	var isSensitive bool
	var validateFunc starlark.Callable

	if err := starlark.UnpackArgs("ui.prompt", args, kwargs,
		"message", &message,
		"default?", &defaultValue,
		"is_sensitive?", &isSensitive,
		"validate?", &validateFunc,
	); err != nil {
		return nil, fmt.Errorf("ui.prompt: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", message, defaultValue)
		} else {
			fmt.Print(message + ": ")
		}

		var input string
		var err error

		if isSensitive { //nolint:nestif // nested input handling for sensitive vs plain text with validation
			var inputVal string
			field := huh.NewInput().
				Title(message).
				EchoMode(huh.EchoModePassword).
				Value(&inputVal)
			if err := huh.NewForm(huh.NewGroup(field)).Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return starlark.None, nil
				}
				return nil, fmt.Errorf("prompt failed: %w", err)
			}
			input = inputVal
		} else {
			input, err = reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("read input failed: %w", err)
			}
		}

		input = strings.TrimSpace(input)

		if input == "" && defaultValue != "" {
			input = defaultValue
		}

		if validateFunc != nil { //nolint:nestif // nested validation function call requires nested error handling
			result, err := starlark.Call(thread, validateFunc, starlark.Tuple{starlark.String(input)}, nil)
			if err != nil {
				return nil, fmt.Errorf("validation function error: %w", err)
			}

			if result != starlark.None {
				errorMsg, ok := starlark.AsString(result)
				if !ok {
					return nil, fmt.Errorf("validator must return string or None")
				}
				fmt.Fprintf(os.Stderr, "✗ %s\n", errorMsg) // validation error message is safe to display
				continue                                   // Ask again
			}
		}

		return starlark.String(input), nil
	}
}

// uiConfirm prompts the user for Y/n confirmation.
func uiConfirm(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var prompt string
	var defaultValue bool

	if err := starlark.UnpackArgs("ui.confirm", args, kwargs,
		"prompt", &prompt,
		"default?", &defaultValue,
	); err != nil {
		return nil, fmt.Errorf("ui.confirm: %w", err)
	}

	result, err := ui.Confirm(prompt, defaultValue)
	if err != nil {
		return nil, fmt.Errorf("confirm failed: %w", err)
	}

	return starlark.Bool(result), nil
}

func uiSelect(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit // complexity inherent in interactive selection with multi-select support
	var prompt string
	var itemsVal starlark.Value
	var allowMultiple bool
	var isFuzzy bool
	var limit int
	var placeholder string
	var initialQuery string
	var allowNew bool
	var shouldReturnIndex bool
	var labelKey string
	var valueKey string
	var metaKey string
	if err := starlark.UnpackArgs("ui.select", args, kwargs,
		"prompt", &prompt,
		"items", &itemsVal,
		"allow_multiple?", &allowMultiple,
		"is_fuzzy?", &isFuzzy,
		"limit?", &limit,
		"placeholder?", &placeholder,
		"initial_query?", &initialQuery,
		"allow_new?", &allowNew,
		"should_return_index?", &shouldReturnIndex,
		"label_key?", &labelKey,
		"value_key?", &valueKey,
		"meta_key?", &metaKey,
	); err != nil {
		return nil, fmt.Errorf("ui.select: %w", err)
	}

	if labelKey == "" {
		labelKey = "label"
	}
	if valueKey == "" {
		valueKey = "value"
	}
	if metaKey == "" {
		metaKey = "meta"
	}

	items, err := dataToSelectItems(itemsVal, labelKey, valueKey, metaKey)
	if err != nil {
		return nil, err
	}

	result, err := ui.RunSelect(ui.SelectOptions{
		Title:        prompt,
		Items:        items,
		Multi:        allowMultiple,
		Fuzzy:        isFuzzy,
		Limit:        limit,
		Placeholder:  placeholder,
		InitialQuery: initialQuery,
		AllowNew:     allowNew,
		ReturnIndex:  shouldReturnIndex,
		Theme:        defaultTheme,
	})
	if err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}
	if result.Canceled {
		return starlark.None, nil
	}
	if result.NewValue != "" {
		return starlark.String(result.NewValue), nil
	}

	if allowMultiple {
		list := new(starlark.List)
		for _, item := range result.Items {
			if shouldReturnIndex {
				_ = list.Append(starlark.MakeInt(item.Index)) //nolint:errcheck // starlark list operations with known-compatible types
			} else {
				_ = list.Append(starlark.String(item.Value)) //nolint:errcheck // starlark list operations with known-compatible types
			}
		}
		return list, nil
	}

	if len(result.Items) == 0 {
		return starlark.None, nil
	}
	if shouldReturnIndex {
		return starlark.MakeInt(result.Items[0].Index), nil
	}
	return starlark.String(result.Items[0].Value), nil
}

func makeRenderFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value starlark.Value
		var query string
		if err := starlark.UnpackArgs("ui.render", args, kwargs, "value", &value, "query?", &query); err != nil {
			return nil, fmt.Errorf("ui.render: %w", err)
		}

		rendered, err := renderAuto(value, query)
		if err != nil {
			return nil, err
		}
		if err := writeLine(writer, ui.IndentLines(rendered, indent)); err != nil {
			return nil, fmt.Errorf("ui.render write failed: %w", err)
		}
		return starlark.None, nil
	}
}

func renderAuto(value starlark.Value, query string) (string, error) { //nolint:gocognit,gocyclo // complexity inherent in rendering multiple Starlark value types
	if value == nil || value == starlark.None {
		return "", nil
	}

	if text, ok := starlark.AsString(value); ok {
		rendered, err := ui.RenderMarkdown(text, ui.TerminalWidth(120), false)
		if err == nil {
			return rendered, nil
		}
		return text, nil
	}

	if list, ok := value.(*starlark.List); ok { //nolint:nestif // nested type dispatch requires nested value processing
		if list.Len() == 0 {
			return "", nil
		}
		item := list.Index(0)
		if hasKey(item, "file_path") && hasKey(item, "score") {
			columns := []string{"file_path", "score"}
			if hasKey(item, "snapshot") {
				columns = append(columns, "snapshot")
			}
			rows, err := dataToRows(list, columns)
			if err != nil {
				return "", err
			}
			return ui.RenderTable(rows, columns, ui.TableOptions{
				Title:    "Search results",
				Query:    query,
				MaxWidth: ui.TerminalWidth(120),
				Theme:    defaultTheme,
			}), nil
		}

		columns, err := inferColumns(item)
		if err != nil || len(columns) == 0 {
			columns = []string{"value"}
		}
		rows, err := dataToRows(list, columns)
		if err != nil {
			return "", err
		}
		return ui.RenderTable(rows, columns, ui.TableOptions{
			Query:    query,
			MaxWidth: ui.TerminalWidth(120),
			Theme:    defaultTheme,
		}), nil
	}

	if hasKey(value, "raw") {
		if raw, ok := getValue(value, "raw"); ok {
			rendered := ui.RenderDiff(raw, defaultTheme, defaultRenderOptions)
			return rendered, nil
		}
	}

	return value.String(), nil
}

func parseStringList(value starlark.Value) ([]string, error) {
	switch v := value.(type) {
	case *starlark.List:
		items := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i)
			text, ok := starlark.AsString(item)
			if !ok {
				return nil, fmt.Errorf("columns must be strings")
			}
			items = append(items, text)
		}
		return items, nil
	case starlark.Tuple:
		items := make([]string, 0, v.Len())
		for _, item := range v {
			text, ok := starlark.AsString(item)
			if !ok {
				return nil, fmt.Errorf("columns must be strings")
			}
			items = append(items, text)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("columns must be a list")
	}
}

func dataToRows(value starlark.Value, columns []string) ([]map[string]string, error) {
	rows := []map[string]string{}
	list, ok := value.(*starlark.List)
	if ok {
		for i := 0; i < list.Len(); i++ {
			row, err := rowFromValue(list.Index(i), columns)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		return rows, nil
	}
	return nil, fmt.Errorf("data must be a list")
}

func rowFromValue(value starlark.Value, columns []string) (map[string]string, error) { //nolint:gocognit // complexity inherent in mapping Starlark value types to table row
	row := map[string]string{}
	switch v := value.(type) {
	case starlark.String:
		if len(columns) == 1 {
			row[columns[0]] = string(v)
			return row, nil
		}
		return nil, fmt.Errorf("string rows require a single column")
	case *starlark.Dict:
		for _, col := range columns {
			if val, ok := dictGetString(v, col); ok {
				row[col] = val
			} else {
				row[col] = ""
			}
		}
		return row, nil
	case *starlarkstruct.Struct:
		for _, col := range columns {
			if val, ok := structGetString(v, col); ok {
				row[col] = val
			} else {
				row[col] = ""
			}
		}
		return row, nil
	default:
		for _, col := range columns {
			row[col] = valueToString(value)
		}
		return row, nil
	}
}

func dataToSelectItems(value starlark.Value, labelKey, valueKey, metaKey string) ([]ui.SelectItem, error) {
	items := []ui.SelectItem{}
	list, ok := value.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("items must be a list")
	}
	for i := 0; i < list.Len(); i++ {
		item := list.Index(i)
		switch v := item.(type) {
		case starlark.String:
			text := string(v)
			items = append(items, ui.SelectItem{Label: text, Value: text})
		case *starlark.Dict:
			label := dictGetStringOrFallback(v, labelKey, valueKey)
			val := dictGetStringOrFallback(v, valueKey, labelKey)
			meta := dictGetStringOrFallback(v, metaKey, "")
			items = append(items, ui.SelectItem{Label: label, Value: val, Meta: meta})
		case *starlarkstruct.Struct:
			label := structGetStringOrFallback(v, labelKey, valueKey)
			val := structGetStringOrFallback(v, valueKey, labelKey)
			meta := structGetStringOrFallback(v, metaKey, "")
			items = append(items, ui.SelectItem{Label: label, Value: val, Meta: meta})
		default:
			text := valueToString(item)
			items = append(items, ui.SelectItem{Label: text, Value: text})
		}
	}
	return items, nil
}

func valueToString(value starlark.Value) string {
	if value == nil {
		return ""
	}
	if s, ok := starlark.AsString(value); ok {
		return s
	}
	return value.String()
}

func dictGetString(dict *starlark.Dict, key string) (string, bool) {
	val, ok, err := dict.Get(starlark.String(key))
	if err != nil || !ok {
		return "", false
	}
	return valueToString(val), true
}

func dictGetStringOrFallback(dict *starlark.Dict, key string, fallback string) string {
	if key == "" {
		if fallback == "" {
			return ""
		}
		value, _ := dictGetString(dict, fallback)
		return value
	}
	value, ok := dictGetString(dict, key)
	if ok {
		return value
	}
	if fallback == "" {
		return ""
	}
	value, _ = dictGetString(dict, fallback)
	return value
}

func structGetString(value *starlarkstruct.Struct, key string) (string, bool) {
	val, err := value.Attr(key)
	if err != nil || val == nil {
		return "", false
	}
	return valueToString(val), true
}

func structGetStringOrFallback(value *starlarkstruct.Struct, key string, fallback string) string {
	if key == "" {
		if fallback == "" {
			return ""
		}
		val, _ := structGetString(value, fallback)
		return val
	}
	val, ok := structGetString(value, key)
	if ok {
		return val
	}
	if fallback == "" {
		return ""
	}
	val, _ = structGetString(value, fallback)
	return val
}

func hasKey(value starlark.Value, key string) bool {
	switch v := value.(type) {
	case *starlark.Dict:
		_, ok, _ := v.Get(starlark.String(key)) //nolint:errcheck // starlark dict lookup; error only on unhashable key
		return ok
	case *starlarkstruct.Struct:
		val, err := v.Attr(key)
		return err == nil && val != nil
	default:
		return false
	}
}

func getValue(value starlark.Value, key string) (string, bool) {
	switch v := value.(type) {
	case *starlark.Dict:
		return dictGetString(v, key)
	case *starlarkstruct.Struct:
		return structGetString(v, key)
	default:
		return "", false
	}
}

func inferColumns(value starlark.Value) ([]string, error) {
	switch v := value.(type) {
	case *starlark.Dict:
		keys := v.Keys()
		cols := make([]string, 0, len(keys))
		for _, key := range keys {
			if s, ok := starlark.AsString(key); ok {
				cols = append(cols, s)
			}
		}
		return cols, nil
	case *starlarkstruct.Struct:
		fields := v.AttrNames()
		cols := make([]string, 0, len(fields))
		cols = append(cols, fields...)
		return cols, nil
	default:
		return nil, fmt.Errorf("unsupported type")
	}
}

// makeCodeFunc creates the ui.code() function.
func makeCodeFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content, language, title string
		var maxLines int
		if err := starlark.UnpackArgs("ui.code", args, kwargs, "content", &content, "lang?", &language, "title?", &title, "max_lines?", &maxLines); err != nil {
			return nil, fmt.Errorf("ui.code: %w", err)
		}
		if language == "" {
			language = "text"
		}
		rendered := ui.RenderCodeWithMaxLines(content, language, title, maxLines, defaultTheme, defaultRenderOptions)
		_, _ = fmt.Fprintln(writer, ui.IndentLines(rendered, indent)) //nolint:errcheck // write errors to output are intentionally ignored
		return starlark.None, nil
	}
}

// makeDiffFunc creates the ui.diff() function.
func makeDiffFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content, title string
		var maxLines int
		if err := starlark.UnpackArgs("ui.diff", args, kwargs, "content", &content, "title?", &title, "max_lines?", &maxLines); err != nil {
			return nil, fmt.Errorf("ui.diff: %w", err)
		}
		rendered := ui.RenderDiffEnhancedWithMaxLines(content, title, maxLines, defaultTheme, defaultRenderOptions)
		_, _ = fmt.Fprintln(writer, ui.IndentLines(rendered, indent)) //nolint:errcheck // write errors to output are intentionally ignored
		return starlark.None, nil
	}
}

// makeTreeFunc creates the ui.tree() function.
func makeTreeFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var dataVal starlark.Value
		var title string
		if err := starlark.UnpackArgs("ui.tree", args, kwargs, "data", &dataVal, "title?", &title); err != nil {
			return nil, fmt.Errorf("ui.tree: %w", err)
		}

		data := starlarkToGo(dataVal)

		rendered := ui.RenderTree(data, title, defaultTheme, defaultRenderOptions)
		_, _ = fmt.Fprintln(writer, ui.IndentLines(rendered, indent)) //nolint:errcheck // write errors to output are intentionally ignored
		return starlark.None, nil
	}
}

// makeProgressBarFunc creates the ui.progress_bar() function.
func makeProgressBarFunc(_ string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var total int
		var message string
		if err := starlark.UnpackArgs("ui.progress_bar", args, kwargs, "total", &total, "message?", &message); err != nil {
			return nil, fmt.Errorf("ui.progress_bar: %w", err)
		}
		if message == "" {
			message = "Progress"
		}

		bar := ui.NewProgressBar(total, message, defaultTheme, defaultRenderOptions, writer)
		return &ProgressBarHandle{bar: bar}, nil
	}
}

// makeBannerFunc creates the ui.banner() function.
func makeBannerFunc(indent string, writer interface{ Write([]byte) (int, error) }) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var title, subtext string
		if err := starlark.UnpackArgs("ui.banner", args, kwargs, "title", &title, "subtext?", &subtext); err != nil {
			return nil, fmt.Errorf("ui.banner: %w", err)
		}

		rendered := ui.RenderBanner(title, subtext, defaultTheme, defaultRenderOptions)
		_, _ = fmt.Fprintln(writer, ui.IndentLines(rendered, indent)) //nolint:errcheck // write errors to output are intentionally ignored
		return starlark.None, nil
	}
}

// makeLinkFunc creates the ui.link() function.
func makeLinkFunc(_ string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var text, url string
		if err := starlark.UnpackArgs("ui.link", args, kwargs, "text", &text, "url", &url); err != nil {
			return nil, fmt.Errorf("ui.link: %w", err)
		}

		rendered := ui.RenderLink(text, url, defaultRenderOptions)
		return starlark.String(rendered), nil
	}
}

// makePagerFunc creates the ui.pager() function.
func makePagerFunc(_ string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content, title string
		var showLineNumbers bool
		if err := starlark.UnpackArgs("ui.pager", args, kwargs, "content", &content, "title?", &title, "show_line_numbers?", &showLineNumbers); err != nil {
			return nil, fmt.Errorf("ui.pager: %w", err)
		}

		err := ui.RenderWithPager(content, title, showLineNumbers, defaultRenderOptions)
		if err != nil {
			return nil, fmt.Errorf("pager failed: %w", err)
		}
		return starlark.None, nil
	}
}
