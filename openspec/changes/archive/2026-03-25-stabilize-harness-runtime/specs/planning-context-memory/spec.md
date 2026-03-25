## MODIFIED Requirements

### Requirement: 内置规划能力
系统 MUST 为主 Agent 提供内置的 Planning 能力，用于在执行开始时生成结构化计划，并在必要时支持重规划。

#### Scenario: 子任务结果请求重规划
- **WHEN** child `Run` 返回结构化结果，并显式标记 `needs_replan=true`
- **THEN** 系统 MUST 基于当前 `Plan` 触发一次重规划
- **THEN** 系统 MUST 更新 `Plan` 的版本信息
- **THEN** 系统 MUST 记录 `plan.updated` 事件

#### Scenario: 缺少结构化信号时不触发重规划
- **WHEN** child `Run` 返回 `needs_replan=true`，但结果中没有可消费的摘要、发现、风险或建议
- **THEN** 系统 MUST 不触发 `plan.updated`
- **THEN** 系统 MUST 继续使用当前 `Plan`
