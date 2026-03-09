package starlark

import (
	"fmt"
	"regexp"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewRegexpModule creates the regexp module.
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

// regexpMatch checks if string matches pattern.
func regexpMatch(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	if err := starlark.UnpackPositionalArgs("regexp.match", args, kwargs, 2, &pattern, &text); err != nil {
		return nil, fmt.Errorf("regexp.match: %w", err)
	}

	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		return nil, fmt.Errorf("regexp.match: %w", err)
	}

	return starlark.Bool(matched), nil
}

// regexpStringList compiles a pattern and returns a list of string results.
func regexpStringList(fnName string, args starlark.Tuple, kwargs []starlark.Tuple, run func(*regexp.Regexp, string, int) []string) (starlark.Value, error) {
	var (
		pattern string
		text    string
		limit   = -1
	)
	if err := starlark.UnpackArgs(fnName, args, kwargs, "pattern", &pattern, "text", &text, "limit?", &limit); err != nil {
		return nil, fmt.Errorf("%s: %w", fnName, err)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fnName, err)
	}

	results := run(re, text, limit)
	items := make([]starlark.Value, len(results))
	for i, r := range results {
		items[i] = starlark.String(r)
	}

	return starlark.NewList(items), nil
}

// regexpFindAll finds all matches.
func regexpFindAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return regexpStringList("regexp.find_all", args, kwargs, func(re *regexp.Regexp, text string, limit int) []string {
		return re.FindAllString(text, limit)
	})
}

// regexpReplace replaces matches with replacement string.
func regexpReplace(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text, replacement string
	if err := starlark.UnpackPositionalArgs("regexp.replace", args, kwargs, 3, &pattern, &text, &replacement); err != nil {
		return nil, fmt.Errorf("regexp.replace: %w", err)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regexp.replace: %w", err)
	}

	result := re.ReplaceAllString(text, replacement)
	return starlark.String(result), nil
}

// regexpSplit splits string by pattern.
func regexpSplit(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return regexpStringList("regexp.split", args, kwargs, func(re *regexp.Regexp, text string, limit int) []string {
		return re.Split(text, limit)
	})
}
