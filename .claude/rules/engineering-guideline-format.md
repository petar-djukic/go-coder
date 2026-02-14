<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Engineering Guideline Format

Engineering guidelines document conventions, practices, and patterns that sit above the code and architecture. They are not PRDs (no numbered requirements) and not architecture docs (no component descriptions). They describe how the team works with the system.

## File and Naming

- **Location**: `docs/engineering/eng[NN]-[short-name].md`
- **Prefix**: `eng` followed by a two-digit sequence number (e.g., `eng01`, `eng02`)
- **Short name**: Lowercase, hyphenated, describing the topic (e.g., `git-integration`, `testing-strategy`)

Examples:
- `docs/engineering/eng01-git-integration.md`
- `docs/engineering/eng02-testing-strategy.md`

## Required Sections

1. **Title** — H1 heading describing the guideline topic
2. **Introduction** — One or two paragraphs: what this guideline covers and why it exists
3. **Body** — The guideline content, organized with H2 sections as needed. Use tables for structured information.

## Optional Sections

- **References** — Links to ARCHITECTURE, PRDs, or external resources

## Writing Guidelines

- **Audience**: Developers and agents working with the system
- **Tone**: Use "we" in active voice per documentation-standards
- **Scope**: Conventions and practices, not requirements. If it needs numbered requirements and acceptance criteria, it belongs in a PRD.
- **Tables**: Name all tables per documentation-standards
