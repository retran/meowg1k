// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

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

// xmlElement represents a parsed XML element
type xmlElement struct {
	Name     string
	Attrs    map[string]string
	Text     string
	Children []xmlElement
}

// parseXMLToElement parses XML into our custom element structure
func parseXMLToElement(data string) (*xmlElement, error) {
	decoder := xml.NewDecoder(strings.NewReader(data))

	// Parse root element
	root, err := parseElement(decoder, nil)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// parseElement recursively parses an XML element
func parseElement(decoder *xml.Decoder, start *xml.StartElement) (*xmlElement, error) {
	var elem xmlElement
	var textParts []string

	// If we don't have a start element, read the first one
	if start == nil {
		for {
			token, err := decoder.Token()
			if err != nil {
				if err == io.EOF {
					return nil, fmt.Errorf("no root element found")
				}
				return nil, err
			}

			if se, ok := token.(xml.StartElement); ok {
				start = &se
				break
			}
		}
	}

	elem.Name = start.Name.Local
	elem.Attrs = make(map[string]string)

	// Extract attributes
	for _, attr := range start.Attr {
		elem.Attrs[attr.Name.Local] = attr.Value
	}

	// Parse content
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// Nested element
			child, err := parseElement(decoder, &t)
			if err != nil {
				return nil, err
			}
			elem.Children = append(elem.Children, *child)

		case xml.CharData:
			// Text content
			text := strings.TrimSpace(string(t))
			if text != "" {
				textParts = append(textParts, text)
			}

		case xml.EndElement:
			// End of this element
			if len(textParts) > 0 {
				elem.Text = strings.Join(textParts, " ")
			}
			return &elem, nil
		}
	}

	if len(textParts) > 0 {
		elem.Text = strings.Join(textParts, " ")
	}
	return &elem, nil
}

// elementToStarlark converts an xmlElement to Starlark values
func elementToStarlark(elem *xmlElement) starlark.Value {
	dict := starlark.NewDict(0)

	// Add attributes with @ prefix
	for key, value := range elem.Attrs {
		dict.SetKey(starlark.String("@"+key), starlark.String(value))
	}

	// Handle children
	if len(elem.Children) > 0 {
		// Group children by name
		childMap := make(map[string][]xmlElement)
		for _, child := range elem.Children {
			childMap[child.Name] = append(childMap[child.Name], child)
		}

		// Add each group to the dict
		for name, children := range childMap {
			if len(children) == 1 {
				// Single child - add as dict
				dict.SetKey(starlark.String(name), elementToStarlark(&children[0]))
			} else {
				// Multiple children with same name - add as list
				items := make([]starlark.Value, len(children))
				for i, child := range children {
					items[i] = elementToStarlark(&child)
				}
				dict.SetKey(starlark.String(name), starlark.NewList(items))
			}
		}
	}

	// Add text content if present and no children (or mixed content)
	if elem.Text != "" {
		if len(elem.Children) == 0 {
			// No children - this is a simple text element, return just the text
			// unless there are attributes
			if len(elem.Attrs) == 0 {
				return starlark.String(elem.Text)
			}
			// Has attributes - add text as #text
			dict.SetKey(starlark.String("#text"), starlark.String(elem.Text))
		} else {
			// Has children and text (mixed content) - add text as #text
			dict.SetKey(starlark.String("#text"), starlark.String(elem.Text))
		}
	}

	return dict
}

// xmlParse parses an XML string into Starlark values
func xmlParse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("xml.parse", args, kwargs, 1, &data); err != nil {
		return nil, err
	}

	elem, err := parseXMLToElement(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Wrap in a dict with the root element name as key
	result := starlark.NewDict(1)
	result.SetKey(starlark.String(elem.Name), elementToStarlark(elem))

	return result, nil
}

// starlarkToXMLElement converts Starlark values to XML elements
func starlarkToXMLElement(name string, value starlark.Value) (*xmlElement, error) {
	elem := &xmlElement{
		Name:  name,
		Attrs: make(map[string]string),
	}

	switch v := value.(type) {
	case starlark.String:
		// Simple text element
		elem.Text = string(v)

	case starlark.Int, starlark.Float, starlark.Bool:
		// Convert primitive to string
		elem.Text = v.String()

	case *starlark.Dict:
		// Process dict - attributes (@key), text (#text), and children
		for _, item := range v.Items() {
			key, ok := item[0].(starlark.String)
			if !ok {
				continue
			}

			keyStr := string(key)

			if strings.HasPrefix(keyStr, "@") {
				// Attribute
				attrName := strings.TrimPrefix(keyStr, "@")
				elem.Attrs[attrName] = item[1].String()
			} else if keyStr == "#text" {
				// Text content
				elem.Text = item[1].String()
			} else {
				// Child element
				if list, ok := item[1].(*starlark.List); ok {
					// Multiple children with same name
					for i := 0; i < list.Len(); i++ {
						child, err := starlarkToXMLElement(keyStr, list.Index(i))
						if err != nil {
							return nil, err
						}
						elem.Children = append(elem.Children, *child)
					}
				} else {
					// Single child
					child, err := starlarkToXMLElement(keyStr, item[1])
					if err != nil {
						return nil, err
					}
					elem.Children = append(elem.Children, *child)
				}
			}
		}

	case *starlark.List:
		// Lists should be handled by parent
		return nil, fmt.Errorf("cannot convert list directly to XML element")

	default:
		elem.Text = v.String()
	}

	return elem, nil
}

// renderXMLElement renders an XML element to a string
func renderXMLElement(elem *xmlElement, indent bool, level int) string {
	var sb strings.Builder

	// Indentation
	prefix := ""
	if indent {
		prefix = strings.Repeat("  ", level)
	}

	// Opening tag
	sb.WriteString(prefix)
	sb.WriteString("<")
	sb.WriteString(elem.Name)

	// Attributes
	for key, value := range elem.Attrs {
		sb.WriteString(` `)
		sb.WriteString(key)
		sb.WriteString(`="`)
		sb.WriteString(escapeXMLText(value))
		sb.WriteString(`"`)
	}

	// Self-closing if no content
	if elem.Text == "" && len(elem.Children) == 0 {
		sb.WriteString("/>")
		if indent {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	sb.WriteString(">")

	// Content
	hasChildren := len(elem.Children) > 0
	if hasChildren && indent {
		sb.WriteString("\n")
	}

	// Children
	for _, child := range elem.Children {
		sb.WriteString(renderXMLElement(&child, indent, level+1))
	}

	// Text content
	if elem.Text != "" {
		if !hasChildren {
			sb.WriteString(escapeXMLText(elem.Text))
		} else {
			// Mixed content
			sb.WriteString(prefix)
			if indent {
				sb.WriteString("  ")
			}
			sb.WriteString(escapeXMLText(elem.Text))
			if indent {
				sb.WriteString("\n")
			}
		}
	}

	// Closing tag
	if hasChildren && indent {
		sb.WriteString(prefix)
	}
	sb.WriteString("</")
	sb.WriteString(elem.Name)
	sb.WriteString(">")
	if indent {
		sb.WriteString("\n")
	}

	return sb.String()
}

// Helper to escape XML text
func escapeXMLText(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	).Replace(s)
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

	// If root is not specified and value is a dict, use first key as root
	var elem *xmlElement
	var err error

	if root == "" {
		if dict, ok := value.(*starlark.Dict); ok && dict.Len() == 1 {
			// Extract root from dict
			items := dict.Items()
			if len(items) > 0 {
				if keyStr, ok := items[0][0].(starlark.String); ok {
					root = string(keyStr)
					value = items[0][1]
				}
			}
		}
	}

	if root == "" {
		return nil, fmt.Errorf("root element name required (use root= parameter or pass dict with single key)")
	}

	elem, err = starlarkToXMLElement(root, value)
	if err != nil {
		return nil, err
	}

	// Render to string
	var sb strings.Builder
	sb.WriteString(xml.Header)
	sb.WriteString(renderXMLElement(elem, indent, 0))

	return starlark.String(sb.String()), nil
}
