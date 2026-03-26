## Context

当前代码库的核心分层大体已经成形：

- `internal/app` 负责应用层编排
- `internal/runtime / planner / tool / store / model` 等包承载核心能力
- `internal/httpapi`、`internal/agui`、`internal/cli` 承担不同入口的适配

但随着本地 Web UI 和 AG-UI 聊天链路加入，仓库结构暴露出两个问题：

1. `cmd` 层只有一个 `cmd/harness`，无法从目录上表达“CLI”和“Web server”是两种独立入口。
2. `internal` 根层把核心包和适配包并列摆放，导致 `cli / httpapi / agui` 看起来和 `runtime / planner / tool / store` 处在同一个抽象层次。

这次变更的目标不是重写分层，而是让入口职责和适配层语义先变清楚。

## Goals / Non-Goals

**Goals**
- 明确 CLI 和 Web 的独立可执行入口
- 让 `serve` 不再是 CLI 的附属职责
- 把接口适配代码收敛到更清楚的命名和位置
- 保持核心业务逻辑和外部行为兼容

**Non-Goals**
- 不重写 `internal/app` 或核心 runtime 设计
- 不修改 HTTP API 合同
- 不改变前端路由或聊天/调试功能
- 不在本次做大规模“核心包全部改名”的整理

## Decisions

### 1. 将入口拆成 `cmd/cli` 和 `cmd/web`

两类启动方式拆成两个可执行入口：

- `cmd/cli/main.go`
- `cmd/web/main.go`

这样目录本身就能表达：

- CLI 是一个独立程序
- Web server 是另一个独立程序

同时也让 `make`、README 和本地开发脚本的语义更清晰。

### 2. `cmd` 只保留很薄的启动逻辑

虽然用户期望“CLI 相关代码放到 `cmd/cli`，Web 相关代码放到 `cmd/web`”，但为了避免把 `cmd` 变成业务代码新家，本次仍保持 Go 里常见的薄入口原则：

- `cmd/cli` 只负责装配并执行 root command
- `cmd/web` 只负责装配配置、服务和 HTTP server

真正的命令定义、router、handler 和 AG-UI adapter 仍放在 `internal`，但会收敛到更清楚的适配层目录。

### 3. 将适配层从 `internal` 根层收敛到明确命名空间

本次推荐的目标结构是：

```text
internal/
  app/
  config/
  runtime/
  planner/
  prompt/
  context/
  memory/
  delegation/
  model/
  tool/
  store/

  interfaces/
    cli/
    http/
      agui/
```

考虑到一次性全量迁移风险较高，本次实现按两阶段推进：

- 第一阶段：先引入 `cmd/cli`、`cmd/web`，并让 Web 启动从 CLI 中剥离
- 第二阶段：将 `internal/cli`、`internal/httpapi`、`internal/agui` 迁到新的 `internal/interfaces/*`

### 4. 保持应用层契约稳定

无论入口或适配层如何调整，以下契约保持不变：

- `app.NewServices(...)`
- run / session / replay / events 的应用层方法
- HTTP API 的请求与响应格式
- 现有 CLI 的用户可见行为

这样可以把本次变更收敛为“结构重整”，而不是功能重写。

## Target Structure

### Before

```text
cmd/
  harness/
    main.go

internal/
  app/
  cli/
  httpapi/
  agui/
  ...
```

### After (phase 1)

```text
cmd/
  cli/
    main.go
  web/
    main.go

internal/
  app/
  cli/
  httpapi/
  agui/
  ...
```

### After (phase 2 target)

```text
cmd/
  cli/
    main.go
  web/
    main.go

internal/
  app/
  runtime/
  planner/
  prompt/
  context/
  memory/
  delegation/
  model/
  tool/
  store/
  config/
  interfaces/
    cli/
    http/
      agui/
```

## Migration Plan

1. 新增 `cmd/cli/main.go`，复用当前 root command 初始化逻辑
2. 新增 `cmd/web/main.go`，直接启动本地 HTTP server
3. 从 CLI root command 中移除 `serve`
4. 更新 `Makefile`、README、脚本和测试，使其指向新的双入口
5. 在功能保持不变的前提下，把 `internal/cli`、`internal/httpapi`、`internal/agui` 收敛到新的 `internal/interfaces/*`
6. 做一次回归验证，确认 CLI、HTTP API 和前端页面行为无回退

## Risks / Trade-offs

- [风险] 一次性重命名太多包会引入较大 import churn
  → Mitigation: 先做双入口，再做适配层迁移

- [风险] `serve` 从 CLI 中剥离后，现有开发命令可能失效
  → Mitigation: 同步更新 Makefile、README 和脚本，保留兼容别名或提供清晰迁移说明

- [风险] “internal 只放核心”如果执行过度，可能把适配层硬塞到 `cmd`
  → Mitigation: 明确 `cmd` 只放入口，适配层仍留在 `internal`，但迁到 `internal/interfaces/*`
