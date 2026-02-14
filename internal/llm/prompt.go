// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package llm wraps the AWS Bedrock ConverseStream API for LLM access.
// Implements: prd005-llm-client R1, R2, R3, R4, R5, R6;
//
//	docs/ARCHITECTURE § LLM Client.
package llm

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/petar-djukic/go-coder/pkg/types"

	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// TemplateData holds the values injected into the system prompt template.
type TemplateData struct {
	OS        string
	GoVersion string
}

// RenderSystemPrompt renders the system prompt template with the given data.
//
// Implements: prd005-llm-client R3.1-R3.6.
func RenderSystemPrompt(data TemplateData) (string, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/system.tmpl")
	if err != nil {
		return "", fmt.Errorf("parsing system template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing system template: %w", err)
	}

	return buf.String(), nil
}

// ConstructMessages builds the Bedrock API message array from system prompt,
// repo map, file contents, and user prompt.
//
// The message order is:
//  1. System message (separate field, not in messages array)
//  2. User message with repository map
//  3. User message with file contents (paths and numbered lines)
//  4. User message with the coding task
//
// Implements: prd005-llm-client R2.1-R2.5.
func ConstructMessages(systemPrompt, repoMap string, files []types.FileContent, userPrompt string) ([]brtypes.SystemContentBlock, []brtypes.Message) {
	system := []brtypes.SystemContentBlock{
		&brtypes.SystemContentBlockMemberText{Value: systemPrompt},
	}

	var messages []brtypes.Message

	// Repo map message.
	if repoMap != "" {
		messages = append(messages, userMessage(
			"## Repository Map\n\n"+repoMap,
		))
	}

	// File contents message.
	if len(files) > 0 {
		var buf strings.Builder
		buf.WriteString("## File Contents\n\n")
		for _, f := range files {
			buf.WriteString(formatFileContent(f))
			buf.WriteString("\n")
		}
		messages = append(messages, userMessage(buf.String()))
	}

	// User prompt (coding task) as the final message.
	messages = append(messages, userMessage(userPrompt))

	return system, messages
}

// ConstructRetryMessages appends a feedback message containing compiler/test
// errors after the assistant's previous response. The conversation continues
// with the error output as a follow-up user message.
//
// Implements: prd005-llm-client R2.6.
func ConstructRetryMessages(prevMessages []brtypes.Message, assistantResponse, errorOutput string) []brtypes.Message {
	// Append the assistant's previous response.
	messages := append(prevMessages, assistantMessage(assistantResponse))

	// Append the error feedback as a new user message.
	feedback := "## Errors\n\nThe previous edits produced the following errors. Please fix them:\n\n" + errorOutput
	messages = append(messages, userMessage(feedback))

	return messages
}

// formatFileContent formats a file's content with path header and line numbers.
func formatFileContent(f types.FileContent) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("### %s\n\n", f.Path))

	lines := strings.Split(f.Content, "\n")
	for i, line := range lines {
		buf.WriteString(fmt.Sprintf("%4d │ %s\n", i+1, line))
	}

	return buf.String()
}

// userMessage creates a user message with text content.
func userMessage(text string) brtypes.Message {
	return brtypes.Message{
		Role: brtypes.ConversationRoleUser,
		Content: []brtypes.ContentBlock{
			&brtypes.ContentBlockMemberText{Value: text},
		},
	}
}

// assistantMessage creates an assistant message with text content.
func assistantMessage(text string) brtypes.Message {
	return brtypes.Message{
		Role: brtypes.ConversationRoleAssistant,
		Content: []brtypes.ContentBlock{
			&brtypes.ContentBlockMemberText{Value: text},
		},
	}
}
