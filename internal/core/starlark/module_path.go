package starlark

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewPathModule creates the path module.
func NewPathModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "path",
		Members: starlark.StringDict{
			"join":      starlark.NewBuiltin("path.join", pathJoin),
			"dirname":   starlark.NewBuiltin("path.dirname", pathDirname),
			"basename":  starlark.NewBuiltin("path.basename", pathBasename),
			"ext":       starlark.NewBuiltin("path.ext", pathExt),
			"abs":       starlark.NewBuiltin("path.abs", pathAbs),
			"clean":     starlark.NewBuiltin("path.clean", pathClean),
			"rel":       starlark.NewBuiltin("path.rel", pathRel),
			"stem":      starlark.NewBuiltin("path.stem", pathStem),
			"parent":    starlark.NewBuiltin("path.parent", pathParent),
			"parts":     starlark.NewBuiltin("path.parts", pathParts),
			"extension": starlark.NewBuiltin("path.extension", pathExtension),
		},
	}
}

// pathJoin joins path elements.
func pathJoin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		return starlark.String(""), nil
	}

	parts := make([]string, len(args))
	for i, arg := range args {
		str, ok := arg.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("path.join: argument %d must be string, got %s", i, arg.Type())
		}
		parts[i] = string(str)
	}

	return starlark.String(filepath.Join(parts...)), nil
}

// pathDirname returns the directory name.
func pathDirname(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.dirname", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.dirname: %w", err)
	}

	return starlark.String(filepath.Dir(path)), nil
}

// pathBasename returns the base name.
func pathBasename(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.basename", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.basename: %w", err)
	}

	return starlark.String(filepath.Base(path)), nil
}

// pathExt returns the file extension.
func pathExt(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.ext", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.ext: %w", err)
	}

	return starlark.String(filepath.Ext(path)), nil
}

// pathAbs returns the absolute path.
func pathAbs(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.abs", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.abs: %w", err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("path.abs: %w", err)
	}

	return starlark.String(abs), nil
}

// pathClean cleans the path.
func pathClean(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.clean", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.clean: %w", err)
	}

	return starlark.String(filepath.Clean(path)), nil
}

// pathRel returns relative path.
func pathRel(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var base, target string
	if err := starlark.UnpackPositionalArgs("path.rel", args, kwargs, 2, &base, &target); err != nil {
		return nil, fmt.Errorf("path.rel: %w", err)
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return nil, fmt.Errorf("path.rel: %w", err)
	}

	return starlark.String(rel), nil
}

// pathStem returns the filename without extension.
func pathStem(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.stem", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.stem: %w", err)
	}

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	return starlark.String(stem), nil
}

// pathParent returns the parent directory.
func pathParent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.parent", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.parent: %w", err)
	}

	return starlark.String(filepath.Dir(path)), nil
}

// pathParts splits path into components.
func pathParts(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackPositionalArgs("path.parts", args, kwargs, 1, &path); err != nil {
		return nil, fmt.Errorf("path.parts: %w", err)
	}

	parts := []starlark.Value{}
	path = filepath.Clean(path)

	for path != "." && path != "/" && path != "" {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]starlark.Value{starlark.String(file)}, parts...)
		}
		path = filepath.Clean(strings.TrimSuffix(dir, string(filepath.Separator)))
		if path == "." || path == "/" {
			break
		}
	}

	return starlark.NewList(parts), nil
}

// pathExtension returns the file extension (alias for pathExt for consistency).
func pathExtension(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return pathExt(thread, b, args, kwargs)
}
