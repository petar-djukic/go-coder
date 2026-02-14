<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Test Suite Format

A test suite is a YAML document that groups related test cases under shared preconditions. Each suite contains metadata, setup, and a list of individual test cases with explicit inputs and expected outputs. Test suites trace back to use cases or PRD requirements.

## File and Naming

- **Location**: `docs/specs/test-suites/test-[use-case-id].yaml`
- **Prefix**: `test-` followed by the use case ID that the suite validates
- **Use case ID**: The full use case identifier (e.g., `rel01.0-uc001-cupboard-lifecycle`)

Examples:

- `docs/specs/test-suites/test-rel01.0-uc001-cupboard-lifecycle.yaml`
- `docs/specs/test-suites/test-rel02.1-uc001-issue-tracking-cli.yaml`

## Required Fields

### 1. id

The suite identifier, matching the filename without extension.

```yaml
id: test-rel02.1-uc001-issue-tracking-cli
```

### 2. title

A short human-readable name for the suite.

```yaml
title: Issue-tracking CLI operations
```

### 3. traces

What this suite validates. List one or more use case IDs, PRD requirement IDs, or both.

```yaml
traces:
  - rel02.1-uc003-self-hosting
  - prd001-cupboard-core R4
```

### 4. preconditions

Shared setup that applies to all test cases in this suite. Each test case starts from this state.

```yaml
preconditions:
  - Cupboard initialized with SQLite backend
  - No existing crumbs in the table
```

### 5. test_cases

The list of individual test cases. Each test case has a name, inputs, and expected outputs.

```yaml
test_cases:
  - name: Create task with required fields
    inputs:
      command: cupboard create --type task --title "Implement feature" --description "Details"
    expected:
      exit_code: 0
      stdout_contains: "Created"
      state:
        crumb_count: 1
        crumb_type: task
        crumb_title: "Implement feature"
        crumb_state: open

  - name: Create task without title fails
    inputs:
      command: cupboard create --type task
    expected:
      exit_code: 1
      stderr_contains: "title is required"

  - name: Create epic with labels
    inputs:
      command: cupboard create --type epic --title "Storage layer" --labels "code,infra"
    expected:
      exit_code: 0
      state:
        crumb_type: epic
        crumb_labels:
          - code
          - infra
```

Each test case contains:

Table 1: Test case fields

| Field | Required | Description |
|-------|----------|-------------|
| name | yes | Short description of the scenario |
| description | no | Longer explanation when the name is not sufficient |
| inputs | yes | What to feed the system (commands, arguments, data, environment) |
| expected | yes | What the system should produce (exit codes, output, state changes) |
| cleanup | no | Per-case teardown if different from the shared cleanup |

### Inputs

Inputs describe what goes into the system.

Table 2: Common input fields

| Field | Description |
|-------|-------------|
| command | CLI command to execute |
| args | Map of named arguments when command is not a raw string |
| stdin | Data piped to standard input |
| env | Environment variables to set |
| files | Files to create before the test case runs |

Use whichever fields fit the scenario. At minimum, provide a command or an equivalent action.

### Expected Outputs

Expected outputs describe what the system should produce.

Table 3: Common expected output fields

| Field | Description |
|-------|-------------|
| exit_code | Process exit code (0 for success) |
| stdout_contains | Substring that must appear in standard output |
| stdout_equals | Exact standard output (trimmed) |
| stdout_json | JSON structure the output must match |
| stderr_contains | Substring that must appear in standard error |
| state | Post-condition checks on system state (database, files, etc.) |
| files | Files that should exist with expected content |

Include only the checks that matter for the test case.

## Optional Fields

### 6. description

A longer explanation of what this suite covers.

```yaml
description: >
  Validates cupboard create across different input combinations,
  including success paths, missing required fields, and optional flags.
```

### 7. tags

Labels for filtering and grouping suites.

```yaml
tags:
  - cli
  - smoke
```

### 8. cleanup

Shared teardown that runs after all test cases complete.

```yaml
cleanup:
  - Remove the test data directory
```

## Full Example

```yaml
id: test-rel01.0-uc002-sqlite-crud
title: Table interface CRUD operations
description: >
  Validates Set, Get, Fetch, Delete across entity types with JSONL persistence.
traces:
  - rel01.0-uc002-sqlite-crud
tags:
  - cli
  - smoke

preconditions:
  - Cupboard initialized with SQLite backend
  - No existing crumbs in the table

test_cases:
  - name: Create task with required fields
    inputs:
      command: cupboard create --type task --title "Implement feature" --description "Details"
    expected:
      exit_code: 0
      stdout_contains: "Created"
      state:
        crumb_count: 1
        crumb_type: task
        crumb_state: open

  - name: Create without title fails
    inputs:
      command: cupboard create --type task
    expected:
      exit_code: 1
      stderr_contains: "title is required"

  - name: Create epic with labels
    inputs:
      command: cupboard create --type epic --title "Storage" --labels "code"
    expected:
      exit_code: 0
      state:
        crumb_type: epic
        crumb_labels:
          - code

  - name: Create with JSON output
    inputs:
      command: cupboard create --type task --title "Test" --description "Desc" --json
    expected:
      exit_code: 0
      stdout_json:
        type: task
        title: "Test"
        state: open

cleanup:
  - Remove the test data directory
```

## Implementation Layout

Test suite code lives in a single Go file or directory that matches the YAML spec filename.

Table 4: Test suite code layout

| YAML spec | Go test code |
| --------- | ------------ |
| `docs/specs/test-suites/test-rel02.1-uc003-self-hosting.yaml` | `tests/integration/test_rel02_1_uc003_self_hosting_test.go` |
| `docs/specs/test-suites/test-rel01.1-uc002-jsonl-git-roundtrip.yaml` | `tests/integration/test_rel01_1_uc002_jsonl_git_roundtrip_test.go` |
| Long suite (many test cases) | `tests/integration/test_rel02_1_uc002_table_benchmarks/` (directory with multiple files) |

Rules:

- One test suite per file. If the suite is long, use a directory named after the suite.
- Use Go table-driven tests to avoid repeating setup and assertion logic across test cases. Each YAML test case maps to one row in the table.
- The Go file or directory name matches the YAML filename: replace hyphens with underscores and append `_test.go`.

Example table-driven pattern:

```go
func TestCreateCrumb(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
        check   func(t *testing.T, output string)
    }{
        {"required fields", []string{"create", "--type", "task", "--title", "Test"}, false, nil},
        {"missing title", []string{"create", "--type", "task"}, true, nil},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // shared setup, exec, assertions
        })
    }
}
```

## Writing Guidelines

- **One feature per suite**: Group test cases that exercise the same command or operation. Split unrelated behaviors into separate suites.
- **Concrete inputs**: Use real commands, flags, and representative data. Avoid placeholders.
- **Checkable outputs**: Each expected field must be verifiable without judgment. Prefer exact values over vague descriptions.
- **Traceability**: Every suite must trace to at least one use case or PRD requirement.
- **Independent test cases**: Each test case should be runnable in isolation given the preconditions. Do not rely on ordering between test cases unless explicitly noted.
- **Table-driven tests**: Use Go table-driven tests to implement the test cases from the YAML spec. Each row in the table corresponds to one test case in the YAML.

## Relationship to Other Docs

| Document | Role |
|----------|------|
| **Use cases** | Define scenarios that test suites validate. Success criteria map to one or more test cases. |
| **PRDs** | Define requirements that test cases verify. The traces field links suites to requirement IDs. |
| **ARCHITECTURE** | Describes the components under test. |

## Completeness Checklist

- [ ] id matches the filename
- [ ] title describes the feature under test
- [ ] traces lists at least one use case or PRD requirement
- [ ] preconditions describe the shared starting state
- [ ] Each test case has a name, inputs, and expected outputs
- [ ] Inputs use real commands and data
- [ ] Expected outputs are specific and checkable
- [ ] File saved as `test-[use-case-id].yaml` in `docs/specs/test-suites/`
