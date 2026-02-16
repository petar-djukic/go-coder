// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Command go-coder is a test CLI for the go-coder library.
// Implements: prd009-technology-stack R4.1-R4.12;
//
//	docs/ARCHITECTURE § Project Structure.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const version = "0.1.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "go-coder",
		Short: "AST-driven Go coding agent",
		Long:  "go-coder takes a natural language prompt, generates code edits via LLM, and applies them to your repository.",
	}

	// Global flags.
	rootCmd.PersistentFlags().String("workdir", ".", "Repository root directory")
	rootCmd.PersistentFlags().String("model", "", "Bedrock model ID")
	rootCmd.PersistentFlags().String("region", "", "AWS region for Bedrock")
	rootCmd.PersistentFlags().Int("max-retries", 3, "Maximum feedback loop iterations")
	rootCmd.PersistentFlags().String("test-cmd", "", "Test command (e.g., 'go test ./...')")
	rootCmd.PersistentFlags().Int("map-token-budget", 2048, "Token budget for repository map")
	rootCmd.PersistentFlags().Int("max-tokens", 4096, "Maximum tokens for LLM response")
	rootCmd.PersistentFlags().Bool("no-git", false, "Disable git operations")

	// Bind flags to viper.
	viper.BindPFlag("workdir", rootCmd.PersistentFlags().Lookup("workdir"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("region", rootCmd.PersistentFlags().Lookup("region"))
	viper.BindPFlag("max-retries", rootCmd.PersistentFlags().Lookup("max-retries"))
	viper.BindPFlag("test-cmd", rootCmd.PersistentFlags().Lookup("test-cmd"))
	viper.BindPFlag("map-token-budget", rootCmd.PersistentFlags().Lookup("map-token-budget"))
	viper.BindPFlag("max-tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	viper.BindPFlag("no-git", rootCmd.PersistentFlags().Lookup("no-git"))

	// Env vars: GO_CODER_MODEL, GO_CODER_REGION, etc.
	viper.SetEnvPrefix("GO_CODER")
	viper.AutomaticEnv()

	// Config file.
	viper.SetConfigName(".go-coder")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.ReadInConfig() // Ignore error; config file is optional.

	// Add commands.
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newUndoCmd())
	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newVersionCmd creates the "version" command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print go-coder version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("go-coder %s\n", version)
		},
	}
}
