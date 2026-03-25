## Why

This repository needs a real agent harness foundation rather than a single-purpose demo agent so that future capabilities can be added without rewriting the core execution model. We should establish the platform now because the project is still greenfield and we have already aligned on a self-built Go runtime, Cobra-based CLI, event-first architecture, and extensibility for future protocol, messaging, and scheduling integrations.

## What Changes

- Introduce a Go-based harness platform foundation centered on explicit `Task`, `Run`, `Session`, and `Event` models.
- Add a Cobra-based CLI as the first transport for starting, inspecting, replaying, and resuming harness runs.
- Implement the first version of the agent runtime and loop with structured events, file-backed run state, and a clear application/core boundary.
- Add a built-in planning capability, prompt assembly, context management, compaction flow, and long-term memory hooks so the harness can support longer-running tasks.
- Add a tool runtime with initial file system capabilities for workspace interaction.
- Add controlled sub-agent delegation through child runs, bounded policies, and result merging.
- Establish architecture seams for later integrations such as Hertz HTTP APIs, skills, MCP, ACP, message routing, and task dispatching without including those integrations in this change.

## Capabilities

### New Capabilities
- `harness-runtime-core`: Define the harness runtime foundation, including task/session/run/event models, run lifecycle, event persistence, and the Cobra CLI entrypoints for operating runs.
- `planning-context-memory`: Provide built-in planning, starter prompt composition, context assembly, compaction, and long-term memory recall/write-back for sustained agent execution.
- `filesystem-tools`: Provide the initial tool system and workspace-scoped file system tools needed for reading, writing, listing, searching, and inspecting files safely.
- `subagent-delegation`: Provide controlled delegation through child runs, delegation policies, bounded depth/concurrency, and result collection for plan-driven sub-task execution.

### Modified Capabilities
- None.

## Impact

- Adds the initial Go project structure for a harness platform rather than a simple script-style agent.
- Introduces Cobra as the CLI framework and file-backed runtime artifacts such as run state, plans, summaries, and `events.jsonl`.
- Establishes the core contracts that future HTTP, messaging, protocol, and scheduling systems must integrate with.
- Requires new internal packages for runtime, loop, planner, context, memory, tooling, delegation, store, prompt, and application services.
