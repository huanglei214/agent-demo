## 1. Split Entry Binaries

- [x] 1.1 新增 `cmd/cli/main.go`，承接当前 CLI 可执行入口
- [x] 1.2 新增 `cmd/web/main.go`，承接当前本地 HTTP server 启动逻辑
- [x] 1.3 移除或下线 CLI 中的 `serve` 子命令，避免双重入口语义混淆

## 2. Converge Adapter Packages

- [x] 2.1 规划并落地接口适配层的新目录结构，例如 `internal/interfaces/cli` 与 `internal/interfaces/http`
- [x] 2.2 将当前 `internal/httpapi` 与 `internal/agui` 收敛到新的 HTTP 适配层命名空间
- [x] 2.3 视实现成本决定是否将当前 `internal/cli` 同步迁移到新的 CLI 适配层命名空间

## 3. Preserve Behavior

- [x] 3.1 确保 `app.Services` 和核心 runtime 包不因目录整理而改变职责
- [x] 3.2 确保现有 CLI 命令行为保持兼容
- [x] 3.3 确保现有 HTTP API 合同与前端页面行为保持兼容

## 4. Developer Surface

- [x] 4.1 更新 `Makefile`，使开发命令指向新的双入口
- [x] 4.2 更新 README 和相关文档，说明 `cmd/cli`、`cmd/web` 与新的目录分层
- [x] 4.3 补充或更新测试，覆盖新的入口装配和关键路径

## 5. Verification

- [x] 5.1 运行 Go 测试，确认目录迁移后行为无回退
- [x] 5.2 构建并验证本地 Web UI，确认前端仍能通过本地 HTTP 服务工作
- [x] 5.3 手动验证 CLI 与 Web 两类入口都能独立启动
