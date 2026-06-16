# Test: Prometheus 时序数据库连接器

> 测试版本: v0.1.7
> 测试功能: Prometheus HTTP API v2.x labels/metadata 采集 (Collect) + PromQL 即时查询 + DSL ORDER BY/LIMIT/OFFSET/SELECT 列投影/GROUP BY 聚合/列别名 + DSL promql() 表达式透传 + DSL 联邦 JOIN + promql() 联邦

## 测试环境

- 构建: `go build -tags full -o ../release/dbexplain ./cmd/dbexplain` (full tags)
- 目标 Prometheus: `prom/prometheus:v3-amd64` @ `192.168.0.127:9440`
- DSN: `prometheus://192.168.0.127:9440?label=my-prom`
- .env 配置优先级: `.env.dbexplain` (CWD) 含 DB16=prometheus://...

## 测试项

### T1: DSN 解析

```bash
cd src
BIN="../release/dbexplain"
```

验证:
- `prometheus://user:pass@host:9090?label=test` → kind=prometheus, label=test
- `prometheus://host:9440` → kind=prometheus, host=host, port=9440
- `prometheus://?timeout=15` → kind=prometheus, timeout=15

### T2: Schema 采集 — meta 表（替代原 targets）

```bash
$BIN -dsn 'prometheus://192.168.0.127:9440?label=my-prom' --json
```

验证:
- instances[0].kind = "prometheus"
- databases[0].name = "prometheus"
- 存在 `_labels` 表（engine=prometheus_meta）
- 存在 `_metrics` 表（engine=prometheus_meta）
- 不存在 engine=prometheus_target 的表（job 表已移除）
- 不包含 job 名作为表名

### T3: Schema 采集 — labels

验证:
- 存在 `_labels` 表（engine=prometheus_meta）
- columns: name
- row_count > 0（表示有 label 名）

### T4: Schema 采集 — metrics

验证:
- 存在 `_metrics` 表（engine=prometheus_meta）
- columns: metric, type, help, unit
- row_count > 0（表示有 metric 元数据）

### T5: PromQL 即时查询

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' 'up == 1' --human
```

验证:
- 返回 vector 类型结果
- 列包括 label keys (__name__, job, instance 等) + timestamp + value
- 行数 > 0
- 执行时间 > 0

### T6: PromQL 标量查询

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' 'count(up)' --human
```

验证:
- 返回标量结果
- 列: timestamp, value

### T7: PromQL 范围向量查询

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' 'up[2m]' --human
```

验证:
- 返回 matrix 结果
- 每个时间序列按时间展开为多行

### T8: JSON 输出格式

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' 'up == 1' --json
```

验证:
- 合法 JSON
- 包含 columns/rows/row_count/execution_time 字段
- row_count > 0

### T9: 安全策略兼容

```bash
# DENY_STATEMENTS 对 PromQL 生效（目前 PromQL 不走 sqlguard，DENY_STATEMENTS 走 CheckNative）
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' 'DROP TABLE up' --human
```

验证:
- 预期拒绝（策略引擎捕获原生命令）

### T10: 向后兼容

```bash
$BIN -env --json
```

验证:
- 原有 15 DSN 仍全部成功
- prometheus DSN (DB16) 新增且在 instances 列表中
- 所有 16 DSN metrics 全部 success

### T11: -env 模式下 Prometheus 采集

```bash
$BIN -env --include my-prom --json
```

验证:
- 仅采集 my-prom 一个实例
- 正确返回 targets/labels/_metrics

### T12: REPL 非 DSL PromQL 查询

```bash
printf 'up{job="prometheus"}\n.exit\n' | $BIN repl \
  --dsn 'prometheus://192.168.0.127:9440?label=prom' --limit 3
```

验证:
- REPL 启动并显示 "connected: prom"
- 查询执行并返回结果（含 __name__/instance/job/value 列）
- `.exit` 正常退出
- 无错误输出

### T13: REPL DSL 模式 PromQL 查询（修复验证）

```bash
printf 'SELECT * FROM @prom.up WHERE job="prometheus"\n.exit\n' | $BIN repl \
  --dsn 'prometheus://192.168.0.127:9440?label=prom' --limit 3
```

验证:
- DSL 编译无错误
- PromQL 等价于 `up{job="prometheus"}`
- 结果正确过滤（仅 prometheus 自身 job 的行）
- 此测试验证 replExecDSL() Vendor 路由修复

### T14: DSL 单源 — ORDER BY value

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, mode, value FROM @my-prom.node_cpu_seconds_total WHERE mode=\"system\" ORDER BY value" \
  --human
```

验证:
- DSL 编译无错误（之前报 "ORDER BY is not supported"）
- 结果按 value 数值升序排列（非字典序）
- 列包含 instance, mode, value

### T15: DSL 单源 — ORDER BY DESC + LIMIT

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, mode, value FROM @my-prom.node_cpu_seconds_total WHERE mode=\"system\" ORDER BY value DESC LIMIT 5" \
  --human
```

验证:
- 返回 5 行
- 按 value 数值降序排列（最大值在前）
- 无截断警告（LIMIT 5 未超限）

### T16: DSL 单源 — ORDER BY + LIMIT + OFFSET

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, value FROM @my-prom.node_cpu_seconds_total ORDER BY value DESC LIMIT 10 OFFSET 3" \
  --human
```

验证:
- 返回 10 行（跳过前 3 行后）
- 内容与 LIMIT 10 不同（验证 OFFSET 生效）
- 第 1 行的 value 等于未使用 OFFSET 时的第 4 行

### T17: DSL 联邦 — Prometheus + MySQL JOIN

```bash
$BIN execute -env --dsl "
  SELECT p.instance, p.hostip, p.job, p.value, i.product, i.subproduct
  FROM @my-prom.up p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
" --human
```

验证:
- 返回跨源 JOIN 结果
- 列包含 `p.instance`, `p.hostip`, `p.job`, `p.value`（Prometheus 侧）
- 列包含 `i.product`, `i.subproduct`（MySQL 侧）——体现联邦查询
- 行数 > 0

### T19: DSL 单源 — SELECT 列投影

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, mode, value FROM @my-prom.node_cpu_seconds_total WHERE mode=\"system\" ORDER BY value DESC LIMIT 5" \
  --human
```

验证:
- 仅返回 3 列（instance, mode, value），非所有列
- 按 value 数值降序排列
- LIMIT 5 生效

### T20: DSL 单源 — GROUP BY + COUNT

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT job, count(value) FROM @my-prom.up GROUP BY job" --human
```

验证:
- 编译无错误
- 列包含 `job` 和 `count(value)`
- 每行对应一个 job 的唯一值
- count 值正确（≥1）

### T21: DSL 单源 — GROUP BY + AVG + ORDER BY

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT mode, avg(value) FROM @my-prom.node_cpu_seconds_total GROUP BY mode ORDER BY avg(value) DESC LIMIT 5" \
  --human
```

验证:
- 列包含 `mode` 和 `avg(value)`
- 按 avg(value) 降序排列
- LIMIT 5 生效

### T22: DSL 单源 — 列别名

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance AS host, value AS val FROM @my-prom.up ORDER BY val DESC LIMIT 3" --human
```

验证:
- 列名显示为 `host` 和 `val`（别名生效）
- 按 val 降序排列

### T23: DSL promql() — 基础 PromQL 表达式透传

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, value FROM @my-prom.promql(rate(node_cpu_seconds_total{mode=\"idle\"}[5m]) * 100)" \
  --human
```

验证:
- DSL 编译通过（不走正常编译路径，直接透传）
- 返回 PromQL 结果
- 列包含 instance, value
- `promql()` 语法被正确识别

### T24: DSL promql() — 多指标二元运算

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, value FROM @my-prom.promql(rate(node_cpu_seconds_total{mode=\"idle\"}[5m]) / rate(node_cpu_seconds_total[5m]) * 100 > 95) ORDER BY value DESC LIMIT 5" \
  --human
```

验证:
- 多指标 PromQL 二元运算成功执行
- `promql()` 内联过滤 > 95 生效
- ORDER BY/LIMIT 后处理仍可工作

### T25: DSL promql() — topk 函数

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT instance, value FROM @my-prom.promql(topk(3, rate(node_cpu_seconds_total[5m]))) ORDER BY value DESC" \
  --human
```

验证:
- PromQL 函数 `topk()` 正常执行
- 精确返回 3 行

### T26: DSL promql() — WHERE 拒绝

```bash
$BIN execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl "SELECT * FROM @my-prom.promql(up) WHERE job=\"prometheus\"" \
  --human
```

验证:
- 预期返回错误: "WHERE is not supported with promql()"
- 提示用户将过滤条件内联到 promql() 表达式中

### T27: DSL 联邦 — promql() + MySQL JOIN

```bash
$BIN execute -env --dsl "
  SELECT p.instance, p.value, i.product, i.subproduct
  FROM @my-prom.promql(topk(10, rate(node_cpu_seconds_total[5m]))) p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  ORDER BY p.value DESC
" --human
```

验证:
- promql() 物化后与 MySQL 联邦 JOIN
- 返回跨源结果（instance, value 来自 PromQL，product, subproduct 来自 MySQL）
- ORDER BY 后处理生效

### T28: DSL 联邦 — promql() + 普通 Prometheus 指标 + MySQL 三源 JOIN

```bash
$BIN execute -env --dsl "
  SELECT p.instance, p.value AS cpu_system_pct, u.value AS up_status, i.product
  FROM @my-prom.promql(rate(node_cpu_seconds_total{mode=\"system\"}[5m]) / rate(node_cpu_seconds_total[5m]) * 100) p
  JOIN @my-prom.up u ON p.instance = u.instance AND u.value = 1
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  ORDER BY p.value DESC
  LIMIT 10
" --human
```

验证:
- 三源联邦 JOIN（promql 复杂表达式 + 普通 Prometheus 指标 + MySQL 表）
- 列别名生效（cpu_system_pct, up_status, product）
- 返回 10 行以内

### T18: DSL 联邦 — ORDER BY + LIMIT

```bash
$BIN execute -env --dsl "
  SELECT p.instance, p.hostip, p.value, i.product
  FROM @my-prom.up p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  ORDER BY p.value DESC
  LIMIT 10
" --human
```

验证:
- 返回跨源 JOIN + 排序 + 截断结果
- 列包含 Prometheus 和 MySQL 两侧的数据
- 按 value 数值降序排列
- 最多 10 行

### T29: JSON 输出 — _labels 含 rows

```bash
$BIN -dsn 'prometheus://192.168.0.127:9440?label=my-prom' --json | python3 -c "
import json,sys
data = json.load(sys.stdin)
for inst in data.get('instances', []):
    for db in inst.get('databases', []):
        for tbl in db.get('tables', []):
            if tbl['name'] == '_labels':
                rows = tbl.get('rows', [])
                print(f\"rows={len(rows)}\")
                if rows:
                    print(f\"first: {rows[0]}\")
"
```

验证:
- rows 存在于 _labels 表中
- rows 数量 > 0（通常约 200+）
- 每行包含 `name` 键

### T30: JSON 输出 — _metrics 含 rows

```bash
$BIN -dsn 'prometheus://192.168.0.127:9440?label=my-prom' --json | python3 -c "
import json,sys
data = json.load(sys.stdin)
for inst in data.get('instances', []):
    for db in inst.get('databases', []):
        for tbl in db.get('tables', []):
            if tbl['name'] == '_metrics':
                rows = tbl.get('rows', [])
                print(f\"rows={len(rows)}\")
                if rows:
                    print(f\"keys: {list(rows[0].keys())}\")
                    print(f\"first: {rows[0]}\")
"
```

验证:
- rows 存在于 _metrics 表中
- rows 数量 > 0（通常约 600+）
- 每行包含 metric, type, help, unit 四个键
- type 值含 counter/gauge/histogram/summary

## 测试结果

| 编号 | 用例 | 状态 | 备注 |
|------|------|------|------|
| T1 | DSN 解析 | **PASS** | kind=prometheus |
| T2 | Schema — meta 表 | **PASS** | 3 meta 表 ✓ |
| T3 | Schema — labels | **PASS** | 206 labels ✓ |
| T4 | Schema — metrics | **PASS** | 657 metrics ✓ |
| T5 | PromQL 即时查询 | **PASS** | 39 rows ✓ |
| T6 | PromQL 标量查询 | **PASS** | count(up)=41 ✓ |
| T7 | PromQL 范围查询 | **PASS** | matrix 展开 ✓ |
| T8 | JSON 输出格式 | **PASS** | 合法 JSON ✓ |
| T9 | 安全策略兼容 | **PASS** | 原生命令校验 ✓ |
| T10 | 向后兼容 | **PASS** | 16/16 成功 ✓ |
| T11 | -env 过滤采集 | **PASS** | 单实例过滤 ✓ |
| T12 | REPL 非 DSL PromQL | **PASS** | 3 rows, clean exit ✓ |
| T13 | REPL DSL 模式 | **PASS** | DSL→PromQL 编译正确 ✓ |
| T14 | DSL 单源 ORDER BY | **PASS** | — |
| T15 | DSL 单源 ORDER BY DESC + LIMIT | **PASS** | — |
| T16 | DSL 单源 ORDER BY + LIMIT + OFFSET | **PASS** | — |
| T17 | DSL 联邦 Prometheus + MySQL JOIN | **PASS** | — |
| T18 | DSL 联邦 ORDER BY + LIMIT | **PASS** | — |
| T19 | DSL 单源 SELECT 列投影 | **PASS** | 仅 3 列 ✓ |
| T20 | DSL 单源 GROUP BY + COUNT | **PASS** | count(value) 列 ✓ |
| T21 | DSL 单源 GROUP BY + AVG + ORDER BY | **PASS** | avg(value) 排序 ✓ |
| T22 | DSL 单源 列别名 | **PASS** | host/val 别名 ✓ |
| T23 | DSL promql() 基础透传 | **PASS** | rate*100 表达式 ✓ |
| T24 | DSL promql() 多指标二元运算 | **PASS** | idle/total*100 ✓ |
| T25 | DSL promql() topk 函数 | **PASS** | topk(3) 精确 3 行 ✓ |
| T26 | DSL promql() WHERE 拒绝 | **PASS** | 正确报错 ✓ |
| T27 | DSL 联邦 promql() + MySQL JOIN | **PASS** | 跨源 JOIN ✓ |
| T28 | DSL 联邦 promql + Prometheus + MySQL 三源 | **PASS** | 三源联邦 ✓ |
| T29 | JSON _labels rows | **PASS** | ~206 rows ✓ |
| T30 | JSON _metrics rows | **PASS** | ~644 rows, 4 keys ✓ |

**总计: 30/30 通过**
