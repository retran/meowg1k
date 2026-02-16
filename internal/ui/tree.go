// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"sort"
	"strings"
)

// RenderTree renders a tree structure from nested data.
func RenderTree(data interface{}, title string, theme Theme, opts RenderOptions) string {
	var result strings.Builder
	
	if title != "" {
		if opts.Plain || !opts.Terminal {
			result.WriteString(fmt.Sprintf("=== %s ===\n", title))
		} else {
			result.WriteString(theme.SystemStyle.Bold(true).Render(title) + "\n")
		}
	}
	
	renderTreeNode(&result, data, "", true, opts)
	
	return strings.TrimSuffix(result.String(), "\n")
}

func renderTreeNode(result *strings.Builder, node interface{}, prefix string, isLast bool, opts RenderOptions) {
	var branch, pipe, elbow, tee string
	
	if opts.SupportsUnicode && !opts.Plain {
		pipe = "│   "
		elbow = "└── "
		tee = "├── "
	} else {
		pipe = "|   "
		elbow = "`-- "
		tee = "|-- "
	}
	
	switch v := node.(type) {
	case map[string]interface{}:
		// Sort keys for consistent output
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		
		for i, key := range keys {
			value := v[key]
			isLastChild := (i == len(keys)-1)
			
			if isLastChild {
				branch = elbow
			} else {
				branch = tee
			}
			
			result.WriteString(prefix + branch + key)
			
			// If value is also a map/slice, add newline and recurse
			switch childValue := value.(type) {
			case map[string]interface{}, []interface{}:
				result.WriteString("\n")
				newPrefix := prefix
				if isLastChild {
					newPrefix += "    "
				} else {
					newPrefix += pipe
				}
				renderTreeNode(result, childValue, newPrefix, isLastChild, opts)
			default:
				// Leaf node - print value inline
				result.WriteString(fmt.Sprintf(": %v\n", value))
			}
		}
		
	case []interface{}:
		for i, item := range v {
			isLastChild := (i == len(v)-1)
			
			if isLastChild {
				branch = elbow
			} else {
				branch = tee
			}
			
			switch childValue := item.(type) {
			case map[string]interface{}, []interface{}:
				result.WriteString(prefix + branch + fmt.Sprintf("[%d]\n", i))
				newPrefix := prefix
				if isLastChild {
					newPrefix += "    "
				} else {
					newPrefix += pipe
				}
				renderTreeNode(result, childValue, newPrefix, isLastChild, opts)
			default:
				result.WriteString(prefix + branch + fmt.Sprintf("%v\n", item))
			}
		}
		
	default:
		// Leaf value
		result.WriteString(fmt.Sprintf("%v\n", v))
	}
}
