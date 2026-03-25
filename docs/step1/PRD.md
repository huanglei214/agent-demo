# Agent Harness Platform PRD

## 1. Executive Summary

- **Problem Statement**: 当前仓库缺少一个清晰、可演进的 Agent Harness 产品定义，容易在实现过程中遗忘关键讨论、混淆范围，或者过早陷入局部技术细节。对于一个以学习为目的的自研平台，如果没有统一的产品视角，后续能力扩展会缺乏稳定边界。
- **Proposed Solution**: 构建一个面向单机本地运行的 Go 版 Agent Harness 平台，优先打稳自研核心运行时、CLI、Planning、Compaction、Memory、File System Tools 和 Sub-agents 的基础能力，并用 OpenSpec 和本 PRD 作为统一范围控制文档。
- **Success Criteria**:
  - 单机本地环境下，用户可以通过 CLI 成功完成至少 1 条完整执行链：`run -> planning -> tool execution -> event persistence -> result output`。
  - 系统必须支持 `run`、`inspect`、`replay`、`resume` 四个核心 CLI 操作，并能在本地运行目录中持久化 `run.json`、`state.json`、`events.jsonl`、`result.json`。
  - 系统必须同时提供 `Ark provider` 和 `mock provider`，并且 mock provider 能覆盖 planning、compaction、delegation 的主要流程测试。
  - 系统必须支持至少 5 个文件系统工具：`fs.read_file`、`fs.write_file`、`fs.list_dir`、`fs.search`、`fs.stat`，且所有文件操作都限制在 workspace 内。
  - 至少 3 类典型样例任务可以在本地稳定执行并通过验证：基础规划任务、文件系统读写任务、包含 child run 的委派任务。

## 2. User Experience & Functionality

- **User Personas**:
  - 个人开发者 / 学习者：希望通过亲手搭建 Harness 平台理解 Agent Runtime、事件模型、上下文管理、工具调用和子代理协作。
  - 未来的内部平台维护者：希望后续能够在现有基础上继续接入 Hertz、skills、MCP、ACP、MessageProxy 和调度能力，而不推翻核心架构。

- **User Stories**:
  - As a 学习中的平台开发者, I want to 通过 CLI 启动一次本地 agent run so that 我可以观察完整的运行链路和事件流。
  - As a 学习中的平台开发者, I want to 查看、回放和恢复 run so that 我可以理解运行时状态和排查错误。
  - As a 学习中的平台开发者, I want to 让 agent 拥有规划、压缩上下文、写入长期记忆的能力 so that 我可以验证一个真实 Harness 的关键能力闭环。
  - As a 学习中的平台开发者, I want to 让 agent 安全地访问工作区文件 so that 我可以验证工具调用与执行边界。
  - As a 学习中的平台开发者, I want to 将子任务委派给 child run so that 我可以理解受控的 sub-agent 机制如何工作。

- **Acceptance Criteria**:
  - 对于“启动本地 run”故事：
    - 用户执行 `harness run` 后，系统必须创建 `Task`、`Session`、`Run` 并写入事件。
    - 执行结束后必须落盘结果与运行状态。
  - 对于“查看、回放和恢复 run”故事：
    - `harness inspect <run-id>` 必须展示当前状态或最终结果。
    - `harness replay <run-id>` 必须按顺序读取并输出事件流。
    - `harness resume <run-id>` 必须能够恢复未完成运行。
  - 对于“规划、压缩、记忆”故事：
    - 每个 run 启动时必须生成结构化计划。
    - 上下文预算不足时必须触发 compaction 并写入摘要。
    - 运行结束后必须提取并可提交结构化长期记忆。
  - 对于“文件系统工具”故事：
    - 文件系统工具必须全部限制在 workspace 内。
    - `fs.write_file` 覆盖写入必须要求显式 `overwrite=true`。
    - 写入成功时必须区分“新建文件”和“更新文件”事件。
  - 对于“sub-agent 委派”故事：
    - 只有标记为 `delegatable` 的计划步骤才允许委派。
    - child run 必须返回固定结构化结果，至少包含 `summary` 和 `needs_replan`。

- **Non-Goals**:
  - 本阶段不做服务化部署，不支持多人共享，不设计多租户能力。
  - 本阶段不实现 Hertz HTTP API、飞书 MessageProxy、MCP、ACP、TaskDispatcher、Cronjob、Heartbeat 的具体接入。
  - 本阶段不引入生产级数据库、向量数据库或分布式队列。
  - 本阶段不追求开放式多代理自治，只支持受控的 child run delegation。

## 3. AI System Requirements

- **Tool Requirements**:
  - 必须提供内置 Planning 能力。
  - 必须提供 File System Tools：`fs.read_file`、`fs.write_file`、`fs.list_dir`、`fs.search`、`fs.stat`。
  - 必须提供 Prompt Builder，支持 `base`、`role`、`task`、`tooling` 四层模板组装。
  - 必须提供 ContextManager，支持上下文组装和 compaction。
  - 必须提供 MemoryManager，支持结构化长期记忆的 recall 和 write-back。
  - 必须提供 DelegationManager，支持 child run 的创建、边界控制和结果合并。
  - 必须提供统一 `Model` 接口，并落地 `Ark provider` 与 `mock provider`。

- **Evaluation Strategy**:
  - 构建 3 个本地样例场景：
    - 场景 A：只包含规划与最终输出的基础任务。
    - 场景 B：包含文件系统读取、写入和目录遍历的工具任务。
    - 场景 C：包含 child run 的委派任务。
  - 通过标准：
    - 3 个场景都必须成功完成 run，并生成完整运行工件。
    - 每个场景的 `events.jsonl` 都必须包含必须事件类型，缺失率为 0。
    - 使用 mock provider 的流程测试必须覆盖 planning、compaction、delegation 三条关键分支。
    - `replay` 必须能够完整重放一个已完成 run 的事件顺序。
    - `resume` 必须能够恢复一个中断前已持久化状态的未完成 run。

## 4. Technical Specifications

- **Architecture Overview**:
  - 入口层：当前采用 Cobra CLI，未来可扩展至 Hertz HTTP 和消息入口。
  - 应用层：统一承接 `run`、`inspect`、`replay`、`resume` 等用例。
  - 核心运行时：围绕 `Task`、`Run`、`Session`、`Event` 工作，采用 event-first 模型。
  - Agent Loop：执行 `planning -> prompt build -> context assembly -> model call -> tool execution -> event persistence -> result` 主循环。
  - 扩展能力层：包含 planning、compaction、memory、filesystem tools、subagent delegation。
  - 存储层：第一版采用文件型存储，工件位于 `.runtime/runs/<run-id>/`。

- **Integration Points**:
  - 模型集成：首版接入 Ark，并通过统一 `ModelConfig` 从环境变量 `ARK_API_KEY`、`ARK_BASE_URL`、`ARK_MODEL_ID` 映射配置。
  - CLI 集成：使用 Cobra 提供 `run`、`inspect`、`replay`、`resume`、`tools list`、`debug events`。
  - 文件系统集成：只允许对 workspace 根目录内的路径进行读写和搜索。
  - 认证与数据库：当前阶段均为 `TBD / Not Applicable`，因为产品只面向单机本地运行。

- **Security & Privacy**:
  - 所有文件系统工具必须限制在 workspace 范围内，禁止越界访问。
  - `fs.write_file` 对已有文件的覆盖必须显式确认，防止误写。
  - 长期记忆仅保存结构化、可解释的条目，不保存未经筛选的全部上下文历史。
  - 当前阶段不处理多人共享和远程访问，因此不引入复杂认证机制；若未来服务化，则需重新定义鉴权与敏感数据处理策略。

## 5. Risks & Roadmap

- **Phased Rollout**:
  - MVP:
    - Go 模块初始化
    - Cobra CLI
    - `Task / Run / Session / Event`
    - 文件型存储
    - Ark + mock provider
    - Planning / Context / Compaction / Memory
    - File System Tools
    - Child run delegation
    - `run / inspect / replay / resume`
  - v1.1:
    - 更稳定的测试样例与评估框架
    - 更细粒度的事件类型和调试输出
    - starter prompts 与 plan step 元数据优化
    - 更完善的 memory recall 规则
  - v2.0:
    - Hertz HTTP API
    - skills / MCP / ACP
    - MessageProxy
    - TaskDispatcher / Cronjob / Heartbeat
    - 可能的持久化数据库替换

- **Technical Risks**:
  - 自研 Agent Loop 初期实现速度会慢于直接接现成框架。
  - 文件型存储在 run 数量变多后检索效率会下降。
  - Compaction 质量不足时可能导致关键上下文丢失。
  - Sub-agent 机制如果边界控制不好，容易导致状态复杂度失控。
  - Ark 真实调用依赖外部模型返回，若没有 mock provider，会显著降低测试稳定性。
