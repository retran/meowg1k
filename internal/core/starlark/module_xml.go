// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"encoding/xml"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewXMLModule creates the xml module
func NewXMLModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "xml",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("xml.parse", xmlParse),
			"stringify": starlark.NewBuiltin("xml.stringify", xmlStringify),
		},
	}
}

// xmlParse parses an XML string into Starlark values
func xmlParse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("xml.parse", args, kwargs, 1, &data); err != nil {
		return nil, err
	}

	var result interface{}
	if err := xml.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}

	return goToStarlark(result), nil
}

// xmlStringify converts Starlark values to XML string
func xmlStringify(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value  starlark.Value
		indent bool = false
		root   string
	)
	if err := starlark.UnpackArgs("xml.stringify", args, kwargs, "value", &value, "indent?", &indent, "root?", &root); err != nil {
		return nil, err
	}

	goValue := starlarkToGo(value)

	// If root element name is provided, wrap the value
	if root != "" {
		goValue = map[string]interface{}{root: goValue}
	}

	var data []byte
	var err error
	if indent {
		data, err = xml.MarshalIndent(goValue, "", "  ")
	} else {
		data, err = xml.Marshal(goValue)
	}

	if err != nil {
		return nil, err
	}

	// Add XML header
	result := xml.Header + string(data)
	return starlark.String(result), nil
}

// xmlNode represents an XML node with attributes and content
type xmlNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content []byte     `xml:",innerxml"`
}

// UnmarshalXML implements custom unmarshaling for generic XML parsing
func (n *xmlNode) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.XMLName = start.Name
	n.Attrs = start.Attr

	type node xmlNode
	return d.DecodeElement((*node)(n), &start)
}

// xmlToMap converts an XML node to a map structure suitable for Starlark
func xmlToMap(node *xmlNode) interface{} {
	result := make(map[string]interface{})

	// Add attributes with @ prefix
	for _, attr := range node.Attrs {
		key := "@" + attr.Name.Local
		result[key] = attr.Value
	}

	// Add content
	if len(node.Content) > 0 {
		// Try to parse as text content
		content := string(node.Content)
		result["#text"] = content
	}

	return result
}

// xmlParseGeneric is an alternative XML parser that preserves structure better
func xmlParseGeneric(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("xml.parse_generic", args, kwargs, 1, &data); err != nil {
		return nil, err
	}

	var node xmlNode
	if err := xml.Unmarshal([]byte(data), &node); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	result := xmlToMap(&node)
	return goToStarlark(result), nil
}
