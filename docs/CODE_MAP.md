# Docs-Code Index

快速定位：问题/功能 → 文档 → 源码。

---

## 1. 模块 ↔ 文件映射

| 模块 | 包/目录 | 源码文件 | 文档 |
|------|---------|---------|------|
| **入口/CLI** | `main` | `src/main.go` | — |
| **查询执行** | `main` | `src/execute.go`, `src/execute_test.go` | `docs/EXECUTE.md`, `docs/CLI_EXAMPLES.md` |
| **DSN 解析** | `dsn` | `src/dsn/dsn.go`, `src/dsn/dsn_test.go` | `docs/CONFIG_SEARCH.md` |
| **Schema 数据模型** | `schema` | `src/schema/types.go`, `src/schema/errors.go`, `src/schema/infer.go`, `src/schema/infer_test.go` | — |
| **Connector 接口** | `connector` | `src/connector/connector.go` (接口), `src/connector/registry.go` (注册表), `src/connector/runner.go` (Panic保护), `src/connector/query.go` (SQL执行共享实现) | — |
| **MySQL** | `connector` | `src/connector/mysql.go` | `docs/MYSQL.md` |
| **PostgreSQL** | `connector` | `src/connector/postgres.go` | `docs/POSTGRESQL.md` |
| **SQLite** | `connector` | `src/connector/sqlite.go` | `docs/SQLITE.md` |
| **ClickHouse** | `connector` | `src/connector/clickhouse.go` | `docs/CLICKHOUSE.md` |
| **Redis** | `connector` | `src/connector/redis.go` | `docs/REDIS.md` |
| **Elasticsearch** | `connector` | `src/connector/elasticsearch.go` | `docs/ELASTICSEARCH.md` |
| **MongoDB** | `connector` | `src/connector/mongo.go` | `docs/MONGO.md` |
| **Qdrant** | `connector` | `src/connector/qdrant.go` | `docs/QDRANT.md` |
| **CSV/TSV** | `connector` | `src/connector/csv.go`, `src/connector/csv_test.go` | `docs/FILE_PROCESSING.md` |
| **XLSX** | `connector` | `src/connector/xlsx.go` | `docs/FILE_PROCESSING.md` |
| **类型推断** | `connector` | `src/connector/infer.go` | `docs/FILE_PROCESSING.md` |
| **能力声明** | `capabilities` | `src/capabilities/capabilities.go` | `docs/ALGORITHMS.md` |
| **策略引擎** | `policy` | `src/policy/policy.go`, `src/policy/policy_test.go` | `docs/POLICY.md` |
| **SQL 只读校验** | `sqlguard` | `src/sqlguard/sqlguard.go`, `src/sqlguard/sqlguard_test.go` | `docs/EXECUTE.md` |
| **查询类型** | `query` | `src/query/types.go`, `src/query/query_test.go` | `docs/EXECUTE.md` |
| **关系分析/图** | `analyze` | `src/analyze/analyze.go`, `src/analyze/ranking.go` | `docs/ALGORITHMS.md` |
| **诊断** | `diagnostics` | `src/diagnostics/diagnostics.go` | `docs/ALGORITHMS.md` |
| **缓存/增量扫描** | `cache` | `src/cache/cache.go` | `docs/ALGORITHMS.md` |
| **上下文压缩** | `context` | `src/context/compress.go` | `docs/ALGORITHMS.md` |
| **渲染输出** | `render` | `src/render/render.go` | — |
| **加密** | `crypto` | `src/crypto/crypto.go`, `src/crypto/fingerprint*.go` | `docs/CONFIG_SEARCH.md` |
| **编码处理** | `main` | `src/encode.go`, `src/encode_windows.go` | — |
| **构建** | — | `src/build.sh` | — |

---

## 2. Capability 矩阵（connector → 能力 → 源码）

| Connector | CapSQL | CapFile | CapFK | CapIndex | CapRowCount | CapSampling | CapTTL | CapPartition | CapVector |
|-----------|--------|---------|-------|----------|-------------|-------------|--------|--------------|-----------|
| MySQL | ✓ | — | ✓ | ✓ | — | ✓ | — | — | — |
| PostgreSQL | ✓ | — | ✓ | ✓ | ✓ | ✓ | — | — | — |
| SQLite | ✓ | — | ✓ | — | — | ✓ | — | — | — |
| ClickHouse | ✓ | — | — | — | ✓ | ✓ | — | ✓ | — |
| Redis | — | — | — | — | — | ✓ | ✓ | — | — |
| ES | ✓ | — | — | ✓ | — | — | — | — | — |
| MongoDB | — | — | — | — | ✓ | — | — | — | — |
| Qdrant | — | — | — | — | ✓ | — | — | — | ✓ |
| CSV/TSV | — | ✓ | — | — | ✓ | — | — | — | — |
| XLSX | — | ✓ | — | — | ✓ | — | — | — | — |

定义位置：`src/capabilities/capabilities.go:17-48`

---

## 3. 文档 ↔ 源码映射（按文档）

| 文档 | 对应的主要源码 | 关键内容 |
|------|--------------|---------|
| **文档索引（入口）** | — | `docs/CODE_MAP.md` — 本文件 | 文档-代码双向索引，新增/修改文档或代码时须同步更新 |
| `ALGORITHMS.md` | `analyze/`, `diagnostics/`, `cache/`, `context/`, `capabilities/`, `connector/redis.go` | 命名推断、图聚类、重要性评分、Redis 风险诊断、Capability 系统 |
| `ARCHITECTURE.md` | 全局 | 架构愿景、Phase 路线图、目录结构 |
| `EXECUTE.md` | `execute.go`, `sqlguard/`, `query/`, `policy/` | 三层安全、AutoLimit、sqlguard 动词白名单 |
| `CLI_EXAMPLES.md` | `execute.go` | 13 条实测查询案例 |
| `POLICY.md` | `policy/policy.go`, `policy/policy_test.go` | DENY_TABLES/COLUMNS/STATEMENTS + MASK_COLUMNS |
| `CONFIG_SEARCH.md` | `main.go`, `dsn/dsn.go`, `crypto/` | 6 级路径搜索、加密配置自动解密 |
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
| `GAUSSDB.md` | `connector/postgres.go` | GaussDB（PG 协议兼容） |
| `COMPATIBILITY_GAUSSDB_TDSQL.md` | 无代码变更 | 兼容性验证记录 |

---

## 4. 4-Stage Pipeline 文件映射

```
INPUT ──→ COLLECT ──→ ANALYZE ──→ OUTPUT
 │          │            │            │
 │    src/connector/  src/analyze/  src/render/
 │    registry.go     analyze.go    render.go
 │    runner.go       ranking.go
 │    mysql.go etc.   src/schema/
 │                    types.go
 │                    infer.go
 │
 src/main.go          src/diagnostics/   JSON/--human
 src/dsn/dsn.go       diagnostics.go    --context/--cache
```

---

## 5. CLI 子命令 → 处理函数

| 子命令 | 源码位置 | 说明 |
|--------|---------|------|
| `dbexplain` (默认) | `main.go` → `handleCollect()` | Schema 采集 |
| `dbexplain execute` | `execute.go` → `handleExecute()` | 只读查询执行 |
| `dbexplain list` | `main.go` → `handleList()` | 列出已配置 DSN |
| `dbexplain all` | `main.go` → `handleManual()` | 全部参考手册 |
| `dbexplain <dbtype>` | `main.go` → `handleManual()` | 专项手册（mysql/redis/...） |
| `dbexplain encrypt` | `main.go` → `handleEncrypt()` | 配置加密 |
| `dbexplain version` | `main.go` → `--version` flag | 版本信息 |

---

## 6. 测试文档映射

| 测试文档 | 覆盖内容 | 关联源码 |
|---------|---------|---------|
| `docs/test/RESULTS.md` | 测试结果报告 v0.1.0 (91/91 通过) | — |
| `docs/test/01-environment.md` | 环境准备、DSN 模板 | `dsn/dsn.go` |
| `docs/test/02-schema-collection.md` | 全数据源 Schema 采集 | 所有 `connector/*.go` |
| `docs/test/03-execute-sql.md` | SQL 查询执行 | `execute.go`, `sqlguard/` |
| `docs/test/04-execute-nosql.md` | 非 SQL 查询执行 | `connector/redis.go`, `connector/mongo.go` 等 |
| `docs/test/05-file-processing.md` | CSV/TSV/XLSX | `connector/csv.go`, `connector/xlsx.go` |
| `docs/test/06-security-sqlguard.md` | sqlguard 只读校验 | `sqlguard/sqlguard.go` |
| `docs/test/07-policy-engine.md` | 策略引擎绕过测试 | `policy/policy.go` |
| `docs/test/08-concurrent-limit.md` | 并发限制 | `connector/runner.go`, `query/types.go` |
| `docs/test/09-cli-help.md` | CLI 帮助、子命令 | `main.go` |
| `docs/test/10-regression.md` | 回归测试 | 全局 |
| `docs/test/11-end-to-end.md` | 端到端集成测试 | 全局 |
| `docs/test/12-capability-routing.md` | CapSQL 路由、PostgreSQL 多 Schema | `capabilities/`, `connector/postgres.go` |

---

## 7. 常见问题定位指南

| 问题/需求 | 先看什么文档 | 再看什么源码 |
|-----------|-------------|-------------|
| 新增数据库类型 | `CONSTITUTION.md` 原则 1 | `connector/registry.go`, `capabilities/capabilities.go` |
| Schema 采集结果不对 | 对应 DB 专项文档 | `connector/<db>.go` 中的 `Collect()` |
| 查询被误拦截 | `EXECUTE.md`, `POLICY.md` | `sqlguard/sqlguard.go` (readOps), `policy/policy.go` |
| `DENY_TABLES` 不生效 | `POLICY.md` (排障参考) | `policy/policy.go` → `CheckSQL()` |
| JSON 输出格式 | `README.md` 参数速查 | `render/render.go`, `schema/types.go` |
| 版本号未更新 | `CHANGELOG.md` | `main.go:33`, `build.sh:25` |
| 构建失败 | `DEPLOY.md` | `build.sh` |
| `--human` 不生效 | `README.md` → execute | `execute.go:38-42` (fs.Args 扫描) |
| `-env` 找不到配置 | `CONFIG_SEARCH.md` | `main.go` → `findConfigFile()` |
| 加密文件无法解密 | `CONFIG_SEARCH.md` | `crypto/crypto.go`, `fingerprint*.go` |
| 日志找不到 | `README.md` → `--log-dir` | `main.go` → `resolveLogDir()` |
| Issue/修复记录 | `issues.json` | 对应版本的 `CHANGELOG.md` |
| 文档与代码不一致 | `CHANGELOG.md` v0.1.0 "文档对齐" | 按 `CODE_MAP.md` 逐模块比对 |

---

> 维护规范：每次新增代码模块或文档时，同步更新此文件的三张映射表（§1 模块映射、§3 文档映射、§6 测试映射）。
