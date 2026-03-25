## ADDED Requirements

### Requirement: 统一的核心领域模型
系统 MUST 定义并持久化 `Task`、`Run`、`Session` 和 `Event` 四类核心对象，并保持它们之间的引用关系清晰可追踪。

#### Scenario: 创建一次新的运行
- **WHEN** 用户通过 CLI 发起一个新的 harness 任务
- **THEN** 系统 MUST 创建一个新的 `Task`
- **THEN** 系统 MUST 创建一个新的 `Session` 或复用显式指定的 `Session`
- **THEN** 系统 MUST 创建一个新的 `Run`
- **THEN** 系统 MUST 记录 `task.created` 事件
- **THEN** 系统 MUST 记录 `session.created` 事件
- **THEN** 系统 MUST 为该 `Run` 记录 `run.created` 事件

### Requirement: Run 生命周期状态管理
系统 MUST 管理 `Run` 的生命周期状态，并在状态变化时写入结构化事件。

#### Scenario: Run 成功完成
- **WHEN** 一次 `Run` 正常结束并产生最终结果
- **THEN** 系统 MUST 将该 `Run` 标记为 `completed`
- **THEN** 系统 MUST 持久化最终结果
- **THEN** 系统 MUST 记录 `run.status_changed` 事件
- **THEN** 系统 MUST 记录 `run.completed` 事件

#### Scenario: Run 失败结束
- **WHEN** 一次 `Run` 因模型调用、工具执行或内部错误而无法继续
- **THEN** 系统 MUST 将该 `Run` 标记为 `failed`
- **THEN** 系统 MUST 记录失败原因
- **THEN** 系统 MUST 记录 `run.status_changed` 事件
- **THEN** 系统 MUST 记录 `run.failed` 事件

### Requirement: 事件优先的执行轨迹
系统 MUST 将执行过程中的关键行为记录为结构化事件，并以追加方式保存为事件流。

#### Scenario: 工具执行产生事件
- **WHEN** Agent 调用任意一个工具
- **THEN** 系统 MUST 在工具执行前记录 `tool.called` 事件
- **THEN** 系统 MUST 在工具成功后记录 `tool.succeeded` 事件，或在失败后记录 `tool.failed` 事件

### Requirement: 标准运行时事件契约
系统 MUST 为运行时关键阶段使用固定的标准事件名称，以保证 inspect、replay 和后续入口扩展的一致性。

#### Scenario: 生成结果时记录标准事件
- **WHEN** 一次 `Run` 生成最终结果
- **THEN** 系统 MUST 记录 `result.generated` 事件
- **THEN** 系统 MUST 在结果持久化后记录 `run.completed` 事件

### Requirement: 文件型运行时存储
系统 MUST 为每个 `Run` 创建独立的文件型运行时目录，并保存运行所需的最小可观测工件。

#### Scenario: Run 工件落盘
- **WHEN** 系统创建一个新的 `Run`
- **THEN** 系统 MUST 为该 `Run` 创建独立目录
- **THEN** 系统 MUST 持久化 `run.json`
- **THEN** 系统 MUST 持久化 `plan.json`
- **THEN** 系统 MUST 持久化 `events.jsonl`
- **THEN** 系统 MUST 在运行过程中维护 `state.json`
- **THEN** 系统 MUST 在运行结束后持久化 `result.json`

### Requirement: Cobra CLI 作为首个入口
系统 MUST 提供基于 Cobra 的 CLI 入口，用于驱动运行时能力，而 CLI 层 MUST 通过应用层访问核心模块。

#### Scenario: 使用 CLI 启动运行
- **WHEN** 用户执行 `harness run`
- **THEN** 系统 MUST 通过应用层创建并启动新的 `Run`
- **THEN** CLI MUST 输出该次运行的标识信息或结果摘要

#### Scenario: 使用 CLI 查看运行
- **WHEN** 用户执行 `harness inspect <run-id>`
- **THEN** 系统 MUST 读取并展示指定 `Run` 的当前状态、计划摘要或最终结果

### Requirement: 事件回放与恢复能力
系统 MUST 支持基于持久化工件进行事件回放和运行恢复。

#### Scenario: 回放运行轨迹
- **WHEN** 用户执行 `harness replay <run-id>`
- **THEN** 系统 MUST 仅读取对应 `Run` 的 `events.jsonl`
- **THEN** 系统 MUST 按事件顺序输出执行轨迹

#### Scenario: 恢复未完成运行
- **WHEN** 用户执行 `harness resume <run-id>` 且目标 `Run` 尚未完成
- **THEN** 系统 MUST 从该 `Run` 的 `run.json`、`state.json` 和 `plan.json` 恢复上下文
- **THEN** 系统 MUST 继续执行该 `Run`
