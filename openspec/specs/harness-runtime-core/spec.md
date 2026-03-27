# harness-runtime-core

## Purpose
定义 Harness 平台的核心运行时模型、生命周期、事件契约、文件型工件存储，以及基于 Cobra 的首个 CLI 入口。

## Requirements

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

#### Scenario: 在已有 session 中创建新的运行
- **WHEN** 用户显式指定一个已有 `session_id` 发起新的输入
- **THEN** 系统 MUST 复用该 `Session`
- **THEN** 系统 MUST 创建新的 `Task`
- **THEN** 系统 MUST 创建新的 `Run`
- **THEN** 系统 MUST 保持该 `Run` 与原 `Session` 的关联关系

#### Scenario: 工具运行时收敛为核心工具面
- **WHEN** 系统启动内置工具注册表
- **THEN** 系统 MUST 暴露一组高频核心工具面
- **THEN** 该核心工具面 MUST 包含本地工作区工具、外部检索工具和命令执行工具
- **THEN** 系统 MUST 不要求低频辅助工具存在于默认核心集

#### Scenario: 运行时按激活 skill 收窄工具面
- **WHEN** 一次 `Run` 激活了某个 skill，且该 skill 声明了 `allowed-tools`
- **THEN** 系统 MUST 在该次 `Run` 中根据 skill 约束收窄可用工具集
- **THEN** 系统 MUST 不影响其它未激活该 skill 的运行

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
- **THEN** 系统 MUST 为失败事件包含结构化失败类别和是否可重试的信息
- **THEN** 系统 MUST 记录 `run.status_changed` 事件
- **THEN** 系统 MUST 记录 `run.failed` 事件

#### Scenario: 可观察执行入口驱动 run 生命周期
- **WHEN** 系统通过面向实时消费的可观察执行入口启动 `Run`
- **THEN** 系统 MUST 与现有同步执行路径保持相同的运行状态转换
- **THEN** 系统 MUST 向实时观察者暴露生命周期事件和最终终态

### Requirement: 事件优先的执行轨迹
系统 MUST 将执行过程中的关键行为记录为结构化事件，并以追加方式保存为事件流。

#### Scenario: 工具执行产生事件
- **WHEN** Agent 调用任意一个工具
- **THEN** 系统 MUST 在工具执行前记录 `tool.called` 事件
- **THEN** 系统 MUST 在工具成功后记录 `tool.succeeded` 事件，或在失败后记录 `tool.failed` 事件

#### Scenario: 事件持久化同时对实时观察者广播
- **WHEN** 运行时追加任意关键事件到持久化事件流
- **THEN** 系统 MUST 保持原有事件持久化行为不变
- **THEN** 系统 MUST 允许将同一事件广播给一个可选的实时观察者
- **THEN** 实时观察者机制 MUST 不成为持久化事件写入成功的前置条件

#### Scenario: 运行时记录 skill 激活
- **WHEN** 某次 `Run` 激活了一个 skill
- **THEN** 系统 MUST 记录该 skill 的名称或等效结构化标识
- **THEN** 该记录 MUST 能在 inspect、replay 或调试路径中被查看

### Requirement: 标准运行时事件契约
系统 MUST 为运行时关键阶段使用固定的标准事件名称，以保证 inspect、replay 和后续入口扩展的一致性。

#### Scenario: 生成结果时记录标准事件
- **WHEN** 一次 `Run` 生成最终结果
- **THEN** 系统 MUST 记录 `result.generated` 事件
- **THEN** 系统 MUST 在结果持久化后记录 `run.completed` 事件

#### Scenario: 记录会话消息事件
- **WHEN** 一轮对话中产生用户输入和助手回复
- **THEN** 系统 MUST 在对应 `Run` 的事件流中记录 `user.message` 事件
- **THEN** 系统 MUST 在对应 `Run` 的事件流中记录 `assistant.message` 事件

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

#### Scenario: CLI 由独立二进制入口启动
- **WHEN** 用户构建或启动 CLI 程序
- **THEN** 系统 MUST 通过独立的 `cmd/cli` 入口启动 Cobra root command
- **THEN** CLI 入口 MUST 只负责初始化配置和命令装配，而不直接承载核心业务逻辑

#### Scenario: 使用 CLI 启动运行
- **WHEN** 用户执行 `harness run`
- **THEN** 系统 MUST 通过应用层创建并启动新的 `Run`
- **THEN** CLI MUST 输出该次运行的标识信息或结果摘要

#### Scenario: 使用 CLI 查看运行
- **WHEN** 用户执行 `harness inspect <run-id>`
- **THEN** 系统 MUST 读取并展示指定 `Run` 的当前状态、计划摘要或最终结果

#### Scenario: 使用 chat 命令进行多轮对话
- **WHEN** 用户执行 `harness chat`
- **THEN** 系统 MUST 进入交互式多轮对话模式
- **THEN** 系统 MUST 在会话结束前持续复用同一个 `Session`

### Requirement: 事件回放与恢复能力
系统 MUST 支持基于持久化工件进行事件回放和运行恢复。

#### Scenario: 回放运行轨迹
- **WHEN** 用户执行 `harness replay <run-id>`
- **THEN** 系统 MUST 仅读取对应 `Run` 的 `events.jsonl`
- **THEN** 系统 MUST 按事件顺序输出执行轨迹

#### Scenario: 查看原始事件流
- **WHEN** 用户执行 `harness debug events <run-id>`
- **THEN** 系统 MUST 返回原始事件记录
- **THEN** 系统 MUST 不将原始事件替换为摘要文案

#### Scenario: 查看运行摘要时间线
- **WHEN** 用户执行 `harness replay <run-id>`
- **THEN** 系统 MUST 返回按事件顺序组织的摘要时间线
- **THEN** 系统 MUST 保留阶段信息，帮助定位计划、模型、工具和子运行行为

#### Scenario: 恢复未完成运行
- **WHEN** 用户执行 `harness resume <run-id>` 且目标 `Run` 尚未完成
- **THEN** 系统 MUST 从该 `Run` 的 `run.json`、`state.json` 和 `plan.json` 恢复上下文
- **THEN** 系统 MUST 继续执行该 `Run`

#### Scenario: 恢复工具后续阶段
- **WHEN** 某个 `Run` 已经成功执行工具并持久化了工具结果，但在生成最终答案前中断
- **THEN** 系统 MUST 基于 `state.json` 中的续跑状态恢复执行
- **THEN** 系统 MUST 不重复执行已经成功的工具调用

#### Scenario: 拒绝恢复不可自动恢复的运行
- **WHEN** 用户执行 `harness resume <run-id>` 且目标 `Run` 已处于 `blocked`、终态或已经存在持久化结果
- **THEN** 系统 MUST 拒绝自动恢复
- **THEN** 系统 MUST 返回清晰说明，指出该 `Run` 需要人工处理或已经结束

### Requirement: 本地 HTTP API 入口
系统 MUST 提供一个面向本机开发的 HTTP API 入口，用于复用现有应用层服务，而不改变核心运行时模型。

#### Scenario: Web 服务由独立二进制入口启动
- **WHEN** 用户构建或启动本地 Web 服务
- **THEN** 系统 MUST 通过独立的 `cmd/web` 入口启动 HTTP server
- **THEN** Web 入口 MUST 直接复用现有应用层服务与 HTTP 适配层
- **THEN** 系统 MUST 不要求通过 CLI 子命令才能启动本地 Web 服务

#### Scenario: 启动本地 API 服务
- **WHEN** 用户执行本地 server 启动命令
- **THEN** 系统 MUST 启动一个基于 `chi` 的 HTTP 服务
- **THEN** 该服务 MUST 复用应用层 `Services`，而不是重新实现运行时逻辑

### Requirement: 运行时 HTTP 查询接口
系统 MUST 通过 HTTP API 暴露当前前端所需的运行时查询能力。

#### Scenario: 查询 run 详情
- **WHEN** 客户端请求某个 `Run` 的详情
- **THEN** 系统 MUST 返回 run、plan、state、result、current step 和 child run 摘要

#### Scenario: 查询摘要时间线和原始事件
- **WHEN** 客户端请求某个 `Run` 的 replay 或 events 数据
- **THEN** 系统 MUST 提供摘要时间线接口
- **THEN** 系统 MUST 提供原始事件接口

#### Scenario: 查询工具列表时暴露扩展访问级别
- **WHEN** 客户端请求工具列表
- **THEN** 系统 MUST 返回所有已注册工具的结构化描述
- **THEN** 每个工具描述 MUST 暴露访问级别
- **THEN** 访问级别 MUST 至少支持 `read_only`、`write` 和 `exec`

### Requirement: 运行时 HTTP 操作接口
系统 MUST 通过 HTTP API 暴露创建 run、恢复 run 和创建 session 的能力。

#### Scenario: 通过 HTTP 发起新的 run
- **WHEN** 客户端通过 HTTP 提交运行请求
- **THEN** 系统 MUST 创建并执行新的 `Run`
- **THEN** 系统 MUST 返回该次运行的结构化响应

#### Scenario: 通过 HTTP 恢复运行
- **WHEN** 客户端请求恢复某个未完成 `Run`
- **THEN** 系统 MUST 复用现有恢复逻辑
- **THEN** 系统 MUST 返回结构化恢复结果或清晰错误

### Requirement: 运行时显式区分 lead-agent 与 subagent 角色
系统 MUST 在运行时 prompt 构建与执行过程中显式区分 `lead-agent` 和 `subagent` 两种角色，而不是让所有运行默认复用同一套 agent 语义。

#### Scenario: 主运行使用 lead-agent 角色
- **WHEN** 系统为一次面向用户的主 `Run` 构建首次模型调用 prompt
- **THEN** 系统 MUST 使用 `lead-agent` 角色模板
- **THEN** 该角色模板 MUST 明确主 `Run` 对规划、委派决策和最终用户答案负责

#### Scenario: child run 使用 subagent 角色
- **WHEN** 系统为一次 delegated child `Run` 构建首次模型调用 prompt
- **THEN** 系统 MUST 使用 `subagent` 角色模板
- **THEN** 该角色模板 MUST 明确 child `Run` 不直接面向用户
- **THEN** 该角色模板 MUST 明确 child `Run` 只处理被委派的单一子任务

### Requirement: 角色语义在同一运行的 follow-up 调用中保持一致
系统 MUST 在同一 `Run` 的后续模型调用中保持既定角色语义，避免一次运行在工具调用或子结果合并后切换角色。

#### Scenario: 主运行在工具后续推理中保持 lead-agent 语义
- **WHEN** 主 `Run` 在工具调用成功后继续进行 `post_tool` 推理
- **THEN** 系统 MUST 继续使用 `lead-agent` 角色语义
- **THEN** 系统 MUST 不将主 `Run` 降级为仅做局部摘要的 worker 角色

#### Scenario: child run 在工具后续推理中保持 subagent 语义
- **WHEN** child `Run` 在工具调用成功后继续进行 `post_tool` 推理
- **THEN** 系统 MUST 继续使用 `subagent` 角色语义
- **THEN** 系统 MUST 不允许 child `Run` 在后续推理中转变为直接面向用户的最终答复角色

### Requirement: 运行时与 inspect 路径可识别运行角色
系统 MUST 让主运行与 child run 的角色标识可在运行时调试与 inspect 路径中被识别，以便通过 CLI 验证 lead-agent / subagent 分层是否生效。

#### Scenario: 运行时记录主运行角色
- **WHEN** 系统创建并启动一次主 `Run`
- **THEN** 系统 MUST 为该 `Run` 记录可区分为 `lead-agent` 的结构化角色标识

#### Scenario: inspect 能查看 child run 角色
- **WHEN** 用户通过 inspect 或等效调试路径查看某个包含 child run 的主 `Run`
- **THEN** 系统 MUST 能区分主 `Run` 与 child `Run` 的角色
- **THEN** 角色信息 MUST 可用于 CLI 验证 lead-agent 和 subagent 的职责边界

### Requirement: delegation 权限必须与运行角色绑定
系统 MUST 将 delegation 权限绑定到运行角色：只有 `lead-agent` 可以发起委派，`subagent` 不得继续创建新的 child run。

#### Scenario: lead-agent 允许发起委派
- **WHEN** 一次主 `Run` 以 `lead-agent` 角色执行并判断当前子任务适合委派
- **THEN** 系统 MAY 接受该运行产生的 `delegate` 决策
- **THEN** 系统 MUST 将该委派转化为新的 child `Run`

#### Scenario: subagent 不允许继续委派
- **WHEN** 一次 child `Run` 以 `subagent` 角色执行
- **THEN** 系统 MUST 不允许该运行通过模型输出或运行时决策继续创建新的 child `Run`
- **THEN** 如该运行无法完成任务，系统 MUST 要求其通过结构化结果返回阻塞信息，而不是继续 delegation

### Requirement: child run 的输入必须是 task-scoped 的
系统 MUST 将 child run 的模型输入限制为委派任务本身及最小必要的 task-local context，而不能默认继承主运行的完整对话和父目标信息。

#### Scenario: child run 不继承完整会话历史
- **WHEN** 系统为一次 `subagent` child `Run` 构建模型输入
- **THEN** 系统 MUST 不默认注入主 `Run` 的完整 `Conversation History`
- **THEN** 系统 MUST 不把主 `Run` 的多轮用户对话作为 child 的主要输入来源

#### Scenario: child run 不继承 parent goal
- **WHEN** 系统为一次 `subagent` child `Run` 构建模型输入
- **THEN** 系统 MUST 不默认注入 `parent_goal`
- **THEN** 系统 MUST 不默认注入 `parent_goal` 的摘要版本
- **THEN** child `Run` 的主要输入 MUST 是当前 delegation task 的 `goal`、`allowed_tools`、`constraints` 与 `completion_criteria`
