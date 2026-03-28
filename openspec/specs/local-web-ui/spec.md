# local-web-ui

## Purpose
定义本地 Web UI 的页面入口、交互能力、实时更新行为，以及面向本机开发的基础体验约束。

## Requirements

### Requirement: 本地 Web UI 入口
系统 MUST 提供一个面向本机开发的 Web UI，用于展示当前 Harness 的核心运行时信息。

#### Scenario: 本地 Web UI 由独立 Web 入口服务
- **WHEN** 用户启动本地 Web UI 所依赖的后端服务
- **THEN** 系统 MUST 通过独立的 Web 可执行入口启动本地 HTTP 服务
- **THEN** 该入口 MUST 保持与现有 Web UI 兼容的 API 和路由行为
- **THEN** 用户 MUST 不需要先进入 CLI 再通过子命令附带启动 Web 服务

#### Scenario: 打开本地 Web UI
- **WHEN** 用户启动本地 Web UI 并在浏览器中访问页面
- **THEN** 系统 MUST 展示一个可交互的前端页面
- **THEN** 页面 MUST 能够访问本地 HTTP API 获取运行时数据

#### Scenario: 打开首页时进入 chat-first 页面
- **WHEN** 用户打开本地 Web UI 首页
- **THEN** 系统 MUST 展示一个 chat-first 页面作为默认入口
- **THEN** 页面 MUST 展示最近的 `Session` 历史
- **THEN** 用户 MUST 可以直接选择某个 `Session` 继续对话，或发起新的聊天
- **THEN** 页面 MAY 将 run/session 调试能力以内嵌调试抽屉的方式提供，而不是独立详情页

#### Scenario: 在 chat-first 入口查看最近活动
- **WHEN** 用户打开 chat-first 页面
- **THEN** 系统 MUST 展示最近的 `Session` 列表
- **THEN** 系统 MAY 展示最近的 `Run` 或等效调试入口
- **THEN** 用户 MUST 可以从该入口继续已有对话或打开相关调试视图

#### Scenario: 调试台与聊天入口并存
- **WHEN** 系统新增 AG-UI 聊天入口
- **THEN** 现有基于 `sessions / runs / replay / events` 的调试台能力 MUST 继续可用
- **THEN** 系统 MAY 将这些调试能力收敛到 chat-first 页面中的统一调试抽屉或等效内嵌视图

### Requirement: 会话与运行详情查看
系统 MUST 在 Web UI 中展示 `Session` 和 `Run` 的核心信息，便于调试和观察执行过程。

#### Scenario: 查看 session 详情
- **WHEN** 用户在 Web UI 中打开某个 `Session`
- **THEN** 系统 MUST 展示最近消息
- **THEN** 系统 MUST 提供查看关联 `Run` 信息的入口或内嵌视图

#### Scenario: chat-first 页面接入 AG-UI 链路
- **WHEN** 用户通过 chat-first 页面发起一次对话
- **THEN** 系统 MUST 通过 AG-UI 兼容事件流实时展示消息、步骤和工具活动
- **THEN** 系统 MUST 允许前端同时获得聊天事件流与现有 run/session 标识
- **THEN** 用户 MUST 可以查看 inspect、原始事件和模型调用等调试信息，无论这些信息呈现在独立页面还是内嵌调试视图中

#### Scenario: 查看 run 详情
- **WHEN** 用户在 Web UI 中打开某个 `Run`
- **THEN** 系统 MUST 展示 `Run` 状态、当前步骤、计划、结果和 child run 摘要
- **THEN** 系统 MUST 提供摘要时间线、原始事件或等效调试信息视图

#### Scenario: 查看正在执行中的 run 详情
- **WHEN** 用户在 Web UI 中打开一个仍在执行中的 `Run`
- **THEN** 系统 MUST 自动订阅该 `Run` 的增量事件流
- **THEN** Web UI MUST 在新事件到达后刷新时间线、原始事件和关键状态指标
- **THEN** 当该 `Run` 进入 terminal 状态时，系统 MUST 结束增量订阅

#### Scenario: 增量事件流短暂断开后恢复
- **WHEN** `Run` 调试视图的 SSE 连接发生瞬时中断，且该 `Run` 还未进入 terminal 状态
- **THEN** Web UI MUST 自动发起重连
- **THEN** 重连时 MUST 从当前已接收的最大 `sequence` 之后继续订阅
- **THEN** Web UI MUST 避免重复追加已经收到过的事件

### Requirement: 通过 Web UI 发起运行
系统 MUST 支持用户通过 Web UI 发起新的运行，或向已有 `Session` 追加一轮输入。

#### Scenario: 发起新的 run
- **WHEN** 用户在 Web UI 中提交新的指令
- **THEN** 系统 MUST 发起新的 `Run`
- **THEN** Web UI MUST 展示该次运行的结果或详情入口

#### Scenario: 在已有 session 中追加一轮输入
- **WHEN** 用户在 Web UI 中对已有 `Session` 提交新的输入
- **THEN** 系统 MUST 在该 `Session` 下创建新的 `Run`
- **THEN** Web UI MUST 刷新消息与运行列表

#### Scenario: 在首页选择某个 session 继续
- **WHEN** 用户在首页选择一个已有 `Session`
- **THEN** Web UI MUST 展示该 `Session` 最近的消息摘要
- **THEN** Web UI MUST 展示该 `Session` 最近关联的 `Run`
- **THEN** 用户 MUST 可以直接在首页继续向该 `Session` 追加一轮输入

### Requirement: Web UI 需要提供基础容错与双语切换能力
系统 MUST 在不引入重型前端框架的前提下，提供面向本地开发的基础容错和中英文切换能力。

#### Scenario: 页面发生前端渲染异常
- **WHEN** 某个页面组件发生未捕获的前端渲染错误
- **THEN** Web UI MUST 显示错误恢复界面，而不是直接白屏
- **THEN** 用户 MUST 可以回到首页或刷新页面继续操作

#### Scenario: 用户切换页面语言
- **WHEN** 用户在 Web UI 中切换语言
- **THEN** 首页、会话页、运行页和错误恢复界面的主要文案 MUST 切换为所选语言
- **THEN** 语言设置 MUST 在浏览器本地保存，以便后续继续使用
