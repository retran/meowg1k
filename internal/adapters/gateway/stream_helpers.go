// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"github.com/retran/meowg1k/internal/domain/gateway"
)

// synthesizeStreamEvents fires stream events derived from a completed GenerateContentResponse
// through callback, then fires a final StreamEventDone. Returns the same response unmodified.
// Used by providers that do not yet have native streaming support.
func synthesizeStreamEvents(resp *gateway.GenerateContentResponse, callback gateway.StreamCallback) (*gateway.GenerateContentResponse, error) {
	if callback == nil {
		return resp, nil
	}

	if resp != nil {
		for _, block := range resp.Blocks {
			switch block.Kind {
			case gateway.ContentBlockText:
				if err := callback(gateway.StreamEvent{Kind: gateway.StreamEventText, Delta: block.Text}); err != nil {
					return resp, err
				}
			case gateway.ContentBlockReasoning:
				if err := callback(gateway.StreamEvent{Kind: gateway.StreamEventThinking, Delta: block.Text}); err != nil {
					return resp, err
				}
			}
		}
	}

	var usage *gateway.UsageMetadata
	if resp != nil && resp.Usage != nil {
		u := *resp.Usage
		usage = &u
	}

	if err := callback(gateway.StreamEvent{Kind: gateway.StreamEventDone, Usage: usage}); err != nil {
		return resp, err
	}

	return resp, nil
}
