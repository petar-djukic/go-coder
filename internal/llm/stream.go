// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd005-llm-client R4 (streaming), R5 (token tracking).
package llm

import (
	"context"
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"

	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// EventStream abstracts the Bedrock ConverseStream event stream for testing.
type EventStream interface {
	Events() <-chan brtypes.ConverseStreamOutput
	Close() error
	Err() error
}

// consumeStream reads events from a Bedrock ConverseStream, sends text tokens
// through the provided channel, and accumulates the full response. The channel
// is closed when streaming completes or the context is cancelled.
//
// Implements: prd005-llm-client R4.1-R4.5, R5.1-R5.2.
func consumeStream(ctx context.Context, stream EventStream, tokenCh chan<- string) *types.StreamResponse {
	defer close(tokenCh)

	var text strings.Builder
	response := &types.StreamResponse{}

	events := stream.Events()
	for {
		select {
		case <-ctx.Done():
			// Context cancelled; return what we have so far.
			stream.Close()
			response.FullText = text.String()
			return response

		case event, ok := <-events:
			if !ok {
				// Channel closed; streaming complete.
				response.FullText = text.String()
				return response
			}

			switch v := event.(type) {
			case *brtypes.ConverseStreamOutputMemberContentBlockDelta:
				if delta, ok := v.Value.Delta.(*brtypes.ContentBlockDeltaMemberText); ok {
					text.WriteString(delta.Value)
					// Send the token through the channel, respecting cancellation.
					select {
					case tokenCh <- delta.Value:
					case <-ctx.Done():
						stream.Close()
						response.FullText = text.String()
						return response
					}
				}

			case *brtypes.ConverseStreamOutputMemberMetadata:
				if v.Value.Usage != nil {
					if v.Value.Usage.InputTokens != nil {
						response.Usage.InputTokens = int(*v.Value.Usage.InputTokens)
					}
					if v.Value.Usage.OutputTokens != nil {
						response.Usage.OutputTokens = int(*v.Value.Usage.OutputTokens)
					}
				}
			}
		}
	}
}
