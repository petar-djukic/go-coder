# go-coder

## Executive Summary

go-coder is a Go library that turns an LLM into a coding agent. It takes a natural language prompt describing a code change, assembles relevant context from the repository, sends it to an LLM via AWS Bedrock, parses the response into file edits, applies those edits, and verifies the result through the Go compiler and test runner. When errors occur, it feeds them back to the LLM for correction.

We are not a chat application, an IDE plugin, or a general-purpose AI framework. We are a library that other Go programs import to gain code generation capabilities.

## Introduction

### The Problem

The mage-claude-orchestrator runs Claude Code CLI inside Podman containers to generate code. This works but introduces heavy dependencies: Node.js runtime, npm, the Claude Code CLI, container orchestration, and an Anthropic API key routed through the CLI. Each invocation pays the cost of container startup, CLI initialization, and an opaque tool-calling protocol we do not control.

We want to replace the Claude Code CLI with a native Go library that the orchestrator imports directly. This eliminates the container layer for code generation, gives us control over the edit format and feedback loop, and lets us use AWS Bedrock as the LLM provider.

### What This Does

go-coder treats a Go repository as a structured database rather than a bag of text files. It parses Go source into abstract syntax trees, builds a symbol table, and ranks symbols by relevance using a PageRank-inspired algorithm. When the LLM proposes edits, go-coder applies them through AST mutation for Go files and text-based search/replace for everything else. After each edit cycle, it runs the compiler and test suite, feeding errors back to the LLM for correction.

The library exposes a single entry point: a `Coder` interface with a `Run` method. Callers pass a prompt and receive a result describing what changed. The orchestrator calls `Run` instead of launching a container. A test CLI (Cobra/Viper) exercises the same interface for development and debugging.

### Research Context

This design draws from two existing systems. Aider, a Python coding agent, pioneered prompt-based edit formats where the LLM outputs structured text blocks (search/replace, unified diff, patch format) that the agent parses and applies. Aider uses tree-sitter and PageRank for repository context but applies edits as text operations. The GopherMind concept extends this by using Go's native `go/ast` package for type-safe AST mutation, leveraging Go's standard library to treat source code as a mutable tree rather than a string.

go-coder combines both approaches: AST mutation for Go files (where we have native tooling) and text-based edits for everything else (YAML, markdown, configuration).

## Why This Team

We already operate the mage-claude-orchestrator, which manages the full lifecycle of AI-driven code generation: task proposal, isolated worktree creation, code generation, merge, and metrics collection. go-coder replaces only the code generation step, fitting into the existing orchestration pipeline without changing the workflow. The orchestrator's measure-stitch cycle, issue tracking, and git branching remain unchanged.

We write Go daily. The `go/ast`, `go/parser`, `go/format`, and `go/types` packages in the standard library provide everything we need for structural code editing without external dependencies. AWS Bedrock is our LLM provider, and the AWS SDK for Go v2 gives us native access.

## Planning and Implementation

### Success Criteria

We measure success along three dimensions.

Table 1 Success Criteria

| Dimension | Metric | Target |
|-----------|--------|--------|
| Edit accuracy | Percentage of LLM-proposed edits that apply without errors | Above 90% on Go files |
| Feedback loop effectiveness | Percentage of compiler/test errors resolved within 3 retries | Above 70% |
| Integration overhead | Time to invoke Coder.Run vs Claude Code CLI in container | Under 2 seconds cold start |

### What "Done" Looks Like

The mage-claude-orchestrator imports go-coder and uses `Coder.Run()` to generate code in its stitch phase. No Podman containers, no Node.js, no Claude Code CLI. The orchestrator passes a task prompt, go-coder returns a result with the list of modified files and any remaining errors. The orchestrator commits the changes and moves to the next task.

A developer can also run the test CLI to invoke the same library from the command line, passing a prompt and observing the edits.

### Implementation Phases

Table 2 Implementation Phases

| Phase | Focus | Deliverables |
|-------|-------|-------------|
| 01.0 | AST foundation | Go AST parser, symbol table, file scanner |
| 01.1 | AST mutation | Function body replacement, struct modification, import management |
| 02.0 | Edit format and text edits | LLM response parsing, search/replace engine for non-Go files |
| 03.0 | LLM integration | AWS Bedrock client, prompt construction, response streaming |
| 04.0 | Feedback loop | Compiler/test runner, error-to-prompt formatting, retry logic |
| 05.0 | Repository map | Tree-sitter symbol extraction, dependency graph, PageRank ranking |
| 06.0 | Git integration | Auto-commit, undo, dirty file handling |
| 07.0 | CLI and orchestrator integration | Test CLI (Cobra/Viper), library API finalization |

### Risks and Mitigations

Table 3 Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| AST mutation drops comments | Malformed output files | Medium | Use `parser.ParseComments` and `ast.CommentMap` to preserve comment attachment |
| LLM edit format unreliable | Edits fail to parse or apply | Medium | Multi-stage matching (exact, whitespace-normalized, fuzzy) and clear error feedback |
| Bedrock API latency | Slow generation cycles | Low | Streaming responses, timeout configuration |
| Large repos exceed context window | Incomplete or irrelevant context | Medium | PageRank-based context selection, configurable token budget |

## What This Is NOT

We are not a chat application. There is no interactive REPL, no conversation history across invocations, no user-facing UI.

We are not an agentic framework. We do not manage tasks, propose work, or track issues. The orchestrator handles that. We execute a single prompt and return results.

We are not a general-purpose code editor. We target Go repositories. We support text edits for non-Go files (YAML, markdown) as a secondary capability, but Go AST mutation is the primary edit mechanism.

We are not a replacement for the orchestrator. We replace only the code generation step within the orchestrator's stitch phase. Task management, git branching, worktree isolation, and metrics collection remain in the orchestrator.

We are not provider-agnostic. We target AWS Bedrock. Supporting additional LLM providers is not a goal for the initial releases.
