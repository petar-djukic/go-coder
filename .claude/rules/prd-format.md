<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# PRD Format

PRDs are YAML files stored at `/docs/specs/product-requirements/prd[NNN]-[feature-name].yaml`.

## Top-Level Fields

Every PRD has these top-level fields in this order.

```yaml
id: prd001-feature-name
title: Human-readable title
problem: |
  Multi-line text explaining the problem.
goals:
  - G1: First goal
  - G2: Second goal
requirements:
  R1:
    title: Requirement Group Name
    items:
      - R1.1: First sub-requirement
      - R1.2: Second sub-requirement
  R2:
    title: Another Group
    items:
      - R2.1: A sub-requirement
non_goals:
  - What we are NOT building
  - Another explicit boundary
acceptance_criteria:
  - Checkable outcome one
  - Checkable outcome two
```

### id

The PRD identifier, matching the filename without extension. Lowercase, hyphenated.

```yaml
id: prd001-cupboard-core
```

### title

A short human-readable name.

```yaml
title: Cupboard Core Interface
```

### problem

A multi-line string explaining what problem we solve, why now, and why it matters. Use the YAML literal block scalar (`|`) for multi-line text. Write in paragraph form per documentation-standards.

```yaml
problem: |
  Applications using Crumbs need a consistent way to initialize storage,
  access data tables, and manage the cupboard lifecycle. Without a
  well-defined core interface, each application handles backend
  initialization differently.
```

### goals

A list of measurable objectives. Each goal has a number prefix (`G1`, `G2`, ...) followed by a colon and the goal text.

```yaml
goals:
  - G1: Define a Config struct that selects backends
  - G2: Define the Cupboard interface with uniform table access
  - G3: Specify error handling for post-detach operations
```

### requirements

A map of requirement groups. Each group key is a requirement number (`R1`, `R2`, ...) and contains a `title` and an `items` list. Each item has a sub-number prefix (`R1.1`, `R1.2`, ...) followed by a colon and the requirement text.

```yaml
requirements:
  R1:
    title: Configuration
    items:
      - R1.1: The Config struct must include Backend (string) and DataDir (string)
      - R1.2: Config validation must fail if Backend is empty or unrecognized
      - R1.3: Config validation must fail if DataDir is empty when Backend is "sqlite"
  R2:
    title: Cupboard Interface
    items:
      - R2.1: The Cupboard interface must define the contract for storage access
      - R2.2: "GetTable must return a Table interface for the specified table name"
```

Requirements must be specific and actionable. Each requirement starts with "must", "must not", or a concrete verb. Quote strings in YAML when they contain colons or special characters.

Requirements may include tables, code blocks, or other structured data as nested fields when the text alone is insufficient. Use a `detail` field for extended content.

```yaml
requirements:
  R1:
    title: Configuration
    items:
      - R1.1: The Config struct must include the following fields
        detail: |
          | Field   | Type   | Description              |
          |---------|--------|--------------------------|
          | Backend | string | Backend type: "sqlite"   |
          | DataDir | string | Directory for the backend |
```

### non_goals

A list of strings stating what we are NOT building. Each entry is a complete sentence.

```yaml
non_goals:
  - This PRD does not define migration tooling from JSON arrays to JSONL
  - This PRD does not define backwards compatibility with JSON array files
  - This PRD does not define configuration file encryption
```

### acceptance_criteria

A list of checkable outcomes. Each entry must be verifiable without ambiguity.

```yaml
acceptance_criteria:
  - Config struct includes Backend and DataDir fields
  - GetTable returns ErrTableNotFound for unrecognized names
  - Attach creates data directory if it does not exist
  - All requirements numbered and specific
```

## Optional Fields

These fields follow acceptance_criteria when present.

### constraints

A list of technical or business limitations.

```yaml
constraints:
  - JSONL files must remain human-readable
  - Atomic write pattern required for data integrity
```

### open_questions

A list of unresolved issues. Each entry has a number prefix (`Q1`, `Q2`, ...).

```yaml
open_questions:
  - Q1: Should we support reading legacy JSON array files during transition?
  - Q2: Should stash_history.jsonl use compaction to avoid unbounded growth?
```

### references

A list of related documents or external references.

```yaml
references:
  - prd001-cupboard-core
  - prd002-sqlite-backend
  - "XDG Base Directory Specification"
```

## Numbering Rules

| Element | Format | Example |
| ------- | ------ | ------- |
| Goals | `G` + integer | G1, G2, G3 |
| Requirement groups | `R` + integer | R1, R2, R3 |
| Requirement items | `R` + group + `.` + integer | R1.1, R1.2, R2.1 |
| Open questions | `Q` + integer | Q1, Q2 |

Numbers are sequential within their scope. Do not skip numbers.

## Writing Guidelines

- **Audience**: Junior developer (explicit, no jargon)
- **Requirements**: Specific, actionable, starting with "must" or a concrete verb
- **Style**: Follow documentation standards (concise, active voice, no forbidden terms)

## Before Writing

Ask 3-5 clarifying questions if the request is ambiguous:
- Number questions (1, 2, 3)
- List options as A, B, C for easy selection
- Focus on: Problem clarity, Core functionality, Scope boundaries

## Completeness Checklist

- [ ] id matches the filename without extension
- [ ] title describes the feature
- [ ] problem states what we solve and why it matters
- [ ] goals are numbered (G1, G2, ...) and measurable
- [ ] requirements are grouped (R1, R2, ...) with numbered items (R1.1, R1.2, ...)
- [ ] Each requirement is specific and actionable
- [ ] non_goals define what is out of scope
- [ ] acceptance_criteria are checkable without ambiguity
- [ ] File saved as `prd[NNN]-[feature-name].yaml` in `/docs/specs/product-requirements/`
