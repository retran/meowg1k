/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package ui provides utilities for user interface interactions.
package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

const (
	spinnerCharset = 11 // ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏
	spinnerDelay   = 100 * time.Millisecond
	spinnerColor   = "yellow"
	spinnerPrefix  = " "
	spinnerSuffix  = " "
	spinnerMessage = "Processing..."
)

// RunWithSpinner executes the provided action function with a spinner displayed in the terminal.
func RunWithSpinner[T any](action func() (T, error)) (T, error) {
	return RunWithSpinnerWithMessage(action, spinnerMessage)
}

// RunWithSpinnerWithMessage executes the provided action function with a spinner and a custom message
// displayed in the terminal.
func RunWithSpinnerWithMessage[T any](action func() (T, error), message string) (T, error) {
	s := spinner.New(spinner.CharSets[spinnerCharset], spinnerDelay, spinner.WithWriter(os.Stderr))

	if err := s.Color(spinnerColor); err != nil {
		return *new(T), fmt.Errorf("internal ui error: invalid spinner color: %w", err)
	}

	s.Prefix = spinnerPrefix
	s.Suffix = spinnerSuffix + message
	s.Start()
	defer s.Stop()

	result, err := action()
	if err != nil {
		return *new(T), err
	}

	return result, nil
}
