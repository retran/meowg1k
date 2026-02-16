package starlark

import (
	"os"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewEnvModule creates the env module
func NewEnvModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "env",
		Members: starlark.StringDict{
			"get":  starlark.NewBuiltin("env.get", envGet),
			"set":  starlark.NewBuiltin("env.set", envSet),
			"list": starlark.NewBuiltin("env.list", envList),
		},
	}
}

// envGet retrieves an environment variable
func envGet(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		key        string
		defaultVal starlark.Value = starlark.None
	)
	if err := starlark.UnpackArgs("env.get", args, kwargs, "key", &key, "default?", &defaultVal); err != nil {
		return nil, err
	}

	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultVal, nil
	}

	return starlark.String(value), nil
}

// envSet sets an environment variable
func envSet(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, value string
	if err := starlark.UnpackPositionalArgs("env.set", args, kwargs, 2, &key, &value); err != nil {
		return nil, err
	}

	if err := os.Setenv(key, value); err != nil {
		return nil, err
	}

	return starlark.None, nil
}

// envList returns all environment variables as a dict
func envList(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("env.list", args, kwargs, 0); err != nil {
		return nil, err
	}

	environ := os.Environ()
	dict := starlark.NewDict(len(environ))

	for _, pair := range environ {
		for i := 0; i < len(pair); i++ {
			if pair[i] == '=' {
				key := pair[:i]
				value := pair[i+1:]
				dict.SetKey(starlark.String(key), starlark.String(value))
				break
			}
		}
	}

	return dict, nil
}
