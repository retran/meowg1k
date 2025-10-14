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

package ratelimit

import (
	"fmt"
	"time"
)

// NotEnoughTokensError is returned when there are not enough tokens in a bucket.
type NotEnoughTokensError struct {
	BucketID string
	Need     int
	Have     int
}

func (e *NotEnoughTokensError) Error() string {
	return fmt.Sprintf("not enough tokens in bucket %q: need %d, have %d", e.BucketID, e.Need, e.Have)
}

// BucketConfig defines the configuration for a rate limit bucket.
type BucketConfig struct {
	ID          string
	Capacity    int
	RefillRate  int
	RefillEvery time.Duration
}

// AcquisitionRequest represents a request to acquire tokens from a specific bucket.
type AcquisitionRequest struct {
	ID    string
	Count int
}
