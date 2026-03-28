## MODIFIED Requirements

### Requirement: 运行时核心目录必须按执行引擎与服务编排分层
系统 MUST 将运行时核心代码按“执行引擎”和“服务编排”进行清晰分层，而不是继续将两类职责长期混放在同一个 `internal/app` 包中。

#### Scenario: 执行引擎职责归入 agent 层
- **WHEN** 系统组织单次运行的执行循环、action dispatch、action 解析与执行期观察逻辑
- **THEN** 这些职责 MUST 归属于统一的执行引擎层
- **THEN** 执行引擎层 MUST 不反向依赖服务编排层

#### Scenario: 服务编排职责归入 service 层
- **WHEN** 系统组织 run/session 的创建、恢复、inspect、replay、list 与工具查询等应用服务
- **THEN** 这些职责 MUST 归属于统一的服务编排层
- **THEN** 服务编排层 MUST 通过执行引擎层完成运行执行，而不是在服务层内复制执行逻辑

### Requirement: 领域策略文件必须归位到对应领域包
系统 MUST 将运行时中的领域策略实现放置到各自语义明确的领域包中，而不是继续混放在通用应用包下。

#### Scenario: delegation 执行逻辑归位
- **WHEN** 系统实现 child run 创建、委派执行和委派结果整合逻辑
- **THEN** 相关执行逻辑 MUST 位于 `delegation` 领域包或等效语义位置

#### Scenario: replan 策略归位
- **WHEN** 系统实现运行中的 replan 判断与策略逻辑
- **THEN** 相关策略 MUST 位于 `planner` 领域包或等效语义位置

#### Scenario: retrieval 进展与收口策略归位
- **WHEN** 系统实现网页检索进展跟踪、循环保护与强制收口逻辑
- **THEN** 相关策略 MUST 位于独立 retrieval 语义包或等效语义位置
- **THEN** 系统 MUST 不将该逻辑作为纯上下文构建的一部分长期保留在 `context` 包内

### Requirement: 接口适配层必须保持为内部边界
系统 MUST 将 CLI、HTTP 与 AG-UI 适配层保持为内部接口边界，而不是为了目录命名调整将其提升为新的公开顶层包。

#### Scenario: CLI 适配层保持内部语义
- **WHEN** 系统组织 Cobra CLI 命令入口和调试子命令
- **THEN** 相关适配层 MUST 保持为内部接口层的一部分
- **THEN** 系统 MUST 不要求仅为目录命名原因将其迁移为新的公开顶层包

#### Scenario: HTTP 与 AG-UI 适配层保持内部语义
- **WHEN** 系统组织本地 HTTP API、AG-UI 事件映射和 SSE 服务
- **THEN** 相关适配层 MUST 保持为内部接口层的一部分
- **THEN** 目录重构 MUST 不改变其“对外协议适配、对内复用服务层”的边界语义

### Requirement: 依赖方向必须与目录语义一致
系统 MUST 保持运行时核心依赖方向清晰，避免因目录重构引入新的循环依赖或职责倒置。

#### Scenario: service 依赖 agent 而非反向依赖
- **WHEN** 服务编排层启动或恢复一次运行
- **THEN** 服务编排层 MUST 调用执行引擎层提供的执行入口
- **THEN** 执行引擎层 MUST 不依赖服务编排层

#### Scenario: delegation 通过窄接口与执行引擎协作
- **WHEN** delegation 领域逻辑需要触发 child run 执行
- **THEN** 系统 SHOULD 通过窄接口与执行引擎协作
- **THEN** 系统 MUST 避免通过双向具体类型引用形成包级循环依赖

#### Scenario: runtime 保持底层共享类型语义
- **WHEN** 系统定义 Task、Run、Session、Plan、Event 和基础错误类型
- **THEN** 这些共享类型 MUST 继续位于底层运行时语义层
- **THEN** 目录重构 MUST 不使该层反向依赖上层服务、接口或执行引擎实现

### Requirement: 目录重构不得有意改变现有对外行为
系统 MUST 将本次目录重构控制为内部结构调整，不得有意改变现有 CLI、HTTP、Web 和运行时工件的外部行为。

#### Scenario: CLI 与 HTTP 行为保持稳定
- **WHEN** 系统完成目录迁移与 import 调整
- **THEN** 现有 CLI 命令语义 MUST 保持不变
- **THEN** 现有 HTTP 路由与返回契约 MUST 保持不变

#### Scenario: 运行时工件格式保持稳定
- **WHEN** 系统完成运行时核心目录重构
- **THEN** `.runtime/` 下已有 run/session 工件格式 MUST 保持兼容
- **THEN** 系统 MUST 不将目录重构扩展为工件协议重写
