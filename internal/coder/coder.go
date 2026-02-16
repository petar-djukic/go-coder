// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package coder implements the Coder orchestrator, wiring all internal
// components to execute a coding task.
// Implements: prd001-coder-interface R2;
//
//	docs/ARCHITECTURE § Coder Interface, Lifecycle.
package coder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/petar-djukic/go-coder/internal/editformat"
	"github.com/petar-djukic/go-coder/internal/editor"
	"github.com/petar-djukic/go-coder/internal/feedback"
	gitpkg "github.com/petar-djukic/go-coder/internal/git"
	"github.com/petar-djukic/go-coder/internal/llm"
	"github.com/petar-djukic/go-coder/internal/repomap"
	"github.com/petar-djukic/go-coder/pkg/types"

	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

const defaultFuzzyThreshold = 0.8

// Prompter abstracts LLM interactions so the orchestrator is testable.
type Prompter interface {
	Generate(ctx context.Context, system []brtypes.SystemContentBlock, messages []brtypes.Message) (string, error)
	Usage() types.TokenUsage
}

// RunResult holds the outcome of a Runner.Run invocation. This is the
// internal result type; pkg/coder converts it to the public Result.
type RunResult struct {
	ModifiedFiles []string
	Errors        []string
	TokensUsed    types.TokenUsage
	Retries       int
	Success       bool
}

// Deps holds injected dependencies for the runner.
type Deps struct {
	LLMClient      *llm.Client // Real client; nil when Prompter is set.
	Prompter       Prompter    // Mock for testing; overrides LLMClient.
	WorkDir        string
	Model          string
	MaxRetries     int
	TestCmd        string
	MapTokenBudget int
	NoGit          bool
}

// Runner orchestrates the coding lifecycle.
type Runner struct {
	deps Deps
}

// NewRunner creates a Runner with the given dependencies.
func NewRunner(deps Deps) *Runner {
	return &Runner{deps: deps}
}

// Run executes the full coding lifecycle: index, map, prompt, parse, apply,
// verify, retry, commit.
//
// Implements: prd001-coder-interface R2.1-R2.4.
func (r *Runner) Run(ctx context.Context, prompt string) (*RunResult, error) {
	result := &RunResult{}

	// Step 1: Handle git (dirty files).
	var gitRepo *gitpkg.Repo
	if !r.deps.NoGit {
		repo, err := gitpkg.Open(gitpkg.Config{
			WorkDir:     r.deps.WorkDir,
			AutoCommit:  true,
			DirtyCommit: true,
		})
		if err == nil {
			gitRepo = repo
			if err := repo.HandleDirty(); err != nil {
				return result, fmt.Errorf("handling dirty files: %w", err)
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}

	// Step 2: Build repository map.
	mapResult, err := repomap.BuildMap(ctx, r.deps.WorkDir, nil, float64(r.deps.MapTokenBudget))
	if err != nil {
		return result, fmt.Errorf("building repo map: %w", err)
	}

	// Step 3: Render system prompt.
	systemPrompt, err := llm.RenderSystemPrompt(llm.TemplateData{
		OS:        runtime.GOOS,
		GoVersion: runtime.Version(),
	})
	if err != nil {
		return result, fmt.Errorf("rendering system prompt: %w", err)
	}

	// Step 4: Read relevant files for context.
	files := readRelevantFiles(r.deps.WorkDir)

	// Step 5: Construct initial messages and send to LLM.
	system, messages := llm.ConstructMessages(systemPrompt, mapResult.Map, files, prompt)

	responseText, err := r.generate(ctx, system, messages)
	if err != nil {
		return result, fmt.Errorf("LLM call failed: %w", err)
	}

	// Step 6: Parse edits from response.
	parseResult, err := editformat.Parse(responseText)
	if err != nil {
		return result, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Step 7: Apply edits.
	textEditor := &editor.TextEditor{FuzzyThreshold: defaultFuzzyThreshold}
	router := &editformat.Router{
		ASTApplier:  textEditor, // TODO: replace with AST engine when available
		TextApplier: textEditor,
	}
	routeResult := router.ApplyAll(parseResult.Edits)

	for _, applied := range routeResult.Applied {
		result.ModifiedFiles = append(result.ModifiedFiles, applied.FilePath)
	}
	for _, e := range routeResult.Errors {
		result.Errors = append(result.Errors, e.Error())
	}

	// Step 8: Verify and retry via feedback loop.
	prevMessages := messages
	prevResponse := responseText

	loopResult, loopErr := feedback.Run(ctx, feedback.LoopConfig{
		VerifyConfig: feedback.VerifyConfig{
			WorkDir: r.deps.WorkDir,
			TestCmd: r.deps.TestCmd,
		},
		FormatConfig: feedback.FormatConfig{
			ContextLines:  5,
			MaxTestOutput: 4096,
		},
		MaxRetries: r.deps.MaxRetries,
	}, result.ModifiedFiles, func(ctx context.Context, errorPrompt string) ([]string, error) {
		retryMessages := llm.ConstructRetryMessages(prevMessages, prevResponse, errorPrompt)

		retryText, err := r.generate(ctx, system, retryMessages)
		if err != nil {
			return nil, fmt.Errorf("retry LLM call: %w", err)
		}

		retryParse, err := editformat.Parse(retryText)
		if err != nil {
			return nil, fmt.Errorf("parsing retry response: %w", err)
		}

		retryRoute := router.ApplyAll(retryParse.Edits)
		var modified []string
		for _, a := range retryRoute.Applied {
			modified = append(modified, a.FilePath)
		}

		prevMessages = retryMessages
		prevResponse = retryText
		return modified, nil
	})

	if loopResult != nil {
		result.Retries = loopResult.Retries
		result.ModifiedFiles = loopResult.ModifiedFiles
		result.Success = loopResult.Success
	}

	if loopErr != nil && !result.Success {
		if loopResult != nil && loopResult.FinalResult != nil {
			result.Errors = nil
			for _, ce := range loopResult.FinalResult.Errors {
				result.Errors = append(result.Errors, ce.String())
			}
			if !loopResult.FinalResult.TestOK && loopResult.FinalResult.TestOutput != "" {
				result.Errors = append(result.Errors, "test failure: "+loopResult.FinalResult.TestOutput)
			}
		}
	}

	// Step 9: Get token usage.
	result.TokensUsed = r.usage()

	// Step 10: Auto-commit on success.
	if result.Success && gitRepo != nil {
		if err := gitRepo.AutoCommit(result.ModifiedFiles, prompt); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("auto-commit failed: %v", err))
		}
	}

	return result, nil
}

// generate sends a prompt to the LLM and returns the full response text.
func (r *Runner) generate(ctx context.Context, system []brtypes.SystemContentBlock, messages []brtypes.Message) (string, error) {
	if r.deps.Prompter != nil {
		return r.deps.Prompter.Generate(ctx, system, messages)
	}
	if r.deps.LLMClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	tokenCh, responseCh := r.deps.LLMClient.SendPrompt(ctx, system, messages)
	for range tokenCh {
	}

	resp := <-responseCh
	if resp == nil {
		return "", fmt.Errorf("no response from LLM")
	}
	return resp.FullText, nil
}

// usage returns cumulative token usage.
func (r *Runner) usage() types.TokenUsage {
	if r.deps.Prompter != nil {
		return r.deps.Prompter.Usage()
	}
	if r.deps.LLMClient != nil {
		return r.deps.LLMClient.CumulativeUsage()
	}
	return types.TokenUsage{}
}

// readRelevantFiles reads source files from the working directory.
func readRelevantFiles(workDir string) []types.FileContent {
	var files []types.FileContent

	_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".py" && ext != ".js" && ext != ".ts" && ext != ".yaml" && ext != ".yml" && ext != ".md" {
			return nil
		}
		if info.Size() > 32*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(workDir, path)
		files = append(files, types.FileContent{
			Path:    relPath,
			Content: string(content),
		})
		return nil
	})

	return files
}
