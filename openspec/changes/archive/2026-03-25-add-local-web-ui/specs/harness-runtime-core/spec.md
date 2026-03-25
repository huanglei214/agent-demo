## ADDED Requirements

### Requirement: 本地 HTTP API 入口
系统 MUST 提供一个面向本机开发的 HTTP API 入口，用于复用现有应用层服务，而不改变核心运行时模型。

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
