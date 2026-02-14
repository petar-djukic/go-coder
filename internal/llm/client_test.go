// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
)

// mockEventStream implements EventStream for testing.
type mockEventStream struct {
	ch  chan brtypes.ConverseStreamOutput
	err error
}

func (m *mockEventStream) Events() <-chan brtypes.ConverseStreamOutput {
	return m.ch
}

func (m *mockEventStream) Close() error {
	return nil
}

func (m *mockEventStream) Err() error {
	return m.err
}

// mockConverseStreamOutput wraps a mockEventStream to mimic the SDK output.
type mockConverseStreamOutput struct {
	stream *mockEventStream
}

// mockBedrockAPI implements BedrockAPI for testing.
type mockBedrockAPI struct {
	tokens       []string
	inputTokens  int
	outputTokens int
	throttleN    int // Number of times to return ThrottlingException before success
	callCount    int
	failWithErr  error // Return this error on every call
}

func (m *mockBedrockAPI) ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error) {
	m.callCount++

	if m.failWithErr != nil {
		return nil, m.failWithErr
	}

	if m.callCount <= m.throttleN {
		return nil, &brtypes.ThrottlingException{
			Message: aws.String("Rate exceeded"),
		}
	}

	// Create a channel and send events.
	ch := make(chan brtypes.ConverseStreamOutput, len(m.tokens)+1)

	for _, token := range m.tokens {
		ch <- &brtypes.ConverseStreamOutputMemberContentBlockDelta{
			Value: brtypes.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(0),
				Delta: &brtypes.ContentBlockDeltaMemberText{
					Value: token,
				},
			},
		}
	}

	// Send metadata with token usage.
	ch <- &brtypes.ConverseStreamOutputMemberMetadata{
		Value: brtypes.ConverseStreamMetadataEvent{
			Usage: &brtypes.TokenUsage{
				InputTokens:  aws.Int32(int32(m.inputTokens)),
				OutputTokens: aws.Int32(int32(m.outputTokens)),
				TotalTokens:  aws.Int32(int32(m.inputTokens + m.outputTokens)),
			},
			Metrics: &brtypes.ConverseStreamMetrics{
				LatencyMs: aws.Int64(100),
			},
		},
	}

	close(ch)

	// Return the output with the mock stream.
	// We use the real ConverseStreamOutput by providing our mock stream
	// through the internal mechanism. Since we can't directly construct
	// ConverseStreamOutput (unexported eventStream field), we use a
	// test-specific approach with consumeStream directly.
	return nil, &useDirectStreamError{stream: &mockEventStream{ch: ch}}
}

// useDirectStreamError signals that we should bypass the SDK output and
// use the embedded stream directly. This is a test-only mechanism.
type useDirectStreamError struct {
	stream EventStream
}

func (e *useDirectStreamError) Error() string {
	return "use direct stream"
}

func TestConsumeStream_TokensDelivered(t *testing.T) {
	tokens := []string{"Here", " is", " the", " code"}
	ch := make(chan brtypes.ConverseStreamOutput, len(tokens)+1)

	for _, token := range tokens {
		ch <- &brtypes.ConverseStreamOutputMemberContentBlockDelta{
			Value: brtypes.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(0),
				Delta: &brtypes.ContentBlockDeltaMemberText{
					Value: token,
				},
			},
		}
	}

	ch <- &brtypes.ConverseStreamOutputMemberMetadata{
		Value: brtypes.ConverseStreamMetadataEvent{
			Usage: &brtypes.TokenUsage{
				InputTokens:  aws.Int32(150),
				OutputTokens: aws.Int32(42),
				TotalTokens:  aws.Int32(192),
			},
			Metrics: &brtypes.ConverseStreamMetrics{
				LatencyMs: aws.Int64(100),
			},
		},
	}
	close(ch)

	stream := &mockEventStream{ch: ch}
	tokenCh := make(chan string, 64)

	ctx := context.Background()
	response := consumeStream(ctx, stream, tokenCh)

	// Collect tokens from channel.
	var received []string
	for token := range tokenCh {
		received = append(received, token)
	}

	assert.Equal(t, tokens, received)
	assert.Equal(t, "Here is the code", response.FullText)
	assert.Equal(t, 150, response.Usage.InputTokens)
	assert.Equal(t, 42, response.Usage.OutputTokens)
}

func TestConsumeStream_AccumulatesFullText(t *testing.T) {
	tokens := []string{"func ", "Hello", "() ", "string"}
	ch := make(chan brtypes.ConverseStreamOutput, len(tokens))

	for _, token := range tokens {
		ch <- &brtypes.ConverseStreamOutputMemberContentBlockDelta{
			Value: brtypes.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(0),
				Delta: &brtypes.ContentBlockDeltaMemberText{
					Value: token,
				},
			},
		}
	}
	close(ch)

	stream := &mockEventStream{ch: ch}
	tokenCh := make(chan string, 64)
	ctx := context.Background()

	response := consumeStream(ctx, stream, tokenCh)

	// Drain token channel.
	for range tokenCh {
	}

	assert.Equal(t, "func Hello() string", response.FullText)
}

func TestConsumeStream_ContextCancellation(t *testing.T) {
	ch := make(chan brtypes.ConverseStreamOutput, 4)

	tokens := []string{"partial", " content", " not", " received"}
	for _, token := range tokens {
		ch <- &brtypes.ConverseStreamOutputMemberContentBlockDelta{
			Value: brtypes.ContentBlockDeltaEvent{
				ContentBlockIndex: aws.Int32(0),
				Delta: &brtypes.ContentBlockDeltaMemberText{
					Value: token,
				},
			},
		}
	}
	// Don't close ch - we'll cancel context instead.

	stream := &mockEventStream{ch: ch}
	tokenCh := make(chan string, 64)

	ctx, cancel := context.WithCancel(context.Background())

	// Start consuming in a goroutine.
	var response *types.StreamResponse
	done := make(chan struct{})
	go func() {
		response = consumeStream(ctx, stream, tokenCh)
		close(done)
	}()

	// Read the first two tokens, then cancel.
	var received []string
	for i := 0; i < 2; i++ {
		token, ok := <-tokenCh
		if !ok {
			break
		}
		received = append(received, token)
	}
	cancel()
	<-done

	// We got at least the tokens before cancellation.
	assert.GreaterOrEqual(t, len(received), 1)
	assert.NotEmpty(t, response.FullText)
}

func TestConsumeStream_TokenUsageFromMetadata(t *testing.T) {
	ch := make(chan brtypes.ConverseStreamOutput, 2)

	ch <- &brtypes.ConverseStreamOutputMemberContentBlockDelta{
		Value: brtypes.ContentBlockDeltaEvent{
			ContentBlockIndex: aws.Int32(0),
			Delta:             &brtypes.ContentBlockDeltaMemberText{Value: "hello"},
		},
	}
	ch <- &brtypes.ConverseStreamOutputMemberMetadata{
		Value: brtypes.ConverseStreamMetadataEvent{
			Usage: &brtypes.TokenUsage{
				InputTokens:  aws.Int32(150),
				OutputTokens: aws.Int32(42),
				TotalTokens:  aws.Int32(192),
			},
			Metrics: &brtypes.ConverseStreamMetrics{
				LatencyMs: aws.Int64(100),
			},
		},
	}
	close(ch)

	stream := &mockEventStream{ch: ch}
	tokenCh := make(chan string, 64)
	ctx := context.Background()

	response := consumeStream(ctx, stream, tokenCh)
	for range tokenCh {
	}

	assert.Equal(t, 150, response.Usage.InputTokens)
	assert.Equal(t, 42, response.Usage.OutputTokens)
}

func TestNewClientWithAPI(t *testing.T) {
	api := &mockBedrockAPI{}
	client := NewClientWithAPI(api, ClientConfig{
		ModelID:   "anthropic.claude-sonnet-4-5-20250929-v1:0",
		Region:    "us-east-1",
		MaxTokens: 2048,
	})

	assert.NotNil(t, client)
	assert.Equal(t, "anthropic.claude-sonnet-4-5-20250929-v1:0", client.modelID)
	assert.Equal(t, 2048, client.maxTokens)
	assert.Equal(t, defaultTimeout, client.timeout)
}

func TestNewClientWithAPI_Defaults(t *testing.T) {
	client := NewClientWithAPI(&mockBedrockAPI{}, ClientConfig{
		ModelID: "test-model",
		Region:  "us-west-2",
	})

	assert.Equal(t, 4096, client.maxTokens)
	assert.Equal(t, defaultTimeout, client.timeout)
}

func TestClient_ClassifyError_AccessDenied(t *testing.T) {
	client := &Client{modelID: "test-model"}
	err := client.classifyError(&brtypes.AccessDeniedException{
		Message: aws.String("not authorized"),
	})

	assert.True(t, errors.Is(err, ErrLLMFailure))
	assert.Contains(t, err.Error(), "credential")
}

func TestClient_ClassifyError_ResourceNotFound(t *testing.T) {
	client := &Client{modelID: "nonexistent-model"}
	err := client.classifyError(&brtypes.ResourceNotFoundException{
		Message: aws.String("model not found"),
	})

	assert.True(t, errors.Is(err, ErrLLMFailure))
	assert.Contains(t, err.Error(), "nonexistent-model")
}

func TestClient_ClassifyError_Timeout(t *testing.T) {
	client := &Client{modelID: "test", timeout: 30 * time.Second}
	err := client.classifyError(context.DeadlineExceeded)

	assert.True(t, errors.Is(err, ErrLLMFailure))
	assert.Contains(t, err.Error(), "timed out")
}

func TestClient_CumulativeUsage(t *testing.T) {
	client := &Client{
		usage: types.TokenUsage{InputTokens: 100, OutputTokens: 50},
	}

	usage := client.CumulativeUsage()
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
	assert.Equal(t, 150, usage.Total())
}

func TestTokenUsage_Total(t *testing.T) {
	u := types.TokenUsage{InputTokens: 200, OutputTokens: 100}
	assert.Equal(t, 300, u.Total())
}
