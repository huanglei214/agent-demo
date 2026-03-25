## Context

当前仓库仍处于非常早期的阶段，尚未形成可复用的 Agent 运行时结构。项目目标不是实现一个一次性的 Demo Agent，而是搭建一个可以长期演进的 Harness 平台，用于承载后续的工具调用、上下文管理、记忆系统、子代理委派、协议接入、消息路由和任务调度能力。

基于前期讨论，本次变更已经明确以下约束：

- 技术栈选择 Go，核心运行时采用自研方案，而不是依赖重型 Agent Framework。
- CLI 是 MVP 的唯一入口，命令行框架采用 Cobra。
- HTTP 能力后续通过 Hertz 接入，但不纳入本次变更范围。
- 平台必须支持事件驱动的执行模型，并为未来的 skills、MCP、ACP、MessageProxy、TaskDispatcher、Cronjob 和 Heartbeat 预留清晰边界。
- 文档和规格需要便于中文审阅，因此后续 specs 使用中文编写。

当前还没有任何既有规格需要兼容，因此本次设计可以直接定义平台基础对象、执行链路和目录结构，为后续能力建设提供稳定地基。

## Goals / Non-Goals

**Goals:**

- 定义 Harness 平台的核心领域模型，包括 `Task`、`Run`、`Session`、`Event`。
- 设计首个可运行的 Agent Loop，支持规划、工具调用、上下文组装、压缩和结果落盘。
- 设计 CLI MVP 的边界和命令树，使未来 HTTP、消息入口可以复用同一套应用层。
- 引入内置 Planning、Compaction、Memory、File System Tools 和 Sub-agents 的基础抽象。
- 采用文件型状态与事件存储，优先保证可观测性、可回放性和学习成本。
- 为未来协议、消息和调度系统预留扩展点，但不将其实现纳入本次范围。

**Non-Goals:**

- 不在本次变更中实现 Hertz HTTP API、飞书 MessageProxy、MCP、ACP、TaskDispatcher、Cronjob 或 Heartbeat 的具体适配器。
- 不在本次变更中实现生产级持久化数据库、分布式队列或多节点调度。
- 不引入向量数据库、复杂检索系统或长期记忆评分系统，Memory 第一版以结构化记忆为主。
- 不实现开放式、无限递归的多代理协作，Sub-agents 第一版采用受控委派。
- 不追求一次性覆盖全部工具类型，首批仅提供与文件系统相关的最小工具集。

## Decisions

### 1. 核心运行时采用“自研核心 + 基础框架”

平台核心不依赖 LangChain 风格的 Agent Framework，而是围绕 `Run / Session / Task / Event` 自研运行时。CLI 层使用 Cobra，后续 HTTP 层使用 Hertz，但这些框架只负责入口和传输，不负责 Agent 行为建模。

这样设计的原因是 Harness 的本质是运行时平台，不是单个 Agent 应用。核心执行模型、事件模型和扩展机制必须由项目自身掌控，避免后续接入新协议、新调度器或消息入口时被外部框架的抽象束缚。

备选方案：

- 使用 LangChain 类 Go 框架作为主运行时：可以更快起步，但会让运行时模型、工具协议和上下文管理受限于框架设计，不利于长期演进。
- 完全使用标准库手写 CLI：可行，但命令层会逐步膨胀，且不利于快速建立稳定的命令树。

### 2. 采用事件优先的执行模型

所有关键执行行为都必须写入结构化事件流，事件是系统的唯一可信执行轨迹。`Event` 记录发生过什么，`Run` 记录当前状态，`Task` 记录要做什么，`Session` 记录上下文容器。

第一版事件存储采用 `events.jsonl`，每次 run 独立持久化到 `.runtime/runs/<run-id>/` 目录，并配套 `run.json`、`state.json`、`plan.json`、`result.json` 等文件。

这样设计的原因是：

- 便于学习和调试，可以直接查看执行轨迹。
- 便于后续实现 replay、inspect、resume。
- 为未来迁移到 SQLite/Postgres 保留清晰的数据模型。

备选方案：

- 直接使用关系型数据库：更接近生产，但在当前阶段增加了实现复杂度，不利于快速迭代和观察运行过程。
- 仅保存最终结果，不保存事件：实现简单，但会丢失回放、审计、排障和可解释性。

### 3. CLI 通过应用层驱动运行时，不直接访问内部模块

命令树采用 Cobra，首批实现 `run`、`inspect`、`replay`、`resume`、`tools list` 和 `debug events` 等命令。CLI 只负责参数解析和输出格式，真正的业务逻辑由 `internal/app` 下的服务对象承接。

推荐调用链为：

`Cobra CLI -> App Service -> Runtime / Loop / Store / Tool / Memory`

这样设计的原因是：

- 保证未来接入 Hertz 或消息入口时可以重用应用层。
- 避免命令层和运行时强耦合。
- 让测试可以直接针对应用层和核心模块进行。

备选方案：

- CLI 直接调用 runtime internals：实现快，但未来接 HTTP 或消息入口时需要重构。

### 4. Agent Loop 采用“计划驱动 + 工具执行 + 事件回写”的结构

主循环围绕以下步骤展开：

1. 创建 `Task / Session / Run`
2. 构建 starter prompts、召回长期记忆、组装当前上下文
3. 调用内置 Planning 能力生成结构化 `Plan`
4. 根据当前 `PlanStep` 调用模型
5. 解析模型输出，执行普通回复或工具调用
6. 将工具结果写回事件流，并继续下一轮
7. 根据上下文预算触发 compaction
8. 在必要时触发 sub-agent delegation
9. run 完成后提取长期记忆候选并提交

Planning 不是一段普通 prompt，而是一级平台能力，输出结构化 `Plan` 和 `PlanStep`。这样后续的委派、重规划、上下文压缩都可以围绕计划对象协同。

备选方案：

- 纯对话式 loop，不引入显式计划对象：实现简单，但不利于任务拆分、回放、审计和 delegation。

### 5. Compaction 由 ContextManager 统一管理，不与 Memory 混用

Compaction 的目标是控制当前上下文窗口，而不是存储长期知识。因此它归属于 `ContextManager`，负责在 token 预算不足、步骤过多或工具输出过大时，将近期事件压缩为 `step summary` 或 `run summary`。

Memory 的目标是跨会话保留稳定、可复用的信息，因此由 `MemoryManager` 管理，只保存偏好、事实、约定和重要决策等长期有价值的内容。

这样设计的原因是：

- 避免把上下文裁剪逻辑和长期知识存储混为一谈。
- 避免大量短期过程污染长期记忆。
- 使 prompt 组装逻辑更清晰，可按优先级拼接 `Pinned Context`、`Active Step`、`Summaries` 和 `Recalled Memories`。

备选方案：

- 将 compaction 结果直接当作 memory 写入：简单但语义混乱，后续召回质量会迅速下降。

### 6. 首个模型适配器优先接入 Ark，并同步提供 Mock Provider

系统需要先定义统一的 `Model` 接口，再基于该接口实现首个真实模型适配器。第一版优先接入 Ark Provider，并同步提供 `mock provider` 以支持本地测试、事件回放和流程级验证。

Ark Provider 的配置加载采用“环境变量输入 + 内部统一配置映射”的方式。系统首先读取当前环境中已有的 Ark 相关配置，例如 `ARK_API_KEY`、`ARK_BASE_URL`、`ARK_MODEL_ID`，再映射到内部统一的 `ModelConfig` 结构，避免将特定供应商的环境变量格式直接泄漏到运行时核心接口。

这样设计的原因是：

- 统一接口可以让模型供应商变化不影响核心运行时和 Agent Loop。
- 当前项目环境已经具备 Ark 相关配置，优先接入 Ark 可以降低落地成本。
- `mock provider` 可以在不依赖外部网络和真实模型响应的情况下，稳定验证 planning、compaction、delegation 和 replay 等流程。

备选方案：

- 第一版优先接入 OpenAI-compatible provider：抽象同样成立，但与当前环境配置相比没有明显优势。
- 不提供 mock provider：会让测试过度依赖真实模型返回，降低开发效率和可重复性。

### 7. Memory 第一版采用结构化长期记忆，并固定 kind、放开 tags

Memory 第一版不引入向量数据库，而是采用结构化条目存储，并在数据结构上区分固定的 `kind` 和开放的 `tags`。

固定 `kind` 包括：

- `preference`
- `fact`
- `decision`
- `convention`

`tags` 允许自由扩展，但推荐使用带前缀的命名规范，例如：

- `project:*`
- `tool:*`
- `arch:*`
- `user:*`
- `runtime:*`

写入时机主要有三类：

- run 完成后的集中提炼
- 关键设计决策形成时的增量提炼
- 用户显式要求“记住”时的强制写入

召回时按任务目标、标签和类别筛选，不全量注入模型上下文。

Memory 第一版的召回策略采用“过滤 + 稳定排序 + Top N”模式，而不是引入复杂评分机制。系统先基于 `kind`、`tags` 和当前任务目标进行过滤，再按固定规则排序，例如：

- `Session` 范围的记忆优先于更宽范围的记忆
- `decision`、`convention` 优先于 `fact`、`preference`
- 最近写入的记忆优先

排序后仅选取前 N 条结果注入当前模型上下文。

这样设计的原因是：

- 当前项目仍在搭基础设施，优先保证可解释性和可控性。
- 向量检索会引入新的依赖和召回质量问题，不适合作为 MVP 的前置条件。
- 固定排序规则比引入早期评分模型更容易测试、调试和解释。

备选方案：

- 直接引入向量数据库：利于模糊召回，但超出当前 MVP 复杂度预算。

### 8. 工具系统采用统一抽象，首批仅提供文件系统工具

所有工具统一通过 Tool 接口注册和执行，第一版只实现 workspace 范围内的文件系统工具，包括：

- `fs.read_file`
- `fs.write_file`
- `fs.list_dir`
- `fs.search`
- `fs.stat`

其中 `fs.write_file` 第一版采用安全写入模式：

- 仅允许写入 workspace 内路径
- 默认允许创建新文件
- 覆盖已有文件时必须显式指定 `overwrite=true`
- 未显式允许覆盖时，目标文件已存在 MUST 返回错误
- 工具成功写入后，系统区分“新建文件”和“更新已有文件”两类事件，便于回放和审计

所有工具都需要结构化输入输出，并记录 `tool.called`、`tool.succeeded`、`tool.failed` 等事件。

这样设计的原因是：

- 文件系统工具是构建代码类 Agent 的最小闭环能力。
- 统一的 Tool 接口有利于未来平滑接入 MCP Tool、ACP 能力和 skills 暴露的能力包。

备选方案：

- 先做更复杂的 shell、network、browser 工具：覆盖面更广，但安全风险和实现复杂度更高。

### 9. Starter Prompts 第一版仅提供默认角色，但保留多角色模板结构

Starter Prompts 采用可组合模板结构，但第一版仅启用一个默认角色模板 `default-agent`。模板体系上保留以下层次：

- `base`
- `role`
- `task`
- `tooling`

这样设计的原因是：

- 第一版应优先稳定 Agent Loop，而不是扩展大量角色变体。
- 统一模板结构可以让未来增加 `coder`、`planner`、`researcher` 等角色时无需重写 prompt 组装机制。
- Planning 已经是平台内建能力，因此第一版不需要通过新增独立角色来获得规划能力。

备选方案：

- 第一版直接引入多个角色模板：扩展性更强，但会增加 prompt 变体数量和测试复杂度。

### 10. Sub-agents 以 child run 方式实现受控委派

子代理不是长期驻留实体，而是主 run 为特定 `PlanStep` 派发出的 child run。子代理拥有独立的 run 状态、事件流、上下文压缩能力和结果摘要，但不拥有全局调度权。

第一版委派策略为：

- 只有 `delegatable` 的 plan step 才允许派发
- 最大委派深度为 2
- 最大并发 child run 数为 2
- 默认仅允许只读工具
- child run 不能直接写长期 memory，只能返回 candidate 或 summary

子代理返回的结果必须是结构化摘要，包括完成内容、产物、风险和是否建议主 run replan。

第一版固定 child run 的结果结构，至少包含：

- `summary`
- `artifacts`
- `findings`
- `risks`
- `recommendations`
- `needs_replan`

其中 `summary` 和 `needs_replan` 为必填字段，`artifacts`、`findings`、`risks`、`recommendations` 即使为空也应以数组形式返回，避免主代理在合并阶段处理不稳定的数据结构。

这样设计的原因是：

- 避免无限递归、多代理失控和上下文污染。
- 让主代理可以稳定消费子代理结果，并更容易触发重规划或结果汇总。
- 保留多代理架构的扩展路径，同时控制 MVP 风险。

备选方案：

- 开放式多代理自治：灵活但极易失控，也不利于教学和平台演进。

### 11. PlanStep 为未来调度系统预留成本与工作量字段

`PlanStep` 的结构在第一版中预留可选字段，例如 `estimated_cost` 与 `estimated_effort`，用于支持未来的任务调度、优先级调整和资源评估能力。

第一版中这些字段：

- 可以为空
- 不作为必填项
- 不参与主运行时的执行决策
- 不影响当前的 planning、delegation 或 compaction 流程

这样设计的原因是：

- 后续若接入 TaskDispatcher、Cronjob 或资源感知调度，计划对象很可能需要承载这些元信息。
- 当前阶段先预留字段，可以减少后续数据结构破坏性修改。
- 将其保持为可选字段，可以避免在 MVP 阶段为了估算精度增加不必要复杂度。

备选方案：

- 现在完全不保留这些字段：MVP 更简洁，但后续引入调度能力时需要扩展计划模型。
- 现在就让这些字段参与执行决策：会增加 planning 实现难度，也难以保证估算质量。

### 12. 目录与模块按能力分层，为未来扩展预留接口

推荐目录分层如下：

- `cmd/harness`：CLI 入口
- `internal/cli`：Cobra 命令定义
- `internal/app`：应用层服务
- `internal/runtime`：Task/Run/Session/StateMachine
- `internal/loop`：Agent Loop
- `internal/planner`：Planning
- `internal/context`：ContextManager 与 Compaction
- `internal/memory`：Recall 与 Write-back
- `internal/tool`：Tool Registry 与文件系统工具
- `internal/delegation`：Sub-agents
- `internal/prompt`：Starter Prompts 与 Prompt Builder
- `internal/store`：文件存储
- `internal/model`：模型适配器

后续 `skills`、`protocol/mcp`、`protocol/acp`、`messageproxy`、`dispatcher`、`scheduler`、`heartbeat` 可在不破坏核心运行时的前提下加入。

## Risks / Trade-offs

- [风险] 文件型存储在 run 数量变多后会出现检索和管理成本上升  
  → 缓解：第一版显式抽象 `EventStore` 和 `StateStore` 接口，为未来迁移数据库保留边界。

- [风险] 自研 Agent Loop 和规划机制需要更多前期设计，短期内实现速度可能慢于直接接框架  
  → 缓解：先控制 MVP 范围，只实现 CLI、文件系统工具、结构化计划和基础记忆。

- [风险] Compaction 质量不足会导致上下文丢失关键信息  
  → 缓解：保留 `Pinned Context`，并围绕当前 `PlanStep` 组织摘要，避免纯对话式压缩。

- [风险] Sub-agent 过早放开会增加状态管理复杂度和错误传播难度  
  → 缓解：第一版使用固定深度、固定并发和只读工具策略，主代理保留最终决策权。

- [风险] 过早为未来能力预留过多接口可能让代码显得“空”  
  → 缓解：仅保留明确边界，不提前引入不必要依赖或伪实现。

## Migration Plan

1. 创建 Go 模块和基础目录结构，引入 Cobra 作为 CLI 框架。
2. 定义 `Task`、`Run`、`Session`、`Event` 及文件型存储结构。
3. 实现 `harness run / inspect / replay / resume / tools list / debug events` 的命令层和应用层。
4. 实现首版 Agent Loop、Planning、Prompt Builder、ContextManager、MemoryManager 和文件系统工具。
5. 实现受控的 child run delegation 与结果汇总。
6. 通过事件回放和样例任务验证整条执行链闭环。

回滚策略：

- 若实现过程中发现某个高级能力影响主链路，可暂时关闭该能力的调用入口，但保留接口定义。
- 文件型存储不会影响外部系统，因此回滚主要通过回退代码版本完成，不涉及数据迁移。

## Open Questions

- 后续引入 MessageProxy 和 TaskDispatcher 时，`Session` 的归属边界是否需要从“单一上下文容器”扩展为“多来源会话聚合容器”？
