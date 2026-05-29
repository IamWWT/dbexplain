# dbexplain -execute 只读查询执行

> 安全、受控地执行只读 SQL 查询，返回结构化数据表。
> 输出格式与 schema 采集模式完全分离。

---

## 设计目标

`execute` 子命令为 AI Agent 提供了一种**受沙箱保护**的只读 SQL 执行能力。与 schema 采集模式（`-env` / `-dsn`）不同：

| 维度 | Schema 采集 (`-env`) | 查询执行 (`execute`) |
|------|---------------------|---------------------|
| 输出格式 | Instance/Table/Column 元数据树 | 数据表 (columns + rows) |
| 数据内容 | 表结构（列名、类型、索引、FK） | 查询结果（用户数据行） |
| 消费方 | AI Agent 理解 DB 结构 | AI Agent 验证假设/检查数据 |
| 安全模型 | 固定只读系统表查询 | 用户 SQL 动态校验 |

---

## 安全架构

### 1. SQL 只读校验 (`sqlguard` 包)

所有 SQL 在执行前经过三层校验：

**第一层：操作动词白名单**
```
允许: SELECT, EXPLAIN, WITH, SHOW, DESCRIBE, DESC, PRAGMA, CHECK
拒绝: INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, TRUNCATE,
       RENAME, REPLACE, GRANT, REVOKE, MERGE, LOAD, IMPORT,
       ANALYZE, REINDEX
```
取 SQL 的第一个 token 与白名单/黑名单比对，不区分大小写。

> `ANALYZE` 和 `REINDEX` 曾被错误列入允许列表，v0.1.0 已修正：`ANALYZE` 写入统计表，`REINDEX` 锁表重建索引，均为写操作。
> `CHECK TABLE` 为只读诊断操作，保留在白名单中。

**第二层：多语句检测**
```
拒绝: SELECT 1; DROP TABLE users  → READ_ONLY_VIOLATION
禁止: SELECT * FROM a; SELECT * FROM b  → READ_ONLY_VIOLATION
```
按分号分割后检查是否包含多个非空语句。该策略为保守设计——分号在字符串字面量中的误判（false positive）是安全的（拒绝执行而非放行）。

**第三层：自动 LIMIT 注入**
```
输入: SELECT * FROM huge_table
输出: SELECT * FROM huge_table LIMIT 1000

跳过: SELECT ... LIMIT 10      → 已有 LIMIT，不追加
跳过: SHOW DATABASES            → 非 SELECT，不追加
跳过: EXPLAIN SELECT ...        → 查询计划分析，不追加
```

### 2. 并发控制 (`query` 包)

同一 label 的数据库实例同时只能执行一个查询：

```
goroutine 1: execute -label my-db "SELECT ..."  → 获取锁
goroutine 2: execute -label my-db "SELECT ..."  → CONCURRENT_LIMIT
```

使用 `sync.Mutex.TryLock()` 非阻塞获取，避免排队等待。不同 label 的查询可并发执行。

### 3. 超时保护

双超时机制确保无长时间锁表：

| 层级 | 机制 | 默认值 |
|------|------|--------|
| 应用层 | `context.WithTimeout` | `--timeout`+5s |
| 数据库层 | MySQL: `max_execution_time`, PG: `statement_timeout`, CH: `max_execution_time` | `--timeout` 值 |

### 4. 连接器能力检查

所有 9 种数据库连接器均实现 `query.Queryable` 接口，通过 Go 接口类型断言确认：

```go
q, ok := c.(query.Queryable)
if !ok {
    // QUERY_NOT_SUPPORTED (理论上不会触发)
}
```

**SQL 数据库** (通过 sqlguard 校验 + `database/sql` 或 HTTP 执行)：MySQL, PostgreSQL, GaussDB, SQLite, ClickHouse

**非 SQL 数据库** (各自内部校验，使用原生协议)：

| 数据库 | 查询格式 | 示例 |
|--------|---------|------|
| Elasticsearch | SQL（通过 `_sql` REST 端点） | `SHOW TABLES` / `SELECT * FROM index_name` |
| MongoDB | JSON（find/aggregate） | `{"find":"users","filter":{"age":{"$gt":18}}}` |
| Redis | 原生命令（空格分隔，只读白名单） | `GET mykey` / `HGETALL myhash` / `SCAN 0` |
| Qdrant | JSON（scroll/count） | `{"scroll":"collection_name"}` / `{"count":"collection_name"}` |

**查询路由机制**：`capabilities.FromProvider().Has(CapSQL)` 根据连接器声明的能力决定执行路径——SQL 类走 `sqlguard.Validate()` 校验，非 SQL 类跳过 SQL 校验、由各连接器内部进行只读白名单验证。

### 5. DSN 凭据保护

- DSN 通过 `-env`、`-config`、`-dsn` 加载，复用现有的 `findConfigFile()` + `loadEnvFile()` 路径
- 加密文件（`.env.dbexplain.enc`）同样支持
- DSN 密码在错误消息中自动脱敏（`Redacted()`）
- 查询结果 JSON **不包含**任何连接信息或凭据

### 6. 细粒度访问控制 (`policy` 包，v0.1.0+)

在 sqlguard 动词白名单校验之后，增加第二层访问控制——表级/列级/语句级拒绝策略。适用于**所有数据库类型**（SQL + 非SQL），通过 `.env` 文件配置。

**三层次策略（从快到慢）：**

| 层级 | 检测方法 | SQL 数据库 | MongoDB/Qdrant | Redis |
|------|---------|-----------|---------------|-------|
| 语句级 | 子串匹配 (case-insensitive) | ✅ | ✅ | ✅ |
| 表级 | 从 SQL/JSON 提取表名 | ✅ | ✅ (从 JSON) | — |
| 列级 | 提取 table.column 引用 | ✅ | — | — |

**配置格式：**

```env
# 全局策略（所有 DSN 生效）
DENY_TABLES=sensitive_data,audit_log
DENY_COLUMNS=users.password_hash,orders.card_number
DENY_STATEMENTS=DROP TABLE,ALTER TABLE,FLUSHALL

# 按 DSN 索引追加策略
DB1_DENY_TABLES=internal_audit
DB5_DENY_STATEMENTS=FLUSHALL,CONFIG,SHUTDOWN
DB7_DENY_TABLES=system.users
```

**策略检查位置：** `sqlguard.Validate()` → `policy.CheckSQL()`/`CheckNative()` → `AutoLimit()` → `ExecQuery()`

**错误输出：**
```
ACCESS_DENIED: table "audit_log" is not allowed for query
ACCESS_DENIED: column "users.password_hash" is not allowed for query
ACCESS_DENIED: query matches denied statement pattern "FLUSHALL"
```

**表级提取规则：** SQL 提取 `FROM`/`JOIN`/`TABLE` 后的标识符；MongoDB/Qdrant 从 JSON `"find"`/`"aggregate"`/`"scroll"`/`"count"` 字段提取集合名。

**列级提取规则：** 提取 `table.column` 或 `schema.table.column` 引用，自动过滤 SQL 关键字和数字假阳性。三节名称（`schema.table.column`）同时匹配二节 deny 规则（`table.column`）。

**安全加固（防绕过）：**
- 引用标识符归一化（`normalizeIdentifiers`）：剥离反引号/双引号/方括号后再提取，`` SELECT * FROM `sensitive` `` 不再绕过表级拒绝
- 空白字符归一化（`normalizeWhitespace`）：所有空白折叠为单空格，`DROP  TABLE` 不再绕过语句级拒绝
- SQL 注释剥离（`stripSQLComments`）：`--` 和 `/* */` 注释在提取前移除
- 子查询 LIMIT 检测（`hasOuterLimit`）：剥离括号内子查询内容后再检测 LIMIT 存在性

---

## 输出格式

### 正常查询响应

```json
{
  "columns": [
    {"name": "id", "type": "BIGINT"},
    {"name": "name", "type": "VARCHAR"}
  ],
  "rows": [
    ["1", "Alice"],
    ["2", "Bob"]
  ],
  "row_count": 2,
  "truncated": false,
  "execution_time": "1.23ms"
}
```

**与 schema 采集 JSON 的关键区别**：
- Schema 采集：`{"instances": [...], "refs": [...], "groups": [...], "issues": [...]}`
- 查询执行：`{"columns": [...], "rows": [...], "row_count": N, "execution_time": "..."}`

两套格式完全隔离，不会混淆。

### 错误响应 (stderr)

| 错误类型 | 前缀 | 示例 |
|---------|------|------|
| 写操作 (SQL) | `READ_ONLY_VIOLATION` | `write operation "DROP" is not allowed` |
| 写操作 (Redis) | `READ_ONLY_VIOLATION` | `redis command "SET" is not allowed` |
| 写操作 (MongoDB) | `READ_ONLY_VIOLATION` | `mongo query must specify "find" or "aggregate"` |
| 多语句 | `READ_ONLY_VIOLATION` | `multiple statements detected (2)` |
| 格式错误 (非SQL) | `READ_ONLY_VIOLATION` | `qdrant query must specify "scroll" or "count"` |
| 安全策略拒绝 (表) | `ACCESS_DENIED` | `table "audit_log" is not allowed for query` |
| 安全策略拒绝 (列) | `ACCESS_DENIED` | `column "users.password_hash" is not allowed for query` |
| 安全策略拒绝 (语句) | `ACCESS_DENIED` | `query matches denied statement pattern "FLUSHALL"` |
| 并发冲突 | `CONCURRENT_LIMIT` | `a query is already running for label "my-db"` |
| 查询失败 | `QUERY_ERROR` | `connection refused` |

所有错误输出到 stderr，退出码 1。正常结果输出到 stdout。

---

## 使用示例

### SQL 数据库

```bash
# 通过 DSN 直接查询
dbexplain execute -dsn 'sqlite:///path/to/db?label=local' \
  'SELECT * FROM users WHERE active = 1'

# 通过 .env 文件的 label 匹配
dbexplain execute -env --label aiops-mysql \
  'SELECT COUNT(*) FROM orders WHERE created_at > NOW() - INTERVAL 7 DAY'

# 通过 DB 编号匹配（DB1, DB2, ...）
dbexplain execute -env --db 1 \
  'SHOW INDEX FROM users'

# 自定义超时和行数
dbexplain execute -env --label my-pg \
  --timeout 60 --limit 500 \
  'SELECT * FROM events WHERE event_type = "error"'

# EXPLAIN 查询计划
dbexplain execute -env --label my-pg --explain \
  'SELECT * FROM orders WHERE user_id = 42'
```

### 非 SQL 数据库

```bash
# Elasticsearch (标准 SQL，通过 _sql 端点)
dbexplain execute -env --label es-test 'SHOW TABLES'
dbexplain execute -env --label es-test \
  'SELECT * FROM my-index WHERE status = "active" LIMIT 50'

# MongoDB (JSON 原生查询)
dbexplain execute -env --label mongo-test \
  '{"find":"users","filter":{"active":true},"limit":100}'
dbexplain execute -env --label mongo-test \
  '{"aggregate":"orders","pipeline":[{"$match":{"status":"done"}},{"$group":{"_id":"$user","total":{"$sum":"$amount"}}}]}'

# Redis (原生命令)
dbexplain execute -env --label redis-test 'GET user:1001'
dbexplain execute -env --label redis-test 'HGETALL session:abc123'
dbexplain execute -env --label redis-test 'SCAN 0 MATCH user:* COUNT 100'

# Qdrant (JSON 向量数据库查询)
dbexplain execute -env --label qdrant-test '{"count":"documents"}'
dbexplain execute -env --label qdrant-test '{"scroll":"documents","limit":20}'
```

### AI Agent 集成示例

```python
import subprocess, json

# Agent 校验假设
result = subprocess.run([
    "dbexplain", "execute", "-env", "--label", "aiops-mysql",
    "SELECT user_id, COUNT(*) as cnt FROM orders GROUP BY user_id ORDER BY cnt DESC LIMIT 10"
], capture_output=True, text=True)

if result.returncode == 0:
    data = json.loads(result.stdout)
    print(f"Top users: {data['rows']}")
else:
    print(f"Query failed: {result.stderr}")
```

---

> **`--human` 位置说明**：可放在查询语句之前或之后，两种写法等价。例如：
> ```bash
> dbexplain execute -env --db 1 --human "SELECT * FROM users LIMIT 5"
> dbexplain execute -env --db 1 "SELECT * FROM users LIMIT 5" --human
> ```

## 安全设计要点

| 安全特性 | 实现方式 | 防护目标 |
|---------|---------|---------|
| SQL 只读校验 | sqlguard.Validate() 动词白名单 + 多语句检测 | 防止 SQL 数据篡改/注入 |
| 非 SQL 只读校验 | 各连接器内部白名单（Redis 命令、MongoDB 操作、Qdrant 操作） | 防止非 SQL 数据库篡改 |
| 细粒度访问控制 | policy.Load() + CheckSQL()/CheckNative() | 表级/列级/语句级按需拒绝 |
| 策略防绕过 | normalizeIdentifiers + normalizeWhitespace + stripSQLComments | 防引用标识符/空白/注释绕过策略检测 |
| 自动 LIMIT | AutoLimit() 追加 LIMIT N（SQL 类） | 防止全表扫描 |
| 子查询 LIMIT 防绕过 | hasOuterLimit() 剥离括号内容后检测 | 防子查询内嵌 LIMIT 绕过自动注入 |
| 数据库超时 | max_execution_time/statement_timeout | 防止慢查询阻塞 |
| 并发互斥 | TryLock per-label | 防止并发压力 |
| 凭据保护 | Redacted() + sanitizeErr() + 查询结果不含 DSN | 防止密码泄露 |
| OS 环境隔离 | loadEnvFile() 直接返回 entries，不经过 os.Setenv | 防止 DSN 密码残留进程环境变量 |
| 查询路由 | capabilities.FromProvider().Has(CapSQL) 按能力分流校验 | 防止 SQL 校验器误判原生命令 |
| 沙箱隔离 | 每次新建连接 + 独立 context | 连接故障不影响其他操作 |
| 终端注入防御 | sanitizeCell() 剥离 ANSI 转义和控制字符 (仅 `--human`，全 9 种数据库) | 防止恶意数据注入终端命令 |
| 列宽防护 | maxColWidth=256 截断超长 cell (仅 `--human`，全 9 种数据库) | 防止巨量 cell 撑爆终端/内存 |

---

## 各数据库执行行为速览

| 数据库 | Auto LIMIT 1000 | 校验路径 | 超时机制 | 详细文档 |
|--------|:---:|---------|---------|---------|
| MySQL | ✅ | sqlguard | `SET SESSION max_execution_time` | [MYSQL.md](MYSQL.md) |
| PostgreSQL | ✅ | sqlguard | `SET statement_timeout` | [POSTGRESQL.md](POSTGRESQL.md) |
| GaussDB | ✅ | sqlguard | `SET statement_timeout` | [GAUSSDB.md](GAUSSDB.md) |
| SQLite | ✅ | sqlguard | context 超时 | [SQLITE.md](SQLITE.md) |
| ClickHouse | ✅ | sqlguard | `SETTINGS max_execution_time` + HTTP 超时 | [CLICKHOUSE.md](CLICKHOUSE.md) |
| Elasticsearch | ✅ | sqlguard | HTTP 请求超时 | [ELASTICSEARCH.md](ELASTICSEARCH.md) |
| Redis | ❌ | 内部 30+ 命令白名单 | go-redis context | [REDIS.md](REDIS.md) |
| MongoDB | ❌ | 内部 find/aggregate 白名单 | driver context + `--limit` | [MONGO.md](MONGO.md) |
| Qdrant | ❌ | 内部 scroll/count 白名单 | gRPC context | [QDRANT.md](QDRANT.md) |
| CSV/TSV | ❌ | 无（文件只读） | — | [FILE_PROCESSING.md](FILE_PROCESSING.md) |
| XLSX | ❌ | 无（文件只读） | — | [FILE_PROCESSING.md](FILE_PROCESSING.md) |

> **SQL 数据库**（上表前 6 种）通过 `capabilities.FromProvider().Has(CapSQL)` 路由到 `sqlguard.Validate()` 进行动词白名单校验，并自动注入 `LIMIT 1000`。
> **非 SQL 数据库**（上表后 3 种）跳过 sqlguard，由各连接器内部实现只读白名单。
> **文件数据源**（CSV/TSV/XLSX）绕过 sqlguard——文件本身只读，仅支持 `SELECT * [LIMIT N [OFFSET M]]`，但仍受策略引擎约束（`DENY_TABLES`、`MASK_COLUMNS`）。

## 架构文件清单

| 文件 | 职责 |
|------|------|
| `src/policy/policy.go` | 细粒度访问控制：表级/列级/语句级拒绝策略 |
| `src/sqlguard/sqlguard.go` | SQL 只读校验、多语句检测、自动 LIMIT |
| `src/query/types.go` | Queryable 接口、QueryResult 类型、QueryLock 并发控制 |
| `src/connector/query.go` | executeSQLQuery() 通用 database/sql 查询执行 |
| `src/connector/mysql.go` | MySQL ExecQuery 实现 |
| `src/connector/postgres.go` | PostgreSQL/GaussDB ExecQuery 实现 |
| `src/connector/sqlite.go` | SQLite ExecQuery 实现 |
| `src/connector/clickhouse.go` | ClickHouse HTTP ExecQuery 实现 |
| `src/connector/elasticsearch.go` | Elasticsearch _sql REST ExecQuery 实现 |
| `src/connector/mongo.go` | MongoDB JSON find/aggregate ExecQuery 实现 |
| `src/connector/redis.go` | Redis 只读命令白名单 ExecQuery 实现 |
| `src/connector/qdrant.go` | Qdrant scroll/count ExecQuery 实现 |
| `src/connector/csv.go` | CSV/TSV 文件 schema 采集 + 查询执行（SELECT * only） |
| `src/connector/xlsx.go` | XLSX 文件 schema 采集 + 查询执行（SELECT * only） |
| `src/execute.go` | CLI 入口：参数解析、DSN 匹配、查询路由、file 分发、输出控制 |

---

## 约束 (CONSTITUTION.md 合规)

| 原则 | 合规情况 |
|------|---------|
| Connector 自注册 | Queryable 是独立接口，不修改 Connector 接口 |
| Panic 隔离 | ExecQuery 不通过 CollectSafe，但 SQL 验证在调用前完成 |
| 只读安全 | sqlguard.Validate() + 自动 LIMIT |
| 零 CGO | 无新 CGO 依赖 |
| 无状态设计 | 每次查询新建 connection，无共享可变状态 |
| Deterministic Only | 仅返回查询结果事实，不做 AI 推理 |
