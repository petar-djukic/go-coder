# Press

Agent runtime for specification-to-code generation.

Press is the execution engine inside a cobbler pipeline. Cobbler measures work
and proposes tasks. Crumbs tracks them as trails. Press takes a specification,
invokes an LLM, runs tools, and produces code. The model proposes; the runtime
decides what executes.

The name comes from the printing press. Cobbler composes the work (like a
typesetter arranging type), crumbs holds the pieces (like a type case holding
slugs of lead), and press prints the final output (like a press transferring
ink to paper). Press turns specifications into code the way a printing press
turns composed type into printed pages.

## How it works

Press is a Go library, not a standalone binary. The cobbler orchestrator
imports it and calls invoke with a specification, system prompt, provider
configuration, and workspace path. Press creates a loop trail branching from
the orchestrator's assignment crumb, starts an agentic loop, and returns the
trail ID immediately. The orchestrator observes progress by querying the
cupboard.

Each loop turn: prompt the LLM, receive tool call intents, validate (stash
gating, typed decode, semantic checks), execute, persist the result as a crumb.
Repeat until the specification is satisfied or a blocker is reported.

```
cobbler (orchestrator)
  │
  │ invoke(spec, config)
  ▼
press (agent runtime)
  │
  ├─ create loop trail (branches_from assignment crumb)
  ├─ prompt LLM
  ├─ receive ToolCallIntent
  ├─ stash gating → typed decode → semantic validation
  ├─ execute tool → persist crumb
  └─ repeat until terminal
```

## Project scope and status

Press is in specification phase. No application code has been generated yet.

| Release | Focus | Use cases | Status |
|---------|-------|-----------|--------|
| 01.0 | Core contracts (trail, stash, invocation, undo) | 6 | Specs done |
| 01.1 | Post-core validation (tools, hexagonal wiring, prompts) | 4 | Not started |
| 02.0 | Undo and compensation delivery | 2 | Not started |
| 03.0 | Hardening and measurable validation | 4 | Not started |

## Methodology

Specification-driven development. The chain runs VISION → ARCHITECTURE → PRDs
→ use cases → test suites → code. All specifications are YAML. Cobbler's
measure phase reads the specs and proposes tasks. Stitch executes them.

## Repository structure

```
press/
├── docs/
│   ├── VISION.yaml            Goals and boundaries
│   ├── ARCHITECTURE.yaml      Components and design decisions
│   ├── SPECIFICATIONS.yaml    Cross-reference index
│   ├── road-map.yaml          Release schedule
│   ├── specs/
│   │   ├── product-requirements/   24 PRDs
│   │   ├── use-cases/              4 use cases (spec complete)
│   │   ├── test-suites/            2 test suites (pending)
│   │   ├── interfaces/             10 interface contracts
│   │   └── component-catalog.yaml  Component inventory
│   ├── constitutions/         Writing and coding standards
│   └── prompts/               Prompt templates
├── magefiles/                 Build targets (cobbler-scaffold)
├── configuration.yaml         Orchestrator configuration
└── go.mod
```

## Technology choices

- **Go** for the runtime (type safety, interface boundaries, deterministic behavior).
- **Crumbs** (github.com/petar-djukic/crumbs) for trail and crumb persistence.
- **Cobbler-scaffold** for build orchestration and specification validation.
- **Ollama** and **AWS Bedrock** as LLM providers.
- **Hexagonal architecture** with ports and adapters for dependency inversion.

## Related projects

- [Cobbler](../cobbler) — orchestrator that drives measure/stitch cycles.
- [Crumbs](../crumbs) — storage system for work items, trails, and stashes.
- [Cobbler-scaffold](../cobbler-scaffold) — build orchestration and specification validation.
