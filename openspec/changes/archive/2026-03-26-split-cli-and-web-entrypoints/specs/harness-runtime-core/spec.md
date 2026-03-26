## MODIFIED Requirements

### Requirement: Cobra CLI 作为首个入口
系统 MUST 提供基于 Cobra 的 CLI 入口，用于驱动运行时能力，而 CLI 层 MUST 通过应用层访问核心模块。

#### Scenario: CLI 由独立二进制入口启动
- **WHEN** 用户构建或启动 CLI 程序
- **THEN** 系统 MUST 通过独立的 `cmd/cli` 入口启动 Cobra root command
- **THEN** CLI 入口 MUST 只负责初始化配置和命令装配，而不直接承载核心业务逻辑

### Requirement: 本地 HTTP API 入口
系统 MUST 提供一个面向本机开发的 HTTP API 入口，用于复用现有应用层服务，而不改变核心运行时模型。

#### Scenario: Web 服务由独立二进制入口启动
- **WHEN** 用户构建或启动本地 Web 服务
- **THEN** 系统 MUST 通过独立的 `cmd/web` 入口启动 HTTP server
- **THEN** Web 入口 MUST 直接复用现有应用层服务与 HTTP 适配层
- **THEN** 系统 MUST 不要求通过 CLI 子命令才能启动本地 Web 服务
