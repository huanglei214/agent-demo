## Context

当前系统已经具备 delegated child run 的执行能力，并且在 `subagent-delegation` 能力下支持受控工具面、结构化 child result 与 `needs_replan` 标记。但在运行时实现上，主 run 和 child run 仍主要共用同一套默认 agent prompt 与执行语义，child run 更多是“带附加约束的默认 agent”，而不是一个显式建模的 `subagent` 角色。

这种状态带来几个问题：
- 用户入口、计划更新、工具调用、child run 收口都混在一套角色语义里，不利于稳定地推导“谁对最终答案负责”。
- child run 的行为边界更多依赖补充约束字符串，而不是通过明确角色模板来收紧。
- lead 与 subagent 的职责没有在运行时和 inspect 语义上被清晰表达，不利于 CLI 验证和后续演进。

本次 change 只覆盖后端和 CLI 验证链路，不涉及 Web UI。目标不是引入复杂多代理框架，而是把当前已有 delegation 基础收敛成更清晰的 `lead-agent` / `subagent` 角色模型。

## Goals / Non-Goals

**Goals:**
- 在运行时 prompt 构建层显式区分 `lead-agent` 和 `subagent` 两种角色。
- 让主 run 固定使用 `lead-agent` 角色，并保持其对用户答案、规划、re-plan 和委派决策的最终责任。
- 让 delegated child run 固定使用 `subagent` 角色，并通过角色模板进一步收紧范围、输出和工具使用语义。
- 收紧 delegation 输入输出契约，让 `lead-agent` 更稳定地消费 child result，并更明确地处理 `needs_replan`。
- 通过 CLI 和现有 inspect/replay/debug-events 链路验证角色分层效果。

**Non-Goals:**
- 不改 Web UI，不增加新的 agent 可视化界面。
- 不在本次引入多种 subagent 类型（如 research/code/review 等）。
- 不引入新的外部依赖或复杂 agent marketplace / skill marketplace。
- 不把当前 delegation 演进成真正并行、多层级的多代理编排系统。

## Decisions

### 1. 将 `lead-agent` 设为唯一用户面角色

主 run 在 prompt 构建时始终使用 `lead-agent` 角色模板。`lead-agent` 负责：
- 理解用户目标
- 决定是直接回答、调用工具，还是委派 child run
- 更新或触发 re-plan
- 合并工具结果和 subagent 结果
- 输出最终用户答案

这样可以把“最终谁负责”明确固定在 lead 上，避免 child run 产出的文本直接变成用户答案的语义混乱。

选择该方案而不是继续沿用单一默认角色的原因：
- 更容易在 prompt 层表达职责边界
- 更容易在 inspect/replay 中解释执行行为
- 与当前已有的 child run 结构兼容，不需要引入新的执行框架

### 2. delegated child run 固定使用 `subagent` 角色模板

child run 不再复用默认 agent 角色，而是在 prompt 构建时显式使用 `subagent` 模板。该模板强调：
- 只处理当前委派目标
- 不直接面向用户
- 不扩张任务范围
- 不负责长期记忆写入
- 只返回结构化结果，而不是自由形式的最终答复

选择该方案而不是继续通过 `constraints` 文本补丁约束 child run 的原因：
- 角色约束更稳定，不易在 prompt 组合时被稀释
- 更利于后续扩展不同 subagent 角色类型
- 更容易对 child run 输出进行一致性验证

### 3. 保留现有 delegation 机制，但收紧输入与输出契约

本次不重写 delegation manager，也不引入新的 child execution pipeline，而是在现有基础上收紧契约：
- 输入侧：明确 child run 是一个 `subagent`，且其输入必须是 task-scoped 的委派任务，而不是主运行的用户对话副本。委派上下文中的目标、步骤、约束、allowlist 必须服务于“单一受限子任务”。
- 输出侧：继续使用结构化 `DelegationResult`，并进一步强调固定字段语义，例如：
  - `summary`：一句话结论
  - `findings`：关键发现
  - `risks`：已知风险或不确定性
  - `recommendations`：建议下一步
  - `artifacts`：可引用结果
  - `needs_replan`：是否要求 lead 调整计划

选择该方案而不是新建另一套 subagent 结果结构的原因：
- 可以复用现有 runtime types 和 inspect 逻辑
- 改动范围更小，适合先用 CLI 验证
- 保持与现有 `subagent-delegation` spec 的连续性

进一步的输入边界约束如下：
- child run 默认不继承完整 `Conversation History`
- child run 默认不继承 `parent_goal` 或 `parent_goal` 摘要
- child run 默认不继承 lead 的完整 `Plan Context`、`Recent Events`、`Pinned Context`
- child run 只接收：
  - `goal`
  - `allowed_tools`
  - `constraints`
  - `completion_criteria`
  - 少量与当前 task 直接相关的 task-local context

这样做的原因是：subagent 不负责重新理解用户目标，而只负责执行 lead 已裁剪好的委派任务。

### 4. delegation 只允许单层，只有 lead-agent 可以产生 subagent

本次明确将 delegation 约束为单层编排：
- 只有 `lead-agent` 可以产生 `delegate` 决策并创建 child run
- `subagent` 只能在允许工具面内执行局部工作并返回结构化结果
- `subagent` 不得继续创建新的 subagent，也不得把阻塞情况转化为进一步 delegation

当 `subagent` 无法完成任务时，应通过结构化结果中的 `risks`、`recommendations` 和 `needs_replan` 将阻塞反馈给 `lead-agent`，由 `lead-agent` 决定是否调整计划、补充工具调用或向用户收口。

选择该方案而不是允许递归委派的原因：
- 避免 child run 无约束扩张成多层执行树
- 保持“只有 lead 负责编排和最终决策”的职责边界
- 便于 CLI 验证和 inspect/replay 调试
- 与当前单机 MVP 范围更匹配，不提前引入复杂的多层级代理治理问题

### 5. prompt builder 增加角色分支，而不是复制两套完全独立的构建流程

运行时仍复用当前 prompt builder 主流程，但在模板选择和角色附加指令上分支：
- 主 run：`lead-agent`
- child run：`subagent`
- post-tool / post-delegation follow-up 仍继承当前 run 的角色

选择该方案而不是把 lead/subagent 分裂成两套完全独立 builder 的原因：
- 可以最大程度复用现有 prompt 拼装逻辑
- 可以减少上下文装配层的重复
- 后续若要再引入 `research-subagent` 等类型，也更容易通过模板扩展

### 6. 先用 CLI 做验证，不在本次 change 触碰 UI

本次验证以 CLI 为主，依赖：
- `make run`
- `make chat`
- `make inspect`
- `make replay`
- `make debug-events`

选择该方案而不是同步修改 UI 的原因：
- 当前问题本质是运行时角色边界，不是展示问题
- 可以先验证行为是否变稳，再决定 UI 如何表达 lead/subagent 关系
- 与当前用户约束一致：这轮改动先不动 UI

## Risks / Trade-offs

- [Risk] lead/subagent prompt 语义分层后，可能让原有 prompt 行为发生回归  
  → Mitigation：为 delegation 流程补 targeted tests，并用 CLI 真实回放检查 child run 是否仍按预期返回结构化结果。

- [Risk] child run 约束更强后，某些此前“凑巧可用”的自由回答路径会被压缩，短期内可能降低部分复杂任务的完成率  
  → Mitigation：先保持 `DelegationResult` 结构不变，只收紧角色模板与 contract 语义，不额外增加复杂字段。

- [Risk] 如果 child run 仍继承主运行的完整会话和父目标信息，subagent 可能重新站回“理解用户问题”的位置，继续产出 delegation 或自由答复  
  → Mitigation：将 child 输入改为 task-scoped 模型，默认移除 `Conversation History`、`parent_goal` 与大部分父运行上下文，只保留委派任务本身及最小必要 task-local context。

- [Risk] lead 过度保守，减少委派次数，可能使某些高价值 delegation 场景变慢  
  → Mitigation：本次只明确角色，不强行调整 delegation 触发策略；委派策略优化留到后续 change。

- [Risk] 如果运行时没有对 child run 的 delegation 做硬校验，模型仍可能在 prompt 漂移下产出递归 delegation  
  → Mitigation：除了在 `subagent` prompt 中写明禁止继续委派，还要在 runtime 层做硬校验，发现 child run 试图 delegation 时直接拒绝并记录原因。

- [Risk] inspect/replay 中虽然能看到 child run，但如果没有角色标识，CLI 验证时仍不够直观  
  → Mitigation：在运行时 metadata 或 inspect 输出里补充角色摘要，便于验证主 run 与 child run 的角色差异。

## Migration Plan

1. 先在 `proposal` 对应的 capability 范围内补 `specs`，明确 `harness-runtime-core` 与 `subagent-delegation` 的 requirement 变化。
2. 在 `internal/prompt/` 中拆分 `lead-agent` / `subagent` 模板与角色分支。
3. 在 `internal/app/` 与 `internal/delegation/` 中让 child run 固定走 `subagent` 角色，并收紧 delegation contract。
4. 通过 CLI 和 targeted tests 验证：
   - 主 run 仍由 lead 收口
   - child run 使用 subagent 语义
   - child result 结构稳定
5. 如验证通过，再同步 specs 并归档本次 change。

本次 change 不涉及数据迁移，也不涉及对现有 `.runtime/` 工件格式的破坏性变更；旧 run 仍可照常 inspect/replay。

## Open Questions

- 是否需要在 `DelegationTask` 中补充显式 `role` 字段，还是先通过 child run metadata 隐式标记即可？
- 是否需要在 `InspectRun` 输出中增加更明确的 `lead` / `subagent` 标识，帮助 CLI 验证？
- lead 在消费 child result 时，是否需要增加一层固定摘要提示，进一步降低自由文本漂移？
