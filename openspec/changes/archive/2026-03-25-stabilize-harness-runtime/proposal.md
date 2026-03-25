## Why

当前 Harness 已经具备本地可运行的 MVP：主 Agent loop、文件系统工具、多轮 chat、child run delegation、事件落盘与 `resume` 基础能力都已经到位。但项目现在更突出的风险已经不是“缺功能”，而是“真实运行时是否足够稳定、是否容易验证、是否便于继续迭代”。从现有 `.runtime/` 工件可以看到，真实 provider 路径仍可能留下未收口的 `running` run；同时，当前缺少一组固定的场景化回归入口，调试输出和技术文档也还没有完全跟上实现状态。如果不先收口这些问题，后续继续增加工具、协议或入口层集成时，维护成本和回归风险都会明显上升。

## What Changes

- 收口真实模型调用、工具执行和内部异常场景下的 `Run` 终态与失败原因记录，减少遗留中间态运行。
- 强化 `resume` 的恢复规则和恢复测试，使中断运行的恢复行为可预测、可验证。
- 固化基础规划、文件系统工具、多轮 chat、delegation 四类回归场景，并提供统一验证入口。
- 增强 session 维度的可观察性，以及 `inspect`、`replay`、`debug events` 的调试体验。
- 更新 README 和技术方案文档，使文档与当前实现状态一致，并明确 `.runtime/` 工件职责。
- 为下一轮 planner/replan、工具权限和服务化边界整理更清晰的演进基础，但不在本变更中直接引入 HTTP、MCP 或新协议集成。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `harness-runtime-core`: 强化运行终态收口、恢复规则、运行工件说明以及 inspect/replay/debug 的可观测性体验。
- `planning-context-memory`: 为后续更清晰的 `plan.updated` 触发规则和恢复后的上下文连续性整理基础。
- `subagent-delegation`: 改善 child run 结果在调试输出中的呈现，并为 `needs_replan` 触发主运行后续动作提供更清晰的演进方向。
- `multi-turn-chat`: 增强 session 级历史查看和多轮会话排障体验。

## Impact

- 主要影响 `internal/app`、`internal/cli`、`internal/model/ark`、`internal/store/filesystem`、`README.md` 和 `docs/step1/TECHNICAL_SOLUTION.md`。
- 需要补充更多异常路径和恢复路径测试，并增加一组面向回归验收的固定场景。
- 不改变当前单机本地、文件型工件和 event-first 的整体架构，不引入新的远程入口或持久化后端。
- 这次变更的核心收益是提升运行稳定性、回归效率和后续扩展时的可维护性，而不是扩大产品能力边界。
