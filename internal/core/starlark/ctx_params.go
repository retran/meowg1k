// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// ContextWithParams wraps a context with injected parameters
type ContextWithParams struct {
	baseCtx starlark.Value
	params  starlark.StringDict
}

var _ starlark.HasAttrs = (*ContextWithParams)(nil)

func (c *ContextWithParams) String() string        { return "ctx" }
func (c *ContextWithParams) Type() string          { return "context" }
func (c *ContextWithParams) Freeze()               { c.baseCtx.Freeze() }
func (c *ContextWithParams) Truth() starlark.Bool  { return starlark.True }
func (c *ContextWithParams) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: context") }

// Attr implements starlark.HasAttrs - provides dynamic attribute access
func (c *ContextWithParams) Attr(name string) (starlark.Value, error) {
	// First check if it's an injected parameter
	if val, ok := c.params[name]; ok {
		return val, nil
	}

	// Then delegate to base context
	if hasAttrs, ok := c.baseCtx.(starlark.HasAttrs); ok {
		return hasAttrs.Attr(name)
	}

	return nil, starlark.NoSuchAttrError(fmt.Sprintf("context has no attribute %q", name))
}

// AttrNames implements starlark.HasAttrs
func (c *ContextWithParams) AttrNames() []string {
	names := make([]string, 0)

	// Add parameter names
	for name := range c.params {
		names = append(names, name)
	}

	// Add base context names
	if hasAttrs, ok := c.baseCtx.(starlark.HasAttrs); ok {
		names = append(names, hasAttrs.AttrNames()...)
	}

	return names
}

// CreateContextWithParams wraps a base context with parameter injection
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
