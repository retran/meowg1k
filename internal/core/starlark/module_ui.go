// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/retran/meowg1k/internal/ui"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var (
	defaultRenderOptions = ui.NewRenderOptions()
	defaultTheme         = ui.DefaultThemeWithOptions(defaultRenderOptions)
	currentActivity      *ui.Activity
	activityMu           sync.Mutex
)

// StreamSender is implemented by the output service to forward streaming token
// deltas to the TUI program.  On non-TTY the implementation is a no-op.
type StreamSender interface {
	StreamToken(delta string, done bool)
}

// noopStreamSender discards all stream tokens (used on non-TTY).
type noopStreamSender struct{}

func (noopStreamSender) StreamToken(_ string, _ bool) {}

// noopBuiltin returns a Starlark builtin that accepts any arguments and returns None.
func noopBuiltin(name string) *starlark.Builtin {
	return starlark.NewBuiltin(name, func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})
}

// noopActivityFunc returns a function that creates a no-op ActivityHandle.
// Scripts that call activity.success(...) / .fail(...) / .done() / .update(...) won't crash.
func noopActivityFunc() func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return &ActivityHandle{activity: nil}, nil
	}
}

// noopStepFunc returns a function that creates a no-op StepHandle.
// Scripts that call step.done(...) / .fail(...) won't crash.
func noopStepFunc() func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return &StepHandle{step: nil}, nil
	}
}

// noopProgressBarFunc returns a function that creates a no-op ProgressBarHandle.
// Scripts that call bar.inc() / .set(...) / .done(...) won't crash.
func noopProgressBarFunc() func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return &ProgressBarHandle{bar: nil}, nil
	}
}

// noopUIModule returns a ui module where every function is a no-op.
// Used when stdout is not a TTY so UI chrome is suppressed entirely.
func noopUIModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "ui",
		Members: starlark.StringDict{
			"success":        noopBuiltin("ui.success"),
			"error":          noopBuiltin("ui.error"),
			"warn":           noopBuiltin("ui.warn"),
			"info":           noopBuiltin("ui.info"),
			"prompt":         starlark.NewBuiltin("ui.prompt", uiPrompt),   // interactive — keep
			"confirm":        starlark.NewBuiltin("ui.confirm", uiConfirm), // interactive — keep
			"progress":       noopBuiltin("ui.progress"),
			"progress_bar":   starlark.NewBuiltin("ui.progress_bar", noopProgressBarFunc()),
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
			"divider":        noopBuiltin("ui.divider"),
			"activity":       starlark.NewBuiltin("ui.activity", noopActivityFunc()),
			"banner":         noopBuiltin("ui.banner"),
			"step":           starlark.NewBuiltin("ui.step", noopStepFunc()),
			"action":         noopBuiltin("ui.action"),
			"think":          noopBuiltin("ui.think"),
			"stream":         noopBuiltin("ui.stream"),
			"DIVIDER_THICK":  starlark.String("thick"),
			"DIVIDER_THIN":   starlark.String("thin"),
			"DIVIDER_DOUBLE": starlark.String("double"),
		},
	}
}

// NewUIModule creates the ui module, auto-detecting whether stdout is a TTY
// and writing directly to os.Stdout.
func NewUIModule() *starlarkstruct.Module {
	return NewUIModuleWithWriter(0, ui.IsTerminal(os.Stdout.Fd()), os.Stdout, noopStreamSender{})
}

// NewIndentedUIModule creates a UI module with indentation for nested commands.
func NewIndentedUIModule(depth int) *starlarkstruct.Module {
	return NewUIModuleWithWriter(depth, ui.IsTerminal(os.Stdout.Fd()), os.Stdout, noopStreamSender{})
}

// NewUIModuleWithOptions creates a UI module. When isTTY is false all display
// functions become no-ops so scripts run cleanly when piped or redirected.
// Deprecated: prefer NewUIModuleWithWriter which accepts an explicit writer.
func NewUIModuleWithOptions(depth int, isTTY bool) *starlarkstruct.Module {
	return NewUIModuleWithWriter(depth, isTTY, os.Stdout, noopStreamSender{})
}

// NewUIModuleWithWriter creates a UI module that routes all output through
// the provided writer and forwards stream tokens through streamer.
// When isTTY is false all display functions become no-ops.
// Pass OutputService.LogWriter() as writer and OutputService as streamer so
// that all output goes through the same BubbleTea program as streaming.
func NewUIModuleWithWriter(depth int, isTTY bool, writer io.Writer, streamer StreamSender) *starlarkstruct.Module {
	if !isTTY {
		return noopUIModule()
	}

	indent := strings.Repeat("| ", depth)

	return &starlarkstruct.Module{
		Name: "ui",
		Members: starlark.StringDict{
			// Functions
			"success":      starlark.NewBuiltin("ui.success", makeStatusFunc(indent, defaultTheme.StatusSuccess, "✓ ", writer)),
			"error":        starlark.NewBuiltin("ui.error", makeStatusFunc(indent, defaultTheme.StatusError, "✗ ", writer)),
			"warn":         starlark.NewBuiltin("ui.warn", makeStatusFunc(indent, defaultTheme.StatusWarn, "! ", writer)),
			"info":         starlark.NewBuiltin("ui.info", makeStatusFunc(indent, defaultTheme.StatusInfo, "· ", writer)),
			"prompt":       starlark.NewBuiltin("ui.prompt", uiPrompt),
			"confirm":      starlark.NewBuiltin("ui.confirm", uiConfirm),
			"progress":     starlark.NewBuiltin("ui.progress", makeProgressFunc(indent, writer)),
			"progress_bar": starlark.NewBuiltin("ui.progress_bar", makeProgressBarFunc(indent, writer)),
			"markdown":     starlark.NewBuiltin("ui.markdown", makeMarkdownFunc(indent, writer)),
			"table":        starlark.NewBuiltin("ui.table", makeTableFunc(indent, writer)),
			"panel":        starlark.NewBuiltin("ui.panel", makePanelFunc(indent, writer)),
			"select":       starlark.NewBuiltin("ui.select", uiSelect),
			"render":       starlark.NewBuiltin("ui.render", makeRenderFunc(indent, writer)),
			"link":         starlark.NewBuiltin("ui.link", makeLinkFunc(indent)),
			"pager":        starlark.NewBuiltin("ui.pager", makePagerFunc(indent)),
			"code":         starlark.NewBuiltin("ui.code", makeCodeFunc(indent, writer)),
			"diff":         starlark.NewBuiltin("ui.diff", makeDiffFunc(indent, writer)),
			"tree":         starlark.NewBuiltin("ui.tree", makeTreeFunc(indent, writer)),
			"divider":      starlark.NewBuiltin("ui.divider", makeDividerFunc(indent, writer)),
			"activity":     starlark.NewBuiltin("ui.activity", makeActivityFunc(indent, writer)),
			"banner":       starlark.NewBuiltin("ui.banner", makeBannerFunc(indent, writer)),
			"step":         starlark.NewBuiltin("ui.step", makeStepFunc(depth, writer)),
			"action":       starlark.NewBuiltin("ui.action", makeActionFunc(indent, writer)),
			"think":        starlark.NewBuiltin("ui.think", makeThinkFunc(indent, writer)),
			"stream":       starlark.NewBuiltin("ui.stream", makeStreamFunc(streamer)),

			// Constants
			"DIVIDER_THICK":  starlark.String("thick"),
			"DIVIDER_THIN":   starlark.String("thin"),
			"DIVIDER_DOUBLE": starlark.String("double"),
		},
	}
}

func makeStatusFunc(indent string, style lipgloss.Style, prefix string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var message string
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &message); err != nil {
			return nil, err
		}
		text := style.Render(prefix + message)
		fmt.Fprintln(writer, ui.IndentLines(text, indent))
		return starlark.None, nil
	}
}

func makeProgressFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			message string
			current int
			total   int
		)
		if err := starlark.UnpackArgs("ui.progress", args, kwargs, "message", &message, "current?", &current, "total?", &total); err != nil {
			return nil, err
		}

		var text string
		if total > 0 {
			percentage := float64(current) / float64(total) * 100
			text = fmt.Sprintf("%s [%d/%d] %.0f%%", message, current, total, percentage)
		} else {
			text = message
		}
		fmt.Fprintln(writer, ui.IndentLines(text, indent))
		return starlark.None, nil
	}
}

func makeMarkdownFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		if err := starlark.UnpackPositionalArgs("ui.markdown", args, kwargs, 1, &content); err != nil {
			return nil, err
		}
		rendered, err := ui.RenderMarkdown(content, ui.TerminalWidth(120), false)
		if err != nil {
			rendered = content
		}
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

func makeTableFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var data starlark.Value
		var columnsVal starlark.Value
		var title string
		var query string
		if err := starlark.UnpackArgs("ui.table", args, kwargs, "data", &data, "columns", &columnsVal, "title?", &title, "query?", &query); err != nil {
			return nil, err
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
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

func makePanelFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		var title string
		var style string
		if err := starlark.UnpackArgs("ui.panel", args, kwargs, "content", &content, "title?", &title, "style?", &style); err != nil {
			return nil, err
		}

		rendered := ui.RenderPanel(content, title, style, defaultTheme, defaultRenderOptions)
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

// uiPrompt prompts the user for input.
func uiPrompt(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		return nil, err
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		// Show prompt
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", message, defaultValue)
		} else {
			fmt.Print(message + ": ")
		}

		var input string
		var err error

		// Read input (with or without masking)
		if isSensitive {
			// Use huh for masked/password input
			var inputVal string
			field := huh.NewInput().
				Title(message).
				EchoMode(huh.EchoModePassword).
				Value(&inputVal)
			if err := huh.NewForm(huh.NewGroup(field)).Run(); err != nil {
				if err == huh.ErrUserAborted {
					return starlark.None, nil
				}
				return nil, err
			}
			input = inputVal
		} else {
			// Regular input
			input, err = reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
		}

		input = strings.TrimSpace(input)

		// Use default if empty
		if input == "" && defaultValue != "" {
			input = defaultValue
		}

		// Validate if validator provided
		if validateFunc != nil {
			result, err := starlark.Call(thread, validateFunc, starlark.Tuple{starlark.String(input)}, nil)
			if err != nil {
				return nil, fmt.Errorf("validation function error: %w", err)
			}

			// If validator returns non-None, it's an error message
			if result != starlark.None {
				errorMsg, ok := starlark.AsString(result)
				if !ok {
					return nil, fmt.Errorf("validator must return string or None")
				}
				fmt.Fprintf(os.Stderr, "✗ %s\n", errorMsg)
				continue // Ask again
			}
		}

		// Valid input
		return starlark.String(input), nil
	}
}

// uiConfirm prompts the user for Y/n confirmation.
func uiConfirm(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var prompt string
	var defaultValue bool

	if err := starlark.UnpackArgs("ui.confirm", args, kwargs,
		"prompt", &prompt,
		"default?", &defaultValue,
	); err != nil {
		return nil, err
	}

	result, err := ui.Confirm(prompt, defaultValue)
	if err != nil {
		return nil, err
	}

	return starlark.Bool(result), nil
}

func uiSelect(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
		return nil, err
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
		return nil, err
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
				list.Append(starlark.MakeInt(item.Index))
			} else {
				list.Append(starlark.String(item.Value))
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

func makeRenderFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value starlark.Value
		var query string
		if err := starlark.UnpackArgs("ui.render", args, kwargs, "value", &value, "query?", &query); err != nil {
			return nil, err
		}

		rendered, err := renderAuto(value, query)
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

func renderAuto(value starlark.Value, query string) (string, error) {
	if value == nil || value == starlark.None {
		return "", nil
	}

	if text, ok := starlark.AsString(value); ok {
		rendered, err := ui.RenderMarkdown(text, ui.TerminalWidth(120), false)
		if err != nil {
			return text, nil
		}
		return rendered, nil
	}

	if list, ok := value.(*starlark.List); ok {
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

func rowFromValue(value starlark.Value, columns []string) (map[string]string, error) {
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
			value := dictGetStringOrFallback(v, valueKey, labelKey)
			meta := dictGetStringOrFallback(v, metaKey, "")
			items = append(items, ui.SelectItem{Label: label, Value: value, Meta: meta})
		case *starlarkstruct.Struct:
			label := structGetStringOrFallback(v, labelKey, valueKey)
			value := structGetStringOrFallback(v, valueKey, labelKey)
			meta := structGetStringOrFallback(v, metaKey, "")
			items = append(items, ui.SelectItem{Label: label, Value: value, Meta: meta})
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
		_, ok, _ := v.Get(starlark.String(key))
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
		for _, field := range fields {
			cols = append(cols, field)
		}
		return cols, nil
	default:
		return nil, fmt.Errorf("unsupported type")
	}
}

// makeDividerFunc creates the ui.divider() function.
func makeDividerFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		style := "line"
		if err := starlark.UnpackArgs("ui.divider", args, kwargs, "style?", &style); err != nil {
			return nil, err
		}
		ui.LogDivider(style, defaultTheme, defaultRenderOptions, writer)
		return starlark.None, nil
	}
}

// makeCodeFunc creates the ui.code() function.
func makeCodeFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content, language, title string
		var maxLines int
		if err := starlark.UnpackArgs("ui.code", args, kwargs, "content", &content, "lang?", &language, "title?", &title, "max_lines?", &maxLines); err != nil {
			return nil, err
		}
		if language == "" {
			language = "text"
		}
		rendered := ui.RenderCodeWithMaxLines(content, language, title, maxLines, defaultTheme, defaultRenderOptions)
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

// makeDiffFunc creates the ui.diff() function.
func makeDiffFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content, title string
		var maxLines int
		if err := starlark.UnpackArgs("ui.diff", args, kwargs, "content", &content, "title?", &title, "max_lines?", &maxLines); err != nil {
			return nil, err
		}
		rendered := ui.RenderDiffEnhancedWithMaxLines(content, title, maxLines, defaultTheme, defaultRenderOptions)
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

// makeTreeFunc creates the ui.tree() function.
func makeTreeFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var dataVal starlark.Value
		var title string
		if err := starlark.UnpackArgs("ui.tree", args, kwargs, "data", &dataVal, "title?", &title); err != nil {
			return nil, err
		}

		// Convert Starlark value to Go interface{}
		data := starlarkToGo(dataVal)

		rendered := ui.RenderTree(data, title, defaultTheme, defaultRenderOptions)
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

// ActivityHandle is a Starlark value for activity operations.
// When activity is nil the handle is a no-op (used on non-TTY).
type ActivityHandle struct {
	activity *ui.Activity
}

func (a *ActivityHandle) String() string        { return "<activity>" }
func (a *ActivityHandle) Type() string          { return "activity" }
func (a *ActivityHandle) Freeze()               {}
func (a *ActivityHandle) Truth() starlark.Bool  { return starlark.True }
func (a *ActivityHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: activity") }

func (a *ActivityHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case "update":
		return starlark.NewBuiltin("activity.update", a.update), nil
	case "success":
		return starlark.NewBuiltin("activity.success", a.success), nil
	case "fail":
		return starlark.NewBuiltin("activity.fail", a.fail), nil
	case "done":
		return starlark.NewBuiltin("activity.done", a.done), nil
	default:
		return nil, nil
	}
}

func (a *ActivityHandle) AttrNames() []string {
	return []string{"update", "success", "fail", "done"}
}

func (a *ActivityHandle) update(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string
	if err := starlark.UnpackPositionalArgs("activity.update", args, kwargs, 1, &message); err != nil {
		return nil, err
	}
	if a.activity != nil {
		a.activity.Update(message)
	}
	return starlark.None, nil
}

func (a *ActivityHandle) success(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string
	if err := starlark.UnpackPositionalArgs("activity.success", args, kwargs, 1, &message); err != nil {
		return nil, err
	}
	if a.activity != nil {
		a.activity.Success(message)

		// Clear current activity if this is it
		activityMu.Lock()
		if currentActivity == a.activity {
			currentActivity = nil
		}
		activityMu.Unlock()
	}
	return starlark.None, nil
}

func (a *ActivityHandle) fail(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string
	if err := starlark.UnpackPositionalArgs("activity.fail", args, kwargs, 1, &message); err != nil {
		return nil, err
	}
	if a.activity != nil {
		a.activity.Fail(message)

		// Clear current activity if this is it
		activityMu.Lock()
		if currentActivity == a.activity {
			currentActivity = nil
		}
		activityMu.Unlock()
	}
	return starlark.None, nil
}

func (a *ActivityHandle) done(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("activity.done", args, kwargs, 0); err != nil {
		return nil, err
	}
	if a.activity != nil {
		a.activity.Done()

		// Clear current activity if this is it
		activityMu.Lock()
		if currentActivity == a.activity {
			currentActivity = nil
		}
		activityMu.Unlock()
	}
	return starlark.None, nil
}

// makeActivityFunc creates the ui.activity() function.
func makeActivityFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var message string
		if err := starlark.UnpackPositionalArgs("ui.activity", args, kwargs, 1, &message); err != nil {
			return nil, err
		}

		activity := ui.NewActivity(message, defaultTheme, defaultRenderOptions, writer)

		// Register as current activity
		activityMu.Lock()
		currentActivity = activity
		activityMu.Unlock()

		return &ActivityHandle{activity: activity}, nil
	}
}

// ProgressBarHandle is a Starlark value for progress bar operations.
// When bar is nil the handle is a no-op (used on non-TTY).
type ProgressBarHandle struct {
	bar *ui.ProgressBar
}

func (p *ProgressBarHandle) String() string        { return "<progress_bar>" }
func (p *ProgressBarHandle) Type() string          { return "progress_bar" }
func (p *ProgressBarHandle) Freeze()               {}
func (p *ProgressBarHandle) Truth() starlark.Bool  { return starlark.True }
func (p *ProgressBarHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: progress_bar") }

func (p *ProgressBarHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case "inc":
		return starlark.NewBuiltin("progress_bar.inc", p.inc), nil
	case "set":
		return starlark.NewBuiltin("progress_bar.set", p.set), nil
	case "done":
		return starlark.NewBuiltin("progress_bar.done", p.done), nil
	default:
		return nil, nil
	}
}

func (p *ProgressBarHandle) AttrNames() []string {
	return []string{"inc", "set", "done"}
}

func (p *ProgressBarHandle) inc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	amount := 1
	if err := starlark.UnpackArgs("progress_bar.inc", args, kwargs, "amount?", &amount); err != nil {
		return nil, err
	}
	if p.bar != nil {
		p.bar.Inc(amount)
	}
	return starlark.None, nil
}

func (p *ProgressBarHandle) set(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value int
	if err := starlark.UnpackPositionalArgs("progress_bar.set", args, kwargs, 1, &value); err != nil {
		return nil, err
	}
	if p.bar != nil {
		p.bar.Set(value)
	}
	return starlark.None, nil
}

func (p *ProgressBarHandle) done(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	message := "Complete"
	if err := starlark.UnpackArgs("progress_bar.done", args, kwargs, "message?", &message); err != nil {
		return nil, err
	}
	if p.bar != nil {
		p.bar.Done(message)
	}
	return starlark.None, nil
}

// makeProgressBarFunc creates the ui.progress_bar() function.
func makeProgressBarFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var total int
		var message string
		if err := starlark.UnpackArgs("ui.progress_bar", args, kwargs, "total", &total, "message?", &message); err != nil {
			return nil, err
		}
		if message == "" {
			message = "Progress"
		}

		bar := ui.NewProgressBar(total, message, defaultTheme, defaultRenderOptions, writer)
		return &ProgressBarHandle{bar: bar}, nil
	}
}

// makeBannerFunc creates the ui.banner() function.
func makeBannerFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var title, subtext string
		if err := starlark.UnpackArgs("ui.banner", args, kwargs, "title", &title, "subtext?", &subtext); err != nil {
			return nil, err
		}

		rendered := ui.RenderBanner(title, subtext, defaultTheme, defaultRenderOptions)
		fmt.Fprintln(writer, ui.IndentLines(rendered, indent))
		return starlark.None, nil
	}
}

// makeLinkFunc creates the ui.link() function.
func makeLinkFunc(indent string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var text, url string
		if err := starlark.UnpackArgs("ui.link", args, kwargs, "text", &text, "url", &url); err != nil {
			return nil, err
		}

		rendered := ui.RenderLink(text, url, defaultRenderOptions)
		return starlark.String(rendered), nil
	}
}

// makePagerFunc creates the ui.pager() function.
func makePagerFunc(indent string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content, title string
		var showLineNumbers bool
		if err := starlark.UnpackArgs("ui.pager", args, kwargs, "content", &content, "title?", &title, "show_line_numbers?", &showLineNumbers); err != nil {
			return nil, err
		}

		err := ui.RenderWithPager(content, title, showLineNumbers, defaultRenderOptions)
		if err != nil {
			return nil, err
		}
		return starlark.None, nil
	}
}

// StepHandle is a Starlark value for step operations.
// When step is nil the handle is a no-op (used on non-TTY).
type StepHandle struct {
	step *ui.Step
}

func (s *StepHandle) String() string        { return "<step>" }
func (s *StepHandle) Type() string          { return "step" }
func (s *StepHandle) Freeze()               {}
func (s *StepHandle) Truth() starlark.Bool  { return starlark.True }
func (s *StepHandle) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: step") }

func (s *StepHandle) Attr(name string) (starlark.Value, error) {
	switch name {
	case "done":
		return starlark.NewBuiltin("step.done", s.done), nil
	case "fail":
		return starlark.NewBuiltin("step.fail", s.fail), nil
	default:
		return nil, nil
	}
}

func (s *StepHandle) AttrNames() []string {
	return []string{"done", "fail"}
}

func (s *StepHandle) done(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string
	if err := starlark.UnpackArgs("step.done", args, kwargs, "message?", &message); err != nil {
		return nil, err
	}
	if s.step != nil {
		s.step.Done(message)
	}
	return starlark.None, nil
}

func (s *StepHandle) fail(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string
	if err := starlark.UnpackArgs("step.fail", args, kwargs, "message?", &message); err != nil {
		return nil, err
	}
	if s.step != nil {
		s.step.Fail(message)
	}
	return starlark.None, nil
}

// makeStepFunc creates the ui.step() function.
func makeStepFunc(depth int, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var (
			title string
			icon  string
		)
		if err := starlark.UnpackArgs("ui.step", args, kwargs, "title", &title, "icon?", &icon); err != nil {
			return nil, err
		}

		step := ui.NewStep(title, icon, depth, defaultTheme, defaultRenderOptions, writer)
		return &StepHandle{step: step}, nil
	}
}

// makeActionFunc creates the ui.action() function.
func makeActionFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var message string
		if err := starlark.UnpackPositionalArgs("ui.action", args, kwargs, 1, &message); err != nil {
			return nil, err
		}

		// Pause any active spinner
		activityMu.Lock()
		if currentActivity != nil {
			currentActivity.Pause()
		}
		activityMu.Unlock()

		iw := &indentingWriter{w: writer, indent: indent}
		ui.LogAction(message, defaultTheme, defaultRenderOptions, iw)

		// Resume spinner
		activityMu.Lock()
		if currentActivity != nil {
			currentActivity.Resume()
		}
		activityMu.Unlock()

		return starlark.None, nil
	}
}

// makeThinkFunc creates the ui.think() function.
func makeThinkFunc(indent string, writer io.Writer) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var message string
		if err := starlark.UnpackPositionalArgs("ui.think", args, kwargs, 1, &message); err != nil {
			return nil, err
		}

		// Pause any active spinner
		activityMu.Lock()
		if currentActivity != nil {
			currentActivity.Pause()
		}
		activityMu.Unlock()

		iw := &indentingWriter{w: writer, indent: indent}
		ui.LogThought(message, defaultTheme, defaultRenderOptions, iw)

		// Resume spinner
		activityMu.Lock()
		if currentActivity != nil {
			currentActivity.Resume()
		}
		activityMu.Unlock()

		return starlark.None, nil
	}
}

// makeStreamFunc creates the ui.stream(delta, done=False) function.
// On TTY it forwards token deltas to the TUI StreamBlock via streamer.
// On non-TTY the whole ui module is replaced by noopUIModule so this is
// never called, but the noop entry in that module handles it gracefully.
func makeStreamFunc(streamer StreamSender) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var delta string
		var done bool
		if err := starlark.UnpackArgs("ui.stream", args, kwargs, "delta", &delta, "done?", &done); err != nil {
			return nil, err
		}
		streamer.StreamToken(delta, done)
		return starlark.None, nil
	}
}

type indentingWriter struct {
	w      io.Writer
	indent string
}

func (iw *indentingWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// Simple implementation: indent every line
	// This might be imperfect if Write is called partial lines.
	// For logging functions that typically write one line, it's okay.
	s := string(p)
	if iw.indent != "" {
		// Only indent if not already at start (not easy to track state here without more complexity)
		// But IndentLines handles newlines.
		// The issue is if multiple Writes build one line.
		// Assuming line-based writes from ui helpers.
		s = ui.IndentLines(s, iw.indent)
	}
	_, err = fmt.Fprint(iw.w, s)
	return len(p), err // Return original length to satisfy interface
}
