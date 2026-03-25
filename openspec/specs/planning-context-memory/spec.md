# planning-context-memory

## Purpose
定义 Harness 平台中的规划、上下文组装、上下文压缩、Starter Prompts 和结构化长期记忆能力。

## Requirements

### Requirement: 内置规划能力
系统 MUST 为主 Agent 提供内置的 Planning 能力，用于在执行开始时生成结构化计划，并在必要时支持重规划。

#### Scenario: 初始计划生成
- **WHEN** 新的 `Run` 开始执行
- **THEN** 系统 MUST 为该 `Run` 生成结构化 `Plan`
- **THEN** `Plan` MUST 包含一个或多个 `PlanStep`
- **THEN** `Plan` MUST 包含版本信息
- **THEN** 系统 MUST 记录 `plan.created` 事件

#### Scenario: 基于新信息重规划
- **WHEN** 执行过程中出现新的约束、阻塞或子任务结果导致原计划不再适用
- **THEN** 系统 MUST 支持更新当前 `Plan`
- **THEN** 系统 MUST 更新 `Plan` 的版本信息
- **THEN** 系统 MUST 记录 `plan.updated` 事件

#### Scenario: 子任务结果请求重规划
- **WHEN** child `Run` 返回结构化结果，并显式标记 `needs_replan=true`
- **THEN** 系统 MUST 基于当前 `Plan` 触发一次重规划
- **THEN** 系统 MUST 更新 `Plan` 的版本信息
- **THEN** 系统 MUST 记录 `plan.updated` 事件

#### Scenario: 缺少结构化信号时不触发重规划
- **WHEN** child `Run` 返回 `needs_replan=true`，但结果中没有可消费的摘要、发现、风险或建议
- **THEN** 系统 MUST 不触发 `plan.updated`
- **THEN** 系统 MUST 继续使用当前 `Plan`

#### Scenario: 计划步骤发生状态变化
- **WHEN** 某个 `PlanStep` 的状态发生变化
- **THEN** 系统 MUST 支持 `pending`、`running`、`completed`、`blocked`、`failed`、`cancelled` 这组步骤状态
- **THEN** 系统 MUST 在步骤进入运行态时记录 `plan.step.started` 事件
- **THEN** 系统 MUST 在步骤完成时记录 `plan.step.completed` 事件
- **THEN** 系统 MUST 在其他状态变化时记录 `plan.step.changed` 事件

### Requirement: 上下文组装
系统 MUST 在每轮模型调用前构造统一的模型上下文，并按优先级组装不同来源的信息。

#### Scenario: 构建模型上下文
- **WHEN** 系统准备调用模型进行下一轮推理
- **THEN** 系统 MUST 组装用户目标或固定约束等 `Pinned Context`
- **THEN** 系统 MUST 组装当前 `Plan` 与活动中的 `PlanStep`
- **THEN** 系统 MUST 组装近期事件和已生成的摘要
- **THEN** 系统 MUST 组装与任务相关的长期记忆召回结果
- **THEN** 系统 MUST 记录 `context.built` 事件

#### Scenario: 构建多轮会话上下文
- **WHEN** 当前 `Run` 属于一个已经存在历史消息的 `Session`
- **THEN** 系统 MUST 将最近的用户/助手消息历史注入模型上下文
- **THEN** 系统 MUST 保持这些消息的时间顺序
- **THEN** 系统 MUST 限制注入消息数量，避免无界增长

### Requirement: 上下文压缩机制
系统 MUST 在上下文预算不足或信息密度过高时触发 compaction，并将压缩结果作为可继续消费的摘要保存。

#### Scenario: 因上下文预算不足触发压缩
- **WHEN** 当前模型上下文接近预设 token 预算上限
- **THEN** 系统 MUST 触发 compaction
- **THEN** 系统 MUST 生成 `step summary` 或 `run summary`
- **THEN** 系统 MUST 记录 `context.compacted` 事件

#### Scenario: 压缩后保留关键上下文
- **WHEN** 系统完成一次 compaction
- **THEN** 系统 MUST 保留固定约束和用户目标等高优先级上下文
- **THEN** 系统 MUST 使用摘要替代被压缩的低优先级历史内容

### Requirement: Starter Prompts 组装
系统 MUST 支持使用预设提示词模板构建运行时 prompt，并将角色、任务和工具规则纳入统一组装流程。

#### Scenario: 生成运行时 Prompt
- **WHEN** 系统为一个新的 `Run` 构建 prompt
- **THEN** 系统 MUST 组合基础提示、角色提示、任务提示和工具使用规则
- **THEN** 系统 MUST 将生成的 prompt 用于后续模型调用
- **THEN** 系统 MUST 记录 `prompt.built` 事件

### Requirement: 结构化长期记忆
系统 MUST 支持结构化的长期记忆存储，并区分偏好、事实、约定和决策等不同记忆类型。

#### Scenario: 运行结束后提取长期记忆候选
- **WHEN** 一个 `Run` 正常完成
- **THEN** 系统 MUST 从本次运行中提取长期记忆候选
- **THEN** 候选记忆 MUST 至少包含类型、内容和来源运行信息
- **THEN** 系统 MUST 记录 `memory.candidate_extracted` 事件

#### Scenario: 提交长期记忆
- **WHEN** 系统决定将记忆候选写入长期记忆
- **THEN** 系统 MUST 持久化该记忆条目
- **THEN** 系统 MUST 记录 `memory.committed` 事件

### Requirement: 按任务召回长期记忆
系统 MUST 在运行过程中按任务相关性召回长期记忆，而不是无差别注入全部历史信息。

#### Scenario: 按目标召回相关记忆
- **WHEN** 一个新的 `Run` 启动或当前 `PlanStep` 发生变化
- **THEN** 系统 MUST 按任务目标、标签或类别筛选相关长期记忆
- **THEN** 系统 MUST 按固定规则对筛选结果进行稳定排序
- **THEN** 系统 MUST 仅将召回结果的一部分注入当前模型上下文
- **THEN** 系统 MUST 记录 `memory.recalled` 事件

#### Scenario: 召回结果限制条数
- **WHEN** 符合条件的长期记忆超过上下文允许的数量
- **THEN** 系统 MUST 仅注入排序后的前 N 条结果
