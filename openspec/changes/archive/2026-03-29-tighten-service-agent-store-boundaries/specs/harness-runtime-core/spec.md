## MODIFIED Requirements

### Requirement: Service 与 Agent 必须通过窄接口协作

系统 MUST 让服务编排层通过窄接口调用执行引擎，而不是继续直接暴露或嵌入执行引擎实现类型。

#### Scenario: service 通过 runner 接口触发执行

- **WHEN** service 层需要启动一次 run 执行
- **THEN** service 层 MUST 通过 agent 暴露的窄接口发起执行
- **AND** service 层 MUST NOT 依赖 `agent.Executor` 的具体实现细节

#### Scenario: service 通过 runner 接口触发恢复

- **WHEN** service 层需要恢复一次未完成的 run
- **THEN** service 层 MUST 通过同一窄接口发起恢复
- **AND** service 层 MUST NOT 直接操作 agent 内部 loop、dispatch 或 delegation 细节

### Requirement: Services 不得继续嵌入 Executor

系统 MUST 让 `service.Services` 通过显式依赖持有 agent 与 store 能力，而不是继续通过嵌入方式暴露执行引擎。

#### Scenario: Services 仅暴露 service 自身业务 API

- **WHEN** 上层入口依赖 `service.Services`
- **THEN** `service.Services` MUST 只暴露 service 层定义的业务方法
- **AND** `service.Services` MUST NOT 通过结构体嵌入将 `agent.Executor` 的字段或方法泄漏给上层

### Requirement: Agent 与 Service 必须依赖 Store 接口而非 filesystem 具体实现

系统 MUST 在 `internal/store/` 中定义稳定接口，并让 agent/service 依赖这些接口而不是 `filesystem` 具体类型。

#### Scenario: filesystem 作为 store 实现

- **WHEN** 系统使用本地文件系统保存 run/session/event 工件
- **THEN** `internal/store/filesystem/` MUST 作为 store 接口的一种实现
- **AND** `agent` 与 `service` MUST 只依赖 store 接口

### Requirement: Executor 依赖必须按职责域分组

系统 MUST 将执行引擎的依赖按职责域组织，而不是继续以大量平铺字段承载所有运行时能力。

#### Scenario: Executor 使用职责分组依赖

- **WHEN** 构造执行引擎实例
- **THEN** 依赖 MUST 至少按 runtime、agent、execution 等职责域组织
- **AND** 执行引擎 MUST NOT 继续以当前平铺字段方式直接承载全部依赖

### Requirement: 本轮边界重构不得有意改变外部行为

系统 MUST 将本次重构限制为内部结构调整，不得有意改变 CLI、HTTP、Web 和 `.runtime/` 工件的现有外部行为。

#### Scenario: 入口与工件结构保持兼容

- **WHEN** 本轮边界收敛完成
- **THEN** CLI、HTTP 与 Web 的现有入口行为 MUST 保持兼容
- **AND** `.runtime/` 下的 run/session 工件路径和主要文件结构 MUST 保持兼容
