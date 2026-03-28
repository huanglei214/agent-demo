## MODIFIED Requirements

### Requirement: 文件系统工具边界控制
系统 MUST 将文件系统工具的可操作范围限制在工作区内，并阻止越界访问，包括通过 symlink 解析后的越界路径。

#### Scenario: 工具尝试访问工作区外路径
- **WHEN** 任一文件系统工具收到工作区外的路径参数
- **THEN** 系统 MUST 拒绝执行该操作
- **THEN** 系统 MUST 返回结构化错误信息

#### Scenario: 工具尝试通过 symlink 逃逸工作区
- **WHEN** 任一文件系统工具收到的路径在解析 symlink 后落在工作区外
- **THEN** 系统 MUST 拒绝执行该操作
- **THEN** 系统 MUST 将该路径视为越界访问，而不是工作区内合法路径
- **THEN** 系统 MUST 返回结构化错误信息
