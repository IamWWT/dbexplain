# DSL 语法参考

> DSL 模式（`--dsl`）提供跨数据源联邦查询能力，使用 `@label.table` 语法引用数据源。
> 编译管道：预处理 → AST 解析 → 符号绑定 → 后端路由（全程确定性）。

---

## 1. 基础语法

```bash
dbexplain execute --dsl --label src 'SELECT * FROM @src.users WHERE status = "active"'
```

### @label.table 引用

`@label` = DSN 配置中的 `label=` 值，`table` = 该数据源中的表名。

```sql
-- 单数据源
SELECT * FROM @mysql.orders
-- 指定列
SELECT id, name, status FROM @pg.users WHERE status = 'active'
-- 聚合
SELECT dept, COUNT(*) FROM @clickhouse.sales GROUP BY dept
```

### 数据源映射

DSL 自动识别数据源类型，选择对应执行引擎：

| DSN kind | DSL 类型 | 说明 |
|----------|----------|------|
| mysql/postgres/gaussdb/clickhouse/sqlite/duckdb/oracle/hive | SourceSQL | SQL 原生执行 |
| redis | SourceNative | 原生命令透传 |
| mongodb | SourceJSON | JSON 查询 |
| elasticsearch | SourceNative | ES HTTP API |
| qdrant | SourceNative | gRPC API |
| prometheus | SourcePromQL | PromQL 编译 |
| csv/tsv/xlsx | SourceFile | 文件引擎执行 |

---

## 2. 联邦查询（跨数据源 JOIN）

### SQL ↔ SQL

```bash
dbexplain execute --dsl "
  SELECT u.name, o.total
  FROM @pg.users u
  JOIN @mysql.orders o ON u.id = o.user_id
" --human
```

### SQL ↔ 文件

```bash
dbexplain execute --dsl "
  SELECT u.name, c.avg_rate
  FROM @pg.users u
  JOIN @csv.sales_data c ON u.dept_id = c.dept_id
" --human
```

### Prometheus ↔ SQL

```bash
dbexplain execute --dsl "
  SELECT m.metric_name, u.owner
  FROM @prom._metrics m
  JOIN @pg.users u ON m.metric_name = u.metric
" --human
```

---

## 3. 特殊语法：Prometheus

### 普通 DSL（label matchers 编译）

```sql
-- FROM @label.metric → 编译为 metric{matchers}
SELECT * FROM @prom.http_requests_total
SELECT * FROM @prom.http_requests_total WHERE status = '200'
SELECT status, COUNT(*) FROM @prom.http_requests_total GROUP BY status
```

### 原生 PromQL 透传

```sql
-- promql() 内容原样传递给 Prometheus API，不编译
SELECT * FROM @prom.promql(rate(http_requests_total[5m]))
SELECT * FROM @prom.promql(histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)))
```

> promql() 不支持 WHERE/GROUP BY — 过滤在表达式内联。ORDER BY/LIMIT/OFFSET 在 Go 层后处理。

---

## 4. 联邦查询 + promql()

```bash
dbexplain execute --dsl "
  SELECT p.metric, u.owner
  FROM @prom.promql(rate(http_requests_total[5m])) p
  JOIN @pg.app_owners u ON p.metric = u.service_name
" --human
```

---

## 5. 限制

| 限制 | 说明 |
|------|------|
| 原生源 | Redis/Mongo/ES/Qdrant 不可作为 DSL 联邦查询的 JOIN 端 |
| promql() WHERE | 不支持，过滤需在 PromQL 表达式内联 |
| 物化别名 | 联邦查询必须用实际表名（非 placeholder） |
| 多源排序 | ORDER BY 在 Go 层后处理，非数据库原生排序 |
