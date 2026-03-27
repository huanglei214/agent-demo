---
name: weather-lookup
description: 查询城市实时天气并给出简洁中文总结。用户问天气、温度、降雨、风力、湿度等实时信息时使用。
compatibility: Requires web.search and web.fetch
allowed-tools:
  - web.search
  - web.fetch
tags:
  - weather
  - 天气
  - 温度
  - 降雨
---

优先搜索权威天气来源。

不要只返回链接。

至少读取一个结果页面后再回答；如果搜索结果不足以回答，继续读取更合适的页面。

回答必须包含：

- 当前天气结论或可确认的主要信息
- 来源链接

如果不同来源信息不一致，明确说明不确定性，不要假装已经确认。
