// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package llm

import (
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

func TestRenderSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		data     TemplateData
		contains []string
	}{
		{
			name: "includes edit format markers",
			data: TemplateData{OS: "darwin", GoVersion: "1.23"},
			contains: []string{
				"<<<<<<< SEARCH",
				"=======",
				">>>>>>> REPLACE",
			},
		},
		{
			name: "includes platform info",
			data: TemplateData{OS: "darwin", GoVersion: "1.23"},
			contains: []string{
				"darwin",
				"1.23",
			},
		},
		{
			name: "includes linux platform",
			data: TemplateData{OS: "linux", GoVersion: "1.24"},
			contains: []string{
				"linux",
				"1.24",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderSystemPrompt(tt.data)
			require.NoError(t, err)
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestConstructMessages(t *testing.T) {
	t.Run("full message array with repo map and files", func(t *testing.T) {
		systemPrompt := "You are a coding assistant."
		repoMap := "main.go: func main()\nlib.go: func Helper()"
		files := []types.FileContent{
			{Path: "main.go", Content: "package main\n\nfunc main() {}\n"},
			{Path: "lib.go", Content: "package main\n\nfunc Helper() string { return \"\" }\n"},
		}
		userPrompt := "Add error handling to Helper"

		system, messages := ConstructMessages(systemPrompt, repoMap, files, userPrompt)

		// System content.
		require.Len(t, system, 1)
		textBlock, ok := system[0].(*brtypes.SystemContentBlockMemberText)
		require.True(t, ok)
		assert.Equal(t, "You are a coding assistant.", textBlock.Value)

		// Messages: repo map, file contents, user prompt.
		require.Len(t, messages, 3)

		// All messages are user role.
		for _, m := range messages {
			assert.Equal(t, brtypes.ConversationRoleUser, m.Role)
		}

		// Repo map message.
		repoMapText := extractText(t, messages[0])
		assert.Contains(t, repoMapText, "main.go: func main()")

		// File contents message.
		fileText := extractText(t, messages[1])
		assert.Contains(t, fileText, "main.go")
		assert.Contains(t, fileText, "lib.go")
		assert.Contains(t, fileText, "func Helper()")

		// User prompt message.
		promptText := extractText(t, messages[2])
		assert.Equal(t, "Add error handling to Helper", promptText)
	})

	t.Run("without repo map", func(t *testing.T) {
		system, messages := ConstructMessages("system", "", nil, "do something")

		require.Len(t, system, 1)
		require.Len(t, messages, 1)

		promptText := extractText(t, messages[0])
		assert.Equal(t, "do something", promptText)
	})

	t.Run("without files", func(t *testing.T) {
		system, messages := ConstructMessages("system", "repo map", nil, "task")

		require.Len(t, system, 1)
		require.Len(t, messages, 2)
	})
}

func TestConstructRetryMessages(t *testing.T) {
	_, initialMessages := ConstructMessages("system", "", nil, "fix the bug")

	result := ConstructRetryMessages(initialMessages, "Here is my fix...", "main.go:10: undefined: foo")

	// Original message + assistant response + error feedback.
	require.Len(t, result, 3)

	// First: original user prompt.
	assert.Equal(t, brtypes.ConversationRoleUser, result[0].Role)

	// Second: assistant response.
	assert.Equal(t, brtypes.ConversationRoleAssistant, result[1].Role)
	assistantText := extractText(t, result[1])
	assert.Equal(t, "Here is my fix...", assistantText)

	// Third: error feedback.
	assert.Equal(t, brtypes.ConversationRoleUser, result[2].Role)
	feedbackText := extractText(t, result[2])
	assert.Contains(t, feedbackText, "main.go:10: undefined: foo")
	assert.Contains(t, feedbackText, "Errors")
}

func TestFormatFileContent(t *testing.T) {
	f := types.FileContent{
		Path:    "main.go",
		Content: "package main\n\nfunc main() {}\n",
	}

	result := formatFileContent(f)
	assert.Contains(t, result, "### main.go")
	assert.Contains(t, result, "   1 │ package main")
	assert.Contains(t, result, "   3 │ func main() {}")
}

// extractText returns the text content of the first content block in a message.
func extractText(t *testing.T, m brtypes.Message) string {
	t.Helper()
	require.NotEmpty(t, m.Content)
	textBlock, ok := m.Content[0].(*brtypes.ContentBlockMemberText)
	require.True(t, ok)
	return textBlock.Value
}
