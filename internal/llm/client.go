// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd005-llm-client R1 (Bedrock client), R6 (error handling);
//
//	docs/ARCHITECTURE § LLM Client.
package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/petar-djukic/go-coder/pkg/types"
)

const (
	defaultTimeout    = 300 * time.Second
	maxRetryAttempts  = 3
	baseRetryDelay    = 1 * time.Second
)

// ErrLLMFailure indicates the LLM call failed (network, auth, rate limit).
var ErrLLMFailure = errors.New("LLM failure")

// ClientConfig configures the Bedrock LLM client.
type ClientConfig struct {
	ModelID   string        // Bedrock model ID (required)
	Region    string        // AWS region (required)
	Profile   string        // AWS credential profile (optional, uses default chain if empty)
	Timeout   time.Duration // Request timeout (default 300s)
	MaxTokens int           // Max tokens for the response (default 4096)
}

// BedrockAPI abstracts the Bedrock ConverseStream call for testing.
type BedrockAPI interface {
	ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
}

// Client wraps the AWS Bedrock runtime client for LLM access.
type Client struct {
	api       BedrockAPI
	modelID   string
	timeout   time.Duration
	maxTokens int
	usage     types.TokenUsage // Cumulative usage across calls
}

// NewClient creates a new Bedrock LLM client from the given configuration.
// It initializes the AWS SDK client using the standard credential chain.
//
// Implements: prd005-llm-client R1.1-R1.5.
func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("%w: model ID is required", ErrLLMFailure)
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("%w: region is required", ErrLLMFailure)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Build AWS config options.
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: loading AWS config: %v", ErrLLMFailure, err)
	}

	brClient := bedrockruntime.NewFromConfig(awsCfg)

	return &Client{
		api:       brClient,
		modelID:   cfg.ModelID,
		timeout:   timeout,
		maxTokens: maxTokens,
	}, nil
}

// NewClientWithAPI creates a client with a pre-configured API implementation.
// Used for testing with mock clients.
func NewClientWithAPI(api BedrockAPI, cfg ClientConfig) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	return &Client{
		api:       api,
		modelID:   cfg.ModelID,
		timeout:   timeout,
		maxTokens: maxTokens,
	}
}

// SendPrompt sends a prompt to Bedrock via ConverseStream and returns a channel
// that yields response tokens as they arrive. The StreamResponse is returned
// through the result channel after streaming completes.
//
// Implements: prd005-llm-client R1.2, R4.1-R4.5, R5.1-R5.3, R6.1-R6.4.
func (c *Client) SendPrompt(ctx context.Context, system []brtypes.SystemContentBlock, messages []brtypes.Message) (<-chan string, <-chan *types.StreamResponse) {
	tokenCh := make(chan string, 64)
	resultCh := make(chan *types.StreamResponse, 1)

	go func() {
		defer close(resultCh)

		response, err := c.sendWithRetry(ctx, system, messages, tokenCh)
		if err != nil {
			// On error, close tokenCh (via consumeStream or here) and send
			// an empty response with the error embedded.
			close(tokenCh)
			resultCh <- &types.StreamResponse{}
			return
		}

		// Accumulate usage.
		c.usage.InputTokens += response.Usage.InputTokens
		c.usage.OutputTokens += response.Usage.OutputTokens

		resultCh <- response
	}()

	return tokenCh, resultCh
}

// CumulativeUsage returns the total token usage across all calls.
//
// Implements: prd005-llm-client R5.3.
func (c *Client) CumulativeUsage() types.TokenUsage {
	return c.usage
}

// sendWithRetry calls ConverseStream with exponential backoff retry for
// rate limit errors.
//
// Implements: prd005-llm-client R6.1.
func (c *Client) sendWithRetry(ctx context.Context, system []brtypes.SystemContentBlock, messages []brtypes.Message, tokenCh chan<- string) (*types.StreamResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetryAttempts; attempt++ {
		if attempt > 0 {
			delay := baseRetryDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("%w: context cancelled during retry: %v", ErrLLMFailure, ctx.Err())
			}
		}

		callCtx, cancel := context.WithTimeout(ctx, c.timeout)

		input := &bedrockruntime.ConverseStreamInput{
			ModelId:  aws.String(c.modelID),
			System:   system,
			Messages: messages,
			InferenceConfig: &brtypes.InferenceConfiguration{
				MaxTokens: aws.Int32(int32(c.maxTokens)),
			},
		}

		output, err := c.api.ConverseStream(callCtx, input)
		if err != nil {
			cancel()

			// Check if this is a retryable throttling error.
			var throttle *brtypes.ThrottlingException
			if errors.As(err, &throttle) {
				lastErr = err
				continue
			}

			return nil, c.classifyError(err)
		}

		stream := output.GetStream()
		response := consumeStream(callCtx, stream, tokenCh)
		response.Retries = attempt
		cancel()
		return response, nil
	}

	return nil, fmt.Errorf("%w: rate limited after %d retries: %v", ErrLLMFailure, maxRetryAttempts, lastErr)
}

// classifyError wraps Bedrock errors into ErrLLMFailure with descriptive messages.
//
// Implements: prd005-llm-client R6.2-R6.4.
func (c *Client) classifyError(err error) error {
	var accessDenied *brtypes.AccessDeniedException
	if errors.As(err, &accessDenied) {
		return fmt.Errorf("%w: credential or permission issue: %v", ErrLLMFailure, err)
	}

	var notFound *brtypes.ResourceNotFoundException
	if errors.As(err, &notFound) {
		return fmt.Errorf("%w: model not found: %s", ErrLLMFailure, c.modelID)
	}

	if ctx := context.DeadlineExceeded; errors.Is(err, ctx) {
		return fmt.Errorf("%w: request timed out after %s", ErrLLMFailure, c.timeout)
	}

	return fmt.Errorf("%w: %v", ErrLLMFailure, err)
}
