# REPL 交互模式

> `dbexplain repl` — 交互式查询循环，`.conn` 随时切换数据源，自动计时。

---

## 启动方式

### 从 .env 配置加载

```bash
dbexplain repl -env
```

自动发现 `.env.dbexplain` 配置文件，加载全部 DSN 条目，默认使用第一个作为初始连接。

### 直连一个数据源

```bash
dbexplain repl --dsn 'mysql://user:pass@host:3306/mydb?label=prod'
```

### 自定义参数

```bash
dbexplain repl -env --limit 5000 --timeout 60
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--limit` | 1000 | 每查询最大返回行数，防止意外全表导出 |
| `--timeout` | 30 | 单查询超时秒数，超时自动终止 |

---

## REPL 内命令

| 命令 | 说明 |
|------|------|
| `.connect <dsn>` | 通过 DSN URL 连接新数据源（无需 `-env` 预加载），如 `.connect mysql://user:pass@host:3306/db?label=mydb` |
| `.conn <label>` | 按 label 切换数据源（在 `-env` 加载的全部条目中按 label 精确查找） |
| `.dsn <label>` | `.conn` 的别名，行为完全相同 |
| `.list` / `.databases` | 列出所有已配置的数据源（序号、label、DSN 密码脱敏、kind、当前连接标记） |
| `.help` | 显示帮助信息、支持的数据源类型、不支持的功能 |
| `.exit` / `.quit` | 退出 REPL |
| `Ctrl+D` | 退出 REPL（发送 EOF） |

### 切换行为

`.conn` 在预加载的 DSN 条目列表中按 label 查找，找到后替换当前连接。后续查询立即在新连接上执行。找不到 label 时输出提示，不中断 REPL 会话。

```
dbexplain[aiops-mysql]> .conn aiops-clickhouse
Switched to: aiops-clickhouse

dbexplain[aiops-clickhouse]> .conn nonexistent
No DSN with label "nonexistent" found

dbexplain[aiops-clickhouse]>
```

---

## 自动行为

| 特性 | 说明 |
|------|------|
| **行数上限** | 自动 LIMIT（默认 1000），可通过 `--limit N` 调整 |
| **查询超时** | 超时自动终止（默认 30s），可通过 `--timeout N` 调整 |
| **执行计时** | 每次查询后显示耗时（毫秒精度），不可关闭 |
| **连接池** | 每数据源独立连接，切换时自动建立新连接 |
| **空输入** | 直接回车忽略，不执行任何操作 |
| **未知命令** | `.unknown` 等未识别命令输出 `Unknown command: xxx (try .help)` 并继续 |

---

## 安全机制

REPL 内所有查询受完整安全策略保护：

| 策略 | REPL 中行为 | 测试结果 |
|------|------------|----------|
| **只读校验** | DROP/INSERT/UPDATE/DELETE 等写操作全部拒绝 | ✅ `READ_ONLY_VIOLATION` |
| **DENY_TABLES** | 禁止表名匹配时拒绝查询 | ✅ `ACCESS_DENIED: table "xxx"` |
| **DENY_COLUMNS** | 列名匹配时拒绝查询 | ✅ `ACCESS_DENIED: column "xxx"` |
| **MASK_COLUMNS** | 匹配列值替换为配置的掩码字符串 | ✅ 值被替换 |
| **Redis KEYS** | 生产危险命令被阻断 | ✅ `READ_ONLY_VIOLATION` |
| **MongoDB 写操作** | insert/update/delete 等原生写入拒绝 | ✅ `READ_ONLY_VIOLATION` |

### DENY_COLUMNS 匹配规则

策略引擎要求列引用使用 `表名.列名` 限定格式才能精确匹配。如果查询中使用裸列名（如 `SELECT owner FROM iplist`），策略引擎可能无法匹配 `DENY_COLUMNS=iplist.owner`。

```bash
# 可能不阻断（裸列名）
SELECT owner FROM iplist

# 一定阻断（限定列名）
SELECT iplist.owner FROM iplist
```

这是策略引擎的行为，不限于 REPL 模式。

---

## 已知限制

### 1. Elasticsearch JSON _search 限制

ES 原生 JSON 查询（`{"query":{"match_all":{}}}`）已在 REPL 中支持，通过 `IsSQL=false` 路径绕过 sqlguard 并路由到 `_search` REST 端点。响应结果从 `hits.hits[]._source` 提取动态列名和行数据。

**限制**：
- _search 响应中的嵌套对象和数组会被 `%v` 格式化为字符串
- 列名在每次查询时动态确定（来自所有 hit 的 _source key 并集），不同文档可能有不同列
- ES SQL 查询（标准 SQL 语法）仍然通过 `_sql` 端点执行，支持完整的 sqlguard 安全校验

### 2. MySQL 单连接模式

MySQL 驱动在 `SET max_execution_time` 后强制单连接（`SetMaxOpenConns(1)`），确保超时在当前连接生效。这意味着 REPL 下 MySQL 查询不具备并发性能。

### 3. 无配置启动

REPL 在没有 DSN 配置时进入 `(disconnected)` 状态，通过 `.connect <dsn-url>` 命令动态连接数据源。`.connect` 添加的 DSN 也会被 `.list` 列出，并支持后续的 `.conn` 切换。也可以通过 `-env` 预加载配置后使用 `.conn` 切换。

---

## 各数据源测试结果

### SQL 数据库

| 数据源 | 查询 | 结果 |
|--------|------|------|
| MySQL | `SELECT 1` / `SHOW TABLES` / `SELECT ... JOIN ...` / `COUNT(*)` | ✅ 全部通过 |
| ClickHouse | `SELECT 1`（无分号） / `SELECT COUNT(*) FROM system.tables` | ✅ 通过 |
| ClickHouse | `SHOW TABLES;`（有分号） | ✅ 已修复，自动去除尾部 `;` |
| PostgreSQL | `SELECT 1` / 聚合查询 | ✅ 通过 |
| SQLite | `SELECT 1` / 查询系统表 `sqlite_master` | ✅ 通过 |

### NoSQL 数据库

| 数据源 | 查询 | 结果 |
|--------|------|------|
| Redis | `PING` / `EXISTS key` / `SCAN 0 COUNT 5` | ✅ 通过 |
| Redis | `KEYS *` / `FLUSHALL` | ❌ 拒绝（安全策略） |
| MongoDB | `{"find":"groups","filter":{},"limit":1}` | ✅ 通过 |
| MongoDB | `{"insert":"test","documents":[{"x":1}]}` | ❌ 拒绝（只读） |
| Elasticsearch | `{"query":{"match_all":{}}}` | ✅ 支持（_search 端点） |
| Qdrant | `{"scroll":"collection_name"}` | ✅ 通过 |

| Prometheus | `up == 1` / `count(up)` / PromQL 即时查询 | ✅ 通过（DSL 模式） |

### 文件数据源

| 数据源 | 查询 | 结果 |
|--------|------|------|
| CSV | `SELECT * LIMIT 3` / `SELECT cat, COUNT(*) FROM data GROUP BY cat` | ✅ 全部通过 |
| XLSX | `SELECT * LIMIT 3` | ✅ 通过 |
| TSV | `SELECT * LIMIT 3` | ✅ 通过 |

---

## 完整输出示例

见 [`CLI_EXAMPLES.md`](CLI_EXAMPLES.md) — REPL 章节，包含 17+ DSN 条目的完整切换和查询记录。

---

## 排障指南

见 `references/troubleshooting.md`。
