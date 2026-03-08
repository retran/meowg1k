// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// ContextWithParams wraps a context with injected parameters.
type ContextWithParams struct {
	baseCtx starlark.Value
	params  starlark.StringDict
}

var _ starlark.HasAttrs = (*ContextWithParams)(nil)

func (c *ContextWithParams) String() string { return "ctx" }

// Type returns the Starlark type name for ContextWithParams.
func (c *ContextWithParams) Type() string { return "context" }

// Freeze makes the context immutable by freezing the base context.
func (c *ContextWithParams) Freeze() { c.baseCtx.Freeze() }

// Truth returns True since a context is always truthy.
func (c *ContextWithParams) Truth() starlark.Bool { return starlark.True }

// Hash returns an error since contexts are not hashable.
func (c *ContextWithParams) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: context") }

// Attr implements starlark.HasAttrs - provides dynamic attribute access.
func (c *ContextWithParams) Attr(name string) (starlark.Value, error) {
	if val, ok := c.params[name]; ok {
		return val, nil
	}

	if hasAttrs, ok := c.baseCtx.(starlark.HasAttrs); ok {
		val, err := hasAttrs.Attr(name)
		if err != nil {
			return nil, fmt.Errorf("context attribute %q: %w", name, err)
		}
		return val, nil
	}

	return nil, starlark.NoSuchAttrError(fmt.Sprintf("context has no attribute %q", name))
}

// AttrNames implements starlark.HasAttrs.
func (c *ContextWithParams) AttrNames() []string {
	names := make([]string, 0)

	for name := range c.params {
		names = append(names, name)
	}

	if hasAttrs, ok := c.baseCtx.(starlark.HasAttrs); ok {
		names = append(names, hasAttrs.AttrNames()...)
	}

	return names
}

// CreateContextWithParams wraps a base context with parameter injection.
func CreateContextWithParams(baseCtx starlark.Value, params map[string]starlark.Value) *ContextWithParams {
	paramDict := make(starlark.StringDict)
	for k, v := range params {
		paramDict[k] = v
	}

	return &ContextWithParams{
		baseCtx: baseCtx,
		params:  paramDict,
	}
}
