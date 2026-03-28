## ADDED Requirements

### Requirement: 页面抓取必须阻止本地与内网地址
系统 MUST 为 `web.fetch` 提供目标地址校验，并阻止对本地回环地址、链路本地地址和内网地址的访问。

#### Scenario: 抓取 localhost 或内网地址
- **WHEN** Agent 调用 `web.fetch` 且目标 URL 指向 `localhost`、回环地址或内网地址
- **THEN** 系统 MUST 拒绝执行该抓取
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 不向该目标地址发起实际请求

#### Scenario: 域名解析到受限地址
- **WHEN** Agent 调用 `web.fetch` 且目标域名在解析后落到本地回环地址、链路本地地址或内网地址
- **THEN** 系统 MUST 将该目标视为受限地址
- **THEN** 系统 MUST 拒绝执行该抓取
- **THEN** 系统 MUST 返回结构化错误结果
