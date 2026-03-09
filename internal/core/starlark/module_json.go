package starlark

import (
	"encoding/json"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewJSONModule creates the json module.
func NewJSONModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "json",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("json.parse", jsonParse),
			"stringify": starlark.NewBuiltin("json.stringify", jsonStringify),
		},
	}
}

// jsonParse parses a JSON string into Starlark values.
func jsonParse(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("json.parse", args, kwargs, 1, &data); err != nil {
		return nil, fmt.Errorf("json.parse: %w", err)
	}
	return parseDataToStarlark("json.parse", data, json.Unmarshal)
}

// jsonStringify converts Starlark values to JSON string.
func jsonStringify(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value  starlark.Value
		indent = 0
	)
	if err := starlark.UnpackArgs("json.stringify", args, kwargs, "value", &value, "indent?", &indent); err != nil {
		return nil, fmt.Errorf("json.stringify: %w", err)
	}

	goValue := starlarkToGo(value)

	var data []byte
	var err error
	if indent > 0 {
		data, err = json.MarshalIndent(goValue, "", "  ")
	} else {
		data, err = json.Marshal(goValue)
	}

	if err != nil {
		return nil, fmt.Errorf("json.stringify: %w", err)
	}

	return starlark.String(string(data)), nil
}
