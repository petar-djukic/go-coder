// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-coder-interface R5.5 (Message type);
//
//	prd005-llm-client R2 (message construction types).
package types

// MessageRole identifies the sender of a message in the LLM conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// Message represents a single message in the LLM conversation.
type Message struct {
	Role    MessageRole // Who sent the message
	Content string      // Message text
}

// TokenUsage tracks token consumption for a single LLM call.
type TokenUsage struct {
	InputTokens  int // Tokens in the prompt
	OutputTokens int // Tokens in the response
}

// Total returns the sum of input and output tokens.
func (u TokenUsage) Total() int {
	return u.InputTokens + u.OutputTokens
}

// FileContent represents a file's content for inclusion in a prompt.
type FileContent struct {
	Path    string // File path relative to repository root
	Content string // File text content
}

// StreamResponse holds the result of a streaming LLM call.
type StreamResponse struct {
	FullText string     // Accumulated response text
	Usage    TokenUsage // Token counts from API metadata
	Retries  int        // Number of retries performed (due to rate limits)
}
