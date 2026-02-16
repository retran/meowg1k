package starlark

import (
	"encoding/json"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewJSONModule creates the json module
func NewJSONModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "json",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("json.parse", jsonParse),
			"stringify": starlark.NewBuiltin("json.stringify", jsonStringify),
		},
	}
}

// jsonParse parses a JSON string into Starlark values
func jsonParse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data string
	if err := starlark.UnpackPositionalArgs("json.parse", args, kwargs, 1, &data); err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}

	return goToStarlark(result), nil
}

// jsonStringify converts Starlark values to JSON string
func jsonStringify(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value  starlark.Value
		indent int = 0
	)
	if err := starlark.UnpackArgs("json.stringify", args, kwargs, "value", &value, "indent?", &indent); err != nil {
		return nil, err
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
		return nil, err
	}

	return starlark.String(string(data)), nil
}
