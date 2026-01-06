// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import "errors"

// ExpectedError marks an error as non-fatal for executor status reporting and retry.
//
// The executor will not retry ExpectedError, and will not mark the activity as failed.
// Callers can still decide how to handle the returned error.
//
// This is useful for tool-like activities where "not found" or "no match" should be
// returned to the caller/model as normal output rather than failing the whole flow.
type ExpectedError struct {
	Err error
}

func (e ExpectedError) Error() string {
	if e.Err == nil {
		return "expected error"
	}
	return e.Err.Error()
}

func (e ExpectedError) Unwrap() error { return e.Err }

// Expected wraps err in an ExpectedError. If err is nil, Expected returns nil.
func Expected(err error) error {
	if err == nil {
		return nil
	}
	return ExpectedError{Err: err}
}

// IsExpected reports whether err (or any wrapped error) is an ExpectedError.
func IsExpected(err error) bool {
	var ee ExpectedError
	return errors.As(err, &ee)
}
