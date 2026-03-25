## Why

当前 Harness 只能通过 `harness run <instruction>` 处理一次性输入，用户无法围绕同一个 `Session` 进行连续对话，也无法把上一轮的用户消息和助手回复稳定带入下一轮上下文。这会限制本地学习和调试体验，因为很多真实 Agent 使用方式都依赖多轮澄清、追问和连续任务推进。

## What Changes

- 增加本地 CLI 多轮对话模式，支持在同一个 `Session` 中连续发送多轮用户消息。
- 增加会话级消息持久化，保存 `user` / `assistant` 消息轨迹，并将其纳入上下文构建。
- 新增 `harness chat` 交互式命令，并支持通过 `--session` 继续已有会话。
- 扩展 `harness run`，允许通过 `--session` 将单次输入追加到已有会话。
- 保持当前单机本地、文件型工件和 event-first 运行模型，不引入 HTTP 或消息代理能力。

## Capabilities

### New Capabilities
- `multi-turn-chat`: 定义本地 CLI 多轮会话、消息持久化、会话续聊和交互式聊天命令的行为契约。

### Modified Capabilities
- `harness-runtime-core`: 扩展运行时与 CLI 入口，使 `Session` 能承载多次用户输入与多次 `Run`。
- `planning-context-memory`: 扩展上下文组装，使最近会话消息历史可以被稳定注入模型上下文。

## Impact

- 影响 `internal/runtime`、`internal/app`、`internal/cli`、`internal/context`、`internal/store/filesystem` 等核心模块。
- 需要为 `Session` 增加消息持久化工件，并为事件流增加 `user.message` / `assistant.message` 等会话级事件。
- 需要新增 `harness chat` 命令和 `--session` 续聊能力。
- README 和 Makefile 需要补充多轮对话的使用说明。
