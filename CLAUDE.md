# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Local-run Agent Harness with three usage surfaces: Web chat/debug UI, Cobra CLI, and local `.runtime/` artifacts. The default model provider is `ark` (ByteDance); use `mock` for reproducible local testing.

## Common Commands

```bash
# Development
make dev                          # Start API (:8088) + Vite dev server (:5173)
make dev PROVIDER=mock            # Same, with mock provider
make serve                        # Backend only
make web-dev                      # Frontend only

# CLI
make chat                         # Interactive multi-turn chat
make chat PROVIDER=mock           # Chat with mock provider
make run ARGS='instruction'       # Single execution
make run SESSION=<id> ARGS='...'  # Continue existing session

# Build & Test
make build                        # Build all Go packages
go test ./...                     # Run all tests
make verify-scenarios             # Scenario regression tests (TestScenarioRegression)
make web-build                    # Build frontend

# Targeted testing by area
go test ./internal/agent ./internal/service          # Agent/service changes
go test ./internal/memory ./internal/store/... ./internal/context ./internal/prompt  # Memory/store/context
go test ./internal/tool/... ./internal/service       # Tool changes
go test ./internal/interfaces/... ./cmd/...          # CLI/HTTP changes

# Debugging
make inspect RUN=<id>             # View run state
make replay RUN=<id>              # Replay run events
make debug-events RUN=<id>        # View event stream
make session-inspect SESSION=<id> # View session
```

Note: The Makefile uses local cache dirs (`.gocache`, `.gomodcache`) via `GOMODCACHE`/`GOCACHE` env vars.

## Architecture

### Execution flow

Entry points (`cmd/cli`, `cmd/web`) → Service layer (`internal/service/`) → Agent executor (`internal/agent/`) → Tools/Model

### Key boundaries

- **`internal/service/`** — Orchestration layer. Exposes run/session/inspect/replay/resume/tools. Calls agent via `agent.Runner` interface; never leaks executor internals.
- **`internal/agent/`** — Execution engine: run loop (`loop.go`), action dispatch (`dispatch.go`), delegation handling. Does not depend back on service. Runtime policies (delegation, replan, retrieval) plug in here.
- **`internal/model/`** — Model abstraction. `ark.Provider` (ByteDance API) and `mock.Provider` (deterministic testing). Parses model output into `Action` types (tool calls, answers, delegation, todos).
- **`internal/store/`** — Storage interfaces (`EventStore`, `StateStore`). `internal/store/filesystem/` is the concrete implementation writing to `.runtime/`.
- **`internal/interfaces/`** — Adapter layer only (CLI, HTTP, AG-UI/SSE). Not business logic.

### Domain modules

- `internal/planner/` — Plan generation and replanning strategies
- `internal/delegation/` — Subagent delegation and child result composition
- `internal/retrieval/` — Retrieval progress tracking and forced-final enforcement
- `internal/context/` — Context construction and compression
- `internal/memory/` — Memory persistence and recall
- `internal/prompt/` — Prompt templates and builder
- `internal/runtime/` — Core types (`Task`, `Session`, `Run`), status enums, sequence cursor, policies
- `internal/skill/` — Skill registry
- `internal/tool/` — Tool implementations (filesystem, bash, web/Tavily)

### Frontend

`web/` — React 19 + TypeScript + Vite + Tailwind CSS. Communicates with backend via AG-UI SSE endpoint (`POST /api/agui/chat`).

### Observable execution

`RunObserver` interface streams events in real-time. Events are persisted to store AND broadcast to observer (observer is non-blocking).

## Development Conventions

- Service and agent cooperate through narrow interfaces; don't leak executor details into service or vice versa.
- Both service and agent depend on store interfaces, not the filesystem implementation directly.
- Executor dependencies are grouped by responsibility domain — don't flatten them back.
- `internal/interfaces/` stays as adapter layer, not business core.
- Prefer small, verifiable changes; don't rewrite multiple orthogonal modules simultaneously.
- Changes affecting CLI commands, HTTP routes, SSE/AG-UI events, `.runtime/` artifact format, or OpenSpec main specs must be checked for consistency across all surfaces.
- Scenario test data lives in `testdata/scenarios/` as JSON files.

## Configuration

- `config.json` (from `config.example.json`) — main config, git-ignored
- `.env` (from `.env.example`) — environment variables (`ARK_API_KEY`, `TAVILY_API_KEY`), git-ignored
- Config supports `${VAR_NAME}` env var substitution

## Generated artifacts (do not commit)

`.runtime/`, `bin/`, `log/`, `web/node_modules/`, `web/dist/`, `.gocache/`, `.gomodcache/`, `.tmp/`
