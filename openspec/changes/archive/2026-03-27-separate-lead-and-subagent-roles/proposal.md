## Why

当前运行时已经支持委派 child run，但系统整体仍然更像“一个默认 agent 偶尔拉起助手”，而不是明确区分用户侧主控角色与受限子代理角色。这会让用户入口、委派策略和 child run 行为共享过多的 prompt 与执行语义，导致 delegation 的边界不够清晰，也不利于后续验证和演进。

## What Changes

- 将 agent 角色明确拆分为 `lead-agent` 和 `subagent` 两类运行时角色，并用于 prompt 构建与 delegation 流程。
- 让 `lead-agent` 成为唯一面向用户的角色，负责规划、是否委派的决策以及最终答案的收口。
- 让 delegated child run 固定使用专门的 `subagent` 角色，并施加更严格的 prompt 约束与结构化结果要求。
- 收紧 delegation 的输入与输出契约，使 `lead-agent` 能更稳定地消费 `subagent` 结果，并更明确地决定是否需要 re-plan。
- 本次 change 仅覆盖后端与 CLI 验证链路，不包含 Web UI 行为调整。

## Capabilities

### New Capabilities
<!-- 无 -->

### Modified Capabilities
- `harness-runtime-core`：运行时 prompt 构建与 run 行为需要显式区分 `lead-agent` 和 `subagent` 两种角色。
- `subagent-delegation`：delegated child run 需要遵循更明确的 `subagent` 角色约束，并向 `lead-agent` 返回更清晰的结构化结果。

## Impact

- 影响 `internal/prompt/`、`internal/app/`、`internal/delegation/` 以及相关 runtime types。
- 影响通过 CLI 驱动的 delegation 流验证与 run inspect 输出。
- 本次 change 预计不引入新的外部依赖，也不涉及 Web UI 变更。
