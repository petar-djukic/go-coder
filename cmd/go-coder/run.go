// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd009-technology-stack R4.3-R4.9.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	gitpkg "github.com/petar-djukic/go-coder/internal/git"
	"github.com/petar-djukic/go-coder/pkg/coder"
)

// newRunCmd creates the "run" command.
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a coding task",
		Long:  "Run takes a natural language prompt, sends it to the LLM, and applies the resulting edits to the repository.",
		RunE:  runCoder,
	}

	cmd.Flags().StringP("prompt", "p", "", "Coding task description (required)")
	cmd.MarkFlagRequired("prompt")

	return cmd
}

// runCoder executes the coding task.
func runCoder(cmd *cobra.Command, args []string) error {
	prompt, _ := cmd.Flags().GetString("prompt")

	cfg := coder.Config{
		WorkDir:        viper.GetString("workdir"),
		Model:          viper.GetString("model"),
		Region:         viper.GetString("region"),
		MaxRetries:     viper.GetInt("max-retries"),
		TestCmd:        viper.GetString("test-cmd"),
		MapTokenBudget: viper.GetInt("map-token-budget"),
		MaxTokens:      viper.GetInt("max-tokens"),
		NoGit:          viper.GetBool("no-git"),
	}

	c, err := coder.New(cfg)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	result, err := c.Run(ctx, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if result != nil {
			printResult(result)
		}
		return err
	}

	printResult(result)
	return nil
}

// printResult outputs the result as JSON to stdout.
func printResult(result *coder.Result) {
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling result: %v\n", err)
		return
	}
	fmt.Println(string(out))
}

// newUndoCmd creates the "undo" command.
func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Revert the last go-coder commit",
		Long:  "Undo performs a soft reset of the last commit if it was made by go-coder.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := viper.GetString("workdir")

			repo, err := gitpkg.Open(gitpkg.Config{WorkDir: workDir})
			if err != nil {
				return fmt.Errorf("opening repository: %w", err)
			}

			if err := repo.Undo(); err != nil {
				return fmt.Errorf("undo failed: %w", err)
			}

			fmt.Println("Successfully reverted last go-coder commit.")
			return nil
		},
	}
}
