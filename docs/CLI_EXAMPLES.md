# dbexplain CLI 查询案例库

> 所有查询均已在本环境（v0.0.7, 9 数据源）跑通验证。`--human` 用于可读表格输出，不加则为 JSON（供 AI Agent 消费）。

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

> **注意**：ClickHouse 查询**不要加分号**，否则会触发 `Multi-statements are not allowed` 错误。

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
# scroll 遍历 collection 数据
dbexplain execute -env --db 4 --human \
  '{"scroll":"<collection_name>","limit":20}'

# count 统计
dbexplain execute -env --db 4 --human \
  '{"count":"<collection_name>"}'
```

> 当前环境 Qdrant 无 collection，替换 `<collection_name>` 为实际名称后可用。

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

---

## 当前环境数据库一览

| DB | Label | Kind | 关键表/集合 | 数据量 |
|----|-------|------|-----------|--------|
| DB1 | aiops-mysql | mysql | testdb.iplist | 12 行 |
| DB2 | aiops-clickhouse | clickhouse | ai_obs.otel_traces, tool_registry | 532 / 61 |
| DB3 | aiops-sqlite | sqlite | rules, hit_logs | 10 / 0 |
| DB4 | qdrant-test | qdrant | (空) | 0 |
| DB5 | es-test | elasticsearch | runbooks | 5 |
| DB6 | video-pg | postgres | public.abnormal_events, cameras, video_descriptions | 有数据 |
| DB7 | openim-redis | redis | CONVERSATION:*, MSG_CACHE:*, ... | 有数据 |
| DB8 | video-redis | redis | (空) | 0 |
| DB9 | mongo-test | mongodb | user | 5 |

---

*案例库生成于 v0.0.7，全部查询已通过 --human 实测验证。*
