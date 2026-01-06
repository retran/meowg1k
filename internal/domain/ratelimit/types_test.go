// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import "testing"

func TestNotEnoughTokensError(t *testing.T) {
	err := &NotEnoughTokensError{
		BucketID: "primary",
		Need:     10,
		Have:     3,
	}

	expected := `not enough tokens in bucket "primary": need 10, have 3`
	if err.Error() != expected {
		t.Fatalf("unexpected error string: %q", err.Error())
	}
}
