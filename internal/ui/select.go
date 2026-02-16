// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectItem is a single selectable entry.
type SelectItem struct {
	Index   int
	Label   string
	Value   string
	Meta    string
	Preview string
	Match   []int
	Score   int
}

// SelectOptions controls select rendering and behavior.
type SelectOptions struct {
	Title        string
	Items        []SelectItem
	Multi        bool
	Fuzzy        bool
	Limit        int
	Placeholder  string
	InitialQuery string
	AllowNew     bool
	ReturnIndex  bool
	Theme        Theme
}

// SelectResult is returned after the selection finishes.
type SelectResult struct {
	Canceled bool
	NewValue string
	Items    []SelectItem
}

// RunSelect runs an interactive select prompt.
func RunSelect(opts SelectOptions) (SelectResult, error) {
	if opts.Theme.Text == "" {
		opts.Theme = DefaultTheme()
	}
	model := newSelectModel(opts)
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return SelectResult{}, err
	}
	resultModel := finalModel.(selectModel)
	return resultModel.result(), nil
}

type selectModel struct {
	opts     SelectOptions
	query    textinput.Model
	items    []SelectItem
	filtered []SelectItem
	selected map[int]bool
	cursor   int
	width    int
	height   int
	canceled bool
	newValue string
	finished bool
}

func newSelectModel(opts SelectOptions) selectModel {
	query := textinput.New()
	query.Prompt = ""
	query.Placeholder = opts.Placeholder
	query.SetValue(opts.InitialQuery)
	query.Focus()

	items := make([]SelectItem, len(opts.Items))
	copy(items, opts.Items)
	for i := range items {
		items[i].Index = i
		if items[i].Value == "" {
			items[i].Value = items[i].Label
		}
	}

	model := selectModel{
		opts:     opts,
		query:    query,
		items:    items,
		selected: make(map[int]bool),
	}
	model.filtered = filterItems(items, opts.InitialQuery, opts.Fuzzy)
	return model
}

func (m selectModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			m.finished = true
			return m, tea.Quit
		case "enter":
			return m.finishSelection()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "pgup":
			step := m.pageStep()
			m.cursor = Clamp(m.cursor-step, 0, len(m.filtered)-1)
			return m, nil
		case "pgdown":
			step := m.pageStep()
			m.cursor = Clamp(m.cursor+step, 0, len(m.filtered)-1)
			return m, nil
		case "home":
			m.cursor = 0
			return m, nil
		case "end":
			m.cursor = Clamp(len(m.filtered)-1, 0, len(m.filtered)-1)
			return m, nil
		case " ":
			if m.opts.Multi {
				m.toggleSelection()
			}
			return m, nil
		case "ctrl+a":
			if m.opts.Multi {
				for _, item := range m.filtered {
					m.selected[item.Index] = true
				}
			}
			return m, nil
		case "ctrl+d":
			if m.opts.Multi {
				m.selected = make(map[int]bool)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.query, cmd = m.query.Update(msg)
	m.filtered = filterItems(m.items, m.query.Value(), m.opts.Fuzzy)
	if m.cursor >= len(m.filtered) {
		m.cursor = Clamp(len(m.filtered)-1, 0, len(m.filtered)-1)
	}
	return m, cmd
}

func (m selectModel) View() string {
	theme := m.opts.Theme
	lines := []string{}

	if m.opts.Title != "" {
		lines = append(lines, theme.PanelTitleStyle.Render(m.opts.Title))
	}

	searchLabel := theme.SelectHint.Render("Search: ")
	searchLine := searchLabel + theme.SelectInput.Render(m.query.View())
	lines = append(lines, searchLine)

	items := m.filtered
	if len(items) == 0 && m.opts.AllowNew && m.query.Value() != "" {
		lines = append(lines, theme.SelectHint.Render("Press Enter to use: "+m.query.Value()))
		return strings.Join(lines, "\n")
	}

	start := 0
	limit := m.opts.Limit
	if limit > 0 && len(items) > limit {
		start = Clamp(m.cursor-limit/2, 0, len(items)-limit)
		items = items[start : start+limit]
	}

	for i, item := range items {
		cursor := " "
		if i+start == m.cursor {
			cursor = theme.SelectCursor.Render(">")
		}
		label := item.Label
		if len(item.Match) > 0 {
			label = highlightIndices(label, item.Match, theme.SelectMatch)
		}
		if m.opts.Multi && m.selected[item.Index] {
			label = theme.SelectSelected.Render(label)
		}
		line := cursor + " "
		if m.opts.Multi {
			mark := " "
			if m.selected[item.Index] {
				mark = "x"
			}
			line += "[" + mark + "] "
		}
		line += label
		lines = append(lines, line)
	}

	lines = append(lines, theme.SelectHint.Render(m.hintText()))
	return strings.Join(lines, "\n")
}

func (m selectModel) hintText() string {
	if m.opts.Multi {
		return "Enter: confirm  Space: toggle  Esc: cancel"
	}
	return "Enter: select  Esc: cancel"
}

func (m selectModel) pageStep() int {
	if m.height <= 0 {
		return 5
	}
	step := m.height - 6
	if step < 3 {
		step = 3
	}
	return step
}

func (m selectModel) toggleSelection() {
	if len(m.filtered) == 0 {
		return
	}
	item := m.filtered[m.cursor]
	if m.selected[item.Index] {
		delete(m.selected, item.Index)
	} else {
		m.selected[item.Index] = true
	}
}

func (m selectModel) finishSelection() (tea.Model, tea.Cmd) {
	if m.opts.AllowNew && m.query.Value() != "" && len(m.filtered) == 0 {
		m.newValue = m.query.Value()
		m.finished = true
		return m, tea.Quit
	}

	if len(m.filtered) == 0 {
		m.canceled = true
		m.finished = true
		return m, tea.Quit
	}

	if m.opts.Multi {
		if len(m.selected) == 0 {
			m.toggleSelection()
		}
		m.finished = true
		return m, tea.Quit
	}

	m.selected[m.filtered[m.cursor].Index] = true
	m.finished = true
	return m, tea.Quit
}

func (m selectModel) result() SelectResult {
	if m.canceled {
		return SelectResult{Canceled: true}
	}
	if m.newValue != "" {
		return SelectResult{NewValue: m.newValue}
	}
	items := []SelectItem{}
	for _, item := range m.items {
		if m.selected[item.Index] {
			items = append(items, item)
		}
	}
	return SelectResult{Items: items}
}

func filterItems(items []SelectItem, query string, fuzzy bool) []SelectItem {
	if query == "" {
		filtered := make([]SelectItem, len(items))
		copy(filtered, items)
		for i := range filtered {
			filtered[i].Match = nil
			filtered[i].Score = 0
		}
		return filtered
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return items
	}

	filtered := []SelectItem{}
	for _, item := range items {
		if fuzzy {
			match, score, ok := fuzzyMatch(item.Label, query)
			if ok {
				item.Match = match
				item.Score = score
				filtered = append(filtered, item)
			}
		} else {
			match := substringMatchIndices(item.Label, query)
			if len(match) > 0 {
				item.Match = match
				item.Score = len(match)
				filtered = append(filtered, item)
			}
		}
	}

	if fuzzy {
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].Score > filtered[j].Score
		})
	}

	return filtered
}

func substringMatchIndices(text, query string) []int {
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	index := strings.Index(lowerText, lowerQuery)
	if index == -1 {
		return nil
	}
	indices := []int{}
	for i := 0; i < len(query); i++ {
		indices = append(indices, index+i)
	}
	return indices
}

// fuzzyMatch returns matched rune indices and a score for subsequence matches.
func fuzzyMatch(text, query string) ([]int, int, bool) {
	textRunes := []rune(strings.ToLower(text))
	queryRunes := []rune(strings.ToLower(query))
	if len(queryRunes) == 0 {
		return nil, 0, false
	}
	indices := []int{}
	score := 0
	ti := 0
	for qi := 0; qi < len(queryRunes); qi++ {
		found := false
		for ti < len(textRunes) {
			if textRunes[ti] == queryRunes[qi] {
				indices = append(indices, ti)
				score += 10
				if len(indices) > 1 && indices[len(indices)-1]-indices[len(indices)-2] == 1 {
					score += 5
				}
				ti++
				found = true
				break
			}
			ti++
		}
		if !found {
			return nil, 0, false
		}
	}
	return indices, score, true
}

func highlightIndices(text string, indices []int, style lipgloss.Style) string {
	if len(indices) == 0 {
		return text
	}
	indexSet := map[int]bool{}
	for _, idx := range indices {
		indexSet[idx] = true
	}
	var b strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		if indexSet[i] {
			b.WriteString(style.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
