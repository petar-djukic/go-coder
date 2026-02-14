<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Use Case Format

Use cases are YAML files stored at `docs/specs/use-cases/rel[NN].[N]-uc[NNN]-[short-name].yaml`.

A **use case** describes a concrete usage of the architecture. It specifies a **tracer bullet**: one end-to-end path of functionality through the system. Use cases lead to a **proof of concept** or **demo** and guide **how we develop software**—what to build next and in what order.

Every use case must have a corresponding **test suite** in `docs/specs/test-suites/`. When creating a new use case, also create or update the test suite that validates its success criteria. See the test-case-format rule for the test suite structure.

## Top-Level Fields

Every use case has these top-level fields in this order.

```yaml
id: rel01.0-uc001-cupboard-lifecycle
title: Configuration and Cupboard Lifecycle
summary: |
  An application initializes a Cupboard with a typed Config struct,
  attaches to a SQLite backend, accesses tables, and detaches.
actor: Developer integrating the Crumbs library
trigger: Application startup requiring backend connection
flow:
  - F1: "Create configuration: construct a Config struct with Backend and DataDir"
  - F2: "Attach to backend: call Attach(config) on the Cupboard"
  - F3: "Access tables: call GetTable for each standard table name"
  - F4: "Perform operations: use Table.Get, Set, Delete, Fetch"
  - F5: "Detach: call Detach() to release resources"
touchpoints:
  - T1: "Cupboard interface: Attach, Detach, GetTable"
  - T2: "Table interface: Get, Set, Delete, Fetch"
  - T3: "SQLite backend: NewBackend, schema creation"
success_criteria:
  - S1: Attach(config) returns nil and initializes the backend
  - S2: GetTable returns a valid Table for each standard table name
  - S3: Detach releases all resources
out_of_scope:
  - Connection pooling or retry policies
  - File-based configuration loading
test_suite: test-rel01.0-uc001-cupboard-lifecycle
```

### id

The use case identifier, matching the filename without extension. Format: `rel[NN].[N]-uc[NNN]-[short-name]`.

```yaml
id: rel01.0-uc004-scaffolding-validation
```

### title

A short human-readable name.

```yaml
title: Scaffolding Validation
```

### summary

A multi-line string: who does what, and what outcome is achieved. The elevator pitch for the use case. Use the YAML literal block scalar (`|`).

```yaml
summary: |
  A developer builds the cupboard CLI, runs the version command,
  and verifies that all standard tables and entity types are
  accessible through the Cupboard and Table interfaces.
```

### actor

Who or what initiates the scenario (e.g. developer, agent, system, upstream service).

```yaml
actor: Developer who has cloned the repository
```

### trigger

What event or action starts the scenario.

```yaml
trigger: Running go build on the cupboard CLI
```

### flow

An ordered list of steps from trigger to outcome. Each step has a number prefix (`F1`, `F2`, ...) followed by a colon and the step description. Each step should be testable and map to components or operations. This is the tracer bullet path.

```yaml
flow:
  - F1: "Build the cupboard CLI: run go build ./cmd/cupboard"
  - F2: "Run the version command: execute cupboard version"
  - F3: "Verify entity structs compile: build transitively compiles all types"
  - F4: "Attach and enumerate tables: call GetTable for each standard table"
```

Steps may include a `detail` field for extended content (code snippets, tables).

```yaml
flow:
  - F1: "Create configuration"
    detail: |
      Construct a Config struct specifying Backend as "sqlite"
      and DataDir as the path to the data directory.
  - F2: "Attach to backend"
    detail: |
      Call Attach(config). The operation validates the config,
      creates the DataDir if needed, and initializes the schema.
```

### touchpoints

Architecture elements this use case exercises: interfaces, components, and protocols. Each entry has a number prefix (`T1`, `T2`, ...) followed by a colon and the touchpoint description. Reference PRD requirement IDs where applicable.

```yaml
touchpoints:
  - T1: "Cupboard interface: Attach, GetTable, Detach (prd001-cupboard-core R2)"
  - T2: "Table interface: Get, Set, Delete, Fetch (prd001-cupboard-core R2)"
  - T3: "SQLite backend: NewBackend, schema creation (prd002-sqlite-backend R1)"
  - T4: "Crumb entity: struct fields (prd003-crumbs-interface R1)"
```

### success_criteria

How we know the use case is implemented. Each entry has a number prefix (`S1`, `S2`, ...) followed by a colon and a checkable outcome. Must be verifiable without ambiguity.

```yaml
success_criteria:
  - S1: go build ./cmd/cupboard completes without errors
  - S2: cupboard version prints a version string and exits with code 0
  - S3: GetTable succeeds for all six standard table names
  - S4: GetTable for an unknown table name returns ErrTableNotFound
```

### out_of_scope

A list of strings stating what this use case does *not* cover. Keeps the tracer bullet thin. Each entry is a complete sentence or phrase.

```yaml
out_of_scope:
  - Creating, retrieving, or deleting entities (see rel01.0-uc002)
  - Property operations (see rel02.0-uc001)
  - Trail lifecycle operations (see rel03.0-uc001)
```

### test_suite

The ID of the corresponding test suite in `docs/specs/test-suites/`. Every use case must have a test suite. When creating a new use case, also create the test suite YAML file.

```yaml
test_suite: test-rel01.0-uc004-scaffolding-validation
```

## Optional Fields

These fields follow test_suite when present.

### dependencies

Other use cases or PRD deliverables that must exist first. Each entry has a number prefix (`D1`, `D2`, ...).

```yaml
dependencies:
  - D1: prd001-cupboard-core (Cupboard and Table interface definitions)
  - D2: prd002-sqlite-backend (backend implementation)
  - D3: rel01.0-uc002-sqlite-crud (Table CRUD must work)
```

### risks

What could block the PoC or demo and how we address it. Each entry has a number prefix (`K1`, `K2`, ...) and includes the risk and its mitigation.

```yaml
risks:
  - K1: "Struct fields change as PRDs evolve | Update tests when PRDs change"
  - K2: "Backend lags behind interface | Compile-time assertions catch mismatches"
```

### demo

An observable demo script or code snippet showing the use case in action. Use the YAML literal block scalar (`|`).

```yaml
demo: |
  go build -o cupboard ./cmd/cupboard
  ./cupboard version
  # Output: cupboard v0.1.0
  # Implemented use cases:
  #   rel01.0-uc004  scaffolding-validation
```

### references

A list of related documents or external references.

```yaml
references:
  - prd001-cupboard-core
  - prd002-sqlite-backend
  - docs/ARCHITECTURE.md
```

## Numbering Rules

| Element | Format | Example |
| ------- | ------ | ------- |
| Flow steps | `F` + integer | F1, F2, F3 |
| Touchpoints | `T` + integer | T1, T2, T3 |
| Success criteria | `S` + integer | S1, S2, S3 |
| Dependencies | `D` + integer | D1, D2, D3 |
| Risks | `K` + integer | K1, K2, K3 |

Numbers are sequential within their scope. Do not skip numbers.

## File and Naming

- **Location**: `docs/specs/use-cases/rel[NN].[N]-uc[NNN]-[short-name].yaml`
- **Release prefix**: Major and minor release (e.g., 01.0, 01.1, 02.0). A release delivers all use cases in that release. Minor releases add functionality discovered later.
- **Use case number**: Three-digit sequence number within the release (e.g., 001, 002, 003).
- **Short name**: Lowercase, hyphenated, verb or scenario describing the use case.

### Release Numbering

See road-map.yaml for the authoritative release schedule and use case status. Summary:

| Release | Focus |
| ------- | ----- |
| 01.0 | Core storage with SQLite backend |
| 01.1 | Post-core validation (go install, JSONL roundtrip) |
| 02.0 | Properties with enforcement |
| 02.1 | Issue-tracking CLI, benchmarks, self-hosting |
| 03.0 | Trails and stashes |
| 99.0 | Unscheduled (not yet assigned to a release) |

Minor releases (e.g., 01.1, 02.1) validate completed major releases without renumbering existing use cases.

## Test Suite Requirement

Every use case must have a corresponding test suite. The test suite:

- Lives in `docs/specs/test-suites/test-[use-case-id].yaml`
- Traces back to the use case via the `traces` field
- Contains test cases that validate the success criteria (`S1`, `S2`, ...)
- Follows the test-case-format rule

When creating a new use case, create the test suite in the same work unit. When modifying a use case (adding flow steps or success criteria), update the test suite to cover the changes.

## Roadmap Requirement

Every new use case must have an entry in `docs/road-map.yaml`. When creating a new use case, add it to the appropriate release in road-map.yaml with its id, summary, and status. If the use case does not belong to an existing release, add it to release 99.0 (unscheduled).

## Writing Guidelines

- **One path**: One primary flow per use case. Variants or error paths can be short subsections within flow steps.
- **Concrete**: Use real operations, component names, and data from the architecture.
- **Aligned to docs**: Reference ARCHITECTURE and PRDs so the use case stays consistent with the design.
- **Demo-ready**: Success criteria should be something you can show: "Run X, then Y, then Z; observe W."
- **Testable**: Every success criterion must map to one or more test cases in the test suite.
- **Style**: Follow documentation standards (concise, active voice, no forbidden terms).

## Relationship to Other Docs

| Document | Role |
| -------- | ---- |
| **VISION** | Why we build; use case should support vision goals. |
| **ROADMAP** | Release schedule and use case status; determines work priority. |
| **ARCHITECTURE** | What we build; use case traces a path through it. |
| **PRDs** | Detailed requirements for components; use case motivates which PRD items to implement first. |
| **Test suites** | Test cases with inputs and expected outputs; validate use case success criteria. |
| **Use case** | One tracer bullet through the stack; guides development order. |

## Completeness Checklist

- [ ] id matches the filename without extension
- [ ] title describes the use case
- [ ] summary states who, what, and outcome
- [ ] actor and trigger are explicit
- [ ] flow steps are numbered (F1, F2, ...), end-to-end, and map to real components
- [ ] touchpoints are numbered (T1, T2, ...) and list interfaces, components, and protocols
- [ ] success_criteria are numbered (S1, S2, ...) and checkable without ambiguity
- [ ] out_of_scope keeps the use case focused
- [ ] test_suite references a corresponding test suite ID
- [ ] Test suite YAML exists in `docs/specs/test-suites/` and traces back to this use case
- [ ] Use case added to the appropriate release in `docs/road-map.yaml`
- [ ] File saved as `rel[NN].[N]-uc[NNN]-[short-name].yaml` in `docs/specs/use-cases/`
