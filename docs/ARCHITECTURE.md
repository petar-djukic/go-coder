# go-coder Architecture

## System Overview

go-coder is a Go library that executes a single coding task: it takes a natural language prompt, assembles repository context, sends it to an LLM, parses the response into file edits, applies those edits, verifies the result, and retries on failure. The caller (typically mage-claude-orchestrator) imports the library and calls `Coder.Run(ctx, prompt)`.

### Lifecycle

A single invocation follows this lifecycle.

1. **Index**: Scan the repository and build a symbol table from Go AST and tree-sitter.
2. **Assemble**: Select relevant files and symbols using PageRank, construct the LLM prompt with system instructions, repository map, file contents, and the user prompt.
3. **Generate**: Send the prompt to AWS Bedrock and stream the response.
4. **Parse**: Extract edit instructions from the LLM response text (search/replace blocks).
5. **Apply**: For Go files, apply edits through AST mutation. For other files, apply text-based search/replace.
6. **Verify**: Run `go build`, `go vet`, and optionally `go test`. Collect errors.
7. **Retry**: If errors exist and retries remain, format the errors into a follow-up prompt and return to step 3.
8. **Return**: Package the result (modified files, remaining errors, token usage) and return to the caller.

### Coordination Pattern

go-coder uses a synchronous pipeline within a single goroutine for the main loop. Concurrency appears in two places: the file scanner uses a worker pool of goroutines to parse Go files in parallel during indexing, and the LLM response streams through a channel that the parser consumes as tokens arrive.

The library does not manage state across invocations. Each call to `Run` starts fresh: index, assemble, generate, apply, verify. The caller (the orchestrator) manages persistence, git branching, and task lifecycle.

## Coder Interface

The public API lives in `pkg/coder/`. It consists of one interface and its supporting types.

### Data Structures

Table 1 Public Types

| Type | Package | Role |
|------|---------|------|
| `Config` | `pkg/coder` | Configuration for a Coder instance: working directory, model name, Bedrock credentials, retry limits, test command |
| `Coder` | `pkg/coder` | Interface with a single `Run` method |
| `Result` | `pkg/coder` | Outcome of a Run: modified files, errors, token usage, retry count |
| `Edit` | `pkg/types` | A single file edit: file path, old content, new content |
| `Symbol` | `pkg/types` | A code symbol: name, kind (function, struct, interface, variable), file, line, signature |

PRD prd001-coder-interface defines the full field specifications, constructor, and error conditions.

### Operations

Table 2 Coder Operations

| Operation | Signature | Purpose |
|-----------|-----------|---------|
| `New` | `New(cfg Config) (Coder, error)` | Construct a Coder from configuration. Validates config, initializes the LLM client. |
| `Run` | `Run(ctx context.Context, prompt string) (*Result, error)` | Execute a coding task. Indexes the repo, assembles context, generates edits, applies them, verifies, retries on failure. |

PRD prd001-coder-interface specifies preconditions, postconditions, and error types.

## System Components

### File Scanner

Walks the repository using `filepath.WalkDir`, respects `.gitignore` patterns, and dispatches Go files to the AST parser and non-Go files to the tree-sitter parser. Uses a bounded worker pool for parallel parsing.

PRD prd002-ast-engine covers the scanner, parser, and symbol table.

### AST Engine

Parses Go source files into `ast.File` objects using `go/parser`. Extracts symbols (functions, structs, interfaces, methods, variables) into a symbol table. Provides lookup by name, by file, and by kind. Uses `go/types` for type resolution when needed.

The engine also provides mutation operations: replace a function body, add/remove struct fields, manage imports. Mutations operate on the AST and write back to disk through `go/format`, preserving comments via `ast.CommentMap`.

PRD prd002-ast-engine specifies the symbol table structure, mutation operations, and comment preservation contract.

### Text Editor

Applies search/replace edits to non-Go files. Implements multi-stage matching: exact match, whitespace-normalized match, and fuzzy match using a similarity threshold. When a match fails, produces a diagnostic explaining what was expected and what was found.

PRD prd003-edit-engine specifies the matching stages, threshold, and diagnostic format.

### Edit Format Parser

Parses the LLM's text response into a list of `Edit` structs. The parser recognizes search/replace blocks delimited by `<<<<<<< SEARCH` / `=======` / `>>>>>>> REPLACE` markers, with the target file path on the preceding line. Also recognizes whole-file blocks for file creation.

The parser routes each edit to the appropriate engine: Go files go to the AST engine, everything else goes to the text editor.

PRD prd004-edit-format specifies the block syntax, parsing rules, and routing logic.

### LLM Client

Wraps the AWS SDK for Go v2 Bedrock Runtime client. Sends `InvokeModelWithResponseStream` requests and yields tokens through a channel. Handles model selection, token counting, and timeout. Constructs the message array from system prompt, repository map, file contents, and user prompt.

PRD prd005-llm-client specifies the Bedrock integration, message format, prompt templates, and streaming contract.

### Repository Map

Builds a ranked overview of the repository for the LLM's context window. Uses tree-sitter (via `smacker/go-tree-sitter`) to extract symbol definitions and references across all supported languages. Constructs a directed graph where files are nodes and cross-file references are edges. Applies PageRank with personalization (files mentioned in the prompt or recently edited get higher weight). Renders the top-ranked symbols as a condensed map within a configurable token budget.

PRD prd006-repo-map specifies the graph construction, PageRank parameters, rendering format, and caching strategy.

### Feedback Loop

After edits are applied, runs `go build`, `go vet`, and the configured test command via `os/exec`. Captures stdout and stderr. When errors exist: formats them into a follow-up prompt showing the error output, the affected file contents, and the line numbers. Sends this to the LLM for a correction attempt. Repeats up to `Config.MaxRetries` times (default 3).

PRD prd007-feedback-loop specifies the command execution, error formatting, retry limits, and termination conditions.

### Git Integration

Provides auto-commit after successful edits, dirty file detection (commit pre-existing changes before applying AI edits), and undo (revert the last auto-commit). Uses `go-git` or shells out to the git CLI.

PRD prd008-git-integration specifies commit message generation, attribution, dirty handling, and undo semantics.

## Design Decisions

### Decision 1 Library, Not Application

go-coder is a library that other Go programs import. The test CLI in `cmd/go-coder/` exercises the library for development. The orchestrator imports `pkg/coder` and calls `Run` directly, eliminating container overhead.

Benefits: no container startup cost, no Node.js dependency, type-safe integration with the orchestrator, shared process memory for caching.

### Decision 2 AST Mutation for Go, Text Edits for the Rest

Go files are edited through `go/ast` and `go/format`. This produces correctly formatted, comment-preserving edits that cannot break on whitespace. Non-Go files use text-based search/replace because no universal AST exists for YAML, markdown, and configuration files.

Benefits: type-safe Go edits, native gofmt formatting, graceful fallback for non-Go files.

### Decision 3 Prompt-Based Edit Format

The LLM produces structured text blocks (search/replace) in its response. We parse these blocks rather than using tool calling / function calling. Aider proved this approach across millions of edits. It gives the LLM freedom to reason in natural language between edit blocks and avoids the overhead of structured tool-call schemas.

Benefits: proven by aider at scale, model-agnostic (works with any text-generating LLM), allows interleaved reasoning and edits.

### Decision 4 AWS Bedrock as LLM Provider

We target AWS Bedrock exclusively. The AWS SDK for Go v2 provides native streaming, IAM-based authentication, and model selection. We do not build a provider abstraction layer.

Benefits: single integration path, IAM auth (no API keys to manage), consistent with our AWS infrastructure.

### Decision 5 PageRank for Context Selection

We adopt aider's approach: build a dependency graph from symbol definitions and references, apply PageRank with personalization biased toward task-relevant files, and select the top-ranked symbols up to a token budget. This outperforms naive file listing and keyword search for repository context.

Benefits: relevant context within token limits, scales to large repositories, proven by aider's benchmarks.

## Technology Choices

Table 3 Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Language | Go 1.24+ | Implementation language |
| Build automation | magefile/mage | Build and test automation |
| CLI framework | spf13/cobra | Test CLI command structure |
| Configuration | spf13/viper | Test CLI configuration |
| Go AST | go/ast, go/parser, go/format, go/types (stdlib) | Parse, mutate, and format Go source |
| Tree-sitter | smacker/go-tree-sitter | Multi-language symbol extraction for repo map |
| LLM access | aws-sdk-go-v2/service/bedrockruntime | AWS Bedrock streaming inference |
| Git | go-git/go-git/v5 or os/exec git | Git operations |
| Testing | stretchr/testify | Test assertions |
| Fuzzy matching | sergi/go-diff | Fuzzy text matching for search/replace |
| UUID | google/uuid | Unique identifiers |

PRD prd009-technology-stack specifies versions, constraints, and rationale for each choice.

## Project Structure

```
cmd/
  go-coder/           Entry point for test CLI. Cobra/Viper configuration.
pkg/
  coder/              Public Coder interface, Config, Result types.
  types/              Shared types: Edit, Symbol, RepoMap, Message.
internal/
  ast/                Go AST parser, symbol table, mutation engine.
  editor/             Text-based search/replace engine.
  editformat/         LLM response parser (search/replace block extraction).
  llm/                AWS Bedrock client, prompt construction, streaming.
  repomap/            Repository map: tree-sitter extraction, PageRank ranking.
  feedback/           Compiler/test runner, error formatting, retry loop.
  git/                Git operations: auto-commit, undo, dirty handling.
  coder/              Coder implementation: orchestrates all components.
magefiles/            Build automation (mage targets).
docs/                 VISION, ARCHITECTURE, PRDs, use cases, test suites.
tests/
  integration/        Integration tests.
```

Each `internal/` package implements one component from this architecture. The `pkg/` packages define the contracts between the library and its callers. The `cmd/` package wires dependencies and starts the test CLI.

## Implementation Status

We are in the documentation phase. No code has been written. The current focus is producing complete PRDs, use cases, and test suites so that mage-claude-orchestrator can generate the implementation.

Table 4 Implementation Phases

| Phase | Focus | Status |
|-------|-------|--------|
| 01.0 | AST foundation (parser, symbol table, scanner) | Not started |
| 01.1 | AST mutation (function replacement, struct modification, imports) | Not started |
| 02.0 | Edit format parser and text edit engine | Not started |
| 03.0 | AWS Bedrock LLM client | Not started |
| 04.0 | Feedback loop (compiler, test runner, retry) | Not started |
| 05.0 | Repository map (tree-sitter, PageRank) | Not started |
| 06.0 | Git integration | Not started |
| 07.0 | Test CLI and orchestrator integration | Not started |

## Related Documents

Table 5 Related Documents

| Document | Role |
|----------|------|
| [VISION.md](VISION.md) | Goals, boundaries, success criteria, and what go-coder is not |
| PRDs (docs/specs/product-requirements/) | Numbered requirements for each component |
| Use cases (docs/specs/use-cases/) | Tracer-bullet paths through the system |
| Test suites (docs/specs/test-suites/) | Test cases with inputs and expected outputs |
| road-map.yaml | Release schedule and use case status |
