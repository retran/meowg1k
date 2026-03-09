// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ---------------------------------------------------------------------------
// valueToString
// ---------------------------------------------------------------------------

func TestValueToString(t *testing.T) {
	t.Run("nil value returns empty string", func(t *testing.T) {
		result := valueToString(nil)
		assert.Equal(t, "", result)
	})

	t.Run("starlark.String returns unwrapped string", func(t *testing.T) {
		result := valueToString(starlark.String("hello world"))
		assert.Equal(t, "hello world", result)
	})

	t.Run("starlark.Int returns string representation", func(t *testing.T) {
		result := valueToString(starlark.MakeInt(42))
		assert.Equal(t, "42", result)
	})

	t.Run("starlark.Bool returns string representation", func(t *testing.T) {
		result := valueToString(starlark.True)
		assert.Equal(t, "True", result)
	})

	t.Run("starlark.None returns string representation", func(t *testing.T) {
		result := valueToString(starlark.None)
		assert.Equal(t, "None", result)
	})
}

// ---------------------------------------------------------------------------
// dictGetString
// ---------------------------------------------------------------------------

func makeDict(t *testing.T, pairs ...interface{}) *starlark.Dict {
	t.Helper()
	d := new(starlark.Dict)
	for i := 0; i < len(pairs)-1; i += 2 {
		key := starlark.String(pairs[i].(string))
		val := starlark.String(pairs[i+1].(string))
		require.NoError(t, d.SetKey(key, val))
	}
	return d
}

func TestDictGetString(t *testing.T) {
	t.Run("key exists returns value and true", func(t *testing.T) {
		d := makeDict(t, "name", "Alice")
		val, ok := dictGetString(d, "name")
		assert.True(t, ok)
		assert.Equal(t, "Alice", val)
	})

	t.Run("key missing returns empty string and false", func(t *testing.T) {
		d := makeDict(t, "name", "Alice")
		val, ok := dictGetString(d, "missing")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("empty dict returns false", func(t *testing.T) {
		d := new(starlark.Dict)
		val, ok := dictGetString(d, "key")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

// ---------------------------------------------------------------------------
// dictGetStringOrFallback
// ---------------------------------------------------------------------------

func TestDictGetStringOrFallback(t *testing.T) {
	t.Run("key found returns value", func(t *testing.T) {
		d := makeDict(t, "label", "My Label", "value", "my-value")
		result := dictGetStringOrFallback(d, "label", "value")
		assert.Equal(t, "My Label", result)
	})

	t.Run("key not found falls back to fallback key", func(t *testing.T) {
		d := makeDict(t, "value", "my-value")
		result := dictGetStringOrFallback(d, "label", "value")
		assert.Equal(t, "my-value", result)
	})

	t.Run("key empty uses fallback key", func(t *testing.T) {
		d := makeDict(t, "value", "fallback-val")
		result := dictGetStringOrFallback(d, "", "value")
		assert.Equal(t, "fallback-val", result)
	})

	t.Run("key empty and fallback empty returns empty", func(t *testing.T) {
		d := makeDict(t, "value", "something")
		result := dictGetStringOrFallback(d, "", "")
		assert.Equal(t, "", result)
	})

	t.Run("key not found and fallback empty returns empty", func(t *testing.T) {
		d := makeDict(t, "other", "val")
		result := dictGetStringOrFallback(d, "missing", "")
		assert.Equal(t, "", result)
	})

	t.Run("both key and fallback missing from dict returns empty", func(t *testing.T) {
		d := new(starlark.Dict)
		result := dictGetStringOrFallback(d, "key", "fallback")
		assert.Equal(t, "", result)
	})
}

// ---------------------------------------------------------------------------
// structGetString
// ---------------------------------------------------------------------------

func makeStruct(pairs ...interface{}) *starlarkstruct.Struct {
	members := make(starlark.StringDict)
	for i := 0; i < len(pairs)-1; i += 2 {
		key := pairs[i].(string)
		val := starlark.String(pairs[i+1].(string))
		members[key] = val
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, members)
}

func TestStructGetString(t *testing.T) {
	t.Run("key exists returns value and true", func(t *testing.T) {
		s := makeStruct("name", "Bob")
		val, ok := structGetString(s, "name")
		assert.True(t, ok)
		assert.Equal(t, "Bob", val)
	})

	t.Run("key missing returns empty string and false", func(t *testing.T) {
		s := makeStruct("name", "Bob")
		val, ok := structGetString(s, "missing")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("empty struct returns false", func(t *testing.T) {
		s := makeStruct()
		val, ok := structGetString(s, "key")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

// ---------------------------------------------------------------------------
// structGetStringOrFallback
// ---------------------------------------------------------------------------

func TestStructGetStringOrFallback(t *testing.T) {
	t.Run("key found returns value", func(t *testing.T) {
		s := makeStruct("label", "The Label", "value", "the-value")
		result := structGetStringOrFallback(s, "label", "value")
		assert.Equal(t, "The Label", result)
	})

	t.Run("key not found falls back to fallback key", func(t *testing.T) {
		s := makeStruct("value", "the-value")
		result := structGetStringOrFallback(s, "label", "value")
		assert.Equal(t, "the-value", result)
	})

	t.Run("key empty uses fallback key", func(t *testing.T) {
		s := makeStruct("value", "fallback-val")
		result := structGetStringOrFallback(s, "", "value")
		assert.Equal(t, "fallback-val", result)
	})

	t.Run("key empty and fallback empty returns empty", func(t *testing.T) {
		s := makeStruct("value", "something")
		result := structGetStringOrFallback(s, "", "")
		assert.Equal(t, "", result)
	})

	t.Run("key not found and fallback empty returns empty", func(t *testing.T) {
		s := makeStruct("other", "val")
		result := structGetStringOrFallback(s, "missing", "")
		assert.Equal(t, "", result)
	})

	t.Run("both key and fallback missing returns empty", func(t *testing.T) {
		s := makeStruct()
		result := structGetStringOrFallback(s, "key", "fallback")
		assert.Equal(t, "", result)
	})
}

// ---------------------------------------------------------------------------
// hasKey
// ---------------------------------------------------------------------------

func TestHasKey(t *testing.T) {
	t.Run("dict has key returns true", func(t *testing.T) {
		d := makeDict(t, "foo", "bar")
		assert.True(t, hasKey(d, "foo"))
	})

	t.Run("dict missing key returns false", func(t *testing.T) {
		d := makeDict(t, "foo", "bar")
		assert.False(t, hasKey(d, "missing"))
	})

	t.Run("struct has key returns true", func(t *testing.T) {
		s := makeStruct("name", "Alice")
		assert.True(t, hasKey(s, "name"))
	})

	t.Run("struct missing key returns false", func(t *testing.T) {
		s := makeStruct("name", "Alice")
		assert.False(t, hasKey(s, "missing"))
	})

	t.Run("other value type returns false", func(t *testing.T) {
		assert.False(t, hasKey(starlark.String("hello"), "key"))
	})

	t.Run("none returns false", func(t *testing.T) {
		assert.False(t, hasKey(starlark.None, "key"))
	})
}

// ---------------------------------------------------------------------------
// getValue
// ---------------------------------------------------------------------------

func TestGetValue(t *testing.T) {
	t.Run("dict key found returns value and true", func(t *testing.T) {
		d := makeDict(t, "x", "xval")
		val, ok := getValue(d, "x")
		assert.True(t, ok)
		assert.Equal(t, "xval", val)
	})

	t.Run("dict key not found returns empty and false", func(t *testing.T) {
		d := makeDict(t, "x", "xval")
		val, ok := getValue(d, "missing")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("struct key found returns value and true", func(t *testing.T) {
		s := makeStruct("name", "Charlie")
		val, ok := getValue(s, "name")
		assert.True(t, ok)
		assert.Equal(t, "Charlie", val)
	})

	t.Run("struct key not found returns empty and false", func(t *testing.T) {
		s := makeStruct("name", "Charlie")
		val, ok := getValue(s, "missing")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("other value type returns empty and false", func(t *testing.T) {
		val, ok := getValue(starlark.MakeInt(1), "key")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

// ---------------------------------------------------------------------------
// inferColumns
// ---------------------------------------------------------------------------

func TestInferColumns(t *testing.T) {
	t.Run("dict returns keys as columns", func(t *testing.T) {
		d := makeDict(t, "name", "Alice", "age", "30")
		cols, err := inferColumns(d)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"name", "age"}, cols)
	})

	t.Run("empty dict returns empty columns", func(t *testing.T) {
		d := new(starlark.Dict)
		cols, err := inferColumns(d)
		require.NoError(t, err)
		assert.Empty(t, cols)
	})

	t.Run("struct returns field names as columns", func(t *testing.T) {
		s := makeStruct("x", "1", "y", "2", "z", "3")
		cols, err := inferColumns(s)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"x", "y", "z"}, cols)
	})

	t.Run("empty struct returns empty columns", func(t *testing.T) {
		s := makeStruct()
		cols, err := inferColumns(s)
		require.NoError(t, err)
		assert.Empty(t, cols)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := inferColumns(starlark.String("not a dict or struct"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("starlark.None returns error", func(t *testing.T) {
		_, err := inferColumns(starlark.None)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// noopBuiltin
// ---------------------------------------------------------------------------

func TestNoopBuiltin(t *testing.T) {
	t.Run("returns a builtin with the given name", func(t *testing.T) {
		b := noopBuiltin("ui.markdown")
		require.NotNil(t, b)
		assert.Equal(t, "ui.markdown", b.Name())
	})

	t.Run("calling the builtin returns None with no error", func(t *testing.T) {
		b := noopBuiltin("ui.test")
		thread := &starlark.Thread{Name: "test"}
		result, err := b.CallInternal(thread, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})
}

// ---------------------------------------------------------------------------
// noopTurnFunc
// ---------------------------------------------------------------------------

func TestNoopTurnFunc(t *testing.T) {
	t.Run("returns a non-nil function", func(t *testing.T) {
		fn := noopTurnFunc()
		require.NotNil(t, fn)
	})

	t.Run("calling the function returns a TurnHandle with nil writer", func(t *testing.T) {
		fn := noopTurnFunc()
		thread := &starlark.Thread{Name: "test"}
		result, err := fn(thread, nil, nil, nil)
		require.NoError(t, err)
		th, ok := result.(*TurnHandle)
		require.True(t, ok)
		assert.Nil(t, th.writer)
	})
}

// ---------------------------------------------------------------------------
// noopUIModule
// ---------------------------------------------------------------------------

func TestNoopUIModule(t *testing.T) {
	t.Run("returns a module named ui", func(t *testing.T) {
		m := noopUIModule()
		require.NotNil(t, m)
		assert.Equal(t, "ui", m.Name)
	})

	t.Run("module has expected members", func(t *testing.T) {
		m := noopUIModule()
		expectedKeys := []string{
			"user_turn", "assistant_turn", "prompt", "confirm",
			"progress_bar", "markdown", "table", "panel", "select",
			"render", "link", "pager", "code", "diff", "tree", "banner", "progress",
		}
		for _, key := range expectedKeys {
			assert.Contains(t, m.Members, key, "expected member %q", key)
		}
	})

	t.Run("noop members return None when called", func(t *testing.T) {
		m := noopUIModule()
		thread := &starlark.Thread{Name: "test"}

		for _, key := range []string{"markdown", "table", "panel", "render", "pager", "code", "diff", "tree", "banner", "progress"} {
			member := m.Members[key]
			require.NotNil(t, member, "member %q should not be nil", key)
			b, ok := member.(*starlark.Builtin)
			require.True(t, ok, "member %q should be a Builtin", key)
			result, err := b.CallInternal(thread, nil, nil)
			require.NoError(t, err, "member %q should not return error", key)
			assert.Equal(t, starlark.None, result, "member %q should return None", key)
		}
	})
}

// ---------------------------------------------------------------------------
// NewUIModule / NewUIModuleWithUIWriter
// ---------------------------------------------------------------------------

func TestNewUIModule(t *testing.T) {
	t.Run("returns a non-nil noop module when no writer", func(t *testing.T) {
		m := NewUIModule()
		require.NotNil(t, m)
		assert.Equal(t, "ui", m.Name)
	})
}

func TestNewUIModuleWithUIWriter(t *testing.T) {
	t.Run("nil writer returns noop module", func(t *testing.T) {
		m := NewUIModuleWithUIWriter(0, nil)
		require.NotNil(t, m)
		assert.Equal(t, "ui", m.Name)
	})

	t.Run("non-TTY writer returns noop module", func(t *testing.T) {
		// mockOutputWriter from runtime_test.go returns IsTTY()=false
		writer := &mockOutputWriter{}
		m := NewUIModuleWithUIWriter(0, writer)
		require.NotNil(t, m)
		assert.Equal(t, "ui", m.Name)
	})
}

// ---------------------------------------------------------------------------
// TurnHandle
// ---------------------------------------------------------------------------

func TestTurnHandle(t *testing.T) {
	t.Run("String returns <turn>", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		assert.Equal(t, "<turn>", th.String())
	})

	t.Run("Type returns turn", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		assert.Equal(t, "turn", th.Type())
	})

	t.Run("Freeze does not panic", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		assert.NotPanics(t, th.Freeze)
	})

	t.Run("Truth returns True", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		assert.Equal(t, starlark.True, th.Truth())
	})

	t.Run("Hash returns error", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		_, err := th.Hash()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unhashable")
	})

	t.Run("AttrNames returns expected names", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		names := th.AttrNames()
		assert.Contains(t, names, "step")
		assert.Contains(t, names, "done")
		assert.Contains(t, names, "fail")
		assert.Contains(t, names, "stream")
		assert.Contains(t, names, "info")
		assert.Contains(t, names, "warn")
		assert.Contains(t, names, "subturn")
	})

	t.Run("Attr returns builtin for known attrs", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		for _, name := range []string{"step", "stream", "done", "fail", "info", "warn", "subturn"} {
			val, err := th.Attr(name)
			require.NoError(t, err, "Attr(%q) should not error", name)
			assert.NotNil(t, val, "Attr(%q) should not return nil", name)
		}
	})

	t.Run("Attr returns nil for unknown attr", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		val, err := th.Attr("unknown")
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("done with nil writer succeeds", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		thread := &starlark.Thread{Name: "test"}
		result, err := th.done(thread, nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("fail with nil writer succeeds", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		thread := &starlark.Thread{Name: "test"}
		result, err := th.fail(thread, nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("stream with nil writer succeeds", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("hello")}
		result, err := th.stream(thread, nil, args, nil)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("info with nil writer succeeds", func(t *testing.T) {
		th := &TurnHandle{writer: nil}
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("status")}
		result, err := th.info(thread, nil, args, nil)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})
}

// ---------------------------------------------------------------------------
// makeLinkFunc
// ---------------------------------------------------------------------------

func TestMakeLinkFunc(t *testing.T) {
	t.Run("returns a string starlark value", func(t *testing.T) {
		fn := makeLinkFunc("")
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{}
		kwargs := []starlark.Tuple{
			{starlark.String("text"), starlark.String("Click me")},
			{starlark.String("url"), starlark.String("https://example.com")},
		}
		result, err := fn(thread, nil, args, kwargs)
		require.NoError(t, err)
		_, ok := result.(starlark.String)
		assert.True(t, ok, "result should be a starlark.String")
	})

	t.Run("missing required args returns error", func(t *testing.T) {
		fn := makeLinkFunc("")
		thread := &starlark.Thread{Name: "test"}
		_, err := fn(thread, nil, nil, nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// makeProgressBarFunc
// ---------------------------------------------------------------------------

func TestMakeProgressBarFunc(t *testing.T) {
	t.Run("creates a ProgressBarHandle", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeProgressBarFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("total"), starlark.MakeInt(100)},
		}
		result, err := fn(thread, nil, nil, kwargs)
		require.NoError(t, err)
		_, ok := result.(*ProgressBarHandle)
		assert.True(t, ok)
	})

	t.Run("missing total arg returns error", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeProgressBarFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		_, err := fn(thread, nil, nil, nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// makeBannerFunc
// ---------------------------------------------------------------------------

func TestMakeBannerFunc(t *testing.T) {
	t.Run("writes banner to writer and returns None", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeBannerFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("title"), starlark.String("Hello")},
		}
		result, err := fn(thread, nil, nil, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
		assert.NotEmpty(t, buf.Bytes())
	})

	t.Run("missing title arg returns error", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeBannerFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		_, err := fn(thread, nil, nil, nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// makeMarkdownFunc
// ---------------------------------------------------------------------------

func TestMakeMarkdownFunc(t *testing.T) {
	t.Run("writes markdown and returns None", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeMarkdownFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("# Hello")}
		result, err := fn(thread, nil, args, nil)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("missing content arg returns error", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeMarkdownFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		_, err := fn(thread, nil, nil, nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// makeCodeFunc
// ---------------------------------------------------------------------------

func TestMakeCodeFunc(t *testing.T) {
	t.Run("writes code and returns None", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeCodeFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("content"), starlark.String("fmt.Println(\"hello\")")},
			{starlark.String("lang"), starlark.String("go")},
		}
		result, err := fn(thread, nil, nil, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("missing content arg returns error", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeCodeFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		_, err := fn(thread, nil, nil, nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// makeDiffFunc
// ---------------------------------------------------------------------------

func TestMakeDiffFunc(t *testing.T) {
	t.Run("writes diff and returns None", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeDiffFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("content"), starlark.String("--- a\n+++ b\n@@ -1 +1 @@\n-old\n+new")},
		}
		result, err := fn(thread, nil, nil, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
	})

	t.Run("missing content arg returns error", func(t *testing.T) {
		var buf bytes.Buffer
		fn := makeDiffFunc("", &buf)
		thread := &starlark.Thread{Name: "test"}
		_, err := fn(thread, nil, nil, nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// config_adapter.go: Providers()
// ---------------------------------------------------------------------------

func TestRuntimeProviders(t *testing.T) {
	t.Run("empty runtime returns empty map", func(t *testing.T) {
		r := NewRuntime(t.TempDir())
		result := r.Providers()
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("returns copy of providers map", func(t *testing.T) {
		r := NewRuntime(t.TempDir())
		r.providers = map[string]ProviderConfig{
			"anthropic": {Type: "anthropic", APIKey: "key1"},
			"openai":    {Type: "openai", APIKey: "key2"},
		}
		result := r.Providers()
		assert.Len(t, result, 2)
		assert.Equal(t, "anthropic", result["anthropic"].Type)
		assert.Equal(t, "openai", result["openai"].Type)
	})

	t.Run("modifying returned map does not affect original", func(t *testing.T) {
		r := NewRuntime(t.TempDir())
		r.providers = map[string]ProviderConfig{
			"test": {Type: "test"},
		}
		result := r.Providers()
		delete(result, "test")
		assert.Len(t, r.providers, 1, "original should still have provider")
	})
}

// ---------------------------------------------------------------------------
// runtime.go: SetContext and CreateSessionModuleForCtx
// ---------------------------------------------------------------------------

func TestRuntimeSetContext(t *testing.T) {
	t.Run("sets context on runtime", func(t *testing.T) {
		r := NewRuntime(t.TempDir())
		// context is already set to Background() in constructor;
		// calling SetContext should not panic.
		assert.NotPanics(t, func() {
			ctx := t.Context()
			r.SetContext(ctx)
			assert.Equal(t, ctx, r.ctx)
		})
	})
}

func TestRuntimeCreateSessionModuleForCtx(t *testing.T) {
	t.Run("nil session service returns noop builtin", func(t *testing.T) {
		r := NewRuntime(t.TempDir())
		// sessionService is nil by default
		val := r.CreateSessionModuleForCtx(nil)
		require.NotNil(t, val)
		// calling the noop builtin should return an error about uninitialized service
		b, ok := val.(*starlark.Builtin)
		require.True(t, ok)
		thread := &starlark.Thread{Name: "test"}
		_, err := b.CallInternal(thread, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session service not initialized")
	})
}
