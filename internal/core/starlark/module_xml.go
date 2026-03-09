// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"github.com/clbanning/mxj/v2"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewXMLModule creates the xml module.
func NewXMLModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "xml",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("xml.parse", xmlParse),
			"stringify": starlark.NewBuiltin("xml.stringify", xmlStringify),
		},
	}
}

// xmlParse parses an XML string into Starlark values using mxj library.
func xmlParse(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("xml.parse", args, kwargs, 1, &data); err != nil {
		return nil, fmt.Errorf("xml.parse: %w", err)
	}

	// Parse XML to map using mxj (attributes will have - prefix by default).
	m, err := mxj.NewMapXml([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("xml.parse: %w", err)
	}

	return goToStarlark(map[string]interface{}(m)), nil
}

// xmlStringify converts Starlark values to XML string using mxj library.
func xmlStringify(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value  starlark.Value
		indent = false
		root   string
	)
	if err := starlark.UnpackArgs("xml.stringify", args, kwargs, "value", &value, "indent?", &indent, "root?", &root); err != nil {
		return nil, fmt.Errorf("xml.stringify: %w", err)
	}

	goValue := starlarkToGo(value)

	var m mxj.Map
	if root != "" {
		m = mxj.Map{root: goValue}
	} else {
		if mapVal, ok := goValue.(map[string]interface{}); ok {
			m = mxj.Map(mapVal)
		} else {
			return nil, fmt.Errorf("xml.stringify requires a dict value or root parameter")
		}
	}

	var xmlBytes []byte
	var err error

	if indent {
		xmlBytes, err = m.XmlIndent("", "  ")
	} else {
		xmlBytes, err = m.Xml()
	}

	if err != nil {
		return nil, fmt.Errorf("xml.stringify: %w", err)
	}

	return starlark.String(string(xmlBytes)), nil
}
