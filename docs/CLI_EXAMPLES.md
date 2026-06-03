# dbexplain CLI 查询案例库

> 所有查询均已在本环境（v0.1.2, 15 数据源）跑通验证。`--human` 用于可读表格输出，不加则为 JSON（供 AI Agent 消费）。
> `--human` 可放在查询语句之前或之后：`dbexplain execute -env --db 1 --human "SELECT 1"` 与 `dbexplain execute -env --db 1 "SELECT 1" --human` 等价。

---

## 1. MySQL — IP 清单查询

```bash
dbexplain execute -env --db 1 --human \
  "SELECT hostip, device_type, datacenter, owner
   FROM testdb.iplist
   WHERE product = 'wtaiops' AND isreg = 1;"
```

| hostip | device_type | datacenter | owner |
|--------|-------------|------------|-------|
| 192.168.0.127 | PHY | xa | ... |
| 192.168.0.127:9443 | URL | xa | ... |

---

## 2. PostgreSQL — 异常事件关联摄像头

```bash
# 注意：schema 为 public，非 videomon
dbexplain execute -env --db 6 --human \
  "SELECT a.event_time, a.event_type, a.severity,
          c.name AS camera_name, c.location
   FROM public.abnormal_events a
   JOIN public.cameras c ON a.camera_id = c.id
   ORDER BY a.event_time DESC
   LIMIT 10;"
```

> **注意**：video_descriptions 表在 public schema 下，不是 videomon.video_descriptions。之前的 42P01 错误即源于此。

---

## 3. SQLite — 规则命中率统计

```bash
dbexplain execute -env --db 3 --human \
  "SELECT r.rule_id, r.type, r.analytics_pattern,
          COUNT(h.id) AS hit_count,
          ROUND(AVG(h.confidence), 2) AS avg_confidence,
          MAX(h.created_at) AS last_hit
   FROM rules r
   LEFT JOIN hit_logs h ON r.rule_id = h.rule_id
   WHERE r.status = 'active'
   GROUP BY r.rule_id
   ORDER BY hit_count DESC
   LIMIT 10;"
```

---

## 4. ClickHouse — Trace 延时聚合

```bash
# 按 ServiceName 聚合 otel traces
dbexplain execute -env --db 2 --human \
  "SELECT ServiceName,
          count() AS spans,
          min(Duration)/1000000 AS min_ms,
          avg(Duration)/1000000 AS avg_ms,
          max(Duration)/1000000 AS max_ms
   FROM ai_obs.otel_traces
   GROUP BY ServiceName
   ORDER BY spans DESC
   LIMIT 10"
```

```bash
# 查看已注册 tool 列表
dbexplain execute -env --db 2 --human \
  "SELECT tool_name, server_name, risk_level, status, usage_count
   FROM ai_obs.tool_registry
   ORDER BY usage_count DESC
   LIMIT 10"
```

> **注意**：ClickHouse 查询加分号时 REPL 模式会自动去除尾部 `;`；`execute` 命令行模式建议不加分号。

---

## 5. Elasticsearch — Runbook 排障手册

```bash
# 列出所有 runbook
dbexplain execute -env --db 5 --human \
  "SELECT title, severity, category, ingested_at
   FROM runbooks
   ORDER BY ingested_at DESC
   LIMIT 10"

# 按严重度筛选
dbexplain execute -env --db 5 --human \
  "SELECT title, severity, symptoms
   FROM runbooks
   WHERE severity = 'critical'
   ORDER BY ingested_at DESC
   LIMIT 5"
```

> **注意**：ES SQL 不支持数组字段（如 `alert_triggers`），SELECT * 会报错。只选标量字段。

---

## 6. MongoDB — 用户查询

```bash
# 最新注册用户
dbexplain execute -env --db 9 --human \
  '{"find":"user","filter":{},"sort":{"create_time":-1},"limit":5}'

# 管理员账号
dbexplain execute -env --db 9 --human \
  '{"find":"user","filter":{"app_manger_level":{"$gte":3}},"sort":{"create_time":-1},"limit":5}'
```

> MongoDB 查询格式为 JSON：`{"find":"<collection>","filter":{...},"sort":{...},"limit":N}`
> 支持 `find` 和 `aggregate` 两种操作。字段名使用实际存在的 `user_id`、`nickname`、`create_time` 等。

---

## 7. Redis — 会话/缓存探查

```bash
# 扫描 key（OpenIM 会话相关）
dbexplain execute -env --db 7 --human \
  "SCAN 0 MATCH CONVERSATION:* COUNT 10"

# 检查 key 类型
dbexplain execute -env --db 7 --human \
  "TYPE CONVERSATION:6571689284:sg_3177718841"

# 获取 hash 数据
dbexplain execute -env --db 7 --human \
  "HGETALL CONVERSATION:6571689284:sg_3177718841"
```

> Redis 支持 30+ 只读命令：GET, HGET, HGETALL, SCAN, TYPE, LLEN, SMEMBERS, ZRANGE 等。
> 写命令（SET, DEL, HSET 等）被内部白名单拦截。

---

## 8. Qdrant — 向量数据库

```bash
# scroll 遍历 Qdrant 数据
dbexplain execute -env --db 4 --human \
  '{"scroll":"runbooks","limit":20}'

# count 统计（480 points）
dbexplain execute -env --db 4 --human \
  '{"count":"runbooks"}'
```

> 当前环境 Qdrant 有 2 个 collections：`mcp_tools` 和 `runbooks`（480 points）。

---

## 9. CSV 文件处理

```bash
# Schema 采集
dbexplain -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" --human

# 查询全部行
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human

# 带 LIMIT/OFFSET
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT * LIMIT 1 OFFSET 1" --human
```

> CSV/TSV/XLSX 由内置 SQL 引擎驱动，支持 WHERE / GROUP BY / JOIN / 聚合函数 / ORDER BY / 窗口函数等完整语法。详见 [`sql-syntax.md`](../dbexplain-skill/references/sql-syntax.md) 和 [`FILE_PROCESSING.md`](FILE_PROCESSING.md)。

---

## 10. Schema Diff — 字段级变更追踪

Schema Diff 支持多版本基线管理和字段级变更检测。

### 首次采集 + 保存版本

```bash
# 采集所有数据库并保存为版本基线
dbexplain -env --cache /tmp/schema.cache --json -o /tmp/v1.json --version-label v1.0

# 列出已保存版本
dbexplain diff --cache /tmp/schema.cache --list-versions
# → v1.0
```

### 基线变更检测

```bash
# 再次采集（同数据库，数据可能已变化）
dbexplain -env --cache /tmp/schema.cache --json -o /tmp/v2.json --version-label v2.0

# 跨版本对比（显示新增/删除/变化的表及字段级差异）
dbexplain diff --cache /tmp/schema.cache --since v1.0 --human
```

输出示例：
```
Schema Diff Report — 2 table(s) changed
============================================================

[added] test.v2 (test)
  Columns (4):
    - id [type] → INTEGER
    - name [type] → TEXT

[removed] test.v1 (test)
  Columns (4):
    - id [type] → INTEGER

[changed] db0.SEND_MSG_FAILED_FLAG:{hex} (openim-redis)
  Columns (1):
    - ttl [comment] → 18h17m16s → 18h16m24s
```

### 双文件对比

```bash
# 用两个 JSON 导出文件做对比
dbexplain diff --before /tmp/v1.json --after /tmp/v2.json --human
```

### JSON 输出（供 AI Agent 消费）

```bash
# 默认输出为 JSON
dbexplain diff --cache /tmp/schema.cache --since v1.0

# JSON 包含字段级变更详情
# {
#   "tables": [{
#     "instance": "...",
#     "table": "...",
#     "status": "changed",
#     "columns": [{"name": "ttl", "field": "comment", "old": "...", "new": "..."}]
#   }]
# }
```

### Cache 文件格式

- `fingerprints`: 表级 SHA256 哈希（快速判断是否变化）
- `snapshots`: 完整表元数据快照（用于字段级 diff）
- `versions`: 命名版本历史（跨版本对比）

JSON 文件自动写入 `*_delta.json`（表级变更）+ `*_diff.json`（字段级变更）。

---

## 通用技巧

### 输出模式切换

```bash
# JSON（默认，供 AI Agent 消费）
dbexplain execute -env --db 1 "SELECT 1"

# 人类可读表格
dbexplain execute -env --db 1 --human "SELECT 1"
```

### DSN 匹配方式

```bash
dbexplain execute -env --db N ...       # 按编号
dbexplain execute -env --label xxx ...  # 按标签
dbexplain execute -dsn 'mysql://...' ... # 直接 DSN
```

### EXPLAIN 查询计划

```bash
dbexplain execute -env --db 6 --human --explain \
  "SELECT * FROM public.abnormal_events WHERE severity >= 4"
```

### 自定义超时和行数

```bash
dbexplain execute -env --db 6 --timeout 60 --limit 500 --human \
  "SELECT * FROM public.video_descriptions"
```

### 列出已配置数据库

```bash
dbexplain list -env
```

### 安全策略控制 (v0.1.0+)

可在 `.env` 或 `~/.config/dbexplain/.env.dbexplain` 中添加安全策略，限制查询范围：

```env
# 禁止查询敏感表
DENY_TABLES=sensitive_data,audit_log

# 禁止查询敏感字段（硬阻断）
DENY_COLUMNS=users.password_hash,orders.card_number

# 禁止执行危险语句
DENY_STATEMENTS=DROP TABLE,ALTER TABLE,FLUSHALL

# 列值屏蔽：替代硬阻断，将敏感列值替换为指定文本（执行后替换）
MASK_COLUMNS=password_hash=***,card_number=****,email=REDACTED
```

```bash
# 策略会拦截对禁用表的访问
dbexplain execute -env --db 1 "SELECT * FROM sensitive_data"
# → ACCESS_DENIED: table "sensitive_data" is not allowed for query

# 策略会拦截对禁用列的访问
dbexplain execute -env --db 1 "SELECT users.password_hash FROM users"
# → ACCESS_DENIED: column "users.password_hash" is not allowed for query

# 列值屏蔽：查询正常执行，但敏感列值被替换（硬阻断优先于屏蔽）
MASK_COLUMNS=hostip=*** dbexplain execute -env --db 1 --human \
  "SELECT hostip, device_type FROM testdb.iplist LIMIT 3"
# → hostip 列显示 ***，device_type 保持原值

# 屏蔽对所有数据库生效（含 MongoDB 等非 SQL）
MASK_COLUMNS=user_id=*** dbexplain execute -env --db 9 --human \
  '{"find":"user","filter":{},"limit":3}'
# → user_id 列显示 ***

# 支持通配符和 table. 前缀
MASK_COLUMNS=testdb.iplist.hostip=REDACTED dbexplain execute -env --db 1 --human \
  "SELECT hostip, device_type FROM testdb.iplist LIMIT 2"
# → hostip 列显示 REDACTED

# 正常查询不受影响
dbexplain execute -env --db 1 --human "SELECT id, name FROM users"
```

---

## 13. REPL 交互模式 — 全数据源切换与查询实录

> 以下为实际环境验证结果（v0.1.2，15 个 DSN 条目，12 种数据源类型）。

### 13.1 启动与帮助

```bash
$ dbexplain repl -env
dbexplain REPL (connected: aiops-mysql)
Type .help for commands, .exit to quit

dbexplain[aiops-mysql]> .help

dbexplain REPL — Interactive query mode
========================================
Supported: All 11 data sources (SQL / NoSQL / Files)
Not supported: DSL mode (@label.table), federated cross-source queries

Commands:
  .conn <label>   Switch to another data source by label (load with -env)
  .dsn <label>    Alias for .conn
  .list           List all configured databases
  .databases      Alias for .list
  .help           Show this help
  .exit / .quit   Exit REPL
  Ctrl+D          Exit REPL

dbexplain[aiops-mysql]> .list
Configured databases:
  #    Label             Kind             Status
  ---  ------            ----             ------
  1    aiops-mysql       mysql            ← current
  2    aiops-clickhouse  clickhouse
  3    veinmap-sqlite    sqlite
  4    video-pg          postgres
  5    aiops-pg          postgres
  6    aiops-redis       redis
  7    openim-redis      redis
  8    aiops-mongo       mongodb
  9    aiops-es          elasticsearch
  10   aiops-qdrant      qdrant
  11   aiops-csv         csv
  12   aiops-tsv         tsv
  13   aiops-xlsx        xlsx
```

> `.list` 显示所有 `-env` 加载的 DSN 条目及当前连接状态，方便快速定位目标 label 后通过 `.conn` 切换。

### 13.2 SQL 数据源切换与查询

```bash
# ── MySQL ──
dbexplain[aiops-mysql]> SELECT COUNT(*) AS cnt FROM port
+-----+
| cnt |
+-----+
| 33  |
+-----+
1 row(s) in set (585µs)
(query completed in 2ms)

dbexplain[aiops-mysql]> SELECT ID, hostip, arch, owner FROM iplist LIMIT 3
+----+---------------------+--------------+--------+
| ID | hostip              | arch         | owner  |
+----+---------------------+--------------+--------+
| 34 | 192.168.0.127       | archtestMASK | 王婉婷 |
| 35 | 192.168.0.127:11434 | archtestMASK | 王婉婷 |
| 36 | 192.168.0.127:9443  | archtestMASK | 王婉婷 |
+----+---------------------+--------------+--------+
3 row(s) in set (217µs)
# arch=archtestMASK → MASK_COLUMNS 生效

# ── ClickHouse（不带分号） ──
dbexplain[aiops-mysql]> .conn aiops-clickhouse
Switched to: aiops-clickhouse

dbexplain[aiops-clickhouse]> SELECT COUNT(*) AS total FROM system.tables
+-------+
| total |
+-------+
| 162   |
+-------+
1 row(s) in set (2ms)

# ── PostgreSQL ──
dbexplain[aiops-clickhouse]> .conn video-pg
Switched to: video-pg

dbexplain[video-pg]> SELECT 1 AS test
+------+
| test |
+------+
| 1    |
+------+
1 row(s) in set (120µs)
(query completed in 3ms)

# ── SQLite ──
dbexplain[video-pg]> .conn veinmap-sqlite
Switched to: veinmap-sqlite

dbexplain[veinmap-sqlite]> SELECT name FROM sqlite_master WHERE type='table' LIMIT 5
+-------------------+
| name              |
+-------------------+
| feedback          |
| sqlite_sequence   |
| profile_passwords |
| login_history     |
| profiles          |
+-------------------+
5 row(s) in set (734µs)
```

### 13.3 NoSQL 数据源切换与查询

```bash
# ── Redis ──
dbexplain[aiops-mysql]> .conn openim-redis
Switched to: openim-redis

dbexplain[openim-redis]> PING
+--------+
| result |
+--------+
| PONG   |
+--------+
1 row(s) in set (737µs)

dbexplain[openim-redis]> EXISTS user:1
+--------+
| result |
+--------+
| 0      |
+--------+
1 row(s) in set (264µs)

# SCAN（安全替代 KEYS）
dbexplain[openim-redis]> .conn video-redis
Switched to: video-redis

dbexplain[video-redis]> SCAN 0 COUNT 5
+--------+
| result |
+--------+
| 0      |
| []     |
+--------+
2 row(s) in set (626µs)

# ── MongoDB ──
dbexplain[video-redis]> .conn openim-mongo
Switched to: openim-mongo

dbexplain[openim-mongo]> {"find":"groups","filter":{},"limit":1}
(empty result)
(query completed in 3ms)

# ── Qdrant ──
dbexplain[openim-mongo]> .conn aiops-qdrant
Switched to: aiops-qdrant

dbexplain[aiops-qdrant]> {"scroll":"aiops"}
+------------+--------------+
| collection | points_count |
+------------+--------------+
+------------+--------------+
0 row(s) in set (296µs)
(query completed in 2ms)
```

### 13.4 文件数据源查询

```bash
# ── CSV ──
dbexplain[aiops-qdrant]> .conn ops-data-csv
Switched to: ops-data-csv

dbexplain[ops-data-csv]> SELECT category, COUNT(*) AS cnt FROM data GROUP BY category ORDER BY cnt DESC
+--------------+-----+
| category     | cnt |
+--------------+-----+
| 操作系统运维  | 280 |
| 网络运维      | 180 |
| 中间件运维    | 135 |
| 信息安全运维  | 103 |
| 应用业务运维  | 95  |
| 基础设施运维  | 88  |
| 软件架构运维  | 58  |
| 数据库运维    | 44  |
| 系统架构运维  | 11  |
+--------------+-----+
9 row(s) in set (78µs)
(query completed in 1ms)

# ── XLSX ──
dbexplain[ops-data-csv]> .conn tsf-xlsx
Switched to: tsf-xlsx

dbexplain[tsf-xlsx]> SELECT * LIMIT 2
+-------------+-----------+--------------+------------------+
| 分类        | 模块名称  | 进程名称     | 启停方式         |
+-------------+-----------+--------------+------------------+
| tsf-storage | tsf-mysql | mysqld       | systemctl start  |
|             | tsf-ctsdb | tsf-ctsdb    | cd /data/common/ |
+-------------+-----------+--------------+------------------+
2 row(s) in set (8ms)

# ── TSV ──
dbexplain[tsf-xlsx]> .conn tsv-test-data
Switched to: tsv-test-data

dbexplain[tsv-test-data]> SELECT * LIMIT 3
+---+---------+
| $ | centavo |
+---+---------+
| € | céntimo |
| £ | penique |
| ₩ | chon    |
+---+---------+
3 row(s) in set (0s)
```

### 13.5 安全策略验证

```bash
# 写操作拒绝
dbexplain[aiops-mysql]> DROP TABLE iplist
ERROR: READ_ONLY_VIOLATION: write operation "DROP" is not allowed

dbexplain[aiops-mysql]> INSERT INTO iplist VALUES(1)
ERROR: READ_ONLY_VIOLATION: write operation "INSERT" is not allowed

dbexplain[aiops-mysql]> DELETE FROM iplist
ERROR: READ_ONLY_VIOLATION: write operation "DELETE" is not allowed

# DENY_TABLES
dbexplain[aiops-mysql]> SELECT * FROM information_schema.tables LIMIT 1
ERROR: ACCESS_DENIED: table "information_schema" is not allowed for query

dbexplain[video-pg]> SELECT * FROM pg_catalog.pg_tables LIMIT 1
ERROR: ACCESS_DENIED: table "pg_catalog" is not allowed for query

# DENY_COLUMNS（限定列名）
dbexplain[aiops-mysql]> SELECT iplist.owner FROM iplist LIMIT 1
ERROR: ACCESS_DENIED: column "iplist.owner" is not allowed for query

# MongoDB 写入拒绝
dbexplain[openim-mongo]> {"insert":"test","documents":[{"x":1}]}
ERROR: QUERY_ERROR: READ_ONLY_VIOLATION: mongo query must specify "find"

# Redis 危险命令拒绝
dbexplain[video-redis]> KEYS *
ERROR: QUERY_ERROR: READ_ONLY_VIOLATION: redis command "KEYS"
```

### 13.6 已知限制演示

```bash
# ClickHouse 尾部 ; 已自动去除，不再触发多语句错误

# Elasticsearch 暂不支持
dbexplain[aiops-es]> {"query":{"match_all":{}}}
ERROR: READ_ONLY_VIOLATION: unknown or unsupported SQL verb

# 无效 label
dbexplain[aiops-mysql]> .conn nonexistent
No DSN with label "nonexistent" found

# 未知命令
dbexplain[aiops-mysql]> .unknown
Unknown command: .unknown (try .help)

# .dsn 别名
dbexplain[aiops-mysql]> .dsn openim-redis
Switched to: openim-redis
```

> 完整 REPL 文档见 [`docs/REPL.md`](REPL.md)。已知限制详情及绕过方案见 REPL.md 已知限制章节。

---

| DB | Label | Kind | 关键表/集合 | 数据量 |
|----|-------|------|-----------|--------|
| DB1 | aiops-mysql | mysql | testdb.iplist, port | 12 / 30 行 |
| DB2 | aiops-clickhouse | clickhouse | ai_obs.otel_traces, tool_registry | 2 数据库 |
| DB3 | intentapparatus-sqlite | sqlite | rules, hit_logs | 5+ 表 |
| DB4 | aiops-qdrant | qdrant | mcp_tools, runbooks | 480 points |
| DB5 | aiops-es | elasticsearch | runbooks 等 | 17 索引 |
| DB6 | video-pg | postgres | public.abnormal_events, cameras | 5+ 表 |
| DB7 | openim-redis | redis | CONVERSATION:*, MSG_CACHE:* | 有数据 |
| DB8 | video-redis | redis | _server_info | 有数据 |
| DB9 | openim-mongo | mongodb | user, system.users | 5+ collections |
| DB10 | veinmap-sqlite | sqlite | 4 表 | 有数据 |
| DB11 | tsf-xlsx | xlsx | 3 sheets | 45+14+6 行 |
| DB12 | tdmq-xlsx | xlsx | 1 sheet | 有数据 |
| DB13 | csv-users | csv | users | 3 行 |
| DB14 | csv-test-data | csv | users, products, types | 3 表 |
| DB15 | tsv-test-data | tsv | data | 2 行 |

---

*案例库持续更新中。v0.1.2 新增 DSL 联邦查询 + REPL 模式（`--dsl`）、Schema Diff、窗口函数、`internal/` 结构整理。全部查询已通过 --human 实测验证。*
