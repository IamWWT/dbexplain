# Docs-Code Index

快速定位：问题/功能 → 文档 → 源码。

---

## 1. 模块 ↔ 文件映射

| 模块 | 包/目录 | 源码文件 | 文档 |
|------|---------|---------|------|
| **入口/CLI** | `main` | `src/cmd/dbexplain/main.go` | — |
| **查询执行** | `main` | `src/cmd/dbexplain/execute.go` | `docs/EXECUTE.md`, `docs/CLI_EXAMPLES.md` |
| **DSN 解析** | `internal/dsn` | `src/internal/dsn/dsn.go`, `src/internal/dsn/dsn_test.go` | `docs/CONFIG_SEARCH.md` |
| **配置管理** | `internal/config` | `src/internal/config/config.go` | `docs/CONFIG_SEARCH.md` |
| **Schema 数据模型** | `schema` | `src/internal/schema/types.go`, `src/internal/schema/errors.go`, `src/internal/schema/infer.go`, `src/internal/schema/infer_test.go` | — |
| **Connector 接口** | `connector` | `src/internal/connector/connector.go` (接口), `src/internal/connector/registry.go` (注册表), `src/internal/connector/runner.go` (Panic保护), `src/internal/connector/query.go` (SQL执行共享实现) | — |
| **DuckDB** | `connector` | `src/internal/connector/duckdb.go` | `docs/databases/relational/DUCKDB.md` |
| **MySQL** | `connector` | `src/internal/connector/mysql.go` | `docs/databases/relational/MYSQL.md` |
| **PostgreSQL** | `connector` | `src/internal/connector/postgres.go` | `docs/databases/relational/POSTGRESQL.md` |
| **SQLite** | `connector` | `src/internal/connector/sqlite.go` | `docs/databases/relational/SQLITE.md` |
| **ClickHouse** | `connector` | `src/internal/connector/clickhouse.go` | `docs/databases/analytical/CLICKHOUSE.md` |
| **Redis** | `connector` | `src/internal/connector/redis.go` | `docs/databases/nosql/REDIS.md` |
| **Elasticsearch** | `connector` | `src/internal/connector/elasticsearch.go` | `docs/databases/nosql/ELASTICSEARCH.md` |
| **MongoDB** | `connector` | `src/internal/connector/mongo.go` | `docs/databases/nosql/MONGO.md` |
| **Qdrant** | `connector` | `src/internal/connector/qdrant.go` | `docs/databases/nosql/QDRANT.md` |
| **Oracle** | `connector` | `src/internal/connector/oracle.go` | `docs/databases/relational/ORACLE.md` |
| **Hive** | `connector` | `src/internal/connector/hive.go` | `docs/databases/analytical/HIVE.md` |
| **Prometheus** | `connector` | `src/internal/connector/prometheus.go` | `docs/databases/prometheus.md` |
| **CSV/TSV** | `connector` | `src/internal/connector/csv.go`, `src/internal/connector/csv_test.go` | `docs/file-sources/FILE_PROCESSING.md` |
| **XLSX** | `connector` | `src/internal/connector/xlsx.go` | `docs/file-sources/FILE_PROCESSING.md` |
| **类型推断** | `connector` | `src/internal/connector/infer.go` | `docs/file-sources/FILE_PROCESSING.md` |
| **能力声明** | `capabilities` | `src/internal/capabilities/capabilities.go` | `docs/ALGORITHMS.md` |
| **策略引擎** | `policy` | `src/internal/policy/policy.go`, `src/internal/policy/policy_test.go` | `docs/POLICY.md` |
| **SQL 只读校验** | `sqlguard` | `src/internal/sqlguard/sqlguard.go`, `src/internal/sqlguard/sqlguard_test.go` | `docs/EXECUTE.md` |
| **查询类型** | `internal/query` | `src/internal/query/types.go`, `src/internal/query/query_test.go` | `docs/EXECUTE.md` |
| **执行引擎** | `internal/executor` | `src/internal/executor/executor.go`, `src/internal/executor/executor_test.go` | `docs/EXECUTE.md` |
| **SQL AST** | `sqlast` | `src/internal/sqlast/types.go`, `lexer.go`, `parser.go` | — |
| **DSL 查询** | `dsl` | `src/internal/dsl/ast.go`, `preprocess.go`, `parser.go`, `binder.go`, `compiler.go` | `docs/EXECUTE.md` |
| **关系分析/图** | `analyze` | `src/internal/analyze/analyze.go`, `src/internal/analyze/ranking.go` | `docs/ALGORITHMS.md` |
| **诊断** | `diagnostics` | `src/internal/diagnostics/diagnostics.go` | `docs/ALGORITHMS.md` |
| **缓存/增量扫描** | `cache` | `src/internal/cache/cache.go` | `docs/ALGORITHMS.md` |
| **字段级 Schema 对比** | `diff` | `src/internal/diff/diff.go`, `src/internal/diff/types.go`, `src/internal/diff/diff_test.go` | — |
| **上下文压缩** | `context` | `src/internal/context/compress.go` | `docs/ALGORITHMS.md` |
| **渲染输出** | `render` | `src/internal/render/render.go` | — |
| **指标采集** | `metrics` | `src/internal/metrics/collect.go` | `docs/operations/metrics.md` |
| **加密** | `crypto` | `src/internal/crypto/crypto.go`, `src/internal/crypto/fingerprint*.go` | `docs/CONFIG_SEARCH.md` |
| **编码处理** | `main` | `src/cmd/dbexplain/encode.go`, `src/cmd/dbexplain/encode_windows.go` | — |
| **构建** | — | `src/build.sh` | — |

---

## 2. Capability 矩阵（connector → 能力 → 源码）

| Connector | CapSQL | CapFile | CapFK | CapIndex | CapRowCount | CapSampling | CapTTL | CapPartition | CapVector | CapPromQL |
|-----------|--------|---------|-------|----------|-------------|-------------|--------|--------------|-----------|-----------|
| MySQL | ✓ | — | ✓ | ✓ | ✓ | ✓ | — | — | — | — |
| PostgreSQL | ✓ | — | ✓ | ✓ | ✓ | ✓ | — | — | — | — |
| SQLite | ✓ | — | ✓ | — | — | ✓ | — | — | — | — |
| DuckDB | ✓ | — | ✓ | ✓ | ✓ | ✓ | — | — | — | — |
| ClickHouse | ✓ | — | — | — | ✓ | ✓ | — | ✓ | — | — |
| Redis | — | — | — | — | — | ✓ | ✓ | — | — | — |
| ES | ✓ | — | — | ✓ | — | — | — | — | — | — |
| MongoDB | — | — | — | — | ✓ | — | — | — | — | — |
| Qdrant | — | — | — | — | ✓ | — | — | — | ✓ | — |
| CSV/TSV | — | ✓ | — | — | ✓ | — | — | — | — | — |
| XLSX | — | ✓ | — | — | ✓ | — | — | — | — | — |
| Prometheus | — | — | — | — | ✓ | — | — | — | — | ✓ |

定义位置：`src/internal/capabilities/capabilities.go:17-52`

---

## 3. 文档 ↔ 源码映射（按文档）

| 文档 | 对应的主要源码 | 关键内容 |
|------|--------------|---------|
| **文档索引（入口）** | — | `docs/CODE_MAP.md` — 本文件 | 文档-代码双向索引，新增/修改文档或代码时须同步更新 |
| `ALGORITHMS.md` | `analyze/`, `diagnostics/`, `cache/`, `context/`, `capabilities/`, `connector/redis.go` | 命名推断、图聚类、重要性评分、Redis 风险诊断、Capability 系统 |
| `ARCHITECTURE.md` | 全局 | 架构愿景、Phase 路线图、目录结构 |
| `EXECUTE.md` | `cmd/dbexplain/execute.go`, `sqlguard/`, `internal/query/`, `internal/executor/`, `policy/` | 三层安全、AutoLimit、sqlguard 动词白名单 |
| `USAGE_GUIDE.md` | 无（综合用法文档） | 傻瓜用法手册：从下载到查询的 5 分钟完整流程 |
| `CLI_EXAMPLES.md` | `execute.go`, `repl.go` | 15+ 条实测查询案例（含 REPL 全流程） |
| `REPL.md` | `cmd/dbexplain/repl.go` | REPL 交互模式：启动/命令/安全/限制/测试结果 |
| `POLICY.md` | `policy/policy.go`, `policy/policy_test.go` | DENY_TABLES/COLUMNS/STATEMENTS + MASK_COLUMNS |
| `CONFIG_SEARCH.md` | `cmd/dbexplain/main.go`, `internal/dsn/dsn.go`, `crypto/` | 7 级路径搜索、加密配置自动解密 |
| `DEPLOY.md` | `build.sh`, `dbexplain-skill/scripts/` | 安装部署、Skill 集成 |
| `SECURITY_CHECKLIST.md` | 全局 | 发布前安全检查清单 |
| `FILE_PROCESSING.md` | `connector/csv.go`, `connector/xlsx.go`, `connector/infer.go` | CSV/TSV/XLSX 处理细节 |
| `MYSQL.md` | `connector/mysql.go` | MySQL 专项采集手册 |
| `POSTGRESQL.md` | `connector/postgres.go` | PostgreSQL 专项采集手册 |
| `SQLITE.md` | `connector/sqlite.go` | SQLite 专项采集手册 |
| `CLICKHOUSE.md` | `connector/clickhouse.go` | ClickHouse 专项采集手册 |
| `REDIS.md` | `connector/redis.go` | Redis 专项采集手册 |
| `ELASTICSEARCH.md` | `connector/elasticsearch.go` | ES 专项采集手册 |
| `MONGO.md` | `connector/mongo.go` | MongoDB 专项采集手册 |
| `QDRANT.md` | `connector/qdrant.go` | Qdrant 专项采集手册 |
| `prometheus.md` | `connector/prometheus.go` | Prometheus 时序数据库连接器：targets/labels/metrics 采集 + PromQL 查询 |
| `metrics.md` | `metrics/collect.go` | 采集指标收集与 Prometheus 文本格式输出 |
| `GAUSSDB.md` | `connector/gaussdb.go` + `connector/postgres.go` | GaussDB（PG 协议兼容），独立 gaussdbConnector，独立 DSN 构建器 buildGaussDBDSN()，复用 collectPGDB() 采集逻辑 |
| `gaussdb.md` | `connector/gaussdb.go` | GaussDB 兼容性指南（Oracle 模式限制说明） |
| `COMPATIBILITY_GAUSSDB_TDSQL.md` | 无代码变更 | TDSQL（MySQL 兼容）兼容性验证记录 |

---

## 4. 4-Stage Pipeline 文件映射

```
INPUT ──→ COLLECT ──→ ANALYZE ──→ OUTPUT
 │          │            │            │
 │    src/internal/   src/internal/  src/internal/
 │    connector/      analyze/       render/
 │    registry.go     analyze.go     render.go
 │    runner.go       ranking.go
 │    mysql.go etc.   src/internal/
 │                    schema/
 │                    types.go
 │                    infer.go
 │
 src/cmd/dbexplain/   src/internal/   JSON/--human
 main.go              diagnostics/    --context/--cache
 execute.go           diagnostics.go
 src/internal/dsn/    src/internal/
   dsn.go               query/
                        executor/
                        dsl/
                        diff/
                       sqlast/

---

## 5. CLI 子命令 → 处理函数

| 子命令 | 源码位置 | 说明 |
|--------|---------|------|
| `dbexplain` (默认) | `cmd/dbexplain/main.go` → `handleCollect()` | Schema 采集 |
| `dbexplain execute` | `cmd/dbexplain/execute.go` → `handleExecute()` | 只读查询执行（原生 SQL + `--dsl` DSL 模式） |
| `dbexplain repl` | `cmd/dbexplain/repl.go` → `handleREPL()` | 交互式 REPL 查询模式 |
| `dbexplain collect` | `cmd/dbexplain/main.go` → `handleCollect()` | 显式 Schema 采集子命令 |
| `dbexplain diff` | `cmd/dbexplain/main.go` → `handleDiff()` | Schema 变更对比 |
| `dbexplain list` | `internal/list/handler.go` → `handleList()` | 列出已配置 DSN |
| `dbexplain check` | `internal/check/handler.go` → `Handle()` | 配置验证+连通性检测 |
| `dbexplain all` | `internal/manual/` → `handleManual()` | 全部参考手册 |
| `dbexplain <dbtype>` | `internal/manual/` → `handleManual()` | 专项手册（mysql/redis/...） |
| `dbexplain encrypt` | `internal/encrypt/handler.go` → `handleEncrypt()` | 配置加密 |
| `dbexplain version` | `version` package → `--version` flag | 版本信息 |

---

## 6. 测试文档映射

| 测试文档 | 覆盖内容 | 关联源码 |
|---------|---------|---------|
| `docs/test/RESULTS.md` | 测试结果报告 v0.1.7 | — |
| `docs/test/01-environment.md` | 环境准备、DSN 模板 | `internal/dsn/dsn.go` |
| `docs/test/02-schema-collection.md` | 全数据源 Schema 采集 | 所有 `connector/*.go` |
| `docs/test/03-execute-sql.md` | SQL 查询执行 | `execute.go`, `sqlguard/` |
| `docs/test/04-execute-nosql.md` | 非 SQL 查询执行 | `connector/redis.go`, `connector/mongo.go` 等 |
| `docs/test/05-file-processing.md` | CSV/TSV/XLSX | `connector/csv.go`, `connector/xlsx.go` |
| `docs/test/06-security-sqlguard.md` | sqlguard 只读校验 | `sqlguard/sqlguard.go` |
| `docs/test/07-policy-engine.md` | 策略引擎绕过测试 | `policy/policy.go` |
| `docs/test/08-concurrent-limit.md` | 并发限制 | `connector/runner.go`, `internal/query/types.go` |
| `docs/test/09-cli-help.md` | CLI 帮助、子命令、REPL | `main.go`, `repl.go` |
| `docs/test/10-regression.md` | 回归测试 | 全局 |
| `docs/test/11-end-to-end.md` | 端到端集成测试 | 全局 |
| `docs/test/12-capability-routing.md` | CapSQL 路由、PostgreSQL 多 Schema | `capabilities/`, `connector/postgres.go` |
| `docs/test/14-schema-diff.md` | Schema Diff 24 项 | `internal/diff/` |
| `docs/test/15-window-functions.md` | 窗口函数 36 项 | `connector/filequery/` |
| `docs/test/16-duckdb.md` | DuckDB 连接器 20 项 | `connector/duckdb.go` |
| `docs/test/17-metrics.md` | 采集指标收集 5 项 | `metrics/collect.go` |
| `docs/test/18-prometheus.md` | Prometheus 连接器 11 项 | `connector/prometheus.go`, `dsl/` |
| `docs/test/19-oracle-hive.md` | Oracle + Hive 连接器 | `connector/oracle.go`, `connector/hive.go` |
| `docs/test/20-cte-write-detection.md` | CTE 写检测加固 | `sqlguard/sqlguard.go` |
| `docs/test/21-check-command.md` | check 子命令 | `internal/check/handler.go` |
| `docs/test/22-performance-optimization.md` | Schema 采集性能优化 | `connector/postgres.go`, `connector/mysql.go`, `connector/csv.go` |
| `docs/test/23-password-special-chars.md` | DSN 密码特殊字符 | `internal/dsn/dsn.go` |

---

## 7. 常见问题定位指南

| 问题/需求 | 先看什么文档 | 再看什么源码 |
|-----------|-------------|-------------|
| 新手入门 / 不知道怎么用 | `USAGE_GUIDE.md`（傻瓜用法手册） | 按步骤操作即可 |
| 新增数据库类型 | `CONSTITUTION.md` 原则 1 | `connector/registry.go`, `capabilities/capabilities.go` |
| Schema 采集结果不对 | 对应 DB 专项文档 | `connector/<db>.go` 中的 `Collect()` |
| 查询被误拦截 | `EXECUTE.md`, `POLICY.md` | `sqlguard/sqlguard.go` (readOps), `policy/policy.go` |
| `DENY_TABLES` 不生效 | `POLICY.md` (排障参考) | `policy/policy.go` → `CheckSQL()` |
| JSON 输出格式 | `README.md` 参数速查 | `render/render.go`, `schema/types.go` |
| 版本号未更新 | `CHANGELOG.md` | `main.go:33`, `build.sh:25` |
| 构建失败 | `DEPLOY.md` | `build.sh` |
| `--human` 不生效 | `README.md` → execute | `cmd/dbexplain/execute.go:38-42` (fs.Args 扫描) |
| 找不到配置（自动加载） | `CONFIG_SEARCH.md` | `cmd/dbexplain/main.go` → `findConfigFile()` |
| 加密文件无法解密 | `CONFIG_SEARCH.md` | `crypto/crypto.go`, `fingerprint*.go` |
| 日志找不到 | `README.md` → `--log-dir` | `cmd/dbexplain/main.go` → `resolveLogDir()` |
| Issue/修复记录 | `issues.json` | 对应版本的 `CHANGELOG.md` |
| `dbexplain check` 连接 GaussDB 卡死 | `issues.json` → ISSUE-095 | `config/config.go` → `SanitizeErr()` |
| 文档与代码不一致 | `CHANGELOG.md` v0.1.0 "文档对齐" | 按 `CODE_MAP.md` 逐模块比对 |

---

> 维护规范：每次新增代码模块或文档时，同步更新此文件的三张映射表（§1 模块映射、§3 文档映射、§6 测试映射）。
