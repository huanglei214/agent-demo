# Answer Streaming Design

## 背景

当前仓库中的 Web 聊天页已经通过 AG-UI / SSE 消费运行时事件，但 assistant 回答内容仍然不是“真实流式输出”：

- `assistant.message` 只在最终完整回答生成后一次性落盘。
- AG-UI 映射层把这条最终消息拆成 `TEXT_MESSAGE_START`、单次 `TEXT_MESSAGE_CONTENT`、`TEXT_MESSAGE_END`。
- 用户在前端看到的是“整段答案到达后一次性显示”，而不是接近 ChatGPT 的连续打字。

另一个已经暴露的问题是，当前 AG-UI run 生命周期与 HTTP 请求上下文绑定：

- `/api/agui/chat` 使用 `r.Context()` 启动 run。
- run 内部 tool / model 调用直接继承这个 context。
- 浏览器断流、刷新或代理重置会让 run 提前 `context canceled`，并进一步导致 `run.failed`。

本设计的目标，是在不污染 `.runtime` 工件的前提下，为 Web 聊天页提供真实的回答增量输出能力，并把 run 执行与当前连接解耦。

## 设计目标

- Web 聊天页看到接近 ChatGPT 的连续打字感。
- 回答 delta 来自模型真实增量输出，而不是在接口层伪造切片。
- HTTP / SSE 连接断开时，run 继续后台执行。
- `.runtime` 只持久化最终完整 assistant message，不持久化回答 delta。
- replay / inspect / resume 继续围绕最终稳定结果工作。
- provider 不支持 streaming 时，系统可回退到现有非流式路径。

## 非目标

第一版明确不做以下内容：

- 不把回答 delta 写入 `.runtime` EventStore。
- 不支持断线后重新接入同一个 run 的增量回答流。
- 不为 CLI 同步实现 token streaming。
- 不把 tool 输出、plan 事件、memory 事件改成文本流式协议。
- 不在失败场景下持久化半截 assistant 回答。
- 不实现“从已中断的回答流中 resume 并继续吐 token”。

## 顶层语义

本设计将回答 streaming 明确定义为“在线观察能力”，而不是“运行工件的一部分”。

系统拆成两条通道：

- 持久化通道
  - 写入 `EventStore`
  - 进入 `.runtime`
  - 用于 replay / inspect / resume / CLI 概览
- 瞬时流通道
  - 只发给当前在线 observer
  - 不写盘
  - 只用于当前连接的打字式显示

这意味着：

- 用户在线时可以看到连续回答增量。
- 用户断线后，看不到剩余增量。
- run 结束后，用户仍然可以通过最终结果看到完整回答。

## 执行与连接解耦

第一版的核心前提是：run 不再绑定到单个 HTTP 请求生命周期。

语义如下：

- `POST /api/agui/chat` 创建并观察一个 run。
- 当前 HTTP 连接只代表一个临时 observer。
- observer 写失败、浏览器刷新、网络断流，只移除这条观察通道。
- run 自身继续执行，直到 `run.completed` 或真实 `run.failed`。

这要求 AG-UI handler 与 service 不再把 `r.Context()` 直接作为 run 的根 context 传入执行内核。

推荐语义：

- 请求 context 只负责当前响应还能不能继续写出。
- run 使用独立 context 执行。
- 若请求结束，只停止流事件投递，不取消 run 本体。

## 模型层能力

当前模型接口以一次性 `Generate(...)` 为中心。第一版需要新增可选 streaming 能力，但保留现有接口兼容路径。

推荐方向：

- 现有同步接口继续保留
- 新增 streaming 接口供最终回答阶段使用
- provider 可声明自己是否支持真实 streaming

第一版不要求一次把所有 provider 强制切到 streaming，只要求：

- `mock` provider 支持稳定的测试用 streaming
- `ark` provider 如果能支持真实 streaming，就接入
- 若某 provider 暂不支持，则回退到现有非流式 `Generate`

streaming 接口的职责应至少支持：

- 开始生成
- 多次文本 delta
- 正常完成
- 失败中止

第一版建议采用 callback / sink 风格，而不是直接把 provider channel 暴露给上层，原因是 callback 风格更容易控制错误收束、聚合与 observer 写失败时的隔离。

## Executor 行为

第一版只在“最终回答生成阶段”进入 streaming 路径。

其它执行阶段保持现状：

- tool 调用仍是离散副作用
- delegation / planner / retrieval policy 仍按现有流程工作
- memory candidate extraction 与 commit 仍在最终回答之后发生

进入最终回答阶段后的推荐行为：

1. executor 创建一个内存 buffer，用于累积完整回答文本。
2. executor 创建一个瞬时 stream sink，用于把回答 delta 发给当前 observer。
3. provider 每产出一段文本：
   - 追加到 buffer
   - 向 observer 发出瞬时 delta 事件
4. provider 完成后：
   - 停止 stream sink
   - 用 buffer 的完整文本一次性落盘 `assistant.message`
   - 继续现有 `result.generated`、memory 处理、`run.completed`

必须保持以下约束：

- 最终持久化 `assistant.message` 的内容必须等于所有 delta 拼接结果。
- `result.generated`、memory commit、`run.completed` 必须发生在回答流结束之后。
- 瞬时流事件不会写入 EventStore。

## Observer 分层

当前 `RunObserver` 只有 runtime event 这一条观察通道，不足以表达“只投递给在线连接、不写盘”的回答增量。

第一版需要把 observer 能力分层：

- runtime event observer
  - 继续接收 `harnessruntime.Event`
  - 继续服务于持久化事件与现有回放链路
- stream observer
  - 接收瞬时 assistant answer 流事件
  - 不要求具备 replay 能力
  - 只服务于当前在线客户端

stream observer 需要表达的最小语义：

- answer started
- answer delta
- answer completed
- answer failed

第一版不要求 stream observer 覆盖 tool / memory / step 事件，只覆盖 assistant answer 流即可。

## AG-UI / Web 事件契约

前端当前已经能消费以下事件：

- `TEXT_MESSAGE_START`
- `TEXT_MESSAGE_CONTENT`
- `TEXT_MESSAGE_END`

第一版可以保留这组契约，但数据来源必须改变：

- `TEXT_MESSAGE_START`
  - 在回答真实开始输出时发送
- `TEXT_MESSAGE_CONTENT`
  - 允许多次发送
  - 每次携带新的文本 delta
- `TEXT_MESSAGE_END`
  - 在回答真实结束后发送

AG-UI 层的职责应是“映射瞬时 stream 事件”，而不是“把最终完整消息伪装成一段 delta”。

Web 前端的职责保持简单：

- `START` 创建空 assistant message
- 多次 `CONTENT` append
- `END` 收尾

前端不应再通过定时器或字符串切片模拟打字节奏。

## 失败处理

第一版推荐以下失败语义：

### provider 不支持 streaming

- 自动回退到现有 `Generate(...)`
- 用户失去连续打字感，但功能正确
- 最终仍能得到完整回答

### observer 写失败 / 连接断开

- 只移除当前 observer
- run 不失败
- provider 与 executor 继续完成最终回答
- `.runtime` 中仍会出现最终 `assistant.message` 与 `run.completed`

### provider 在回答流中报错

- run 失败
- 不落盘最终 assistant message
- 当前在线用户可能已经看到部分增量文本
- 这些增量文本不进入 `.runtime`

### provider 已输出部分 delta 后失败

这是第一版接受的边界：

- 用户当前连接中会看到半截回答
- 系统最终记录为 `run.failed`
- replay / inspect 不会把半截回答当正式结果持久化

## `.runtime` 与回放语义

第一版明确保持 `.runtime` 简洁：

- 不新增 answer delta 持久化事件
- 不让 replay 重现打字过程
- 只继续持久化最终 `assistant.message`

这意味着 replay 与实时体验会有刻意差异：

- 实时体验：能看到连续打字
- replay / inspect：只能看到最终完整回答

这是本设计刻意接受的取舍，用来避免 `.runtime` 被高频 delta 事件污染。

## 与现有边界的关系

本设计需要保持当前架构边界清楚：

- `internal/model/`
  - 提供 streaming 能力与 provider 适配
- `internal/agent/`
  - 拥有回答流生命周期与最终落盘时机
- `internal/service/`
  - 继续负责入口编排，不直接接管回答流内部细节
- `internal/interfaces/http/agui/`
  - 只负责把瞬时 stream 事件映射给当前连接
- `web/`
  - 只负责真实 delta 拼接与展示

不应把回答 streaming 的主逻辑下沉到 AG-UI 映射层，也不应让 HTTP handler 成为 run 生命周期的拥有者。

## 实施顺序

推荐按以下顺序小步落地：

1. 先修 run 与请求 context 的解耦
   - 先解决“断流导致 `context canceled -> run.failed`”
   - 暂不要求已经有真实 token stream

2. 引入 stream observer 通道
   - 先打通“瞬时回答流事件”基础设施
   - 允许 `mock` provider 先伪造多段 delta 验证链路

3. 为模型层补 streaming 能力
   - `mock` provider 优先
   - `ark` 再接真实流式能力
   - 保留非流式回退

4. 接 AG-UI 与 Web UI
   - 让前端真正消费多次 `TEXT_MESSAGE_CONTENT`
   - 把“流断开”和“run 失败”分开处理

## 测试要求

第一版至少应覆盖以下场景：

- AG-UI 请求连接断开后，run 不会因为 request context cancel 而失败
- stream observer 能收到多次回答 delta
- EventStore 中不出现回答 delta 持久化事件
- 最终落盘 `assistant.message` 等于所有 delta 拼接结果
- provider 不支持 streaming 时，会正确回退到非流式路径
- provider 在流中失败时，run 失败且不落最终 assistant message
- Web 端接收多次 `TEXT_MESSAGE_CONTENT` 时能持续 append

## 风险与后续扩展位

第一版最主要的风险点：

- observer 双通道引入后，要避免影响 CLI / HTTP 其它既有观察路径
- 最终消息落盘时机如果过早，会出现 UI 看到的文本与持久化结果不一致
- 前端如果把“流断开”继续渲染成“run failed”，产品语义仍然会混乱

后续若要继续扩展，可沿着以下方向推进：

- delta 持久化 replay
- 断线后继续接入同一 run 的后续回答流
- CLI 的回答 token streaming
- tool 输出的实时 streaming
- 更细粒度的流节流 / 聚合策略

但这些都不属于第一版范围。
