# Tavily Web Tools Design

## Context

仓库当前提供两个 Web 检索工具：

- `web.search`：通过 DuckDuckGo HTML 端点执行公开网页搜索。
- `web.fetch`：对给定 URL 直接发起抓取，并在响应中提取标题和正文。

当前实现没有 Web 检索 provider 抽象，也没有 Tavily 集成。用户希望在不改变工具名和调用方式的前提下，增加 Tavily 支持，并满足以下运行约束：

- 只要检测到 `TAVILY_API_KEY`，系统就自动优先使用 Tavily。
- 如果没有配置 `TAVILY_API_KEY`，系统维持现有行为。
- 如果 Tavily 限流或失败，系统自动回退到现有实现。
- `web.fetch` 的本地地址和内网地址拦截能力必须保留，不能因为 Tavily 集成被绕过。

## Goals

- 为 `web.search` 增加 Tavily Search 集成。
- 为 `web.fetch` 增加 Tavily Extract 集成。
- 保持现有工具名、调用输入和返回结构兼容。
- 在 Tavily 不可用时自动回退，避免破坏当前 chat-first 使用体验。
- 保持现有安全边界，尤其是 `web.fetch` 的公网地址校验。

## Non-Goals

- 这次不新增面向用户的 `web provider` 显式配置项。
- 这次不新增新的工具名，如 `web.search_tavily` 或 `web.fetch_tavily`。
- 这次不修改 retrieval guard、prompt tool surface 或 skill 元数据。
- 这次不引入新的通用外部 provider 框架；仅在 `internal/tool/web` 内部完成最小必要抽象。

## User-Visible Behavior

### `web.search`

`web.search` 继续接受：

```json
{
  "query": "武汉天气",
  "limit": 5
}
```

并继续返回：

```json
{
  "query": "武汉天气",
  "results": [
    {
      "title": "示例标题",
      "url": "https://example.com",
      "snippet": "摘要"
    }
  ],
  "truncated": false
}
```

行为变更为：

1. 如果进程环境中存在非空 `TAVILY_API_KEY`，先调用 Tavily Search。
2. 如果 Tavily Search 成功并返回至少一个有效结果，则将 Tavily 结果映射为当前结构并直接返回。
3. 如果 Tavily Search 因以下原因失败，则回退到当前 DuckDuckGo 搜索实现：
   - `429 Too Many Requests`
   - 任意 `5xx`
   - 网络错误、超时、连接失败
   - 返回成功但没有可用的结构化结果
4. 如果没有配置 `TAVILY_API_KEY`，直接走当前 DuckDuckGo 实现。

Tavily 到当前输出结构的映射规则：

- `title` -> `title`
- `url` -> `url`
- `content` -> `snippet`

返回结果仍受当前 limit 规范约束，并沿用当前 `truncated` 语义。

### `web.fetch`

`web.fetch` 继续接受：

```json
{
  "url": "https://example.com/weather"
}
```

并继续返回：

```json
{
  "url": "https://example.com/weather",
  "final_url": "https://example.com/weather",
  "status_code": 200,
  "title": "页面标题",
  "content": "提取出的正文",
  "truncated": false
}
```

行为变更为：

1. 无论是否配置 Tavily，先执行现有的 URL 合法性和公网地址校验。
2. 如果 URL 不合法、指向本地回环地址、链路本地地址、私有地址，或解析后落到这些地址，直接拒绝执行，不调用 Tavily，也不调用直连抓取。
3. 如果 URL 校验通过且存在非空 `TAVILY_API_KEY`，先调用 Tavily Extract。
4. 如果 Tavily Extract 成功并返回可用正文，则映射为当前输出结构并返回。
5. 如果 Tavily Extract 因以下原因失败，则回退到当前直连抓取实现：
   - `429 Too Many Requests`
   - 任意 `5xx`
   - 网络错误、超时、连接失败
   - 返回成功但没有可用正文
6. 如果没有配置 `TAVILY_API_KEY`，直接走当前直连抓取实现。

Tavily Extract 到当前输出结构的映射规则：

- `url`：保持为工具输入 URL
- `final_url`：第一版保持为工具输入 URL，不额外暴露 Tavily 内部重定向信息
- `status_code`：Tavily 成功路径统一记为 `200`
- `title`：第一版保留为空字符串
- `content`：取 Tavily 返回的 `raw_content`
- `truncated`：复用当前正文截断逻辑

选择不从 Tavily 结果中强行推导 `title`，是因为 Extract 的核心价值在正文提取，而仓库当前对 `web.fetch` 的下游消费更依赖 `content` 是否有证据价值。

## Design

### Internal structure

在 `internal/tool/web/` 内新增最小 provider 分派结构，但不把它扩展为跨包通用框架。

目标结构：

- `SearchTool`
  - 负责参数解析和 `auto -> tavily first -> fallback` 调度。
  - 内部持有 Tavily 客户端和现有 DuckDuckGo 实现依赖。
- `FetchTool`
  - 负责参数解析、URL 公网校验和 `auto -> tavily first -> fallback` 调度。
  - 内部持有 Tavily 客户端和现有直连抓取依赖。
- `tavilyClient`
  - 负责 Tavily Search / Extract 请求构造、认证头和响应解析。
  - 仅暴露当前工具所需的最小方法。

这次不在 `internal/config` 中增加 `WebConfig`。启用逻辑直接读取 `TAVILY_API_KEY`，因为用户明确要求采用“有 key 自动优先、无 key 自动回退”的策略。这样可以把外显配置变更压到最小。

### Fallback policy

只有以下场景才触发 Tavily -> 旧实现回退：

- Tavily 请求返回 `429`
- Tavily 请求返回 `5xx`
- `http.Client.Do` 返回网络错误
- context deadline exceeded / timeout
- Tavily 成功响应但结果为空，无法生成当前工具所需的有效输出

以下场景不回退，直接报错：

- 用户输入为空
- URL 非绝对 `http(s)` 地址
- `web.fetch` 目标触发本地或内网地址拦截
- Tavily 请求体或响应体本身出现本地代码 bug，例如 JSON 编码失败

这样划分的原因是：回退只处理外部 provider 的可用性问题，不掩盖本地校验失败或实现错误。

### Tavily API usage

第一版按 Tavily 当前公开 API 接入：

- Base URL: `https://api.tavily.com`
- Search endpoint: `POST /search`
- Extract endpoint: `POST /extract`
- Auth: `Authorization: Bearer <TAVILY_API_KEY>`

Search 侧仅请求当前工具需要的最小字段，避免引入与现有输出结构无关的复杂选项。
Extract 侧仅请求给定 URL 的正文提取结果，不引入抓取截图、分页深度等附加能力。

## Error handling

- 如果 Tavily 失败且旧实现成功，则用户只看到成功结果，不暴露中间回退细节。
- 如果 Tavily 和旧实现都失败，则沿用当前工具的错误返回方式，向上层返回最终失败错误。
- 第一版不在工具结果中增加 `provider` 字段，避免破坏现有输出契约。

## Security

`web.fetch` 必须继续在任何网络调用之前执行目标校验：

- 校验输入 URL 必须是绝对 `http(s)` 地址。
- 校验 hostname 不是 `localhost`。
- 校验直接 IP 或域名解析结果不能落到 loopback、private、link-local、unspecified。

即使最终走 Tavily Extract，也必须保留这一步。这样可以保持仓库当前“工具面不允许访问本地和内网地址”的安全边界，而不是把限制转嫁给第三方 provider。

`web.search` 不需要增加额外地址校验，因为它接受的是自然语言查询，不直接接受 URL。

## Testing

采用 TDD 扩展 `internal/tool/web/web_test.go`，覆盖以下行为：

1. `web.search` 在存在 `TAVILY_API_KEY` 时优先使用 Tavily。
2. `web.search` 在 Tavily 返回 `429` 时回退到 DuckDuckGo。
3. `web.search` 在 Tavily 返回空结果时回退到 DuckDuckGo。
4. `web.fetch` 在存在 `TAVILY_API_KEY` 时优先使用 Tavily Extract。
5. `web.fetch` 在 Tavily 返回 `429` 时回退到直连抓取。
6. `web.fetch` 即使配置 Tavily，也会先拒绝 localhost / 内网地址。
7. `web.fetch` 在 Tavily 成功时仍遵守当前正文截断规则。

验证命令以最小范围为主：

```bash
go test ./internal/tool/web ./internal/service
```

如果文档或装配层有同步改动，再扩到：

```bash
go test ./...
```

## Documentation updates

实现时需要同步更新：

- `README.md`
  - 说明 `TAVILY_API_KEY` 的作用
  - 说明 `web.search` / `web.fetch` 会自动优先 Tavily 并在失败时回退
- `config.example.json`
  - 不新增配置字段，但应在相邻说明中明确 Tavily 通过环境变量启用
- `openspec/specs/web-retrieval-tools/spec.md`
  - 补充搜索和抓取的 provider 自动优先与失败回退语义

## Implementation boundaries

这次实现应限制在以下边界内：

- `internal/tool/web/`
- `internal/service/services.go` 如需最小依赖注入调整
- `internal/config/` 仅在确有必要读取环境变量帮助函数时调整，否则不动
- `README.md`
- `openspec/specs/web-retrieval-tools/spec.md`

不应顺带修改：

- prompt builder
- retrieval guard
- delegation policy
- model provider 配置逻辑

## Open questions resolved

- 是否让 Tavily 成为默认显式 provider：否。第一版采用 `TAVILY_API_KEY` 自动探测。
- Tavily 限流后是否直接失败：否。限流后自动回退。
- `web.fetch` 是否因 Tavily 存在而跳过 URL 安全校验：否。始终先校验。
- 是否增加新的工具名或输出字段：否。保持兼容。
