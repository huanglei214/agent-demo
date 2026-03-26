## Why

当前 agent 的工具面已经有一个可用雏形，但开始暴露出两类问题：

- 工具集合和我们真正想要的“常用 agent 最小核心能力”还没有对齐
- 少量低频工具和高频缺失工具并存，导致模型经常知道“该做什么”，却没有合适工具去做

具体表现包括：

- `fs.stat` 的价值相对有限，和 `fs.list_dir`、`fs.read_file`、错误反馈存在较强重叠
- 本地编辑场景缺少比整文件重写更安全的 `str_replace`
- 外部信息查询场景缺少标准化 `web.search` 和 `web.fetch`
- 真正执行动作仍缺少受控的 `bash.exec`

为了让 agent 的工具面更接近“日常可用”，需要把核心工具集收敛成一组更清晰、更高频的能力，并用明确的访问级别和输入输出契约描述它们。

## What Changes

- 从核心工具集中移除 `fs.stat`
- 新增 `fs.str_replace`
- 保留并明确 `fs.list_dir`、`fs.read_file`、`fs.write_file`、`fs.search` 的输入输出契约
- 新增外部信息工具：
  - `web.search`
  - `web.fetch`
- 新增命令执行工具：
  - `bash.exec`
- 将工具访问级别从当前的 `read_only | write` 扩展为：
  - `read_only`
  - `write`
  - `exec`
- 明确“核心 8 个 tools”作为后续 agent 默认能力面：
  - `fs.list_dir`
  - `fs.read_file`
  - `fs.write_file`
  - `fs.str_replace`
  - `fs.search`
  - `web.search`
  - `web.fetch`
  - `bash.exec`

## Capabilities

### Modified Capabilities
- `filesystem-tools`: 调整文件系统工具面，移除 `fs.stat`，新增 `fs.str_replace`，并收紧 `fs.search` 与文件操作契约
- `harness-runtime-core`: 工具运行时需要支持新的 `exec` 访问级别，并继续暴露统一工具描述与执行模型

### Added Capabilities
- `web-retrieval-tools`: 提供结构化的 `web.search` 与 `web.fetch`
- `command-execution-tools`: 提供受控的 `bash.exec`

## Impact

- 影响 `internal/tool` 下的工具集合与访问级别定义
- 影响 `internal/app/services.go` 中的工具注册与描述暴露
- 影响提示词中的工具列表和模型可见能力面
- 影响 README 和 Web/CLI 中展示的工具清单
- 不改变当前运行时核心模型、session/run 生命周期或已有 HTTP/AG-UI 总体架构
