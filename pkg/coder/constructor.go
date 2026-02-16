// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-coder-interface R4;
//
//	docs/ARCHITECTURE § Coder Interface.
package coder

import (
	"context"
	"fmt"
	"os"
	"time"

	internalcoder "github.com/petar-djukic/go-coder/internal/coder"
	"github.com/petar-djukic/go-coder/internal/llm"
)

const (
	defaultMaxRetries     = 3
	defaultMapTokenBudget = 2048
	defaultMaxTokens      = 4096
	defaultLLMTimeout     = 5 * time.Minute
)

// New validates the config, initializes the LLM client, and returns a
// ready-to-use Coder. It does not index the repository; that happens in Run.
//
// Implements: prd001-coder-interface R4.1-R4.3.
func New(cfg Config) (Coder, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	applyDefaults(&cfg)

	client, err := llm.NewClient(context.Background(), llm.ClientConfig{
		ModelID:   cfg.Model,
		Region:    cfg.Region,
		Timeout:   defaultLLMTimeout,
		MaxTokens: cfg.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLLMFailure, err)
	}

	runner := internalcoder.NewRunner(internalcoder.Deps{
		LLMClient:      client,
		WorkDir:        cfg.WorkDir,
		Model:          cfg.Model,
		MaxRetries:     cfg.MaxRetries,
		TestCmd:        cfg.TestCmd,
		MapTokenBudget: cfg.MapTokenBudget,
		NoGit:          cfg.NoGit,
	})

	return &coderAdapter{runner: runner}, nil
}

// coderAdapter adapts internal/coder.Runner to the public Coder interface.
type coderAdapter struct {
	runner *internalcoder.Runner
}

func (a *coderAdapter) Run(ctx context.Context, prompt string) (*Result, error) {
	ir, err := a.runner.Run(ctx, prompt)
	if ir == nil {
		return &Result{}, err
	}
	return &Result{
		ModifiedFiles: ir.ModifiedFiles,
		Errors:        ir.Errors,
		TokensUsed:    ir.TokensUsed,
		Retries:       ir.Retries,
		Success:       ir.Success,
	}, err
}

// validateConfig checks that required fields are present.
//
// Implements: prd001-coder-interface R1.8-R1.10.
func validateConfig(cfg Config) error {
	if cfg.WorkDir == "" {
		return fmt.Errorf("WorkDir is required")
	}
	if info, err := os.Stat(cfg.WorkDir); err != nil || !info.IsDir() {
		return fmt.Errorf("WorkDir %q does not exist or is not a directory", cfg.WorkDir)
	}
	if cfg.Model == "" {
		return fmt.Errorf("Model is required")
	}
	if cfg.Region == "" {
		return fmt.Errorf("Region is required")
	}
	return nil
}

// applyDefaults fills in zero-value fields with their defaults.
func applyDefaults(cfg *Config) {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.MapTokenBudget == 0 {
		cfg.MapTokenBudget = defaultMapTokenBudget
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
}
