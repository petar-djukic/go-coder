<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Specification Document Format

A specifications document is a human-readable summary that ties together the project's PRDs, use cases, test suites, and roadmap into one navigable page. It does not duplicate content from those files; it summarizes each artifact in a sentence or two and shows how they relate. A reader who finishes SPECIFICATIONS.md should understand what we are building, in what order, and where the details live.

## File and Naming

- **Location**: `docs/SPECIFICATIONS.md`
- One file per project. Regenerate or update it whenever PRDs, use cases, test suites, or the roadmap change.

## Required Sections

1. **Overview** -- One or two paragraphs stating what the system does and what this document covers. Reference VISION.md for goals and ARCHITECTURE.md for design.

2. **Roadmap Summary** -- A table summarizing each release from `docs/road-map.yaml`. Include release version, name, status (e.g. done, in progress, not started), and the number of use cases in each state.

   Example

   Table 1 Roadmap Summary

   | Release | Name | Use Cases (done / total) | Status |
   |---------|------|--------------------------|--------|
   | 01.0 | Core Storage with SQLite Backend | 3 / 4 | in progress |
   | 01.1 | Post-Core Validation | 0 / 2 | not started |

3. **PRD Index** -- A table listing every PRD in `docs/specs/product-requirements/`. Columns: PRD ID, title, and a one-sentence summary of the problem it solves. Link each ID to the file path.

   Example

   Table 2 PRD Index

   | PRD | Title | Summary |
   |-----|-------|---------|
   | [prd001-cupboard-core](product-requirements/prd001-cupboard-core.yaml) | Cupboard Core Interface | Defines the Cupboard and Table interfaces for backend-agnostic storage access |

4. **Use Case Index** -- A table listing every use case in `docs/specs/use-cases/`. Columns: use case ID, title, release, status, and test suite ID. Link each ID to the file path and each test suite ID to its file.

   Example

   Table 3 Use Case Index

   | Use Case | Title | Release | Status | Test Suite |
   |----------|-------|---------|--------|------------|
   | [rel01.0-uc001](use-cases/rel01.0-uc001-cupboard-lifecycle.yaml) | Cupboard Lifecycle | 01.0 | done | [test004](test-suites/test004-cupboard-lifecycle.yaml) |

5. **Test Suite Index** -- A table listing every test suite in `docs/specs/test-suites/`. Columns: test suite ID, title, traces (which use cases or PRDs it validates), and number of test cases.

   Example

   Table 4 Test Suite Index

   | Test Suite | Title | Traces | Test Cases |
   |------------|-------|--------|------------|
   | [test004](test-suites/test004-cupboard-lifecycle.yaml) | Cupboard lifecycle and CRUD | rel01.0-uc001, rel01.0-uc003 | 12 |

6. **PRD-to-Use-Case Mapping** -- A table showing how each use case relates to each PRD it exercises. For every use case / PRD pair (derived from the touchpoints field), list why the use case needs that PRD and how much of the PRD the use case covers (e.g. "full", "partial (R1-R3 only)", "R2.1 and R5 only"). Build this table by reading each use case's touchpoints and mapping them to PRD requirement IDs.

   Example

   Table 5 PRD-to-Use-Case Mapping

   | Use Case | PRD | Why Required | Coverage |
   | -------- | --- | ------------ | -------- |
   | rel01.0-uc001-cupboard-lifecycle | prd001-cupboard-core | Validates Config, Attach, Detach, GetTable contract | Partial (R1, R2, R4-R7) |
   | rel01.0-uc001-cupboard-lifecycle | prd002-sqlite-backend | Exercises SQLite backend initialization | Partial (R1 only) |
   | rel01.0-uc002-sqlite-crud | prd001-cupboard-core | Uses Table interface for CRUD operations | Partial (R2, R3) |
   | rel02.0-uc001-property-enforcement | prd004-properties-interface | Validates property definition, backfill, and enforcement | Full |

   Columns

   | Column | Description |
   | ------ | ----------- |
   | Use Case | Use case ID (link to file) |
   | PRD | PRD ID (link to file) |
   | Why Required | One sentence explaining why this use case exercises this PRD |
   | Coverage | How much of the PRD the use case covers: "full" if all requirements are exercised, or list the specific requirement IDs (e.g. "R1-R3", "R2.1 and R5 only") |

7. **Traceability Diagram** -- A PlantUML diagram showing the relationships between PRDs, use cases, and test suites. Use the documentation-standards inline PlantUML format. The diagram should show which use cases trace to which PRDs (via touchpoints) and which test suites validate which use cases (via the traces field).

   Use a UML component or object diagram. Group artifacts by type (PRDs on the left, use cases in the center, test suites on the right). Draw arrows from use cases to PRDs they exercise and from test suites to the use cases they validate.

   Example

   ```plantuml
   @startuml
   !theme plain
   skinparam backgroundColor white

   package "PRDs" {
     [prd001-cupboard-core] as prd1
     [prd002-sqlite-backend] as prd2
   }

   package "Use Cases" {
     [rel01.0-uc001\ncupboard-lifecycle] as uc1
     [rel01.0-uc002\nsqlite-crud] as uc2
   }

   package "Test Suites" {
     [test004\ncupboard-lifecycle] as ts4
     [test006\nsqlite-crud] as ts6
   }

   uc1 --> prd1
   uc1 --> prd2
   uc2 --> prd1
   uc2 --> prd2
   ts4 --> uc1
   ts6 --> uc2

   @enduml
   ```

   Caption: `|Figure 1 Traceability between PRDs, use cases, and test suites |`

   Include all PRDs, use cases, and test suites that exist at the time of generation. For large projects, split into one diagram per release if a single diagram becomes unreadable.

8. **Coverage Gaps** -- A short section listing any use cases without test suites, any PRDs not referenced by any use case, or any test suites that trace to nonexistent artifacts. This section may be empty if coverage is complete; in that case write "No gaps identified."

## Optional Sections

- **Frontmatter** -- YAML at top (title, date, generated-by) for tooling.
- **Per-Release Detail** -- One subsection per release with a paragraph describing the release goals, a list of its use cases and their status, and a release-scoped traceability diagram. Use this when the project has many releases and the single traceability diagram is too large.
- **References** -- Links to VISION, ARCHITECTURE, or external resources.

## Generating and Updating

We treat SPECIFICATIONS.md as a generated artifact. Regenerate it whenever any of the following change:

- A PRD is added, removed, or renamed
- A use case is added, removed, or its status changes
- A test suite is added or its traces change
- The roadmap changes (new release, status update)

When regenerating, read all files in `docs/specs/product-requirements/`, `docs/specs/use-cases/`, `docs/specs/test-suites/`, and `docs/road-map.yaml` to build the tables and diagram from current data. Do not rely on stale content from a previous version of SPECIFICATIONS.md.

## Writing Guidelines

- **Audience**: Developers, reviewers, and agents who need a quick overview of what specifications exist and how they connect.
- **Tone**: Use "we" in active voice per documentation-standards.
- **Scope**: Summarize and link; do not duplicate. Each table entry should be one or two sentences at most. The detail lives in the linked files.
- **Tables**: Name all tables per documentation-standards.
- **Figures**: Per documentation-standards. Define PlantUML inline; do not create separate `.puml` files.
- **Links**: Use relative paths from `docs/` (e.g. `specs/product-requirements/prd001-cupboard-core.yaml`, not absolute paths).

## Completeness Checklist

- [ ] Overview references VISION.md and ARCHITECTURE.md
- [ ] Roadmap Summary table covers every release in road-map.yaml
- [ ] PRD Index lists every file in docs/specs/product-requirements/
- [ ] Use Case Index lists every file in docs/specs/use-cases/
- [ ] Test Suite Index lists every file in docs/specs/test-suites/
- [ ] PRD-to-Use-Case Mapping lists every use case / PRD pair with why required and coverage
- [ ] Traceability Diagram shows PRD-to-use-case and use-case-to-test-suite relationships
- [ ] Coverage Gaps section identifies missing test suites or untraced PRDs
- [ ] Style follows documentation-standards (no forbidden terms, tables named, figures per standard)
- [ ] File saved as SPECIFICATIONS.md in docs/

## Relationship to Other Docs

| Document | Role |
|----------|------|
| **VISION** | Goals and boundaries. SPECIFICATIONS references it for context. |
| **ARCHITECTURE** | Components and interfaces. SPECIFICATIONS references it for design context. |
| **PRDs** | Numbered requirements. SPECIFICATIONS indexes and summarizes them. |
| **Use cases** | Tracer bullets. SPECIFICATIONS indexes them and shows their PRD and test suite links. |
| **Test suites** | Validation. SPECIFICATIONS indexes them and shows what they trace to. |
| **Roadmap** | Release schedule. SPECIFICATION summarizes release status. |
| **Engineering guidelines** | Conventions. Not indexed in SPECIFICATIONS (they describe process, not specifications). |
