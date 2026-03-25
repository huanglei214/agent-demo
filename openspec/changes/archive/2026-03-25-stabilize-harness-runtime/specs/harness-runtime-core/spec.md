## MODIFIED Requirements

### Requirement: Run 生命周期状态管理
系统 MUST 管理 `Run` 的生命周期状态，并在状态变化时写入结构化事件。

#### Scenario: Run 失败结束
- **WHEN** 一次 `Run` 因模型调用、工具执行或内部错误而无法继续
- **THEN** 系统 MUST 将该 `Run` 标记为 `failed`
- **THEN** 系统 MUST 记录失败原因
- **THEN** 系统 MUST 为失败事件包含结构化失败类别和是否可重试的信息
- **THEN** 系统 MUST 记录 `run.status_changed` 事件
- **THEN** 系统 MUST 记录 `run.failed` 事件

### Requirement: 事件回放与恢复能力
系统 MUST 支持基于持久化工件进行事件回放和运行恢复。

#### Scenario: 恢复工具后续阶段
- **WHEN** 某个 `Run` 已经成功执行工具并持久化了工具结果，但在生成最终答案前中断
- **THEN** 系统 MUST 基于 `state.json` 中的续跑状态恢复执行
- **THEN** 系统 MUST 不重复执行已经成功的工具调用

#### Scenario: 拒绝恢复不可自动恢复的运行
- **WHEN** 用户执行 `harness resume <run-id>` 且目标 `Run` 已处于 `blocked`、终态或已经存在持久化结果
- **THEN** 系统 MUST 拒绝自动恢复
- **THEN** 系统 MUST 返回清晰说明，指出该 `Run` 需要人工处理或已经结束

## ADDED Requirements

### Requirement: 调试输出分层
系统 MUST 为摘要调试和原始事件排障提供分层的查看能力。

#### Scenario: 查看运行摘要时间线
- **WHEN** 用户执行 `harness replay <run-id>`
- **THEN** 系统 MUST 返回按事件顺序组织的摘要时间线
- **THEN** 系统 MUST 保留阶段信息，帮助定位计划、模型、工具和子运行行为

#### Scenario: 查看原始事件流
- **WHEN** 用户执行 `harness debug events <run-id>`
- **THEN** 系统 MUST 返回原始事件记录
- **THEN** 系统 MUST 不将原始事件替换为摘要文案
