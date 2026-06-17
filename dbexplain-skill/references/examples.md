# 场景示例

> 覆盖 16 种数据源的典型使用场景。每条命令可直接复制运行（替换 label 和实际值）。

---

## 1. Schema 采集

```bash
# 全量采集，输出 AI 上下文
dbexplain --context ./ctx

# 按类型过滤
dbexplain -include mysql,postgres
dbexplain -exclude redis,mongodb

# 增量检测
dbexplain --cache schema.cache
```

---

## 2. SQL 数据库

### MySQL / PostgreSQL / GaussDB / SQLite / DuckDB / Oracle / Hive

```bash
# 行数
dbexplain execute --label mysql 'SELECT COUNT(*) FROM orders' --human

# 分组聚合
dbexplain execute --label pg 'SELECT dept, AVG(salary) FROM employees GROUP BY dept ORDER BY avg DESC' --human

# 最近 N 条
dbexplain execute --label mysql 'SELECT * FROM orders ORDER BY created_at DESC LIMIT 10' --human

# EXPLAIN 分析
dbexplain execute --label mysql 'EXPLAIN SELECT * FROM orders WHERE id = 1' --human

# 跨库查询（需 DSL 模式）
dbexplain execute --dsl "SELECT u.name, o.total FROM @pg.users u JOIN @mysql.orders o ON u.id = o.user_id" --human
```

### ClickHouse

```bash
# 注意：execute 命令行不要加分号
dbexplain execute --label ch 'SELECT COUNT(*) FROM events' --human
dbexplain execute --label ch 'SELECT toDate(event_time), COUNT(*) FROM events GROUP BY toDate(event_time)' --human
```

---

## 3. NoSQL

### Redis

```bash
# 查看所有 user 前缀的 key
dbexplain execute --label redis 'KEYS user:*' --human
# Hash 详情
dbexplain execute --label redis 'HGETALL user:1001' --human
# List 范围
dbexplain execute --label redis 'LRANGE queue 0 10' --human
# ZSet 排行
dbexplain execute --label redis 'ZRANGE leaderboard 0 10 WITHSCORES' --human
```

### MongoDB

```bash
# 简单过滤
dbexplain execute --label mongo '{"find":"users","filter":{"status":"active"}}' --human
# 聚合管道
dbexplain execute --label mongo '[{"$match":{"status":"active"}},{"$group":{"_id":"$dept","count":{"$sum":1}}}]' --human
```

### Elasticsearch

```bash
# 匹配查询
dbexplain execute --label es '{"query":{"match":{"status":"active"}}}' --human
# 聚合
dbexplain execute --label es '{"query":{"match_all":{}},"aggs":{"by_dept":{"terms":{"field":"department.keyword"}}}}' --human
```

### Qdrant

```bash
# 向量搜索
dbexplain execute --label qdrant '{"search":"my_collection","vector":[0.1,0.2,0.3],"limit":5,"with_payload":true}' --human
```

---

## 4. Prometheus

```bash
# 当前值
dbexplain execute --label prom 'SELECT * FROM http_requests_total' --human

# 速率（原生 PromQL）
dbexplain execute --label prom "SELECT * FROM @prom.promql(rate(http_requests_total[5m]))" --human

# Meta 表
dbexplain execute --label prom 'SELECT * FROM _labels' --human
dbexplain execute --label prom 'SELECT * FROM _metrics WHERE type = "counter"' --human

# 联邦查询 Prometheus + MySQL
dbexplain execute --dsl "
  SELECT p.metric, u.owner
  FROM @prom._metrics p
  JOIN @pg.app_owners u ON p.metric = u.service_name
" --human
```

---

## 5. 文件数据源（CSV/TSV/XLSX）

```bash
# 数据预览
dbexplain execute --label csv 'SELECT *' --limit 5 --human

# 聚合
dbexplain execute --label csv 'SELECT department, AVG(rate) FROM data GROUP BY department' --human

# 跨文件 JOIN
dbexplain execute --label org "SELECT t.*, o.branch FROM data t JOIN org_info o ON t.dept_id = o.dept_id" --human

# UNION
dbexplain execute --label csv "SELECT dept, rate FROM Q1 UNION ALL SELECT dept, rate FROM Q2" --human
```

---

## 6. DSL 联邦查询

```bash
# SQL + SQL
dbexplain execute --dsl "
  SELECT u.name, o.total FROM @pg.users u JOIN @mysql.orders o ON u.id = o.user_id
" --human

# SQL + 文件
dbexplain execute --dsl "
  SELECT u.dept, AVG(c.rate) FROM @pg.users u JOIN @csv.sales_data c ON u.dept_id = c.dept_id GROUP BY u.dept
" --human

# Prometheus + SQL
dbexplain execute --dsl "
  SELECT m.metric, u.owner
  FROM @prom._metrics m
  JOIN @pg.app_owners u ON m.metric = u.service_name
" --human

# PromQL + SQL 联邦
dbexplain execute --dsl "
  SELECT p.metric, u.owner
  FROM @prom.promql(rate(http_requests_total[5m])) p
  JOIN @pg.app_owners u ON p.metric = u.service_name
" --human
```

---

## 7. 健康检查与排障

```bash
# 连通性检查
dbexplain check

# 查看帮助手册，过滤关键字
dbexplain all --filter execute
dbexplain all --filter dsl

# 加密配置
dbexplain encrypt .env.dbexplain

# 清单采集
dbexplain list
```
