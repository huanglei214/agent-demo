## MODIFIED Requirements

### Requirement: 本地 Web UI 入口
系统 MUST 提供一个面向本机开发的 Web UI，用于展示当前 Harness 的核心运行时信息。

#### Scenario: 本地 Web UI 由独立 Web 入口服务
- **WHEN** 用户启动本地 Web UI 所依赖的后端服务
- **THEN** 系统 MUST 通过独立的 Web 可执行入口启动本地 HTTP 服务
- **THEN** 该入口 MUST 保持与现有 Web UI 兼容的 API 和路由行为
- **THEN** 用户 MUST 不需要先进入 CLI 再通过子命令附带启动 Web 服务
