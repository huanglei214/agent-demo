## Context

当前仓库已经完成单机本地运行的 Agent Harness MVP，核心能力都通过 Cobra CLI 暴露，并且应用层已经整理为相对清晰的 `app.Services` 边界。要新增前端页面，最关键的不是重写运行时，而是补上一层足够薄的本地 HTTP API，再用前端页面消费这些 API。

这次变更有几个显式约束：

- 仍然只做本机开发用的本地 Web UI
- 不引入鉴权、多用户、远程部署或 SaaS 场景
- 不改变 `.runtime` 文件型工件和现有 event-first 模型
- 后端 HTTP 层优先复用 `app.Services`
- Web server 选用 `chi`

## Goals / Non-Goals

**Goals:**
- 增加一个基于 `chi` 的本地 HTTP server
- 提供前端首版所需的 `sessions / runs / replay / events / tools` API
- 增加一个前端工程骨架和首版页面框架
- 提供足够顺手的本地联调体验与基础页面容错能力
- 保持 CLI、HTTP、前端三者都复用同一套应用层服务

**Non-Goals:**
- 不做鉴权和多用户
- 不做生产部署和服务治理
- 不在本次变更中完整实现 AG-UI 协议
- 不在本次变更中引入 WebSocket；流式能力只覆盖 run 详情页的 SSE

## Decisions

### 1. 先做“本地 UI 适配层”，而不是完整服务平台

本次新增的 HTTP server 只服务本机开发和前端页面，不承担多租户、鉴权、远程部署等平台职责。这样可以把复杂度控制在“前端页面需要的最小 API”范围内。

替代方案：
- 直接读取 `.runtime` 文件：会让前端和存储格式强耦合
- 通过 CLI bridge 调用：交互成本高，不利于页面体验

### 2. HTTP 层采用 `chi`，保持标准库风格

项目现有代码以标准 Go 风格和 Cobra CLI 为主，`chi` 更适合作为一层轻量路由适配器。它足够覆盖 REST 和后续 SSE 需求，同时不会把项目推向过重的 Web 框架结构。

替代方案：
- `net/http`：更轻，但路由组织和中间件体验会更原始
- `Hertz`：更适合长期服务化场景，但对本地 UI 适配层偏重

### 3. API 直接复用 `app.Services`

`internal/httpapi` 只负责：
- 路由
- 请求解析
- 响应编码
- 错误映射

核心运行时仍由 `app.Services` 负责。这样 CLI 和 HTTP 不会各自维护一套业务逻辑。

### 4. 前端采用 React + Vite + TypeScript

前端目标是快速构建一个本地调试面板，而不是 SSR 或内容站点。`React + Vite + TypeScript` 更适合这种本地控制台风格页面。

### 5. 第一版 API 只覆盖可直接支撑页面的信息流

首批接口聚焦：
- `POST /api/sessions`
- `GET /api/sessions/{id}`
- `GET /api/sessions`
- `POST /api/runs`
- `POST /api/runs/{id}/resume`
- `GET /api/runs/{id}`
- `GET /api/runs`
- `GET /api/runs/{id}/replay`
- `GET /api/runs/{id}/events`
- `GET /api/tools`

列表接口只提供“最近摘要”，不做复杂过滤和分页，避免把本地 UI 推向完整 dashboard 平台。

### 6. Run 详情页通过 SSE 获得增量事件

`Run` 详情页是最适合先做实时化的地方，因为它天然以事件流为核心，且已有 `inspect / replay / events` 三种读取接口。这里新增一个只面向本地 UI 的 SSE 端点：

- `GET /api/runs/{id}/stream?after=<sequence>`

行为约束：

- 前端首次打开 run 详情页时，仍先拉一次 `inspect / replay / events`
- 拿到当前最大 `sequence` 后，再通过 SSE 只订阅增量事件
- SSE 服务端每次推送单条原始事件，前端据此刷新 metrics、timeline 和 raw events
- 如果 `Run` 已进入 terminal 状态且没有更多增量事件，服务端发送 `done` 事件后关闭连接

这样可以避免前端在进入页面后重复拉整段事件历史，同时也不用在本次变更中引入更重的 WebSocket 机制

### 7. 本地联调入口采用 `make dev`

为了降低前后端联调成本，本次变更补充一个一键入口，同时启动：

- `make serve`
- `make web-dev`

这不是额外的运行时能力，而是本地开发工作流增强。它不改变后端边界，也不引入新的部署模型，但能显著降低日常开发和演示时的操作摩擦。

### 8. 前端采用轻量级内建 i18n，默认支持中文与英文切换

当前前端页面以本地调试与内部使用为主，最合适的方案不是引入重量级国际化框架，而是先提供一个轻量的语言上下文层：

- 支持 `zh / en` 两种语言
- 语言选择保存在浏览器本地
- 中文作为日常开发的主要体验文案
- run 状态、工具访问级别、相对时间等派生文本也走统一格式化入口

这样既能满足当前中文优先的协作习惯，也不会把页面骨架复杂化。

### 9. Run 详情页的 SSE 需要处理瞬时断连

首版 SSE 已经能提供增量事件，但浏览器页面在开发环境下会遇到：

- 本地服务重启
- 网络瞬时抖动
- Vite 热更新导致的连接中断

因此前端需要自己管理轻量重连策略：

- 记住当前已收到的最大 `sequence`
- 连接断开后延迟短暂时间再重连
- 重连时通过 `after=<latest-sequence>` 继续订阅，而不是重新拉取整段流
- 只有在 run 已进入 terminal 状态时才停止自动重连

这样可以在不修改后端 SSE 协议的前提下，让 run 详情页更接近日常可用的调试界面。

### 10. 页面需要基础错误边界，而不是直接白屏

本地调试页经常会承接快速迭代中的前端改动。如果页面渲染异常直接导致整页白屏，用户很难判断：

- 是后端挂了
- 是数据异常
- 还是前端渲染崩了

因此增加前端错误边界，提供：

- 明确的恢复提示
- 返回首页入口
- 重新加载入口

这可以把“前端崩了但运行时还在”的情况显式暴露出来，减少调试误判。

## Risks / Trade-offs

- [风险] 前端想做 dashboard 时会需要 session/run 列表接口
  → Mitigation: 先以“直达 session/run 详情”和“发起 run”页面为主，第二轮再补列表 API

- [风险] Web UI 和 CLI 的错误呈现风格不同
  → Mitigation: 在 HTTP 层统一定义错误响应格式，避免把 CLI 文案直接透出给前端

- [风险] 未来如果接 AG-UI，当前 API 可能需要再包一层协议适配
  → Mitigation: 保持 HTTP handler 足够薄，后续在 transport 层增加 AG-UI-compatible adapter

## Migration Plan

1. 新增 `chi` 依赖与 `serve` 入口
2. 实现 `internal/httpapi` 路由与基础 handlers
3. 实现前端工程骨架和首版页面
4. 本地联调：Go server + Vite dev server
5. 为 run 详情页补 SSE 增量流
6. 在 README 中补充本地启动和验证方式
7. 增加 `make dev`、错误边界、首页 continuation 面板与中英文切换
8. 为 SSE 详情页补自动重连与续订

## Open Questions

- 后续是否要为列表页增加过滤、搜索或分页
