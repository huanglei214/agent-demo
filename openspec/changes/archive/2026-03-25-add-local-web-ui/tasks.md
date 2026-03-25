## 1. HTTP Server Setup

- [x] 1.1 引入 `chi` 并增加本地 `serve` 入口
- [x] 1.2 新增 `internal/httpapi` 路由层和统一 JSON/错误响应辅助函数
- [x] 1.3 将 HTTP 层接入现有 `app.Services`，避免重复实现运行时逻辑

## 2. First API Slice

- [x] 2.1 实现 `POST /api/sessions` 和 `GET /api/sessions/{id}`
- [x] 2.2 实现 `POST /api/runs`、`POST /api/runs/{id}/resume` 和 `GET /api/runs/{id}`
- [x] 2.3 实现 `GET /api/runs/{id}/replay`、`GET /api/runs/{id}/events` 和 `GET /api/tools`
- [x] 2.4 为 HTTP handlers 补充测试，覆盖成功路径和常见错误路径
- [x] 2.5 实现 `GET /api/runs/{id}/stream` SSE 端点，支持按 sequence 订阅增量事件
- [x] 2.6 实现 `GET /api/sessions` 和 `GET /api/runs` 最近摘要列表接口

## 3. Frontend Scaffold

- [x] 3.1 新建 React + Vite + TypeScript 前端工程
- [x] 3.2 搭建基础页面骨架，至少包含 run 发起页、session 详情页、run 详情页
- [x] 3.3 将前端接到首批 API，并完成本地开发代理配置
- [x] 3.4 在 run 详情页接入 SSE 实时刷新，使时间线和原始事件随增量事件更新
- [x] 3.5 在首页展示可点击的 recent sessions / recent runs 面板
- [x] 3.6 在首页选择 session 后展示最近消息和关联 runs，形成 continuation 面板
- [x] 3.7 增加前端错误边界和恢复入口，避免运行时异常直接白屏
- [x] 3.8 支持中英文切换，并将中文作为前端日常开发的主要体验
- [x] 3.9 为 run 详情页补自动重连和按 sequence 续订，提升 SSE 稳定性

## 4. Documentation & Verification

- [x] 4.1 更新 README，补充 `serve` 和前端本地启动方式
- [x] 4.2 增加至少一条服务端到前端联调验证路径
- [x] 4.3 增加一条 SSE 联调验证路径，确认 run 详情页对应端点会输出增量事件
- [x] 4.4 更新 README，补充 recent sessions / recent runs 首页入口说明
- [x] 4.5 增加 `make dev` 一键联调入口，并同步 README 使用方式
