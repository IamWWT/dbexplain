# Changelog

## [v0.0.3] - 2026-05-19

### Added
- Redis 集群模式支持：通过 `?cluster=true` DSN 参数启用，使用 `ForEachMaster` 扫描所有分片，聚合各分片 keyspace 统计。
- Elasticsearch HTTPS 支持：新增 `elasticsearchs://` scheme 和 `?tls=true` 参数启用 TLS（`InsecureSkipVerify`）。
- DSN 过滤功能：新增 `-include` / `-exclude` CLI 参数，支持按数据库类型、标签、实例编号（如 `DB1,DB3`）过滤采集范围。
- DSN 结构体扩展：新增 `Cluster`、`TLS`、`SSLMode` 字段。
- `--version` 命令行参数，通过 `-ldflags -X main.version` 在编译时注入版本号。
- PostgreSQL 多 schema 支持：自动采集所有非系统 schema（`pg_namespace`），非 public 表名以 `schema.table` 格式展示。
- PostgreSQL SSL 支持：通过 DSN `?sslmode=<mode>` 参数配置 SSL 模式（`disable`/`require`/`verify-ca`/`verify-full`）。
- PostgreSQL 表行数统计：联表 `pg_stat_user_tables` 获取 `n_live_tup`，`pg_total_relation_size` 获取表体积。
- 单元测试覆盖：新增 `dsn/dsn_test.go`（ParseDSN + Redacted，25 用例）和 `schema/infer_test.go`（InferComment，30+ 用例）。
- CI/CD 流水线：`.github/workflows/ci.yml`（go build / go vet / go test）。
- 项目宪法 (`CONSTITUTION.md`) 和记忆文件 (`MEMORY.md`)。
- Skill 一键安装/卸载脚本 (`db-relationship-explainer/install_skill_for_all_platform.sh`, `uninstall_skill_for_all_platform.sh`)：交互式安装到全局目录（`~/.claude/skills`、`~/.deepseek/skills`、`~/.agents/skills`、`~/.aixcoding/skills`）或项目本地目录，自动检测平台并选择对应二进制，支持全平台 symlink 共享安装。内置 `--verify` [dir] 闭环验证、`--update` [dir] 在线升级（支持自定义目录，保留 `.env`），安装/卸载全程 `.env` 凭据安全提示。

### Fixed
- 修复 ClickHouse `fetchCHSampleRow` 和 `queryRows` 双重追加 `FORMAT JSONCompact` 导致采样查询语法错误。
- 修复 `go vet` 警告：`elasticsearch.go` 非恒定格式字符串。
- 修复 `main.go` 两处错误日志泄漏明文密码（改为 `parsed.Redacted()` 脱敏输出）。
- 修复 `render.go` JSON 输出缺少完整 schema 元数据（列、索引、外键、引擎、分区键等），改用 `json.MarshalIndent` 序列化。
- 修复 `mysql.go` 和 `postgres.go` 4 处索引/主键/外键查询静默吞错（`if err == nil { ... }` 无 else 分支）。
- 修复所有 DSN 采集失败时无明确警告提示。
- 修复 `fillMySQLTable` 列查询失败导致整个数据库被跳过（对齐 postgres 的 log-and-continue 策略）。
- 修复 `schema/infer.go` 中 `Contains("ip")` 误匹配 "description" 等词为 IP 地址。
- 修复 `fetchMySQLSampleRow` / `fetchPGSampleRow` 中 `[]byte` 值被格式化为 `[97 105 ...]` 字节数组而非可读字符串。

### Changed
- 更新 `docs/CLICKHOUSE.md`：修正双重 FORMAT bug 的错误归因分析。
- 更新 `docs/REDIS.md`：新增集群模式使用文档。
- 更新 `docs/ELASTICSEARCH.md`：新增 HTTPS/TLS 使用文档。
- 更新 `SKILL.md`：新增 `--version`、Redis 集群、ES TLS、PostgreSQL SSL/Schema 参数文档。
- 新增 `issues.json` 追踪所有已知问题（21 条，20 closed，1 pending-evaluation）。

---

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
- 优化更新SKILL.md

### Fixed
- 修复管道死锁：`captureText` / `captureJSON` 使用 goroutine 异步读取 pipe，解决报告无法输出问题。
- 修复 Bash 历史展开：DSN 密码含 `!` 导致命令解析失败，文档说明使用单引号包裹。
- 修复 Redis 潜在内存风险：限制扫描上限 2000，弃用 `HGETALL`、`GET` 全量读取。
- 修复 ClickHouse 采样 SQL 语法错误（已知局限，不影响核心采集）。
- 修复 `main.go` 编译错误（重复打印完成信息、缺少 `totalTables` 函数等）。

---

## [v0.0.1] - 2026-05-15 (初始版本)

### Added
- 首批数据库支持：MySQL、PostgreSQL、SQLite、ClickHouse、Redis、MongoDB、Qdrant、Elasticsearch。
- 表结构导出、索引、外键采集。
- 跨库/跨实例关系推断。
- 聚类分析和问题诊断（缺主键、未索引外键等）。
- 终端美化输出与 JSON 报告。
- 多 DSN 串联（串行）采集。
- DSN 密码脱敏与日志分离。
