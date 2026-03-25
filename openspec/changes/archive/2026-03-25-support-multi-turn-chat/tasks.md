## 1. 运行时模型与存储扩展

- [x] 1.1 定义 `SessionMessage` 结构体和消息角色枚举
- [x] 1.2 为 `.runtime/sessions/<session-id>/messages.jsonl` 增加路径工具
- [x] 1.3 为 `StateStore` 或独立消息存储增加追加、读取 session messages 的能力
- [x] 1.4 为消息存储编写单元测试

## 2. 应用层多轮会话能力

- [x] 2.1 扩展 `RunRequest`，支持显式传入 `session_id`
- [x] 2.2 调整 `StartRun`，在指定 `session_id` 时复用已有 session，而不是总是创建新 session
- [x] 2.3 在每轮执行开始时保存 `user` 消息并写入 `user.message` 事件
- [x] 2.4 在每轮执行结束时保存 `assistant` 消息并写入 `assistant.message` 事件
- [x] 2.5 为基于已有 session 的单轮续聊编写应用层测试

## 3. 上下文与 Prompt 集成

- [x] 3.1 扩展 `ContextManager.Build` 输入，支持注入 session message 历史
- [x] 3.2 实现“最近 N 条消息”读取策略，并保持消息顺序
- [x] 3.3 将会话消息历史加入 prompt/context 渲染结果
- [x] 3.4 为多轮上下文构建编写测试

## 4. CLI 多轮对话入口

- [x] 4.1 为 `harness run` 增加 `--session` 参数
- [x] 4.2 新增 `harness chat` 命令，支持启动新会话
- [x] 4.3 新增 `harness chat --session <session-id>`，支持继续已有会话
- [x] 4.4 为 chat REPL 实现 `/exit` 和 `/quit` 退出命令
- [x] 4.5 为 CLI 多轮命令编写测试

## 5. 文档与示例

- [x] 5.1 更新 README，加入 `harness chat` 和 `harness run --session` 的使用说明
- [x] 5.2 增加最小多轮对话示例，说明 session 续聊方式
