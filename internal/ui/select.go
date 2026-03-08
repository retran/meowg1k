// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// SelectItem is a single selectable entry.
type SelectItem struct {
	Label   string
	Value   string
	Meta    string
	Preview string
	Match   []int
	Index   int
	Score   int
}

// SelectOptions controls select rendering and behavior.
type SelectOptions struct {
	Theme        Theme
	Title        string
	Placeholder  string
	InitialQuery string
	Items        []SelectItem
	Limit        int
	Multi        bool
	Fuzzy        bool
	AllowNew     bool
	ReturnIndex  bool
}

// SelectResult is returned after the selection finishes.
type SelectResult struct {
	NewValue string
	Items    []SelectItem
	Canceled bool
}

// RunSelect runs an interactive select prompt using charmbracelet/huh.
func RunSelect(opts SelectOptions) (SelectResult, error) { //nolint:gocognit,gocyclo,funlen,gocritic // complexity inherent in interactive selection with multi-select support; hugeParam: SelectOptions passed by value for immutability
	if len(opts.Items) == 0 {
		return SelectResult{Canceled: true}, nil
	}

	huhOpts := make([]huh.Option[string], len(opts.Items))
	for i, item := range opts.Items {
		label := item.Label
		if item.Meta != "" {
			label = label + "  " + item.Meta
		}
		val := item.Value
		if val == "" {
			val = item.Label
		}
		huhOpts[i] = huh.NewOption(label, val)
	}

	// byValue maps each option's effective value back to its SelectItem for
	// result construction after the form completes.
	byValue := make(map[string]SelectItem, len(opts.Items))
	for _, item := range opts.Items {
		v := item.Value
		if v == "" {
			v = item.Label
		}
		byValue[v] = item
	}

	title := opts.Title
	if title == "" {
		title = "Select"
	}

	if opts.Multi { //nolint:nestif // nested multi-select vs single-select handling with result mapping
		var selected []string
		sel := huh.NewMultiSelect[string]().
			Title(title).
			Options(huhOpts...).
			Value(&selected)
		if opts.Limit > 0 {
			sel = sel.Limit(opts.Limit)
		}
		if opts.Placeholder != "" {
			sel = sel.Description(opts.Placeholder)
		}
		if opts.Fuzzy {
			sel = sel.Filterable(true)
		}

		form := huh.NewForm(huh.NewGroup(sel))
		err := form.Run()
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return SelectResult{Canceled: true}, nil
			}
			return SelectResult{}, fmt.Errorf("multi-select form failed: %w", err)
		}

		result := SelectResult{}
		for _, v := range selected {
			if item, ok := byValue[v]; ok {
				result.Items = append(result.Items, item)
			}
		}
		return result, nil
	}

	// Single select.
	var chosen string
	sel := huh.NewSelect[string]().
		Title(title).
		Options(huhOpts...).
		Value(&chosen)
	if opts.Fuzzy {
		sel = sel.Filtering(true)
	}

	form := huh.NewForm(huh.NewGroup(sel))
	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return SelectResult{Canceled: true}, nil
		}
		return SelectResult{}, fmt.Errorf("single-select form failed: %w", err)
	}

	if chosen == "" {
		return SelectResult{Canceled: true}, nil
	}

	if item, ok := byValue[chosen]; ok {
		return SelectResult{Items: []SelectItem{item}}, nil
	}

	// AllowNew: value not found in map — treat as new entry.
	if opts.AllowNew {
		return SelectResult{NewValue: chosen}, nil
	}

	return SelectResult{Canceled: true}, nil
}
