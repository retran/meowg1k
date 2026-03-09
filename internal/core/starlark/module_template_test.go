// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const templateTestWorkingDir = "/tmp"

func TestTemplateParse(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Test simple template parsing
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("Hello {{.Name}}")},
		nil, workingDir)

	require.NoError(t, err)
	require.NotNil(t, result)

	tmpl, ok := result.(*Template)
	require.True(t, ok)
	assert.Equal(t, "template", tmpl.Type())
}

func TestTemplateParseWithName(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("Hello {{.Name}}")},
		[]starlark.Tuple{
			{starlark.String("name"), starlark.String("my-template")},
		}, workingDir)

	require.NoError(t, err)
	tmpl, ok := result.(*Template)
	require.True(t, ok)
	assert.Contains(t, tmpl.String(), "my-template")
}

func TestTemplateParseInvalid(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Test invalid template syntax
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("Hello {{.Name")},
		nil, workingDir)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse template")
}

func TestTemplateLoad(t *testing.T) {
	// Create a temporary template file
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "test.tmpl")
	templateContent := "Hello {{.Name}}, you are {{.Age}} years old"
	err := os.WriteFile(templatePath, []byte(templateContent), 0o644)
	require.NoError(t, err)

	thread := &starlark.Thread{Name: "test"}

	// Test loading template from file
	result, err := templateLoad(thread, starlark.NewBuiltin("load", nil),
		starlark.Tuple{starlark.String(templatePath)},
		nil, tmpDir)

	require.NoError(t, err)
	require.NotNil(t, result)

	tmpl, ok := result.(*Template)
	require.True(t, ok)
	assert.Equal(t, "template", tmpl.Type())
}

func TestTemplateLoadNotFound(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	result, err := templateLoad(thread, starlark.NewBuiltin("load", nil),
		starlark.Tuple{starlark.String("/nonexistent/template.tmpl")},
		nil, workingDir)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to read template file")
}

func TestTemplateRender(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Parse a template
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("Hello {{.Name}}, you are {{.Age}} years old")},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	// Prepare data
	data := starlark.NewDict(2)
	data.SetKey(starlark.String("Name"), starlark.String("Alice"))
	data.SetKey(starlark.String("Age"), starlark.MakeInt(30))

	// Render template
	rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
		starlark.Tuple{data},
		nil)

	require.NoError(t, err)
	renderedStr, ok := rendered.(starlark.String)
	require.True(t, ok)
	assert.Equal(t, "Hello Alice, you are 30 years old", string(renderedStr))
}

func TestTemplateRenderWithList(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Parse a template with range
	templateText := "Users: {{range .Users}}{{.}}, {{end}}"
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String(templateText)},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	// Prepare data with list
	users := starlark.NewList([]starlark.Value{
		starlark.String("Alice"),
		starlark.String("Bob"),
		starlark.String("Charlie"),
	})
	data := starlark.NewDict(1)
	data.SetKey(starlark.String("Users"), users)

	// Render template
	rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
		starlark.Tuple{data},
		nil)

	require.NoError(t, err)
	renderedStr, ok := rendered.(starlark.String)
	require.True(t, ok)
	assert.Equal(t, "Users: Alice, Bob, Charlie, ", string(renderedStr))
}

func TestTemplateRenderWithNestedData(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Parse a template with nested access
	templateText := "User: {{.User.Name}} ({{.User.Email}})"
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String(templateText)},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	// Prepare nested data
	userDict := starlark.NewDict(2)
	userDict.SetKey(starlark.String("Name"), starlark.String("Alice"))
	userDict.SetKey(starlark.String("Email"), starlark.String("alice@example.com"))

	data := starlark.NewDict(1)
	data.SetKey(starlark.String("User"), userDict)

	// Render template
	rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
		starlark.Tuple{data},
		nil)

	require.NoError(t, err)
	renderedStr, ok := rendered.(starlark.String)
	require.True(t, ok)
	assert.Equal(t, "User: Alice (alice@example.com)", string(renderedStr))
}

func TestTemplateRenderWithConditional(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Parse a template with conditional
	templateText := "{{if .Active}}User is active{{else}}User is inactive{{end}}"
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String(templateText)},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	// Test with Active=true
	data := starlark.NewDict(1)
	data.SetKey(starlark.String("Active"), starlark.True)

	rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
		starlark.Tuple{data},
		nil)

	require.NoError(t, err)
	assert.Equal(t, "User is active", string(rendered.(starlark.String)))

	// Test with Active=false
	data2 := starlark.NewDict(1)
	data2.SetKey(starlark.String("Active"), starlark.False)

	rendered2, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
		starlark.Tuple{data2},
		nil)

	require.NoError(t, err)
	assert.Equal(t, "User is inactive", string(rendered2.(starlark.String)))
}

func TestTemplateRenderEmpty(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Parse a simple template
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("Static content")},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	// Render with empty data
	data := starlark.NewDict(0)

	rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
		starlark.Tuple{data},
		nil)

	require.NoError(t, err)
	assert.Equal(t, "Static content", string(rendered.(starlark.String)))
}

func TestStarlarkValueToGoInterface(t *testing.T) {
	tests := []struct {
		input    starlark.Value
		expected interface{}
		name     string
	}{
		{
			name:     "none",
			input:    starlark.None,
			expected: nil,
		},
		{
			name:     "bool true",
			input:    starlark.True,
			expected: true,
		},
		{
			name:     "bool false",
			input:    starlark.False,
			expected: false,
		},
		{
			name:     "int",
			input:    starlark.MakeInt(42),
			expected: int64(42),
		},
		{
			name:     "float",
			input:    starlark.Float(3.14),
			expected: float64(3.14),
		},
		{
			name:     "string",
			input:    starlark.String("hello"),
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := starlarkValueToGoInterface(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplateModule(t *testing.T) {
	workingDir := templateTestWorkingDir
	module := NewTemplateModule(workingDir)

	require.NotNil(t, module)

	// Verify module has expected functions
	moduleStruct, ok := module.(*starlarkstruct.Struct)
	require.True(t, ok)

	parseFunc, err := moduleStruct.Attr("parse")
	require.NoError(t, err)
	require.NotNil(t, parseFunc)

	loadFunc, err := moduleStruct.Attr("load")
	require.NoError(t, err)
	require.NotNil(t, loadFunc)
}

func TestTemplateAttr(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	// Create a template
	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("Hello {{.Name}}")},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	// Test Attr method
	renderFunc, err := tmpl.Attr("render")
	require.NoError(t, err)
	require.NotNil(t, renderFunc)

	// Test AttrNames
	names := tmpl.AttrNames()
	assert.Equal(t, []string{"render"}, names)

	// Test invalid attr
	invalidAttr, err := tmpl.Attr("invalid")
	assert.NoError(t, err)
	assert.Nil(t, invalidAttr)
}

func TestTemplateTruth(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("test")},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	assert.Equal(t, starlark.True, tmpl.Truth())
}

func TestTemplateHash(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("test")},
		nil, workingDir)
	require.NoError(t, err)

	tmpl, ok := result.(*Template)
	require.True(t, ok)

	_, err = tmpl.Hash()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not hashable")
}

// TestTemplateParseErrors tests error cases for template.parse().
func TestTemplateParseErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	t.Run("missing text argument", func(t *testing.T) {
		_, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{},
			nil, workingDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "text")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		_, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.MakeInt(123)},
			nil, workingDir)
		assert.Error(t, err)
	})

	t.Run("invalid template with unclosed action", func(t *testing.T) {
		_, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("{{.Name")},
			nil, workingDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template")
	})

	t.Run("invalid template with undefined function", func(t *testing.T) {
		// Undefined functions cause parse errors in Go templates
		_, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("{{unknownFunc .Name}}")},
			nil, workingDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template")
	})

	t.Run("empty template", func(t *testing.T) {
		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("")},
			nil, workingDir)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestTemplateLoadErrors tests error cases for template.load().
func TestTemplateLoadErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	t.Run("missing path argument", func(t *testing.T) {
		_, err := templateLoad(thread, starlark.NewBuiltin("load", nil),
			starlark.Tuple{},
			nil, workingDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		_, err := templateLoad(thread, starlark.NewBuiltin("load", nil),
			starlark.Tuple{starlark.MakeInt(123)},
			nil, workingDir)
		assert.Error(t, err)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := templateLoad(thread, starlark.NewBuiltin("load", nil),
			starlark.Tuple{starlark.String("/nonexistent/path/template.tmpl")},
			nil, workingDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read template file")
	})

	t.Run("invalid template content in file", func(t *testing.T) {
		tmpDir := t.TempDir()
		templatePath := filepath.Join(tmpDir, "invalid.tmpl")
		err := os.WriteFile(templatePath, []byte("{{.Name"), 0o644)
		require.NoError(t, err)

		_, err = templateLoad(thread, starlark.NewBuiltin("load", nil),
			starlark.Tuple{starlark.String(templatePath)},
			nil, tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse template")
	})

	t.Run("relative path resolution", func(t *testing.T) {
		tmpDir := t.TempDir()
		templatePath := filepath.Join(tmpDir, "test.tmpl")
		err := os.WriteFile(templatePath, []byte("Hello {{.Name}}"), 0o644)
		require.NoError(t, err)

		result, err := templateLoad(thread, starlark.NewBuiltin("load", nil),
			starlark.Tuple{starlark.String("test.tmpl")},
			nil, tmpDir)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestTemplateRenderErrors tests error cases for template.render().
func TestTemplateRenderErrors(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	t.Run("missing data argument", func(t *testing.T) {
		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("Hello {{.Name}}")},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)
		_, err = tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data")
	})

	t.Run("wrong argument type for data", func(t *testing.T) {
		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("Hello {{.Name}}")},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)
		_, err = tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{starlark.String("not a dict")},
			nil)
		assert.Error(t, err)
	})

	t.Run("template execution error - missing field", func(t *testing.T) {
		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("Hello {{.MissingField}}")},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)
		data := starlark.NewDict(0)

		rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{data},
			nil)
		// Missing fields don't cause errors in Go templates by default
		require.NoError(t, err)
		assert.Equal(t, "Hello <no value>", string(rendered.(starlark.String)))
	})

	t.Run("template execution error - type mismatch", func(t *testing.T) {
		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("{{if .Name}}{{.Name}}{{end}}")},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)
		data := starlark.NewDict(1)
		data.SetKey(starlark.String("Name"), starlark.String("Alice"))

		rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{data},
			nil)
		require.NoError(t, err)
		assert.Equal(t, "Alice", string(rendered.(starlark.String)))
	})

	t.Run("nil data dict", func(t *testing.T) {
		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String("Static content")},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)
		rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{starlark.NewDict(0)},
			nil)
		require.NoError(t, err)
		assert.Equal(t, "Static content", string(rendered.(starlark.String)))
	})
}

// TestStarlarkValueToGoInterfaceComplex tests complex type conversions.
func TestStarlarkValueToGoInterfaceComplex(t *testing.T) {
	t.Run("list conversion", func(t *testing.T) {
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(1),
			starlark.True,
		})
		result := starlarkValueToGoInterface(list)
		require.NotNil(t, result)

		slice, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, slice, 3)
		assert.Equal(t, "a", slice[0])
		assert.Equal(t, int64(1), slice[1])
		assert.Equal(t, true, slice[2])
	})

	t.Run("nested dict conversion", func(t *testing.T) {
		innerDict := starlark.NewDict(1)
		innerDict.SetKey(starlark.String("nested"), starlark.String("value"))

		outerDict := starlark.NewDict(1)
		outerDict.SetKey(starlark.String("inner"), innerDict)

		result := starlarkValueToGoInterface(outerDict)
		require.NotNil(t, result)

		outerMap, ok := result.(map[string]interface{})
		require.True(t, ok)

		innerMap, ok := outerMap["inner"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", innerMap["nested"])
	})

	t.Run("struct conversion", func(t *testing.T) {
		structVal := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name": starlark.String("test"),
			"age":  starlark.MakeInt(42),
		})

		result := starlarkValueToGoInterface(structVal)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test", resultMap["name"])
		assert.Equal(t, int64(42), resultMap["age"])
	})

	t.Run("empty list", func(t *testing.T) {
		list := starlark.NewList([]starlark.Value{})
		result := starlarkValueToGoInterface(list)
		require.NotNil(t, result)

		slice, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, slice, 0)
	})

	t.Run("empty dict", func(t *testing.T) {
		dict := starlark.NewDict(0)
		result := starlarkValueToGoInterface(dict)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Len(t, resultMap, 0)
	})

	t.Run("dict with non-string keys", func(t *testing.T) {
		dict := starlark.NewDict(1)
		dict.SetKey(starlark.MakeInt(123), starlark.String("value"))

		result := starlarkValueToGoInterface(dict)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		// Non-string keys should be ignored
		assert.Len(t, resultMap, 0)
	})
}

// TestTemplateRenderComplexScenarios tests complex real-world scenarios.
func TestTemplateRenderComplexScenarios(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	t.Run("template with multiple data types", func(t *testing.T) {
		templateText := `Name: {{.Name}}
Age: {{.Age}}
Active: {{.Active}}
Score: {{.Score}}
Tags: {{range .Tags}}{{.}} {{end}}`

		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String(templateText)},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)

		tags := starlark.NewList([]starlark.Value{
			starlark.String("go"),
			starlark.String("python"),
		})

		data := starlark.NewDict(5)
		data.SetKey(starlark.String("Name"), starlark.String("Alice"))
		data.SetKey(starlark.String("Age"), starlark.MakeInt(30))
		data.SetKey(starlark.String("Active"), starlark.True)
		data.SetKey(starlark.String("Score"), starlark.Float(95.5))
		data.SetKey(starlark.String("Tags"), tags)

		rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{data},
			nil)

		require.NoError(t, err)
		result_str := string(rendered.(starlark.String))
		assert.Contains(t, result_str, "Name: Alice")
		assert.Contains(t, result_str, "Age: 30")
		assert.Contains(t, result_str, "Active: true")
		assert.Contains(t, result_str, "Score: 95.5")
		assert.Contains(t, result_str, "Tags: go python")
	})

	t.Run("template with nested iteration", func(t *testing.T) {
		templateText := `{{range .Users}}User: {{.Name}} - Roles: {{range .Roles}}{{.}} {{end}}
{{end}}`

		result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
			starlark.Tuple{starlark.String(templateText)},
			nil, workingDir)
		require.NoError(t, err)

		tmpl := result.(*Template)

		// Create user 1
		roles1 := starlark.NewList([]starlark.Value{
			starlark.String("admin"),
			starlark.String("user"),
		})
		user1 := starlark.NewDict(2)
		user1.SetKey(starlark.String("Name"), starlark.String("Alice"))
		user1.SetKey(starlark.String("Roles"), roles1)

		// Create user 2
		roles2 := starlark.NewList([]starlark.Value{
			starlark.String("user"),
		})
		user2 := starlark.NewDict(2)
		user2.SetKey(starlark.String("Name"), starlark.String("Bob"))
		user2.SetKey(starlark.String("Roles"), roles2)

		users := starlark.NewList([]starlark.Value{user1, user2})
		data := starlark.NewDict(1)
		data.SetKey(starlark.String("Users"), users)

		rendered, err := tmpl.render(thread, starlark.NewBuiltin("render", nil),
			starlark.Tuple{data},
			nil)

		require.NoError(t, err)
		result_str := string(rendered.(starlark.String))
		assert.Contains(t, result_str, "User: Alice - Roles: admin user")
		assert.Contains(t, result_str, "User: Bob - Roles: user")
	})
}

// TestTemplateFreeze tests the Freeze method.
func TestTemplateFreeze(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	workingDir := templateTestWorkingDir

	result, err := templateParse(thread, starlark.NewBuiltin("parse", nil),
		starlark.Tuple{starlark.String("test")},
		nil, workingDir)
	require.NoError(t, err)

	tmpl := result.(*Template)
	// Freeze should not panic
	tmpl.Freeze()
}
