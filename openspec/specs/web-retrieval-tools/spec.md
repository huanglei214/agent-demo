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

#### Scenario: 搜索工具自动优先 Tavily
- **WHEN** Agent 调用 `web.search` 且运行环境中配置了 `TAVILY_API_KEY`
- **THEN** 系统 MUST 优先调用 Tavily Search
- **THEN** 在 Tavily 成功时 MUST 返回与现有结构兼容的搜索结果

#### Scenario: Tavily 搜索失败后回退
- **WHEN** Tavily Search 因限流、5xx、超时、传输错误或空结果不可用
- **THEN** 系统 MUST 自动回退到默认搜索实现

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

#### Scenario: 抓取工具自动优先 Tavily
- **WHEN** Agent 调用 `web.fetch`、目标 URL 通过安全校验且运行环境中配置了 `TAVILY_API_KEY`
- **THEN** 系统 MUST 优先调用 Tavily Extract
- **THEN** 在 Tavily 成功时 MUST 返回与现有结构兼容的页面结果

#### Scenario: Tavily 抓取失败后回退
- **WHEN** Tavily Extract 因限流、5xx、超时、传输错误或空结果不可用
- **THEN** 系统 MUST 自动回退到默认页面抓取实现

#### Scenario: 页面抓取失败
- **WHEN** `web.fetch` 因网络错误、超时或非法 URL 失败
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 记录对应的工具失败事件

### Requirement: 页面抓取必须阻止本地与内网地址
系统 MUST 为 `web.fetch` 提供目标地址校验，并阻止对本地回环地址、链路本地地址和内网地址的访问。

#### Scenario: 抓取 localhost 或内网地址
- **WHEN** Agent 调用 `web.fetch` 且目标 URL 指向 `localhost`、回环地址或内网地址
- **THEN** 系统 MUST 拒绝执行该抓取
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 不向该目标地址发起实际请求
- **THEN** 系统 MUST 不因配置了 Tavily 而绕过该限制

#### Scenario: 域名解析到受限地址
- **WHEN** Agent 调用 `web.fetch` 且目标域名在解析后落到本地回环地址、链路本地地址或内网地址
- **THEN** 系统 MUST 将该目标视为受限地址
- **THEN** 系统 MUST 拒绝执行该抓取
- **THEN** 系统 MUST 返回结构化错误结果
