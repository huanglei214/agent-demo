## 背景

仓库已经完成目录层面的 `internal/agent/` 与 `internal/service/` 拆分，但对象边界仍不够清晰：

- `service.Services` 仍然直接嵌入 `agent.Executor`
- `agent.Executor` 继续以平铺结构持有大量依赖
- `agent / service` 仍直接依赖 `filesystem.EventStore` 与 `filesystem.StateStore` 具体类型

这意味着目录虽然拆开了，但职责边界和依赖方向还没有真正收紧。  
本次设计的目标，是在不改变外部行为的前提下，把 `service -> agent -> store/runtime` 的结构整理成更稳定的内部契约。

## 设计目标

1. Service 层只负责业务编排和查询，不暴露 agent 内部执行能力
2. Agent 层通过窄接口对外提供运行与恢复能力
3. Store 通过接口暴露能力，filesystem 只是具体实现
4. Executor 不再平铺大量依赖字段，而是按职责域组织
5. CLI / HTTP / Web / `.runtime/` 工件结构保持兼容

## 非目标

本轮不处理以下内容：

- tool batch 并发执行
- sequence 自动递增完全下沉到 EventStore
- `NewID` 升级为 UUID / ULID
- memory recall 性能优化
- `listSessionRuns` 的扫描优化
- scanner buffer 上限调整

这些问题后续可以继续作为独立 change 处理，但不和本轮边界收敛绑在一起。

## 方案概述

### 1. 引入 `agent.Runner` 窄接口

在 `internal/agent/` 中定义最小对外契约，只暴露 service 当前真正需要的能力：

- `ExecuteRun(...)`
- `ResumeRun(...)`

这条接口只承载“执行”和“恢复”语义，不额外暴露 plan、dispatch、tool batch、delegation 细节。

这样做的目的：

- `service` 不再依赖 `Executor` 具体实现
- 后续 `Executor` 内部重构不会直接波及 `service`
- 上层入口只通过 `service` 与系统交互

### 2. `service.Services` 去掉对 `Executor` 的嵌入

当前 `service.Services` 通过嵌入把 `Executor` 的全部导出方法和字段泄漏给了 service 层。  
本轮会改为显式持有：

- `runner agent.Runner`
- `eventStore store.EventStore`
- `stateStore store.StateStore`

这样 `Services` 只对外暴露自己的业务 API，例如：

- `StartRun`
- `ResumeRun`
- `InspectRun`
- `ReplayRun`
- `InspectSession`

其中：

- 执行相关方法通过 `runner` 下发到 agent 层
- 查询类方法直接读取 `store` 接口

### 3. 为 store 抽接口

在 `internal/store/` 中新增接口定义，至少覆盖当前 service/agent 真正需要的能力。

建议拆成：

- `EventStore`
- `StateStore`

第一版不追求把 `filesystem` 包里的所有方法一次性都抽干净，只抽当前调用路径必需的方法。

这样做的目的：

- agent/service 不再依赖 `filesystem.EventStore` / `filesystem.StateStore` 具体类型
- 为后续引入 SQLite、远程存储或缓存层留出空间
- 让 `filesystem` 更明确地成为实现层，而不是默认事实标准

### 4. 拆解 `Executor` 的依赖结构

当前 `Executor` 平铺持有大量依赖。  
本轮不把它进一步拆成过细的五六层，而是先按职责收成三组：

- `RuntimeDeps`
  - `Paths`
  - `EventStore`
  - `StateStore`

- `AgentDeps`
  - `Planner`
  - `ContextManager`
  - `MemoryManager`
  - `PromptBuilder`
  - `SkillRegistry`
  - `DelegationManager`

- `ExecutionDeps`
  - `ModelFactory`
  - `ToolRegistry`
  - `ToolExecutor`

`Executor` 最终持有：

- `Config`
- `Runtime`
- `Agent`
- `Execution`

这样可以在不大幅改变现有执行流程的前提下：

- 提高结构可读性
- 降低字段平铺带来的维护负担
- 为后续进一步拆小 `Executor` 留出自然演进路径

## 依赖方向

本轮之后应满足以下依赖方向：

- `interfaces/*` -> `service`
- `service` -> `agent.Runner` + `store` interfaces
- `agent` -> `store` interfaces + runtime/model/tool/prompt/planner 等领域能力
- `filesystem` -> `store` interface contracts
- `runtime` 保持底层类型模型，不反向依赖 service/agent

特别约束：

- `service` 不得继续依赖 `agent.Executor` 具体类型
- `service` 不得感知 agent 内部的 plan step、dispatch、tool execution 细节
- `agent` 不得依赖 `service`

## 兼容性要求

本轮是内部结构重构，以下行为必须保持兼容：

- CLI 命令行为不应有意变化
- HTTP 路由与响应形状不应有意变化
- Web 页面现有调用链不应有意变化
- `.runtime/` 下的工件布局保持不变
- scenario regression 结果保持一致

## 实施顺序

### 步骤 1：定义 store 接口

先在 `internal/store/` 中定义 `EventStore` 和 `StateStore` 接口，并让 `filesystem` 实现满足这些接口。

### 步骤 2：定义 `agent.Runner`

在 agent 层定义最小接口，并让 `Executor` 实现它。

### 步骤 3：改造 `service.Services`

去掉 embed，改成显式持有：

- `runner`
- `eventStore`
- `stateStore`

并调整构造函数和调用路径。

### 步骤 4：重组 `Executor` 依赖

把平铺字段改成 grouped deps，同时保持内部执行流逻辑不变。

### 步骤 5：更新入口和测试

把 CLI / HTTP / Web 装配和测试全部切换到新边界，然后跑：

- 定向测试
- `make verify-scenarios`
- `go test ./...`

## 风险与控制

### 风险 1：service / agent 调用链断裂

控制方式：

- 先定义接口，再逐步替换
- 优先保持方法签名兼容

### 风险 2：store 接口抽象过大

控制方式：

- 第一版只抽当前调用路径需要的方法
- 不追求一次性把所有 filesystem 能力接口化

### 风险 3：Executor 重组影响执行流

控制方式：

- 本轮只重组依赖承载方式，不重写 loop / dispatch / delegation 流程
- 用 scenario regression 做回归保护

## 验收标准

本轮完成后，至少应满足：

1. `service.Services` 不再嵌入 `agent.Executor`
2. `service` 只通过 `agent.Runner` 与 agent 交互执行能力
3. `agent / service` 不再依赖 `filesystem.EventStore` / `filesystem.StateStore` 具体类型
4. `Executor` 不再平铺当前全部依赖字段
5. CLI / HTTP / Web / `.runtime/` 外部行为保持兼容
6. `make verify-scenarios` 通过
7. `go test ./...` 通过
