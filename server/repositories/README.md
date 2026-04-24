# Repository Layer

Repository 是服务层和数据存储之间的边界。业务代码不直接读写 MySQL，而是通过 Repository 完成数据访问。

## 设计目标

- 让 `services` 保持业务语义，不散落 SQL、文件路径或序列化细节。
- 兼容历史数据迁移背景，但当前主存储目标是 MySQL。
- 方便测试，业务层可以注入 mock repository，不依赖真实数据库。
- 统一处理查询超时、事务、分页、错误映射和敏感字段脱敏。

## 命名建议

- `ProjectRepository`
- `UserRepository`
- `SandboxAccountRepository`
- `ExecutionReportRepository`
- `TestcaseHistoryRepository`

每个 Repository 可以有多个实现，例如：

- `ProjectRepository`
- `MySQLProjectRepository`

服务层只依赖接口，不依赖具体实现。
