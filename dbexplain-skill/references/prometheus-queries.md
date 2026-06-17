# Prometheus 查询参考

> Prometheus 查询通过 `dbexplain execute` 执行。LLM 已知的 PromQL 语法不再赘述，只标注 DSL 编译映射和 dbexplain 特有的 Meta 表。

---

## 1. DSL → PromQL 编译映射

| DSL | 编译后 PromQL |
|-----|--------------|
| `SELECT * FROM metric` | `metric` |
| `...WHERE k = "v"` | `{k="v"}` |
| `...WHERE k =~ "v"` | `{k=~"v"}` |
| `...WHERE k != "v"` | `{k!="v"}` |
| `GROUP BY k` | `count by (k) (metric)` |

## 2. 原生 PromQL 透传

```bash
# promql() 内容原样传给 Prometheus API，不编译
dbexplain execute --label prom "SELECT * FROM @prom.promql(rate(http_requests_total[5m]))" --human
```

promql() 不支持 WHERE/GROUP BY — 过滤在表达式内联。ORDER BY/LIMIT/OFFSET 在 Go 层后处理。

## 3. Meta 表（采集时自动生成）

| 表名 | 内容 | 典型列 |
|------|------|--------|
| `_labels` | 所有 label 名 | `label_name` |
| `_metrics` | metric 元数据 | `metric_name`, `type`, `help`, `unit` |

```sql
SELECT * FROM _metrics WHERE type = 'counter'
```

## 4. 限制

- promql() WHERE/GROUP BY：不支持
- ORDER BY / LIMIT：Go 层后处理，非 PromQL 原生
