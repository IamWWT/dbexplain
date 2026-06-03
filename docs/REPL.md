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
| `.conn <label>` | 按 label 切换数据源（在 `-env` 加载的全部条目中按 label 精确查找） |
| `.dsn <label>` | `.conn` 的别名，行为完全相同 |
| `.list` / `.databases` | 列出所有已配置的数据源（序号、label、kind、当前连接标记） |
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

### 1. REPL 不支持 DSL / 联邦查询

DSL `@label.table` 语法和跨源联邦 JOIN 仅在 `dbexplain execute -env --dsl` 命令行模式下可用。REPL 内请使用原生 SQL 或 NoSQL 命令。

### 2. Elasticsearch 暂不支持 REPL

ES 的 JSON 原生查询格式（如 `{"query":{"match_all":{}}}`）在 REPL 中无法正确路由。ES 驱动注册为 SQL 类型（CapSQL），JSON 查询会被送入 SQL 校验器（sqlguard），而 sqlguard 无法解析 JSON 语法结构，返回 `READ_ONLY_VIOLATION: unknown or unsupported SQL verb`。

**绕过方案**：
- 使用 `dbexplain execute -env --label <es-label> --human` 执行 ES 查询
- ES 支持 SQL 语法（通过 `_sql` REST 端点），可在 `execute` 模式下使用标准 SQL 查询 ES 索引
- 采集 Schema 使用 `dbexplain collect -env --label <es-label> --human`

### 3. MySQL 单连接模式

MySQL 驱动在 `SET max_execution_time` 后强制单连接（`SetMaxOpenConns(1)`），确保超时在当前连接生效。这意味着 REPL 下 MySQL 查询不具备并发性能。

### 4. 仅首次连接可用

REPL 默认使用配置中第一个 DSN 条目作为初始连接。如果没有配置项，启动时报错退出。`.conn` 切换必须依赖 `-env` 预加载的全部条目。

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
| Elasticsearch | `{"query":{"match_all":{}}}` | ❌ 暂不支持（见限制 2） |
| Qdrant | `{"scroll":"collection_name"}` | ✅ 通过 |

### 文件数据源

| 数据源 | 查询 | 结果 |
|--------|------|------|
| CSV | `SELECT * LIMIT 3` / `SELECT cat, COUNT(*) FROM data GROUP BY cat` | ✅ 全部通过 |
| XLSX | `SELECT * LIMIT 3` | ✅ 通过 |
| TSV | `SELECT * LIMIT 3` | ✅ 通过 |

---

## 完整输出示例

见 [`CLI_EXAMPLES.md`](CLI_EXAMPLES.md) — REPL 章节，包含 15 个数据源的完整切换和查询记录。

---

## 排障指南

见 `references/troubleshooting.md`。
