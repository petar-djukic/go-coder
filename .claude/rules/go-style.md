<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Go Code Style

Rules for writing maintainable Go code in this project.

## Copyright Header

Every Go file must start with the SPDX copyright header before the package declaration:

```go
// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package mypackage
```

This applies to hand-written files (magefiles/) and generated files (cmd/, pkg/, internal/, tests/). The header must appear on lines 1-2, followed by a blank line, then the package clause.

For markdown files in `.claude/`, use an HTML comment on the first line:

```markdown
<!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->
```

## Never Duplicate Code

Before writing a function, search for existing code that does the same thing. Before adding a field, check whether a struct already carries it.

When two pieces of code share logic, extract the common part. Use struct embedding to share fields. Use helper functions to share behavior. Use interfaces to share contracts. The threshold is two: if you write the same thing twice, extract it.

## Design Patterns

Apply GoF patterns to prevent code from calcifying into unmaintainable tangles. Each pattern solves a specific structural problem. Recognize the problem first, then apply the pattern.

### Strategy (Algorithm Selection)

Use when multiple implementations of the same operation exist and the caller picks one. Define a small interface. Each implementation is a struct (or a function type) satisfying that interface. The caller selects at configuration time, not with `if` chains.

Symptoms that demand Strategy: a `switch` or `if/else` ladder choosing between behaviors, a function parameter named `mode` or `kind`, boolean flags that toggle logic branches.

### Command (Encapsulated Actions)

Use when an action needs to be stored, queued, undone, or composed. Each command is a struct with an `Execute()` method. Commands can carry undo logic, be chained into sequences, or be logged for replay.

Symptoms that demand Command: inline `exec.Command` calls scattered across a function, repeated sequences of shell operations that differ only in arguments, actions that should be retryable or reversible.

### Facade (Simplified Interface to Complex Subsystems)

Use when a caller should not know about the internal steps of a multi-step process. The facade exposes one method; internally it orchestrates several components. Do not add toggle parameters to a facade. If the caller needs to control sub-steps, they should use the components directly.

Symptoms that demand Facade: a function that grew boolean parameters to skip internal steps, a caller that must call three functions in a specific order, an `Init/Do/Cleanup` pattern that keeps getting duplicated.

### Factory (Centralized Construction)

Use `NewXxx()` functions to construct objects. Return concrete types but accept interfaces as dependencies. Factories centralize validation and wiring so callers never construct partially initialized structs.

### Decorator (Layered Behavior)

Use to add cross-cutting concerns (logging, timing, retries) without modifying the wrapped object. In Go, the decorator accepts and returns the same interface. HTTP middleware is the canonical example.

Symptoms that demand Decorator: logging or timing code copy-pasted around every call to an interface, optional behavior toggled by a boolean that wraps the real logic.

### Builder (Complex Object Construction)

Use when an object has many optional fields and constructing it in one call is unwieldy. Prefer the Functional Options variant (`WithXxx` functions) for idiomatic Go over a mutable builder struct.

### Adapter (Interface Translation)

Use when existing code does not satisfy the interface a consumer expects. Write a thin wrapper that translates the foreign API into the local interface. Do not modify the foreign code.

### Observer (Event Notification)

Use channels for asynchronous event notification between goroutines. One producer writes to a channel; multiple consumers listen. Prefer this over callback registration when the producer should not know about its consumers.

## When to Introduce an Interface

Introduce an interface when you have two concrete implementations or when you need to mock a dependency in tests. Do not create an interface for a single implementation "just in case." Accept interfaces as parameters; return concrete structs.

Keep interfaces small: one to three methods. A large interface is a sign that the abstraction is wrong. Split it into focused interfaces and compose them.

## Struct and Function Design

Each struct represents one concept. Each function does one thing. If a function takes more than three parameters, group related parameters into a config struct. If a function exceeds 40 lines, find a seam and split it.

Name structs and functions by what they represent or do, not by how they are called. Avoid generic names (`Manager`, `Handler`, `Helper`, `Processor`) unless the struct genuinely manages, handles, or processes a well-defined resource.

## Error Handling

Handle errors at the point they occur. Use guard clauses: check `err != nil` and return early so the main logic stays at minimal indentation.

Wrap errors with context: `fmt.Errorf("doing X: %w", err)`. Each wrap adds the "why" at that layer, producing a readable chain when the error surfaces.

Never silently discard an error with `_` unless the operation is best-effort cleanup (e.g., removing a temp file after the real work succeeded). When discarding, leave a comment explaining why.

## No Magic Strings

Centralize all string literals that name external binaries, file paths, URLs, or repeated text.

| Category | Mechanism |
|----------|-----------|
| Binary names | `const` (e.g., `binGit`, `binGo`) |
| Paths, prefixes, module names | `const` |
| Shared CLI arg slices | `var` |
| Large prompts with variable interpolation | Embedded `.tmpl` templates |
| Short messages with interpolation | `fmt.Sprintf` at the call site |
| Static messages or labels | `const` |

When adding a new external command or path, define the constant first, then use it. Never scatter raw string literals across files.

## Project Structure and Libraries

Treat each package directory as a library. Other parts of the codebase import a library through its public API and know nothing about its internals. A library hides its data structures, helper functions, and implementation choices behind the package boundary. If a caller needs to reach into a library's private types, the library's API is incomplete.

Align package structure to PRD component structure. Each major component described in a PRD or in ARCHITECTURE.md maps to one package (or a small cluster of packages). Read the PRDs to understand component boundaries before creating new packages. When a PRD defines an interface between two components, that interface lives in `pkg/` and each component implements it in its own `internal/` package.

Define interfaces between major components. The `pkg/` directory holds shared types and interface contracts. The `internal/` directory holds implementations that satisfy those contracts. A package in `internal/storage/` should never import from `internal/cli/`; they communicate through interfaces defined in `pkg/`.

Follow the standard Go layout.

| Directory | Purpose |
|-----------|---------|
| `cmd/` | Entry points. Minimal: parse flags, wire dependencies, start. |
| `internal/` | Private implementation. Not importable outside this module. One package per component. |
| `pkg/` | Shared public types and interfaces. No implementation. The contract layer between libraries. |
| `tests/` | Integration tests. |
| `magefiles/` | Build tooling. Flat directory (mage constraint). One file per concern. |

Avoid package names like `util`, `common`, `helpers`. Name packages by domain: `storage`, `auth`, `config`.

## Standard Ecosystem Packages

The architecture docs do not list every dependency. For well-known tasks, use well-known packages. Do not reinvent what the ecosystem already provides.

| Task | Package |
| ---- | ------- |
| Build automation | `magefile/mage` |
| CLI framework | `spf13/cobra` |
| Configuration | `spf13/viper` |
| Observability (traces, metrics, logs) | `go.opentelemetry.io/otel` |
| Testing assertions | `stretchr/testify` |
| SQL database access | `database/sql` (stdlib) + `mattn/go-sqlite3` |
| HTTP routing | `net/http` (stdlib) or `chi` |
| YAML parsing | `go-yaml/yaml` |
| JSON handling | `encoding/json` (stdlib) |
| UUID generation | `google/uuid` |

Before adding a new dependency, check whether the standard library or an existing dependency already covers the need. Prefer the standard library when it does the job. When a third-party package is warranted, choose the one with broad adoption and active maintenance.

## Shared Config via Struct Embedding

When two or more consumers share configuration fields, extract a common struct and embed it. Provide a `registerXxxFlags` helper when the shared fields map to CLI flags. Do not duplicate fields across sibling structs.

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Exported types and functions | PascalCase | `CupboardConfig` |
| Unexported types and functions | camelCase | `cobblerConfig` |
| CLI flags | kebab-case | `--silence-agent` |
| Constants for binaries | `bin` prefix + PascalCase tool name | `binGit`, `binClaude` |
| Factory functions | `New` prefix | `NewBackend()` |
| Interface names | Action or capability, not `-er` unless it fits naturally | `Table`, `Reader` |

## Concurrency

Pass `context.Context` as the first parameter to any function that does I/O or may block. Never start a goroutine without a plan for how it exits. Use `sync.WaitGroup` or a done channel to manage lifetimes.

## Testing

Tests exist to cover the source and to uncover edge cases. Every exported function and every meaningful branch deserves a test. When writing tests, ask what inputs break assumptions: zero values, nil pointers, empty slices, duplicate keys, boundary lengths, concurrent access. If a bug could hide there, write a case for it.

Organize similar tests into table-driven parameterized tests. Each row is one scenario with named inputs and expected outputs. When ten cases share setup, assertion logic, and teardown, the only thing that varies is the row data. Extract shared setup into test helpers so no two test functions duplicate the same wiring. If a test requires an external dependency, accept an interface and provide a mock.

Reusable test code matters as much as reusable production code. Build test helpers, custom assertion functions, and fixture factories that multiple test files import. A test helper that constructs a fully wired Cupboard with an in-memory backend saves every test suite from repeating that setup. Place shared test utilities in a `testutil` package or in `_test.go` helpers within the package.

## Code Review Checklist (Self)

Before considering code complete, verify:

1. No duplicated logic exists that could be extracted into a shared function or struct.
2. No magic strings remain: all binaries, paths, and repeated text are centralized.
3. Every error is handled or explicitly discarded with a comment.
4. Every struct has a single, nameable responsibility.
5. No function exceeds 40 lines without a strong reason.
6. Interfaces are small (one to three methods) and have at least two implementations or a testing need.
7. No boolean parameters toggle behavior that should be a Strategy or Decorator.
8. Config structs embed shared fields rather than duplicating them.
9. `context.Context` is threaded through I/O paths.
10. Tests cover the contract, not the implementation.
