# command-execution-tools

## Purpose
定义 Harness 平台用于执行本地命令的受控执行工具能力。

## Requirements

### Requirement: 受控命令执行入口
系统 MUST 提供 `bash.exec` 工具，用于在受控工作目录中执行命令。

#### Scenario: 成功执行命令
- **WHEN** Agent 调用 `bash.exec` 并提供 `command` 与 `workdir`
- **THEN** 系统 MUST 在指定工作目录执行该命令
- **THEN** 系统 MUST 返回结构化的 `exit_code`、`stdout` 和 `stderr`

#### Scenario: 非零退出码
- **WHEN** `bash.exec` 执行的命令返回非零退出码
- **THEN** 系统 MUST 保留该退出码
- **THEN** 系统 MUST 以结构化结果返回 `stdout` 和 `stderr`
- **THEN** 系统 MUST 不因为非零退出码而丢失命令输出

#### Scenario: 命令执行超时
- **WHEN** `bash.exec` 超过请求指定或系统默认的超时时间
- **THEN** 系统 MUST 终止该命令
- **THEN** 系统 MUST 返回超时错误
- **THEN** 系统 MUST 为该错误提供结构化超时细节，例如命令、工作目录和超时标记

#### Scenario: 输出被截断
- **WHEN** 命令输出超过系统定义的安全上限
- **THEN** 系统 MUST 截断 `stdout` 或 `stderr`
- **THEN** 系统 MUST 明确标记该结果已截断
