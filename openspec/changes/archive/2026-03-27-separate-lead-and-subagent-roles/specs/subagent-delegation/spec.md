## MODIFIED Requirements

### Requirement: 受控的子代理委派
系统 MUST 支持主 `Run` 将特定子任务以 child run 的形式委派给子代理执行，并保持父子运行关系可追踪。被委派的 child `Run` MUST 以 `subagent` 角色运行，而主 `Run` MUST 保持 `lead-agent` 角色并继续对最终答案负责。

#### Scenario: 为可委派步骤创建 child run
- **WHEN** 主 `Run` 识别当前 `PlanStep` 可委派且满足委派策略
- **THEN** 系统 MUST 创建一个新的 child `Run`
- **THEN** 该 child `Run` MUST 记录父 `Run` 标识
- **THEN** 该 child `Run` MUST 以 `subagent` 角色执行
- **THEN** 主 `Run` MUST 保持 `lead-agent` 角色，不将最终用户答复责任转移给 child `Run`
- **THEN** 系统 MUST 记录 `subagent.spawned` 事件

### Requirement: 子代理结果摘要
系统 MUST 要求 child `Run` 返回结构化摘要，以供主 `Run` 汇总和继续执行。该结构化摘要 MUST 服务于 `lead-agent` 的结果整合，而不是替代主运行向用户给出最终答复。

#### Scenario: 子代理完成后返回摘要
- **WHEN** child `Run` 正常完成
- **THEN** 系统 MUST 产出至少包含完成内容、产物、风险和是否建议重规划的摘要结果
- **THEN** 该结果 MUST 被视为提供给主 `Run` 的证据，而不是直接面向用户的最终答案
- **THEN** 系统 MUST 记录 `subagent.completed` 事件

#### Scenario: 子代理结果字段完整
- **WHEN** child `Run` 返回结构化结果
- **THEN** 结果 MUST 包含 `summary` 字段
- **THEN** 结果 MUST 包含 `needs_replan` 字段
- **THEN** 结果 MUST 包含 `artifacts`、`findings`、`risks`、`recommendations` 数组字段，即使这些数组为空
- **THEN** 系统 MUST 不接受仅包含自由文本最终答复且缺少结构化字段的 child 结果

## ADDED Requirements

### Requirement: 子代理角色约束必须被显式注入
系统 MUST 在 delegated child run 的 prompt 构建中显式注入 `subagent` 角色约束，而不能只依赖零散的补充 constraint 文本来约束 child 行为。

#### Scenario: 子代理 prompt 显式声明职责边界
- **WHEN** 系统为 child `Run` 构建模型调用 prompt
- **THEN** prompt MUST 明确 child `Run` 不直接面向最终用户
- **THEN** prompt MUST 明确 child `Run` 只处理当前被委派的单一子任务
- **THEN** prompt MUST 明确 child `Run` 需要返回结构化结果

#### Scenario: 子代理不得扩张任务范围
- **WHEN** child `Run` 收到一个受限 delegation task
- **THEN** 系统 MUST 通过 `subagent` 角色约束将其执行范围限制在该 task 的目标、步骤、约束和允许工具内
- **THEN** child `Run` MUST 不被鼓励自行扩张为新的开放式任务

### Requirement: 子代理只接收 task-scoped 输入
系统 MUST 让 child `Run` 以委派任务为主输入，而不是让其重新消费主运行的用户对话与父任务语义。

#### Scenario: 子代理只接收委派任务核心字段
- **WHEN** 系统为一次 child `Run` 构建输入
- **THEN** 输入 MUST 至少围绕 `goal`、`allowed_tools`、`constraints` 与 `completion_criteria`
- **THEN** 系统 MAY 提供少量与当前子任务直接相关的 task-local context

#### Scenario: 子代理不接收父运行会话与父目标
- **WHEN** 系统为一次 child `Run` 构建输入
- **THEN** 系统 MUST 不默认传递主 `Run` 的完整 `Conversation History`
- **THEN** 系统 MUST 不默认传递 `parent_goal`
- **THEN** 系统 MUST 不将用户原始多轮问题直接作为 child `Run` 的主要 instruction

### Requirement: 子代理不得继续派生子代理
系统 MUST 将 delegation 限制为单层：`subagent` 只能完成被委派子任务并返回结构化结果，不能继续创建新的 subagent。

#### Scenario: 子代理遇到阻塞时返回结构化阻塞信息
- **WHEN** child `Run` 无法在当前约束和允许工具内完成委派任务
- **THEN** 系统 MUST 要求其在 `risks`、`recommendations` 或 `needs_replan` 中返回阻塞信息
- **THEN** 系统 MUST 不将该阻塞自动转化为进一步的 child delegation

#### Scenario: 运行时拒绝子代理继续委派
- **WHEN** child `Run` 以 `subagent` 角色执行时试图产生新的 delegation 决策
- **THEN** 系统 MUST 拒绝该 delegation
- **THEN** 系统 MUST 记录可用于调试的失败原因或拒绝原因
