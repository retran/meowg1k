// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"github.com/BurntSushi/toml"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewTOMLModule creates the toml module
func NewTOMLModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "toml",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("toml.parse", tomlParse),
			"stringify": starlark.NewBuiltin("toml.stringify", tomlStringify),
		},
	}
}

// tomlParse parses a TOML string into Starlark values
func tomlParse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("toml.parse", args, kwargs, 1, &data); err != nil {
		return nil, err
	}

	var result interface{}
	if err := toml.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}

	return goToStarlark(result), nil
}

// tomlStringify converts Starlark values to TOML string
func tomlStringify(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackPositionalArgs("toml.stringify", args, kwargs, 1, &value); err != nil {
		return nil, err
	}

	goValue := starlarkToGo(value)

	data, err := toml.Marshal(goValue)
	if err != nil {
		return nil, err
	}

	return starlark.String(string(data)), nil
}
