# dbexplain 安全策略引擎 (Policy)

> 细粒度访问控制系统，为 `dbexplain execute` 提供表级、列级、语句级拒绝策略。
> 支持全部 14 种数据源（11 种数据库 + CSV/TSV/XLSX 文件），通过 `.env` 文件统一配置。

---

## 概述

安全策略引擎 (`src/internal/policy/`) 在 `sqlguard` 动词白名单校验之后、查询执行之前，提供 L3 细粒度访问控制。适用于**所有数据库类型**。

**策略链：**
```
sqlguard.Validate() → policy.CheckSQL/CheckNative() → AutoLimit() → Lock() → ExecQuery() → ApplyMask() → StripDeniedColumns()
```

**三层策略 + 列值屏蔽：**

| 层级 | 检测方式 | 作用 |
|------|---------|------|
| 语句级 | 大小写不敏感子串匹配 | 拦截包含特定模式语句的查询 |
| 表级 | 提取表名/集合名/Key 名后匹配 | 禁止访问敏感数据对象 |
| 列级 | 提取 `table.column` 引用后匹配 | 禁止查询敏感字段（仅 SQL） |
| 列屏蔽 | 执行后替换匹配列的值 | 替代硬阻断，输出替换文本（如 `***`） |

---

## 配置方式

### 全局策略（所有 DSN 生效）

```env
DENY_TABLES=table1,table2,collection1
DENY_COLUMNS=schema.table.column,table.column
DENY_STATEMENTS=DROP TABLE,ALTER TABLE,FLUSHALL,SHUTDOWN

# 列值屏蔽（执行后替换，非硬阻断）
MASK_COLUMNS=password_hash=***,card_number=****-****-****-****
```

### 按 DSN 索引追加（与全局策略叠加）

```env
# 仅 DB1 (MySQL) 追加禁止表
DB1_DENY_TABLES=internal_audit,payments

# 仅 DB5 (Redis) 禁止高危命令
DB5_DENY_STATEMENTS=FLUSHALL,CONFIG,SHUTDOWN

# 仅 DB7 (MongoDB) 禁止集合
DB7_DENY_TABLES=system.users,system.profile

# 按 DSN 列值屏蔽
DB2_MASK_COLUMNS=email=REDACTED,phone=HIDDEN
```

### 配置来源

| 来源 | 示例 |
|------|------|
| `.env.dbexplain` 文件 | 在 `DB<n>=...` 条目下方添加 |
| `~/.config/dbexplain/.env.dbexplain` | 全局用户配置 |
| 环境变量直接设置 | `export DENY_TABLES=...` |

---

## 按数据库类型的禁用规则

### 1. MySQL — SQL 三层全支持

| 维度 | 配置 | 提取方式 |
|------|------|---------|
| 语句 | `DENY_STATEMENTS=DROP TABLE,ALTER TABLE` | 子串匹配 |
| 表 | `DENY_TABLES=user_credentials,payment_log` | `FROM`/`JOIN`/`TABLE` 后的标识符 |
| 列 | `DENY_COLUMNS=users.password_hash,orders.card_number` | `table.column` / `schema.table.column` 引用 |

```bash
# 拦截
dbexplain execute -env --db 1 "SELECT * FROM user_credentials"
# → ACCESS_DENIED: table "user_credentials" is not allowed for query

dbexplain execute -env --db 1 "SELECT users.password_hash FROM users"
# → ACCESS_DENIED: column "users.password_hash" is not allowed for query

dbexplain execute -env --db 1 "DROP TABLE users"
# → ACCESS_DENIED: query matches denied statement pattern "DROP TABLE"

# 放行
dbexplain execute -env --db 1 "SELECT id, name FROM users"
# → 正常返回数据
```

---

### 2. PostgreSQL — SQL 三层全支持

| 维度 | 配置 | 提取方式 |
|------|------|---------|
| 语句 | `DENY_STATEMENTS=DROP TABLE` | 子串匹配 |
| 表 | `DENY_TABLES=employees,audit_log` | `FROM`/`JOIN` 后的标识符 |
| 列 | `DENY_COLUMNS=employees.salary` | `table.column` / `schema.table.column` |

```bash
# 拦截
dbexplain execute -env --label video-pg "SELECT * FROM employees"
# → ACCESS_DENIED: table "employees" is not allowed for query

dbexplain execute -env --label video-pg "SELECT employees.salary FROM employees"
# → ACCESS_DENIED: column "employees.salary" is not allowed for query

# 放行
dbexplain execute -env --label video-pg "SELECT id, name FROM public.cameras"
# → 正常返回数据
```

---

### 3. GaussDB — SQL 三层全支持

与 PostgreSQL 行为完全一致（走相同执行路径）。

| 维度 | 配置 |
|------|------|
| 语句 | `DENY_STATEMENTS=DROP TABLE` |
| 表 | `DENY_TABLES=sensitive_schema.table` |
| 列 | `DENY_COLUMNS=sensitive_schema.table.column` |

---

### 4. SQLite — SQL 三层全支持

```bash
# 拦截
dbexplain execute -env --label aiops-sqlite "SELECT * FROM audit_log"
# → ACCESS_DENIED: table "audit_log" is not allowed for query

# 放行
dbexplain execute -env --label aiops-sqlite "SELECT name FROM sqlite_master"
# → 正常返回
```

---

### 5. ClickHouse — SQL 三层全支持

| 维度 | 配置 | 注意 |
|------|------|------|
| 语句 | `DENY_STATEMENTS=DROP TABLE` | ClickHouse 也支持标准 SQL |
| 表 | `DENY_TABLES=ai_obs.otel_traces` | ClickHouse 表名大小写敏感 |
| 列 | `DENY_COLUMNS=otel_traces.Duration` | `schema.table.column` 格式 |

```bash
# 拦截
dbexplain execute -env --db 2 "SELECT * FROM ai_obs.otel_traces"
# → ACCESS_DENIED: table "otel_traces" is not allowed for query
```

---

### 6. Elasticsearch — SQL 三层全支持

Elasticsearch 通过 `_sql` REST 端点使用标准 SQL，因此走 SQL 校验路径。

| 维度 | 配置 |
|------|------|
| 语句 | `DENY_STATEMENTS=SHOW TABLES` |
| 表 | `DENY_TABLES=runbooks` |
| 列 | `DENY_COLUMNS=runbooks.severity` |

```bash
# 拦截
dbexplain execute -env --label es-test "SELECT * FROM runbooks"
# → ACCESS_DENIED: table "runbooks" is not allowed for query
```

---

### 7. MongoDB — 非 SQL: statement + collection 级别

| 维度 | 配置 | 提取方式 |
|------|------|---------|
| 语句 | `DENY_STATEMENTS=DROP,SHUTDOWN` | 原始 JSON 子串匹配 |
| 集合 | `DENY_TABLES=system.users,system.profile` | 从 JSON `"find"`/`"aggregate"` 字段解析 |
| 列 | **支持 (v0.1.0+)** | MongoDB 原生 JSON 查询：`DENY_COLUMNS=collection.field` 匹配，除非查询通过投影显式排除该字段 |

```bash
# 拦截（集合级别）
dbexplain execute -env --label mongo-test '{"find":"system.users","filter":{}}'
# → ACCESS_DENIED: table "system.users" is not allowed for query

# 拦截（语句级别）
dbexplain execute -env --label mongo-test '{"aggregate":"system.profile","pipeline":[]}'
# → ACCESS_DENIED: table "system.profile" is not allowed for query

# 放行
dbexplain execute -env --label mongo-test '{"find":"user","filter":{},"limit":5}'
# → 正常返回数据
```

---

### 8. Qdrant — 非 SQL: statement + collection 级别

| 维度 | 配置 | 提取方式 |
|------|------|---------|
| 语句 | `DENY_STATEMENTS=drop_collection` | 原始 JSON 子串匹配 |
| 集合 | `DENY_TABLES=internal_docs` | 从 JSON `"scroll"`/`"count"` 字段解析 |
| 列 | **支持 (v0.1.0+)** | Qdrant JSON query：`DENY_COLUMNS=collection.field` 匹配，除非查询通过投影显式排除该字段 |

```bash
# 拦截（集合级别）
dbexplain execute -env --label qdrant-test '{"scroll":"internal_docs","limit":10}'
# → ACCESS_DENIED: table "internal_docs" is not allowed for query

# 放行
dbexplain execute -env --label qdrant-test '{"count":"public_collection"}'
# → 正常返回
```

---

### 9. Redis — 非 SQL: statement + key 级别

| 维度 | 配置 | 提取方式 |
|------|------|---------|
| 语句 | `DENY_STATEMENTS=FLUSHALL,CONFIG,SHUTDOWN` | 原始命令子串匹配 |
| Key | `DENY_TABLES=CONVERSATION:*,secret_key` | 提取命令的第一个参数作为 key，支持 `globMatch()` 自定义通配符（与 `filepath.Match` 兼容，但避免将 `/` 视为路径分隔符） |
| 列 | **不支持** | Redis 无列概念 |

**Key 提取规则：** 30+ 只读命令中，有 key 参数的命令提取 `parts[1]` 作为 key 名；无 key 参数的命令（SCAN, PING, ECHO 等）跳过 key 检查。

**Key 匹配支持通配符：**
- `*` — 匹配任意字符序列（如 `CONVERSATION:*` 匹配所有以 `CONVERSATION:` 开头的 key）
- `?` — 匹配单个字符
- 精确字符串也支持

```bash
# 拦截（key 级别 - 通配符）
dbexplain execute -env --label openim-redis 'GET CONVERSATION:test123'
# → ACCESS_DENIED: table "CONVERSATION:*" is not allowed for query

# 拦截（key 级别 - 精确匹配）
dbexplain execute -env --label openim-redis 'GET secret_key'
# → ACCESS_DENIED: table "secret_key" is not allowed for query

# 拦截（语句级别）
dbexplain execute -env --label openim-redis 'FLUSHALL'
# → ACCESS_DENIED: query matches denied statement pattern "FLUSHALL"

# 放行（key 不在禁用列表）
dbexplain execute -env --label openim-redis 'GET user:1001'
# → 正常返回

# 放行（无 key 命令）
dbexplain execute -env --label openim-redis 'PING'
# → PONG
```

---

### 10. MASK_COLUMNS — 列值屏蔽（所有数据库通用）

`MASK_COLUMNS` 是 `DENY_COLUMNS` 的替代选项：不阻断查询，而是将匹配列的值**替换为指定字符串**。屏蔽发生在**查询执行后、输出前**，对所有数据库类型生效。

**与 DENY_COLUMNS 的区别：**

| 特性 | DENY_COLUMNS | MASK_COLUMNS |
|------|-------------|-------------|
| 行为 | 硬阻断，整条查询被拒绝 | 执行后替换列值 |
| 输出 | `ACCESS_DENIED` 到 stderr | 正常输出，敏感列显示替换文本 |
| 触发时机 | 执行前 | 执行后 |
| NULL 值 | — | 保持 NULL 不变 |

**配置格式：** `MASK_COLUMNS=列名=替换文本,列名2=替换文本2`

列名匹配规则（大小写不敏感）：
| 配置 | 匹配列 | 说明 |
|------|--------|------|
| `password_hash=***` | `password_hash` | 裸列名精确匹配 |
| `users.password_hash=***` | `password_hash` | 剥离 `table.` 前缀后匹配 |
| `pass*=***` | `password_hash`, `pass_token` | `filepath.Match` 通配符 |
| `credit_card?=****` | `credit_card1`, `credit_cardX` | `?` 匹配单字符 |

```bash
# 屏蔽（值被替换，查询正常返回）
MASK_COLUMNS=password_hash=*** dbexplain execute -env --label mysql \
  'SELECT id, name, password_hash FROM users LIMIT 3'
# → id | name    | password_hash
#     1  | Alice   | ***
#     2  | Bob     | ***

# 与非 SQL 数据库同样生效
MASK_COLUMNS=ssn=*** dbexplain execute -env --label mongo \
  '{"find":"users","filter":{},"limit":2}'
# → _id   | ssn  | name
#     abc  | ***  | Alice
```


| 数据库 | 语句级 | 表/集合/Key 级 | 列级 | 列屏蔽 | 提取方式 |
|--------|:-----:|:--------------:|:----:|:-----:|---------|
| MySQL | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取 |
| PostgreSQL | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取 |
| GaussDB | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取 |
| SQLite | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取 |
| ClickHouse | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取 |
| Elasticsearch | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取（`_sql` 端点） |
| MongoDB | ✅ | ✅ 集合 | ✅ (v0.1.0+) | ✅ | JSON `find`/`aggregate` 字段 |
| Qdrant | ✅ | ✅ 集合 | ✅ (v0.1.0+) | ✅ | JSON `scroll`/`count` 字段 |
| DuckDB | ✅ | ✅ 表 | ✅ | ✅ | SQL 语法提取 |
| Redis | ✅ | ✅ Key | — | ✅ | 命令第一个参数 + 通配符匹配 |
| Prometheus | ✅ | ✅ metric | ✅ label | ✅ | CheckNative PromQL 解析 |

---

## 错误输出格式

所有策略拒绝都输出到 **stderr**，退出码 **1**：

```
ACCESS_DENIED: table "user_credentials" is not allowed for query
ACCESS_DENIED: column "users.password_hash" is not allowed for query
ACCESS_DENIED: query matches denied statement pattern "FLUSHALL"
```

JSON 结果不受影响（策略拒绝时不会生成 JSON 输出）。

---

## 常见场景

### 1. 保护敏感用户数据

```env
DENY_TABLES=user_credentials,payment_info
DENY_COLUMNS=users.password_hash,users.phone,orders.card_number
```

### 2. 禁止高危管理操作

```env
DENY_STATEMENTS=DROP TABLE,ALTER TABLE,TRUNCATE,GRANT,REVOKE
```

### 3. Redis 保护

```env
DENY_STATEMENTS=FLUSHALL,FLUSHDB,CONFIG,SHUTDOWN,DEBUG
DENY_TABLES=CONVERSATION:*,TOKEN:*,SESSION:*
```

### 4. 按实例隔离（生产 vs 测试）

```env
# 全局基础策略
DENY_STATEMENTS=DROP TABLE,ALTER TABLE

# 生产库（DB1）额外限制
DB1_DENY_TABLES=users,orders,payments
DB1_DENY_COLUMNS=users.password_hash

# 测试库（DB2）宽松
# 不追加额外策略
```

### 5. MongoDB 保护系统集合

```env
DENY_TABLES=system.users,system.profile,system.indexes
```

### 6. 列值屏蔽替代硬阻断

```env
# 不阻断查询，仅替换敏感列的输出值
MASK_COLUMNS=password_hash=***,card_number=****-****-****-****,email=REDACTED
```

---

## 与 sqlguard 的关系

| 特性 | sqlguard | policy |
|------|----------|--------|
| 定位 | 通用只读校验 | 细粒度访问控制 |
| 作用范围 | SQL 语句动词 | 表/列/语句模式 |
| 配置方式 | 硬编码白名单 | .env 文件自定义 |
| 数据库类型 | 仅 SQL 类 | SQL 类 + 原生类共 14 种数据源 |
| 触发时机 | 校验链第一层 | 校验链第二层 |

两者协同工作：sqlguard 阻止写操作，policy 阻止敏感数据访问。

---

## 架构文件

| 文件 | 职责 |
|------|------|
| `src/internal/policy/policy.go` | Config 加载、SQL/原生查询校验、表名列名提取、列值屏蔽 |
| `src/internal/policy/policy_test.go` | 60+ 个测试用例：三层策略 + 列屏蔽 + 14 种数据源覆盖 |
| `src/cmd/dbexplain/execute.go` | 集成点：`sqlguard.Validate()` → `policy.CheckSQL/CheckNative()` → `policy.ApplyMask()` |

### 排障参考

| 现象 | 可能原因 | 排查方向 |
|------|---------|----------|
| `DENY_TABLES=information_schema` 不生效 | `extractTableNames()` regex 未捕获 schema 前缀 | v0.0.9+ 已修复；检查二进制版本 |
| `SELECT *` 绕过 `DENY_COLUMNS` | 无显式列引用，`matchStarSelect` 未触发 | v0.0.9+ 已修复；检查是否使用 `SELECT` + 列名 |
| MongoDB/Qdrant `{"find":"col"}` 绕过列级策略 | `CheckNative()` 未检查列级 | v0.0.9+ 已修复；可加 projection 显示排除 |
| `execute "query" --human` 不生效 | Go flag 遇第一个非 flag 参数停止解析 | v0.0.9+ 已修复；支持 flag 前后任意位置 |

> 完整修复记录见 [`issues.json`](../issues.json#ISSUE-064)（v0.0.9 策略引擎绕过修复）。

---

## 版本历史

| 版本 | 变更 |
|------|------|
| v0.1.7 | Prometheus meta 表 rows 输出；CTE 写检测加固（WITH + 主查询写拦截） |
| v0.1.6 | Bug Bash：全局代码审计，21 项防御性修复（nil panic、静默吞错、错误质量） |
| v0.1.5 | DENY_COLUMNS + SELECT * 后置列剥离（StripDeniedColumns），不再拦截报错 |
| v0.1.4 | Prometheus DenyTables/DenyColumns 支持（CheckNative PromQL 解析）；DSL IR 三层安全（Layer 1 预编译检查） |
| v0.1.0 | MongoDB/Qdrant 列级检测支持（`CheckNative` + `DENY_COLUMNS=collection.field`）；`matchStarSelect()` 全线检测（`\A` → `\b` 防 CTE 绕过）；`extractTableNames()` schema 前缀捕获修复；配置不再泄漏到 `os.Environ`（`LoadFromMap`）；文档对齐 12 种数据源 |
| v0.0.9 | CSV/XLSX 文件数据源：受策略引擎约束（DENY_TABLES + MASK_COLUMNS），绕过 sqlguard |
| v0.0.8 | 初始实现：三层策略 + 9 种数据源全覆盖 + Redis 通配符 key 匹配 + MASK_COLUMNS 列值屏蔽 |
| v0.0.8 (审计) | 安全增强：`normalizeIdentifiers()` 防引用标识符绕过、`normalizeWhitespace()` 防空白字符绕过、`globMatch()` 替代 `filepath.Match` 防路径分隔符截断、`log.Printf` 警告 malformed glob 模式 |
