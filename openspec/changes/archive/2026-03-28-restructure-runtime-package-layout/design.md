## 概述

本次重构的目标是继续收敛运行时核心的目录与职责边界，而不是引入新的产品能力。

当前仓库已经具备较清晰的顶层分层：

- `internal/interfaces/`：CLI / HTTP / AG-UI 适配层
- `internal/model/`：模型提供方实现
- `internal/tool/`：工具抽象与实现
- `internal/store/`：运行时工件持久化
- `internal/runtime/`：共享运行时类型

但 `internal/app/` 仍然混合了三类职责：

1. agent 执行引擎
2. 应用服务编排
3. 领域策略与辅助决策

这会让后续继续演进真实流式输出、批量工具调用、Web 作为主入口等能力时，修改路径和依赖方向变得不够直观。

## 设计目标

### 1. 明确内部层次

重构后内部层次应清晰区分为：

- `internal/interfaces/`
  - 面向 CLI / HTTP / Web 的适配层
- `internal/service/`
  - 面向用例的服务编排层
- `internal/agent/`
  - 面向单次运行生命周期的执行引擎
- 领域包
  - `delegation/`
  - `planner/`
  - `retrieval/`（如本次引入）
  - `context/`
  - `memory/`
  - `prompt/`
  - `store/`
  - `model/`
  - `tool/`
- `runtime/`
  - 最底层共享类型与错误

### 2. 保持用户可见行为不变

本次重构不应有意改变：

- CLI 行为
- HTTP 路由和响应语义
- Web 页面调用方式
- 已存在的运行时工件格式（除非迁移需要且可兼容）

### 3. 控制迁移风险

本次重构应按可分步验证的方式推进，确保每一步都满足：

- 可以编译
- 相关测试可运行
- 核心场景回归可验证

## 当前问题拆解

### 问题一：`internal/app/` 职责仍然过宽

当前 `internal/app/` 中同时存在：

- 引擎类文件
  - `agent_loop.go`
  - `action_dispatch.go`
  - `action_parser.go`
  - `run_observer.go`
- 服务类文件
  - `run_service.go`
  - `session_service.go`
  - `inspect_service.go`
  - `replay_service.go`
  - `resume_service.go`
  - `list_service.go`
  - `tools_service.go`
  - `services.go`
- 领域策略文件
  - `delegation_executor.go`
  - `replan_policy.go`
  - `retrieval_guard.go`

这些文件在包级别上被放在一起，增加了阅读和维护成本。

### 问题二：领域策略未完全归位

部分策略文件更适合放到对应领域包，而不是继续放在 `app/`：

- delegation 执行逻辑应归到 `delegation/`
- replan 策略应归到 `planner/`
- retrieval 进展与强制收口逻辑应归到独立包或与 agent 更贴近的包

### 问题三：目录语义与依赖方向可进一步对齐

当前虽然没有明显不可维护的循环依赖，但目录语义与依赖方向还可以更一致：

- `service` 应负责用例编排，不持有复杂引擎细节
- `agent` 应负责执行循环和 action dispatch，不反向依赖 `service`
- 领域策略应通过接口与 `agent` 协作，而不是直接混进 service 层

## 目标目录结构

本次设计建议的目标结构为：

```text
internal/
  agent/
    loop.go
    dispatch.go
    parser.go
    observer.go
    event_helpers.go
  service/
    services.go
    run.go
    session.go
    inspect.go
    replay.go
    resume.go
    list.go
    tools.go
  delegation/
    manager.go
    executor.go
  planner/
    planner.go
    replan_policy.go
  retrieval/
    guard.go
  context/
  memory/
  model/
  prompt/
  runtime/
  skill/
  store/
    interface.go
    filesystem/
  tool/
  interfaces/
    cli/
    http/
      agui/
```

## 关键设计决策

### 决策一：保留 `internal/interfaces/`

本次不将 `internal/interfaces/` 提升为顶层 `app/`。

原因：

- 当前 `internal/interfaces/` 已经清楚表达“内部适配层”语义
- 提升为顶层目录主要是命名变化，收益有限
- 会扩大公开 import 面，增加外部依赖可能性
- 会引入较大范围的 import 路径迁移与文档噪音

因此本次仅重构运行时核心，不做 `interfaces -> app` 的路径迁移。

### 决策二：拆分 `app` 为 `agent + service`

这是本次重构的核心。

#### `internal/agent/`

负责：

- 运行执行循环
- 模型 action 解析与校验
- tool / delegation / final action 的调度
- 执行期事件观察与辅助

#### `internal/service/`

负责：

- 创建和恢复 run / session
- 装配依赖
- 暴露 inspect / replay / list / tools 等服务用例
- 为 interfaces 层提供稳定服务接口

### 决策三：领域策略归位

本次将以下策略文件归位：

- `delegation_executor.go` -> `internal/delegation/executor.go`
- `replan_policy.go` -> `internal/planner/replan_policy.go`
- `retrieval_guard.go` -> `internal/retrieval/guard.go`

这里选择 `retrieval/` 而不是 `context/`，因为它更接近“检索进展与收口策略”，不是纯上下文拼装。

### 决策四：通过接口避免循环依赖

对于 `delegation` 与 `agent` 的配合，优先使用窄接口而不是直接相互引用具体实现。

例如：

- `delegation` 可依赖 child run 执行接口
- `service` 可依赖 `agent` 暴露的执行入口
- `store` 继续通过接口被 service/agent 使用

### 决策五：优先保持工件与路由稳定

本次重构不主动改变：

- `.runtime/` 下运行工件路径规则
- 现有 HTTP 路由路径
- Web 调用的 API 形态
- CLI 的命令结构

避免把“目录重构”扩展成“协议重构”。

## 迁移策略

### 第一步：引入新目录并迁移引擎文件

先创建：

- `internal/agent/`
- `internal/service/`
- `internal/retrieval/`

然后迁移引擎类文件到 `internal/agent/`。

这一步目标是先把“执行引擎”和“应用服务”切开。

### 第二步：迁移服务文件

将服务文件迁移到 `internal/service/`，并调整：

- `services.go`
- 依赖装配路径
- `cmd/` 与 `internal/interfaces/` 的 import

### 第三步：归位领域策略

依次迁移：

- `delegation_executor.go`
- `replan_policy.go`
- `retrieval_guard.go`

并在迁移过程中补齐对应包的测试。

### 第四步：清理旧路径与重复辅助逻辑

在所有 import 收敛后：

- 删除 `internal/app/` 中已迁移的旧文件
- 清理重复 helpers
- 确认 `internal/app/` 不再作为逻辑承载包存在

## 验证策略

每次迁移至少执行与改动范围相符的最小验证：

- 迁移 `internal/service/` 或 `internal/agent/`：
  - 相关包测试
  - `make verify-scenarios`
- 迁移 `internal/interfaces/` import：
  - 相关包测试
  - `make build`
- 迁移 Web / HTTP 依赖引用：
  - `make web-build`
  - 相关 HTTP 包测试

重构完成后应至少执行：

- `make build`
- `make verify-scenarios`
- `go test ./...`

## 不在本次范围内的内容

以下内容明确不属于本次变更：

- 引入新的产品能力
- 重写前端结构
- 变更 OpenSpec 主规格中的产品语义
- 将 `internal/interfaces/` 提升为顶层公开包
- 改写 `.runtime/` 工件协议
