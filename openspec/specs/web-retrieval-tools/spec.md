# web-retrieval-tools

## Purpose
定义 Harness 平台用于查询外部信息的结构化 Web 工具能力，包括搜索入口和页面抓取入口。

## Requirements

### Requirement: 外部搜索入口
系统 MUST 提供 `web.search` 工具，用于查询外部公开信息源并返回结构化搜索结果。

#### Scenario: 查询外部信息
- **WHEN** Agent 调用 `web.search` 并提供查询词
- **THEN** 系统 MUST 返回结构化搜索结果列表
- **THEN** 每个结果 MUST 至少包含标题、URL 和摘要

#### Scenario: 搜索结果受限
- **WHEN** 搜索结果数量超过系统定义的安全上限
- **THEN** 系统 MUST 截断返回结果
- **THEN** 系统 MUST 明确标记该结果已截断

### Requirement: 外部页面抓取入口
系统 MUST 提供 `web.fetch` 工具，用于读取指定 URL 的页面内容。

#### Scenario: 抓取页面正文
- **WHEN** Agent 调用 `web.fetch` 并提供有效 URL
- **THEN** 系统 MUST 返回结构化页面结果
- **THEN** 返回结果 MUST 包含请求 URL
- **THEN** 返回结果 MUST 包含最终 URL
- **THEN** 返回结果 MUST 包含 HTTP 状态码
- **THEN** 返回结果 MUST 包含页面标题或正文摘要

#### Scenario: 抓取结果受限
- **WHEN** 页面正文长度超过系统定义的安全上限
- **THEN** 系统 MUST 截断返回内容
- **THEN** 系统 MUST 明确标记该结果已截断

#### Scenario: 页面抓取失败
- **WHEN** `web.fetch` 因网络错误、超时或非法 URL 失败
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 记录对应的工具失败事件
