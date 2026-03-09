// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"gopkg.in/yaml.v3"
)

// NewYAMLModule creates the yaml module.
func NewYAMLModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "yaml",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("yaml.parse", yamlParse),
			"stringify": starlark.NewBuiltin("yaml.stringify", yamlStringify),
		},
	}
}

// yamlParse parses a YAML string into Starlark values.
func yamlParse(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("yaml.parse", args, kwargs, 1, &data); err != nil {
		return nil, fmt.Errorf("yaml.parse: %w", err)
	}
	return parseDataToStarlark("yaml.parse", data, yaml.Unmarshal)
}

// yamlStringify converts Starlark values to YAML string.
func yamlStringify(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackPositionalArgs("yaml.stringify", args, kwargs, 1, &value); err != nil {
		return nil, fmt.Errorf("yaml.stringify: %w", err)
	}
	return marshalToStarlarkString("yaml.stringify", value, yaml.Marshal)
}
