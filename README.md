# agent-demo

本项目当前是一个单机本地运行的 Agent Harness 骨架，已经具备：

- Cobra CLI 入口
- 本地 `.runtime/` 工件落盘
- `run / chat / inspect / replay / resume / tools list / debug events` 基础命令
- Ark / Mock provider
- 最小 plan-driven loop：`planning -> memory recall -> context build -> prompt build -> model -> tool -> result`
- 基于 `Session` 的本地多轮对话

## 快速开始

先查看 CLI：

```bash
make help
```

创建一次本地 run：

```bash
make run ARGS='verify scaffold runtime'
```

在已有 session 上追加一轮输入：

```bash
make run SESSION=<session-id> ARGS='继续刚才的话题'
```

启动本地交互式多轮对话：

```bash
make chat PROVIDER=mock
```

如果你想强制使用 mock provider：

```bash
make run ARGS='say hello from mock provider' PROVIDER=mock
```

查看工具列表：

```bash
make tools
```

查看某个 run：

```bash
make inspect RUN=<run-id>
```

回放某个 run 的事件：

```bash
make replay RUN=<run-id>
```

查看某个 run 的原始事件：

```bash
make debug-events RUN=<run-id>
```

## 环境变量

真实调用 Ark 时需要提供：

```bash
export ARK_API_KEY=your_api_key
export ARK_BASE_URL=https://ark.cn-beijing.volces.com/api/v3
export ARK_MODEL_ID=your_model_id
```

可选变量：

```bash
export HARNESS_PROVIDER=ark
export HARNESS_MODEL=$ARK_MODEL_ID
```

如果只是本地验证流程，可以直接切到 mock：

```bash
make run ARGS='请读取 README.md 并总结当前项目状态' PROVIDER=mock
```

## 最小闭环示例

1. 发起一次运行：

```bash
make run ARGS='请读取 README.md 并总结当前项目状态' PROVIDER=mock
```

2. 记下输出里的 `run.id`，然后查看运行工件：

```bash
make inspect RUN=<run-id>
make replay RUN=<run-id>
make debug-events RUN=<run-id>
make resume RUN=<run-id>
```

3. 查看本地工件目录：

```bash
ls .runtime/runs/<run-id>
```

你会看到 `run.json`、`state.json`、`plan.json`、`events.jsonl`、`result.json`、`summaries.json`、`memories.json`。
如果这次运行触发了子代理委派，还会看到 `children/` 目录，里面保存每个 child run 的结构化记录。

4. 启动一个多轮会话：

```bash
make chat PROVIDER=mock
```

你会先看到一行 `session_id: ...`，然后可以连续输入多轮内容。输入 `/exit` 或 `/quit` 结束会话。

5. 如果要继续之前的会话：

```bash
make chat PROVIDER=mock SESSION=<session-id>
```

或者：

```bash
make run PROVIDER=mock SESSION=<session-id> ARGS='补充一个追问'
```

## 说明

- 当前版本会创建 `Task / Session / Run / Plan / State / Events` 工件。
- 当前版本已经支持本地多轮对话：同一个 `Session` 下可以连续创建多个 `Run`。
- 当前版本已经接上最小 plan-driven agent loop：会先生成结构化计划，再进行 memory recall、context build、prompt build，并支持单次文件系统工具调用后生成最终答案。
- 当前版本已经有结构化 memory recall、compaction 和 memory candidate write-back，但还没有接入 sub-agent 主链。
- 当前版本已经支持最小受控 delegation：父 run 可以生成 child run、记录 `subagent.spawned / subagent.completed / subagent.rejected` 事件，并在 `children/` 目录中持久化 child 结果。
- `resume` 现在已经可以基于持久化的 `run.json`、`state.json`、`plan.json` 继续执行未完成运行。
- 所有运行工件默认写入仓库根目录下的 `.runtime/`。
- session 级消息历史会保存在 `.runtime/sessions/<session-id>/messages.jsonl`，并以最近消息的形式注入后续轮次的上下文。
