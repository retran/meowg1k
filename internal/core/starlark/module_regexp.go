package starlark

import (
	"regexp"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewRegexpModule creates the regexp module
func NewRegexpModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "regexp",
		Members: starlark.StringDict{
			"match":    starlark.NewBuiltin("regexp.match", regexpMatch),
			"find_all": starlark.NewBuiltin("regexp.find_all", regexpFindAll),
			"replace":  starlark.NewBuiltin("regexp.replace", regexpReplace),
			"split":    starlark.NewBuiltin("regexp.split", regexpSplit),
		},
	}
}

// regexpMatch checks if string matches pattern
func regexpMatch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	if err := starlark.UnpackPositionalArgs("regexp.match", args, kwargs, 2, &pattern, &text); err != nil {
		return nil, err
	}

	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		return nil, err
	}

	return starlark.Bool(matched), nil
}

// regexpFindAll finds all matches
func regexpFindAll(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern string
		text    string
		limit   int = -1
	)
	if err := starlark.UnpackArgs("regexp.find_all", args, kwargs, "pattern", &pattern, "text", &text, "limit?", &limit); err != nil {
		return nil, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	matches := re.FindAllString(text, limit)
	items := make([]starlark.Value, len(matches))
	for i, match := range matches {
		items[i] = starlark.String(match)
	}

	return starlark.NewList(items), nil
}

// regexpReplace replaces matches with replacement string
func regexpReplace(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text, replacement string
	if err := starlark.UnpackPositionalArgs("regexp.replace", args, kwargs, 3, &pattern, &text, &replacement); err != nil {
		return nil, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	result := re.ReplaceAllString(text, replacement)
	return starlark.String(result), nil
}

// regexpSplit splits string by pattern
func regexpSplit(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern string
		text    string
		limit   int = -1
	)
	if err := starlark.UnpackArgs("regexp.split", args, kwargs, "pattern", &pattern, "text", &text, "limit?", &limit); err != nil {
		return nil, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	parts := re.Split(text, limit)
	items := make([]starlark.Value, len(parts))
	for i, part := range parts {
		items[i] = starlark.String(part)
	}

	return starlark.NewList(items), nil
}
