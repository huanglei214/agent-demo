# AGENTS.md

This file gives project-specific guidance for AI coding agents working in this repository.

## Project Snapshot

- Project name: `agent-demo`
- Current shape: single-machine local Agent Harness MVP
- Main entrypoint:
  - [cmd/harness/main.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/cmd/harness/main.go)
- Main surfaces:
  - Cobra CLI
  - local filesystem runtime artifacts under `.runtime/`
  - local HTTP API via `serve`
  - local React + Vite web UI under `web/`

## Tech Stack

- Backend: Go
- CLI: Cobra
- Frontend: React + TypeScript + Vite
- Frontend styling: plain CSS in `web/src/styles.css`
- Model providers:
  - `mock` for local development and regression verification
  - `ark` for real model calls

## Preferred Workflow

When making changes, follow this default order unless the task clearly needs something else:

1. Read the relevant code and nearby tests before editing.
2. Prefer the smallest coherent change that matches the existing architecture.
3. Verify with the narrowest useful command first.
4. If the change affects cross-cutting runtime behavior, run scenario verification before finishing.

## Commands You Will Likely Need

### General

- Show CLI help:
  - `make help`
- Build the backend:
  - `make build`
- Tidy modules:
  - `make tidy`

### Local runtime flows

- Start a one-off run:
  - `make run ARGS='your instruction' PROVIDER=mock`
- Start chat mode:
  - `make chat PROVIDER=mock`
- Inspect a run:
  - `make inspect RUN=<run-id>`
- Inspect a session:
  - `make session-inspect SESSION=<session-id>`
- Replay a run:
  - `make replay RUN=<run-id>`
- View raw events:
  - `make debug-events RUN=<run-id>`

### API and web UI

- Start backend API:
  - `make serve PROVIDER=mock`
- Start backend + frontend together:
  - `make dev PROVIDER=mock`
- Start frontend only:
  - `make web-dev`
- Build frontend:
  - `make web-build`

### Verification

- Run the fixed regression scenarios:
  - `make verify-scenarios`
- Run all Go tests:
  - `go test ./...`

## Repository Map

### Backend runtime and application services

- `internal/app/`
  - orchestration services for run, replay, resume, inspect, session, tools, and scenario regression
- `internal/runtime/`
  - shared runtime types
- `internal/store/filesystem/`
  - persistence for run/session/event artifacts
- `internal/context/`
  - context assembly
- `internal/memory/`
  - memory management
- `internal/prompt/`
  - prompt building
- `internal/planner/`
  - planning logic
- `internal/delegation/`
  - controlled sub-agent delegation

### CLI and HTTP

- `internal/cli/`
  - Cobra subcommands
- `internal/httpapi/`
  - local API handlers and router
- `internal/agui/`
  - AG-UI adapter and streaming support

### Tools and model providers

- `internal/tool/`
  - tool registry, types, executor, filesystem tools
- `internal/model/mock/`
  - mock provider used for local development
- `internal/model/ark/`
  - Ark provider implementation

### Frontend

- `web/src/pages/`
  - route-level pages
- `web/src/components/`
  - shared UI components
- `web/src/lib/`
  - API clients, types, i18n helpers
- `web/src/styles.css`
  - global app styling

### Specs and docs

- `openspec/specs/`
  - current accepted specs
- `openspec/changes/`
  - in-progress or archived change records
- `docs/`
  - product and technical docs

## Project Conventions

### Use `mock` by default unless the task explicitly needs `ark`

For local development, examples, regression checks, and test-oriented work, prefer:

- `PROVIDER=mock`

This keeps runs deterministic and avoids requiring external credentials.

### Treat `.runtime/` as local generated state

- `.runtime/` contains local run and session artifacts.
- Inspect it for debugging when useful.
- Do not rely on existing `.runtime/` contents being stable across tasks.
- Do not commit runtime artifacts.

### Do not commit dependency install directories

- Do not commit `web/node_modules/`.
- `web/package-lock.json` is fine to commit when dependencies intentionally change.

### Frontend styling

- Current state:
  - the frontend is still implemented with plain CSS in `web/src/styles.css`
  - Tailwind is not yet wired into the repo today
- Direction:
  - the project is preparing to move toward Tailwind for future frontend work
- Until Tailwind is actually installed and configured:
  - do not assume Tailwind utilities are available
  - keep existing pages working with the current CSS approach
- Once Tailwind is introduced:
  - prefer Tailwind for new UI work
  - migrate old styles incrementally instead of rewriting the whole frontend in one pass
  - avoid mixing large new plain-CSS systems with the Tailwind direction unless there is a strong reason
- Execution rule:
  - if a task changes frontend styling and Tailwind is still not installed, implement the change with the current CSS system
  - if a task is specifically about introducing Tailwind, first add the Tailwind toolchain and verify it works before writing new Tailwind-based UI
  - after Tailwind lands, prefer incremental migration page-by-page or component-by-component instead of a full rewrite

### Keep changes aligned with existing surfaces

If a change touches user-facing behavior, check whether it should be reflected in one or more of:

- CLI command behavior in `internal/cli/`
- HTTP handlers in `internal/httpapi/`
- frontend API usage in `web/src/lib/api.ts`
- OpenSpec docs under `openspec/`
- README examples

## Testing Guidance

Use the smallest test that gives confidence:

- For focused logic changes:
  - run targeted `go test` on the affected package
- For runtime flow changes:
  - run `make verify-scenarios`
- For web changes:
  - at minimum run `make web-build`
- For broad or risky changes:
  - run `go test ./...`

If you do not run a test, say so explicitly in your handoff.

### Change Type -> Minimum Verification

- If you change `internal/app/`:
  - run targeted tests in `internal/app`
  - if the change affects run lifecycle, session continuity, replay/resume, delegation, or persisted artifacts, also run `make verify-scenarios`
- If you change `internal/httpapi/`:
  - run targeted tests in `internal/httpapi`
  - if request or response shapes changed, also check `web/src/lib/api.ts` and affected pages
- If you change `internal/cli/`:
  - run targeted tests in `internal/cli` when available
  - also sanity-check the related `make` command or CLI help path if the change affects flags or command wiring
- If you change `internal/agui/`:
  - run targeted tests in `internal/agui`
  - if the change affects streaming or event mapping, also verify the chat web flow if feasible
- If you change `internal/tool/` or filesystem tool behavior:
  - run targeted tests under `internal/tool/...`
  - if tool behavior affects runtime execution, also run `make verify-scenarios`
- If you change `internal/model/`:
  - run targeted tests for the affected provider package
  - prefer `mock` for broader runtime verification unless the task specifically requires live `ark` validation
- If you change `internal/planner/`, `internal/prompt/`, `internal/context/`, `internal/memory/`, or `internal/delegation/`:
  - run targeted tests in the affected package
  - if the change can alter end-to-end agent behavior, also run `make verify-scenarios`
- If you change `internal/store/filesystem/` or artifact persistence:
  - run targeted tests in `internal/store/filesystem`
  - also run `make verify-scenarios` if saved run/session state semantics may have changed
- If you change `web/`:
  - at minimum run `make web-build`
  - if the change touches API integration, also verify the corresponding backend handler or contract
  - if the change touches chat, run/session details, or SSE updates, prefer checking the relevant flow with `make dev PROVIDER=mock` when feasible
- If you change `Makefile`, `scripts/`, or local dev workflow:
  - run the specific command path you edited whenever feasible
  - if the change affects combined backend/frontend startup, prefer checking `make dev PROVIDER=mock`
- If you change `README.md` or `docs/` only:
  - no code test is required by default
  - verify commands and paths in examples against the current repo layout
- If you change `openspec/` only:
  - no code test is required by default
  - verify the related implementation or README is still aligned if the spec changes describe already-landed behavior

## Editing Guidance

- Preserve existing terminology: `run`, `session`, `replay`, `resume`, `inspect`, `tools`, `serve`, `AG-UI`.
- Avoid large refactors unless they are required by the task.
- Prefer extending existing services and routes over inventing parallel abstractions.
- Keep docs and examples in sync when changing CLI flags, API shapes, or key workflows.

## Good First Checks Before Finishing

Before handing work back, quickly ask:

1. Does the change affect only CLI, or also API and web UI?
2. Does the change require updating README examples?
3. Does the change require updating OpenSpec artifacts?
4. Did I avoid committing generated local files and dependency directories?
5. Did I verify with an appropriate command?

## Notes For Future Iteration

This file is intentionally practical rather than exhaustive. It should evolve as the repository gains:

- clearer test tiers
- stricter frontend conventions
- branch / commit conventions
- release or deployment workflows
- stronger guidance for OpenSpec-driven changes
