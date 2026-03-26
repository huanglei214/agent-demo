## MODIFIED Requirements

### Requirement: 本地 HTTP 查询接口
系统 MUST 通过 HTTP API 暴露当前前端所需的运行时查询能力。

#### Scenario: 查询工具列表时暴露扩展访问级别
- **WHEN** 客户端请求工具列表
- **THEN** 系统 MUST 返回所有已注册工具的结构化描述
- **THEN** 每个工具描述 MUST 暴露访问级别
- **THEN** 访问级别 MUST 至少支持 `read_only`、`write` 和 `exec`

### Requirement: 统一的核心领域模型
系统 MUST 定义并持久化 `Task`、`Run`、`Session` 和 `Event` 四类核心对象，并保持它们之间的引用关系清晰可追踪。

#### Scenario: 工具运行时收敛为核心工具面
- **WHEN** 系统启动内置工具注册表
- **THEN** 系统 MUST 暴露一组高频核心工具面
- **THEN** 该核心工具面 MUST 包含本地工作区工具、外部检索工具和命令执行工具
- **THEN** 系统 MUST 不要求低频辅助工具存在于默认核心集
