// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewTimeModule creates the time module
func NewTimeModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "time",
		Members: starlark.StringDict{
			"now":    starlark.NewBuiltin("time.now", timeNow),
			"parse":  starlark.NewBuiltin("time.parse", timeParse),
			"format": starlark.NewBuiltin("time.format", timeFormat),
			"sleep":  starlark.NewBuiltin("time.sleep", timeSleep),
		},
	}
}

// timeNow returns current time as unix timestamp or formatted string
func timeNow(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var format string
	if err := starlark.UnpackArgs("time.now", args, kwargs, "format?", &format); err != nil {
		return nil, err
	}

	now := time.Now()
	if format == "" {
		return starlark.MakeInt64(now.Unix()), nil
	}

	formatted := now.Format(convertTimeFormat(format))
	return starlark.String(formatted), nil
}

// timeParse parses a time string
func timeParse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value, format string
	if err := starlark.UnpackPositionalArgs("time.parse", args, kwargs, 2, &value, &format); err != nil {
		return nil, err
	}

	t, err := time.Parse(convertTimeFormat(format), value)
	if err != nil {
		return nil, err
	}

	return starlark.MakeInt64(t.Unix()), nil
}

// timeFormat formats a unix timestamp
func timeFormat(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var timestamp int64
	var format string
	if err := starlark.UnpackPositionalArgs("time.format", args, kwargs, 2, &timestamp, &format); err != nil {
		return nil, err
	}

	t := time.Unix(timestamp, 0)
	formatted := t.Format(convertTimeFormat(format))
	return starlark.String(formatted), nil
}

// timeSleep pauses execution for the specified duration in seconds
func timeSleep(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var seconds float64
	if err := starlark.UnpackPositionalArgs("time.sleep", args, kwargs, 1, &seconds); err != nil {
		return nil, err
	}

	duration := time.Duration(seconds * float64(time.Second))
	time.Sleep(duration)

	return starlark.None, nil
}

// convertTimeFormat converts Python-style format to Go time format
func convertTimeFormat(format string) string {
	replacements := map[string]string{
		"%Y": "2006",
		"%m": "01",
		"%d": "02",
		"%H": "15",
		"%M": "04",
		"%S": "05",
		"%y": "06",
		"%B": "January",
		"%b": "Jan",
	}

	result := format
	for old, new := range replacements {
		result = strings.Replace(result, old, new, -1)
	}
	return result
}
