# Changelog

## [v0.0.2] - 2026-05-16

### Added
- 并发采集：多个 DSN 并行执行，总耗时约等于最慢库，并输出每个库的耗时统计。
- Panic 隔离：`CollectSafe` 包装器捕获任意 connector 的 panic，转为 error 继续运行，避免单库崩溃导致整体退出。
- 统一错误类型：`schema.DBError` 封装操作上下文（DSN、库名、表名、操作名称），配合 `defer rows.Close()` 实现完整错误链。
- Connector 自注册：`connector/registry.go` 与 `init()` 注册机制，新增数据库无需修改 `connector.go` 的 switch-case，完全遵循开闭原则。
- 字段语义推断：对无注释的列自动获取首行数据，结合 `schema.InferComment` 规则引擎生成语义注释（如“标识符”、“金额/数量”等）。
- 大表/大库进度日志：每个表采集前输出 `[库名] 采集表 X/总数`，缓解大规模库的等待焦虑。
- Redis 生产级分析：
  - 流式聚合：`SCAN` 迭代器边扫边分组，不存储全量 key。
  - Pipeline 批量命令：TYPE、TTL、MEMORY USAGE 一次往返。
  - 安全采样：string 用 `GETRANGE`，hash 用 `HSCAN`，stream 用 `XRANGE` 限制长度。
  - 风险诊断：自动检测无 TTL 敏感键、超大容器、未消费 stream 等。
- `.env` 灵活编号：`loadFromEnv` 扫描所有 `DB<n>` 变量并按数字排序，允许编号跳跃或从任意数字开始。
- 数据库专项文档：`docs/MYSQL.md`、`docs/POSTGRESQL.md`、`docs/CLICKHOUSE.md`、`docs/REDIS.md`、`docs/MONGO.md`、`docs/ELASTICSEARCH.md`，涵盖实现机制、安全策略与排障指南。
- `build.sh` 自动输出二进制到 `db-relationship-explainer/tools/` 目录。

### Changed
- 重构 `connector/connector.go`：移除硬编码 switch-case，改用 `GetConnector` 从注册表查找。
- 优化 Redis 连接器：近乎重写，从全量 key 列表改为流式聚合，增加安全采样与风险检测。
- 增强 MongoDB 连接器：强制要求库名，缩短超时，禁用重试，确保快速失败。
- 统一所有连接器的错误处理：全部返回 error，无忽略；所有查询 defer rows.Close()。
- 改进日志输出：通过 `connector/logf` 从 context 注入 logger，每个 DSN 独立日志文件。

### Fixed
- 修复管道死锁：`captureText` / `captureJSON` 使用 goroutine 异步读取 pipe，解决报告无法输出问题。
- 修复 Bash 历史展开：DSN 密码含 `!` 导致命令解析失败，文档说明使用单引号包裹。
- 修复 Redis 潜在内存风险：限制扫描上限 2000，弃用 `HGETALL`、`GET` 全量读取。
- 修复 ClickHouse 采样 SQL 语法错误（已知局限，不影响核心采集）。
- 修复 `main.go` 编译错误（重复打印完成信息、缺少 `totalTables` 函数等）。

---

## [v0.0.1] - 2026-04-15 (初始版本)

### Added
- 首批数据库支持：MySQL、PostgreSQL、SQLite、ClickHouse、Redis、MongoDB、Qdrant、Elasticsearch。
- 表结构导出、索引、外键采集。
- 跨库/跨实例关系推断。
- 聚类分析和问题诊断（缺主键、未索引外键等）。
- 终端美化输出与 JSON 报告。
- 多 DSN 串联（串行）采集。
- DSN 密码脱敏与日志分离。
