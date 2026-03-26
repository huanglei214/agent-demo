## Overview

这次变更的目标不是“多加几个工具”，而是把 agent 的默认工具面收敛成一组高频、稳定、边界清晰的核心能力。

本次设计采用三个原则：

1. 核心工具面优先
2. 输入输出结构化优先
3. 高风险能力后置但契约先明确

最终的目标核心集为：

- `fs.list_dir`
- `fs.read_file`
- `fs.write_file`
- `fs.str_replace`
- `fs.search`
- `web.search`
- `web.fetch`
- `bash.exec`

## Goals

- 让 agent 具备“本地读取 + 本地编辑 + 外部查询 + 真正执行”的最小闭环
- 保留当前文件系统工具中真正高频的能力
- 将工具描述中的访问级别扩展到 `exec`
- 为接下来真正落地 `web.*` 和 `bash.exec` 做好规格边界

## Non-Goals

- 本次不引入第三方工作流工具，例如 Notion、Slack、Issue 系统
- 本次不引入数据库/SQL 工具
- 本次不引入浏览器自动化
- 本次不改变 run/session/event 运行时模型

## Tool Surface Decisions

### 1. 移除 `fs.stat`

`fs.stat` 的功能主要是查看路径元信息，但在当前项目设定下，这类能力与以下能力重叠较大：

- `fs.list_dir`
- `fs.read_file`
- `bash.exec`

因此本次将 `fs.stat` 从核心工具面中移除，避免模型把注意力浪费在低频辅助工具上。

### 2. 新增 `fs.str_replace`

`fs.write_file` 适合创建或整体覆盖文件，但对局部编辑场景不够安全。

因此新增 `fs.str_replace`，用于：

- 精确替换一段文本
- 在找不到目标文本时返回错误
- 降低整文件重写的风险

这一工具将成为默认文件编辑路径中的首选工具。

### 3. 保留 `fs.search`

`fs.search` 仍然属于高频核心工具，因为它直接决定 agent 是否能快速定位文件和内容。

本次将明确它的契约：

- 输入接受 `pattern`
- 为兼容模型误差，也可接受 `query`
- 空查询必须拒绝
- 结果必须截断
- 结果只返回结构化匹配项，不直接返回大段文件内容

### 4. 新增 `web.search` 与 `web.fetch`

`web.search` 和 `web.fetch` 必须配套出现：

- `web.search` 负责发现信息源
- `web.fetch` 负责读取指定 URL 的正文或摘要

如果只提供其中一个，agent 的联网认知链路都会不完整。

### 5. 新增 `bash.exec`

`bash.exec` 是核心工具面中的高风险能力，但对“真正执行动作”必不可少。

它将遵守这些约束：

- 必须显式指定 `command`
- 必须显式指定 `workdir`
- 必须显式指定或使用默认 `timeout_seconds`
- 输出必须可截断
- 需要返回 `exit_code`、`stdout`、`stderr`
- 第一版不支持交互式命令

## Access Model

当前工具访问级别是：

- `read_only`
- `write`

本次扩展为：

- `read_only`
- `write`
- `exec`

语义如下：

- `read_only`: 只读、不改变工作区或外部系统
- `write`: 修改工作区内容
- `exec`: 执行命令，可能产生副作用

## Capability Mapping

### 文件系统工具

- `fs.list_dir`: 目录结构浏览
- `fs.read_file`: 文件内容读取
- `fs.write_file`: 文件创建/整体覆盖
- `fs.str_replace`: 文件局部替换
- `fs.search`: 工作区搜索

### 外部信息工具

- `web.search`: 查询外部信息入口
- `web.fetch`: 读取指定页面内容

### 执行动作工具

- `bash.exec`: 在受控目录中执行命令

## Rollout Order

实现顺序建议如下：

1. 调整 `filesystem-tools` 主工具面
2. 新增 `fs.str_replace`
3. 新增 `web.search`
4. 新增 `web.fetch`
5. 新增 `bash.exec`
6. 更新提示词、工具描述、README 和测试

## Verification Strategy

至少覆盖以下验证：

- 工具注册表中不再暴露 `fs.stat`
- 工具注册表中暴露 `fs.str_replace`
- `fs.search` 拒绝空查询并截断结果
- `web.search` 返回结构化搜索结果
- `web.fetch` 返回结构化页面内容和截断标记
- `bash.exec` 返回结构化退出码和输出
- 工具列表中的访问级别正确包含 `exec`
