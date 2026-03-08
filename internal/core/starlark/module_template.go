// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewTemplateModule creates the template built-in module.
func NewTemplateModule(workingDir string) starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"parse": starlark.NewBuiltin("parse", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return templateParse(thread, b, args, kwargs, workingDir)
		}),
		"load": starlark.NewBuiltin("load", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return templateLoad(thread, b, args, kwargs, workingDir)
		}),
	})
}

// Template represents a parsed Go text template.
type Template struct {
	tmpl       *template.Template
	workingDir string
}

// String implements starlark.Value.
func (t *Template) String() string {
	return fmt.Sprintf("<template: %s>", t.tmpl.Name())
}

// Type implements starlark.Value.
func (t *Template) Type() string {
	return "template"
}

// Freeze implements starlark.Value.
func (t *Template) Freeze() {}

// Truth implements starlark.Value.
func (t *Template) Truth() starlark.Bool {
	return starlark.True
}

// Hash implements starlark.Value.
func (t *Template) Hash() (uint32, error) {
	return 0, fmt.Errorf("template is not hashable")
}

// Attr implements starlark.HasAttrs.
func (t *Template) Attr(name string) (starlark.Value, error) {
	switch name {
	case "render":
		return starlark.NewBuiltin("render", t.render), nil
	default:
		return nil, nil
	}
}

// AttrNames implements starlark.HasAttrs.
func (t *Template) AttrNames() []string {
	return []string{"render"}
}

// render implements the Template.render() method.
func (t *Template) render(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dataDict *starlark.Dict

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "data", &dataDict); err != nil {
		return nil, fmt.Errorf("template.render: %w", err)
	}

	data := make(map[string]interface{})
	if dataDict != nil {
		for _, item := range dataDict.Items() {
			key, ok := starlark.AsString(item[0])
			if !ok {
				continue
			}
			data[key] = starlarkValueToGoInterface(item[1])
		}
	}

	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template rendering failed: %w", err)
	}

	return starlark.String(buf.String()), nil
}

// templateParse implements template.parse().
func templateParse(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, workingDir string) (starlark.Value, error) {
	var text string
	var name string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &text,
		"name?", &name,
	); err != nil {
		return nil, fmt.Errorf("template.parse: %w", err)
	}

	if name == "" {
		name = "template"
	}

	tmpl, err := template.New(name).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &Template{tmpl: tmpl, workingDir: workingDir}, nil
}

// templateLoad implements template.load().
func templateLoad(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, workingDir string) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("template.load: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDir, path)
	}

	content, err := os.ReadFile(path) //nolint:gosec // user-controlled path for template loading
	if err != nil {
		return nil, fmt.Errorf("failed to read template file '%s': %w", path, err)
	}

	name := filepath.Base(path)
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template from '%s': %w", path, err)
	}

	return &Template{tmpl: tmpl, workingDir: workingDir}, nil
}

// starlarkValueToGoInterface converts a Starlark value to a Go interface{} for template rendering.
func starlarkValueToGoInterface(val starlark.Value) interface{} { //nolint:gocognit // complexity inherent in mapping all Starlark value types to Go
	switch v := val.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(v)
	case starlark.Int:
		i, _ := v.Int64()
		return i
	case starlark.Float:
		return float64(v)
	case starlark.String:
		return string(v)
	case *starlark.List:
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = starlarkValueToGoInterface(v.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]interface{})
		for _, item := range v.Items() {
			key, ok := starlark.AsString(item[0])
			if ok {
				result[key] = starlarkValueToGoInterface(item[1])
			}
		}
		return result
	case *starlarkstruct.Struct:
		// Convert struct to map for template access
		result := make(map[string]interface{})
		for _, name := range v.AttrNames() {
			if attr, err := v.Attr(name); err == nil {
				result[name] = starlarkValueToGoInterface(attr)
			}
		}
		return result
	default:
		return v.String()
	}
}
