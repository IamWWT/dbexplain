# L6: SQL 数据库查询执行

> 验证 MySQL、PostgreSQL、ClickHouse、SQLite、Elasticsearch 的只读查询执行。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

## 3.1 MySQL — 基本查询

```bash
$BIN execute -env --db 1 "SELECT 1 AS n, 'hello' AS s" --human
```

预期：返回 1 行 2 列。

## 3.2 MySQL — 表数据

```bash
$BIN execute -env --db 1 "SELECT hostip, device_type FROM testdb.iplist LIMIT 5" --human
```

## 3.3 PostgreSQL — 基本查询

```bash
$BIN execute -env --db 6 "SELECT 1" --human
```

## 3.4 PostgreSQL — 表关联

```bash
$BIN execute -env --db 6 --human \
  "SELECT a.event_time, a.event_type, c.name AS camera_name
   FROM public.abnormal_events a
   JOIN public.cameras c ON a.camera_id = c.id
   ORDER BY a.event_time DESC
   LIMIT 5"
```

## 3.5 ClickHouse — 基本查询

```bash
$BIN execute -env --db 2 "SELECT 1 AS n" --human
```

## 3.6 ClickHouse — 聚合查询

```bash
$BIN execute -env --db 2 --human \
  "SELECT ServiceName, count() AS spans, avg(Duration)/1000000 AS avg_ms
   FROM ai_obs.otel_traces
   GROUP BY ServiceName
   ORDER BY spans DESC
   LIMIT 5"
```

> 注意：ClickHouse 查询不要加分号，否则触发多语句检测。

## 3.7 SQLite — 基本查询

```bash
$BIN execute -env --db 3 "SELECT 1" --human
```

## 3.8 SQLite — 聚合

```bash
$BIN execute -env --db 3 --human \
  "SELECT r.rule_id, r.type, COUNT(h.id) AS hit_count
   FROM rules r
   LEFT JOIN hit_logs h ON r.rule_id = h.rule_id
   WHERE r.status = 'active'
   GROUP BY r.rule_id
   ORDER BY hit_count DESC
   LIMIT 5"
```

## 3.9 Elasticsearch — SHOW TABLES

```bash
$BIN execute -env --db 5 "SHOW TABLES" --human
```

## 3.10 Elasticsearch — 表查询

```bash
$BIN execute -env --db 5 "SELECT title, severity FROM runbooks LIMIT 5" --human
```

## 3.11 EXPLAIN 查询计划

```bash
$BIN execute -env --db 1 --explain "SELECT * FROM testdb.iplist WHERE device_type = 'PHY'" --human
```

## 3.12 自定义超时和行数

```bash
$BIN execute -env --db 6 --timeout 60 --limit 500 "SELECT * FROM public.video_descriptions" --human
```

## 3.13 JSON 结构验证

```bash
$BIN execute -env --db 1 "SELECT 1 AS n, 'hello' AS s" 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
required = ['columns', 'rows', 'row_count', 'execution_time', 'truncated']
for field in required:
    assert field in d, f'missing field: {field}'
    print(f'  ✓ {field}')
assert d['row_count'] == 1
assert d['columns'][0]['name'] == 'n'
print('Execute JSON structure OK ✓')
"
```

## 3.14 REPL 交互模式 (v0.1.2+)

```bash
# 直连 SQLite 启动 REPL，执行查询后退出
echo "SELECT 1 AS val;" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" 2>&1 | grep -E "val|Goodbye"
# 预期: 显示查询结果 val 列，最后显示 Goodbye.

# REPL .help 命令
echo ".help" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" 2>&1 | grep -E "Supported|Commands"
# 预期: 显示 Supported: All 11 data sources 和 Commands 列表

# REPL 写操作拒绝
echo "DROP TABLE t;" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" 2>&1 | grep "READ_ONLY_VIOLATION"
# 预期: READ_ONLY_VIOLATION
```

## 3.15 DSL 联邦查询 (v0.1.2+)

```bash
# 跨源 JOIN（MySQL + SQLite）
$BIN execute -env --dsl "SELECT i.hostip, i.device_type, r.type AS rule_type
  FROM @aiops-mysql.testdb.iplist i
  JOIN @rules-sqlite.rules r ON i.id = r.rule_id
  LIMIT 5" --human
# 预期: 返回两条数据源关联结果
```
