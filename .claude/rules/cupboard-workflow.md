<\!-- Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT -->

# Cupboard Issue Tracking and Session Completion Workflow

## Working Offline

**We work offline.** There is no access to the remote repository. **Local git commit works** and is required; **do not run `git push`** (or `git pull`). The user will sync with the remote when they have network access.

## Do Not Edit JSONL Files Directly

**Never change JSONL files in the data directory by hand.** Do not edit crumbs.jsonl, trails.jsonl, or any other JSONL file with an editor or script. All issue creation, updates, comments, and status changes must go through the **cupboard** CLI (e.g., `cupboard update`, `cupboard comments add`, `cupboard close`). Commits may include JSONL changes produced by `cupboard`; the agent must not modify those files directly.

## Quick Reference

```bash
cupboard ready              # Find available work
cupboard show <id>          # View issue details
cupboard update <id> --status in_progress  # Claim work
cupboard comments add <id> "tokens: <count>"  # Log token usage
cupboard close <id>         # Close work
```

## Token Tracking

**Track token usage for every issue:**

1. **At start of issue** - Note current token count from context
2. **When closing issue** - Calculate tokens used and log it:
   ```bash
   cupboard comments add <id> "tokens: <count>"
   cupboard close <id>
   ```

Example:
```bash
# Started with 1000000 tokens, now at 965744
# Used: 34256 tokens
cupboard comments add atlas-123 "tokens: 34256"
cupboard close atlas-123
```

## LOC and Documentation Tracking

**Track lines of code and documentation changes per issue:**

1. **At start of issue** - Run `mage stats` and note the baseline:
   ```bash
   mage stats
   # Save: LOC_PROD=441, LOC_TEST=0, DOC_WORDS=21032
   ```

2. **When closing issue** - Run the command again and calculate the delta:
   ```bash
   mage stats
   # New: LOC_PROD=520, LOC_TEST=45, DOC_WORDS=21900
   # Delta: +79 LOC (prod), +45 LOC (test), +868 words (docs)
   ```

3. **Include full stats in commit message** - Add the Stats block with totals and deltas:

   ```text
   Add feature X (issue-id)

   - Description of changes

   Stats:
     Lines of code (Go, production): 520 (+79)
     Lines of code (Go, tests):      45 (+45)
     Words (documentation):          21900 (+868)
   ```

   **Do NOT use a condensed format** like `Delta: +79 LOC (prod)...`. Always use the full Stats block.

## Using the --json Flag

The cupboard CLI supports `--json` flag on most commands for machine-readable output. Use `--json` when you need to parse command results programmatically, especially when working with scripts or when extracting specific fields.

```bash
# Get issue details as JSON
cupboard show <id> --json

# List ready tasks as JSON
cupboard ready --json

# Create issue and capture JSON output
cupboard create --type task --title "Test" --description "..." --json
```

Without `--json`, output is human-readable (formatted tables or key-value pairs).

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until changes are committed locally.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status and log tokens**:
   - Calculate tokens used this session
   - Add comment with token count: `cupboard comments add <id> "tokens: <count>"`
   - Close finished work: `cupboard close <id>`
   - Update in-progress items
4. **COMMIT CHANGES** - This is MANDATORY:
   ```bash
   git add -A
   git commit -m "descriptive message"
   git status  # Verify all changes committed
   ```
   (Do not run `git push`; we have no remote access. Commit works locally.)
5. **Clean up** - Clear stashes; skip remote operations (we are offline).
6. **Verify** - All changes committed locally.
7. **Hand off** - Provide context for next session; inform user that changes are committed locally and they can push when they have network access. **When summarizing changes in code or markdown**, run `mage stats` and include its output (Go production/test LOC, doc words) in the summary.

**CRITICAL RULES:**
- Work is NOT complete until changes are committed locally
- NEVER leave uncommitted changes - commit everything
- **After creating or editing any files** (docs, code, use cases, rules, config), run `git add -A` and `git commit` with a descriptive message **before ending your turn**. Do not hand off with uncommitted changes.
- **We work offline** - Do not push; local commit is required and works. The user will push when they have network access.

## Cupboard Commands Reference

### Finding Work

```bash
# Find ready tasks (default: all ready tasks)
cupboard ready

# Find up to N ready tasks
cupboard ready -n 5

# Find ready tasks of specific type
cupboard ready --type task

# Get JSON output for scripting
cupboard ready --json
```

### Viewing Issues

```bash
# Show issue details (human-readable)
cupboard show <id>

# Show issue details (JSON)
cupboard show <id> --json

# List all issues
cupboard list --json
```

### Creating Issues

```bash
# Create a task
cupboard create --type task --title "Implement feature" --description "Details"

# Create with JSON output
cupboard create --type task --title "Test" --description "..." --json

# Create epic with parent
cupboard create --type epic --title "Major feature" --parent <parent-id>
```

### Updating Issues

```bash
# Claim work (transition to in_progress)
cupboard update <id> --status in_progress

# Update title
cupboard update <id> --title "New title"

# Update with JSON output
cupboard update <id> --status in_progress --json
```

### Closing Issues

```bash
# Close (mark as complete)
cupboard close <id>

# Close with JSON output
cupboard close <id> --json
```

### Comments

```bash
# Add a comment
cupboard comments add <id> "Comment text"

# Log token usage
cupboard comments add <id> "tokens: 34256"
```

### State Transitions

Valid crumb states (per prd003-crumbs-interface R2):
- `draft` - Initial state on creation
- `pending` - Defined but waiting for precondition
- `ready` - Available for work
- `taken` - Work in progress
- `pebble` - Completed successfully (terminal)
- `dust` - Failed or abandoned (terminal)

Terminal states (`pebble`, `dust`) cannot transition to other states.

## Migration from Beads

The cupboard commands provide parity with beads (bd) CLI commands for issue tracking. When migrating from beads:

| Beads command | Cupboard equivalent |
|---------------|---------------------|
| `bd ready` | `cupboard ready` |
| `bd show <id>` | `cupboard show <id>` |
| `bd update <id> --status S` | `cupboard update <id> --status S` |
| `bd comments add <id> "text"` | `cupboard comments add <id> "text"` |
| `bd close <id>` | `cupboard close <id>` |
| `bd sync` | (not needed; cupboard syncs on every write) |

The cupboard backend uses immediate sync strategy by default (per prd002-sqlite-backend R16), so there is no separate sync command. JSONL files are updated on every write.
