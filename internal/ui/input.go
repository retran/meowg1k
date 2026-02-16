// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Confirm prompts the user for Y/n confirmation.
// Returns true for yes, false for no.
func Confirm(prompt string, defaultValue bool) (bool, error) {
	// Build prompt suffix
	suffix := " (Y/n) "
	if !defaultValue {
		suffix = " (y/N) "
	}
	
	fmt.Print(prompt + suffix)
	
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	
	input = strings.ToLower(strings.TrimSpace(input))
	
	// Empty input means use default
	if input == "" {
		return defaultValue, nil
	}
	
	// Check for yes/no
	if input == "y" || input == "yes" {
		return true, nil
	}
	
	if input == "n" || input == "no" {
		return false, nil
	}
	
	// Invalid input, use default
	return defaultValue, nil
}
