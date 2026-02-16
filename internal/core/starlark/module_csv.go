// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewCSVModule creates the csv module
func NewCSVModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "csv",
		Members: starlark.StringDict{
			"parse":     starlark.NewBuiltin("csv.parse", csvParse),
			"stringify": starlark.NewBuiltin("csv.stringify", csvStringify),
		},
	}
}

// csvParse parses a CSV string into Starlark list of lists or list of dicts
func csvParse(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data      string
		hasHeader bool   = false
		delimiter string = ","
	)
	if err := starlark.UnpackArgs("csv.parse", args, kwargs, "data", &data, "has_header?", &hasHeader, "delimiter?", &delimiter); err != nil {
		return nil, err
	}

	if len(delimiter) != 1 {
		return nil, fmt.Errorf("csv.parse: delimiter must be a single character, got %q", delimiter)
	}

	reader := csv.NewReader(bytes.NewReader([]byte(data)))
	reader.Comma = rune(delimiter[0])

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv.parse: %w", err)
	}

	if len(records) == 0 {
		return starlark.NewList(nil), nil
	}

	if hasHeader {
		// Parse as list of dicts with headers
		if len(records) < 2 {
			// Only header, no data rows
			return starlark.NewList(nil), nil
		}

		headers := records[0]
		dataRows := records[1:]

		result := make([]starlark.Value, 0, len(dataRows))
		for _, row := range dataRows {
			dict := starlark.NewDict(len(headers))
			for i, header := range headers {
				var value starlark.Value = starlark.None
				if i < len(row) {
					value = starlark.String(row[i])
				}
				dict.SetKey(starlark.String(header), value)
			}
			result = append(result, dict)
		}
		return starlark.NewList(result), nil
	}

	// Parse as list of lists
	result := make([]starlark.Value, 0, len(records))
	for _, row := range records {
		rowList := make([]starlark.Value, 0, len(row))
		for _, cell := range row {
			rowList = append(rowList, starlark.String(cell))
		}
		result = append(result, starlark.NewList(rowList))
	}
	return starlark.NewList(result), nil
}

// csvStringify converts Starlark list of lists or list of dicts to CSV string
func csvStringify(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value     starlark.Value
		delimiter string = ","
		headers   *starlark.List
	)
	if err := starlark.UnpackArgs("csv.stringify", args, kwargs, "value", &value, "delimiter?", &delimiter, "headers?", &headers); err != nil {
		return nil, err
	}

	if len(delimiter) != 1 {
		return nil, fmt.Errorf("csv.stringify: delimiter must be a single character, got %q", delimiter)
	}

	list, ok := value.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("csv.stringify: expected list, got %s", value.Type())
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = rune(delimiter[0])

	// Write headers if provided
	if headers != nil {
		headerRow := make([]string, 0, headers.Len())
		for i := 0; i < headers.Len(); i++ {
			headerRow = append(headerRow, headers.Index(i).String())
		}
		if err := writer.Write(headerRow); err != nil {
			return nil, fmt.Errorf("csv.stringify: failed to write headers: %w", err)
		}
	}

	// Process each row
	for i := 0; i < list.Len(); i++ {
		item := list.Index(i)

		switch v := item.(type) {
		case *starlark.List:
			// List of lists
			row := make([]string, 0, v.Len())
			for j := 0; j < v.Len(); j++ {
				cell := v.Index(j)
				row = append(row, cell.String())
			}
			if err := writer.Write(row); err != nil {
				return nil, fmt.Errorf("csv.stringify: failed to write row %d: %w", i, err)
			}

		case *starlark.Dict:
			// List of dicts
			if headers == nil {
				return nil, fmt.Errorf("csv.stringify: headers required when value contains dicts")
			}

			row := make([]string, 0, headers.Len())
			for j := 0; j < headers.Len(); j++ {
				headerKey := headers.Index(j)
				cellValue, found, err := v.Get(headerKey)
				if err != nil {
					return nil, fmt.Errorf("csv.stringify: failed to get value for key %s: %w", headerKey, err)
				}
				if !found {
					row = append(row, "")
				} else {
					row = append(row, cellValue.String())
				}
			}
			if err := writer.Write(row); err != nil {
				return nil, fmt.Errorf("csv.stringify: failed to write row %d: %w", i, err)
			}

		default:
			return nil, fmt.Errorf("csv.stringify: expected list or dict at index %d, got %s", i, v.Type())
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("csv.stringify: %w", err)
	}

	return starlark.String(buf.String()), nil
}
