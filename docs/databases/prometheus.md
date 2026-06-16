# Prometheus — 时序数据库连接器

> [English version](#prometheus-time-series-database-connector)

## DSN 格式

```bash
prometheus://[user[:password]@]host:port[?label=name][&timeout=15][&tls=true]
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `user` | 否 | — | Basic Auth 用户名 |
| `password` | 否 | — | Basic Auth 密码 |
| `host` | 否 | `127.0.0.1` | Prometheus 服务器地址 |
| `port` | 否 | `9090` | Prometheus HTTP API 端口 |
| `label` | 否 | `host:port` | 实例别名，用于 DSL 引用和日志标识 |
| `timeout` | 否 | `10` | HTTP 请求超时秒数 |
| `tls` | 否 | `false` | 启用 HTTPS (`true`/`1`) |

### 示例

```bash
# 最小配置（默认 127.0.0.1:9090）
dbexplain -dsn 'prometheus://?label=local-prom'

# 完整配置
dbexplain -dsn 'prometheus://admin:pass@192.168.0.127:9440?label=my-prom&timeout=15'
```

---

## Schema 采集 (Collect)

`dbexplain -dsn 'prometheus://...' --json` 将从 Prometheus HTTP API v2.x 采集两类信息：

### 1. Labels（标签名）
- **API**: `GET /api/v1/labels`
- 表名 `_labels`，列 `name`
- RowCount = label 总数

### 2. Metric 元数据
- **API**: `GET /api/v1/metadata`
- 表名 `_metrics`，列 `metric`、`type`、`help`、`unit`
- RowCount = metric 总数

> **与 MySQL 的映射对比**：
>
> ```
> MySQL:        @aiops-mysql.iplist        → SHOW TABLES → iplist,        DESCRIBE iplist → [hostip, product, ...]
> Prometheus:   @my-prom.up                → _metrics → up(metric名),    PromQL 动态列 → [__name__, instance, job, hostip, product, subproduct, timestamp, value]
> Prometheus:   @my-prom.node_cpu_seconds_total → _metrics → metric名,   PromQL 动态列 → [__name__, instance, job, cpu, mode, hostip, product, subproduct, timestamp, value]
> ```
>
> **发现流程**：
> 1. `_metrics` 查看所有 metric（类似 MySQL `SHOW TABLES`）
> 2. `_labels` 查看所有 label 名（类似 MySQL `SHOW COLUMNS` 的列名集合）
> 3. PromQL 查询 `node_cpu_seconds_total{mode="system"}` 直接查看动态列
> 4. DSL 联邦 `SELECT * FROM @my-prom.up WHERE product="wtaiops"` 跨源 JOIN

### 示例输出

```bash
$ dbexplain -dsn 'prometheus://192.168.0.127:9440?label=my-prom' --human
> DSN mapping:
  DB16 → my-prom              prometheus://***@192.168.0.127:9440

kind     label    db           table              columns              rows
───────  ───────  ───────────  ─────────────────  ───────────────────  ──────
promethe my-prom  prometheus   _labels            name                 206
promethe my-prom  prometheus   _metrics           metric, type...      657
```

### JSON 输出中的 `rows`（v0.1.7+）

`dbexplain -dsn 'prometheus://...' --json` 输出中，`_labels` 和 `_metrics` 表包含 `rows` 字段，携带全量样本数据供 LLM Agent 消费：

```json
{
  "name": "_labels",
  "columns": [{"name": "name", "type": "text"}],
  "row_count": 206,
  "rows": [
    {"name": "__name__"},
    {"name": "instance"},
    {"name": "job"},
    ...
  ]
}
```

```json
{
  "name": "_metrics",
  "columns": [
    {"name": "metric", "type": "text"},
    {"name": "type", "type": "text"},
    {"name": "help", "type": "text"},
    {"name": "unit", "type": "text"}
  ],
  "row_count": 657,
  "rows": [
    {"metric": "up", "type": "gauge", "help": "The up scrape metric", "unit": ""},
    {"metric": "node_cpu_seconds_total", "type": "counter", "help": "CPU time", "unit": "seconds"},
    ...
  ]
}
```

> **为什么 `--human` 不显示 `rows`？** 终端输出面向人类，聚焦表结构（列名、类型、行数）而非数据。JSON 输出面向 LLM Agent，Agent 需要样本数据做 NL→PromQL 语义匹配（知道有哪些 metric 名、type、help 描述）。

> **设计架构**：
> 1. **Schema 层**：`schema.Table.SampleRows []map[string]any` — 通用扩展点，任意连接器可填充
> 2. **采集层**（`prometheus.go`）：`collectLabels()` / `collectMetricsMeta()` 在采集时填充数据
> 3. **JSON 渲染层**（`render.go`）：`buildJSONResult()` 将 `SampleRows` 映射为 `rows`
> 4. **只在 JSON 路径输出**：仅 `--json` 渲染 rows，终端/人类格式不渲染
>
> `map[string]any` 保持通用性 — _labels 只有 `name` 键，_metrics 有 `metric`/`type`/`help`/`unit` 四个键。

---

## PromQL 查询执行 (ExecQuery)

`dbexplain execute -dsn 'prometheus://...' '<promql>' --human`

### 安全机制
- PromQL 是只读查询语言，无写入端点暴露
- 不走 sqlguard（非 SQL），走 `CheckNative()` 策略引擎检查
- 支持 `DENY_STATEMENTS` / `MASK_COLUMNS` 等策略规则

### 查询类型支持

| PromQL 结果类型 | 支持 | 说明 |
|----------------|------|------|
| **vector** (即时向量) | ✅ | 每个 metric + 标签展开为一行，含 timestamp + value |
| **matrix** (范围向量) | ✅ | 多时间序列按时间展开为多行 |
| **scalar** (标量) | ✅ | 单行 timestamp + value |
| **string** (字符串) | ✅ | 单行 value |

### 示例

```bash
# 即时查询：所有 up == 1 的目标
dbexplain execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' 'up == 1' --human

# 聚合查询：总指标数
dbexplain execute -dsn 'prometheus://...' 'count(up)' --human

# 范围查询：过去 5 分钟的数据
dbexplain execute -dsn 'prometheus://...' 'avg(node_cpu_seconds_total[5m])' --human

# JSON 输出
dbexplain execute -dsn 'prometheus://...' 'up{job="node"}' --json
```

### 输出格式

自动从 PromQL 结果的 `metric` 标签映射到列：
- 每个 label key 为一列（如 `__name__`、`job`、`instance`）
- 追加 `timestamp` 和 `value` 列
- 即时向量每行一个时间点，范围向量展开多行

---

## DSL 模式

使用 `--dsl` 以 SQL 语法查询 Prometheus，支持单源和跨源联邦。

> **与 SQL 数据源的核心差异**：SQL 的 `FROM @label.table` 对应物理表，Prometheus 的 `FROM @label.metric` 对应**指标名**，且支持 `promql()` 直接嵌入 PromQL 表达式。详见下方对比。

### DSL 形式对比（vs MySQL）

| 操作 | MySQL SQL | Prometheus DSL |
|------|-----------|----------------|
| 基础查询 | `SELECT * FROM @db.iplist` | `SELECT * FROM @my-prom.up`（指标名作表名） |
| 带过滤 | `WHERE hostip='10.0.0.1'` | `WHERE job="prometheus"`（编译为标签匹配器） |
| 聚合 | `GROUP BY product` | `GROUP BY job`（编译为 PromQL 聚合） |
| 排序 | `ORDER BY value DESC` | `ORDER BY value DESC`（Go 层后处理） |
| **任意 PromQL** | ❌ 不支持 | ✅ `FROM @my-prom.promql(rate(cpu[5m]) / rate(mem[5m]))` |
| **多指标运算** | ❌ 需 JOIN | ✅ `FROM @my-prom.promql(a / b)`（PromQL 原生二元运算） |

### 设计理念：为什么 Prometheus DSL 与 SQL 数据源不同？

Prometheus DSL 的设计目标不是"用 SQL 完整表达 PromQL"，而是**在 SQL 语法和 PromQL 能力之间找到最佳平衡点**。

| 挑战 | 决策 | 理由 |
|------|------|------|
| PromQL 持续演进，DSL 编译有滞后 | `promql()` 透传 — 绕过编译直接执行 | 用户永远不受 DSL 编译能力天花板限制 |
| PromQL 标签列不确定（每 metric 不同） | 不支持固定 Schema 预采集，执行时动态映射 | 避免预采集 1 万个 metric 的 O(n) 开销 |
| PromQL 无排序/分页 | Go 层后处理（先查全量再排序截断） | 保持 PromQL 语义纯洁，不侵入 Prometheus API |
| Metric 名就是表，labels 就是列 | 对标 MySQL 的 `SHOW TABLES`/`DESCRIBE` 模型 | 复用已有 SQL 认知模型，降低学习成本 |
| 多指标关联在 SQL 中需 JOIN | `promql()` 二元运算原生支持 | 避免不必要的联邦物化开销 |

**核心结论**：简单过滤 + 单指标用普通 DSL 模式（享受编译便利），复杂运算 + 多指标用 `promql()`（享受 PromQL 全部能力）。两种模式互补，覆盖从简单查看到复杂分析的完整光谱。

### 单源 DSL — 普通模式（指标名作表名）

```bash
# DSL 模式：SELECT + WHERE 编译为 PromQL
dbexplain execute -env --dsl "SELECT * FROM @my-prom.up WHERE job='node'" --human

# 支持标签过滤（编译为 PromQL label matchers）
dbexplain execute -env --dsl "SELECT * FROM @my-prom.node_load1 WHERE instance='192.168.0.1:9100'" --human
```

DSL 模式会通过 IR 编译将 SQL AST 转为 PromQL。

### 单源 DSL — SELECT 列投影（v0.1.6+）

DSL 支持 `SELECT 列名` 而非仅 `SELECT *`，结果仅返回指定列：

```bash
# 选择特定列
dbexplain execute -env --dsl --human \
  'SELECT instance, mode, value FROM @my-prom.node_cpu_seconds_total WHERE mode="system" ORDER BY value DESC LIMIT 5'

# 列别名
dbexplain execute -env --dsl --human \
  'SELECT instance AS host, value AS val FROM @my-prom.up ORDER BY val DESC LIMIT 3'
```

### 单源 DSL — ORDER BY / LIMIT / OFFSET（v0.1.6+）

由于 PromQL 本身不支持 ORDER BY / LIMIT / OFFSET，DSL 模式在 PromQL 执行后通过 Go 层后处理实现：

```bash
# ORDER BY value 升序
dbexplain execute -env --dsl --human \
  'SELECT * FROM @my-prom.node_cpu_seconds_total WHERE mode="system" ORDER BY value'

# ORDER BY value DESC + LIMIT
dbexplain execute -env --dsl --human \
  'SELECT * FROM @my-prom.node_cpu_seconds_total WHERE mode="system" ORDER BY value DESC LIMIT 5'

# 多列排序 + OFFSET
dbexplain execute -env --dsl --human \
  'SELECT * FROM @my-prom.node_cpu_seconds_total ORDER BY product, value DESC LIMIT 10 OFFSET 5'
```

> **注意**：所有值以字符串形式返回。`value` 和 `timestamp` 列按数值排序，标签列按字典序排序。`NULL` 值排在最后（NULLS LAST）。

### 单源 DSL — GROUP BY + 聚合（v0.1.6+）

DSL 支持 GROUP BY + 聚合函数，编译为 PromQL 聚合表达式：

```bash
# COUNT 聚合
dbexplain execute -env --dsl --human \
  'SELECT job, count(value) FROM @my-prom.up GROUP BY job'

# AVG 聚合 + ORDER BY
dbexplain execute -env --dsl --human \
  'SELECT mode, avg(value) FROM @my-prom.node_cpu_seconds_total GROUP BY mode ORDER BY avg(value) DESC LIMIT 5'
```

**聚合函数映射**：

| SQL 函数 | PromQL | 说明 |
|----------|--------|------|
| `COUNT` | `count` | 计数 |
| `SUM` | `sum` | 求和 |
| `AVG` | `avg` | 平均值 |
| `MIN` | `min` | 最小值 |
| `MAX` | `max` | 最大值 |
| `GROUP` | `group` | 分组 |
| `STDDEV` | `stddev` | 标准差 |
| `STDVAR` | `stdvar` | 方差 |

> **限制**：GROUP BY 必须搭配聚合函数，不支持多聚合函数，不支持 SELECT * with GROUP BY。

### 单源 DSL — promql() 原始表达式透传（v0.1.6+）

这是 **Prometheus DSL 最独特的特性**，SQL 数据源无对应功能。`promql()` 允许在 `FROM` 子句中嵌入**任意 PromQL 表达式**，直接透传给 Prometheus API 执行，完全不经过 DSL 编译。

```bash
# 多指标二元运算：CPU 空闲率 = idle / total * 100，仅显示 > 98 的
dbexplain execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl 'SELECT instance, value FROM @my-prom.promql(rate(node_cpu_seconds_total{mode="idle"}[5m]) / rate(node_cpu_seconds_total[5m]) * 100) ORDER BY value DESC LIMIT 10' \
  --human

# topk 函数：CPU 使用率最高的 3 个
dbexplain execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl 'SELECT instance, value FROM @my-prom.promql(topk(3, rate(node_cpu_seconds_total[5m])))' \
  --human

# 范围向量查询（matrix 结果）
dbexplain execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl 'SELECT instance, mode, value FROM @my-prom.promql(node_cpu_seconds_total{mode="system"}[1m]) ORDER BY value DESC LIMIT 5' \
  --human

# bool 比较运算符
dbexplain execute -dsn 'prometheus://192.168.0.127:9440?label=my-prom' \
  --dsl 'SELECT instance, value FROM @my-prom.promql(rate(node_cpu_seconds_total{mode="idle"}[5m]) > bool 0.9) ORDER BY value DESC LIMIT 5' \
  --human
```

**与普通 DSL 模式的区别**：

| 维度 | 普通模式 (`@label.metric`) | promql() 模式 (`@label.promql(...)`) |
|------|---------------------------|--------------------------------------|
| 编译方式 | DSL SQL → PromQL 编译 | 原始 PromQL 直接透传 |
| WHERE | ✅ 编译为标签匹配器 `{k="v"}` | ❌ 需在表达式中内联 |
| GROUP BY | ✅ 编译为 PromQL `by()` | ❌ 需在表达式中内联 |
| ORDER BY/LIMIT/OFFSET | ✅ Go 层后处理 | ✅ Go 层后处理 |
| 多指标运算 | ❌ 单指标 | ✅ 任意 PromQL 二元运算 |
| PromQL 函数 | ❌ 仅支持聚合函数 | ✅ 全部支持（rate, topk, histogram_quantile 等）|
| 列投影 (SELECT 列名) | ✅ | ✅ |

> **最佳实践**：简单过滤 + 单指标用普通模式（享受 DSL 编译便利），复杂运算 / 多指标用 `promql()`（享受 PromQL 全部能力）。

### 跨源联邦（Prometheus + SQL）

```bash
# Prometheus 指标 + MySQL 表 JOIN（匹配 host 地址）
dbexplain execute -env --dsl "SELECT p.__name__, p.job, p.instance, p.value, i.product, i.subproduct
  FROM @my-prom.up p JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip" --human

# Prometheus + 文件 JOIN
dbexplain execute -env --dsl "SELECT p.*, c.region
  FROM @my-prom.up p JOIN @my-csv.nodes c ON p.hostip = c.ip" --human
```

### 跨源联邦 — promql() + SQL（v0.1.6+）

`promql()` 表达式可以参与联邦 JOIN，物化为内存表后与 MySQL 或其它数据源关联：

```bash
# promql() topk + MySQL JOIN：CPU 使用率 Top10 + 产品信息
dbexplain execute -env --dsl '
  SELECT p.instance, p.value, i.product, i.subproduct
  FROM @my-prom.promql(topk(10, rate(node_cpu_seconds_total[5m]))) p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  ORDER BY p.value DESC' --human

# promql() 多指标 + MySQL JOIN：CPU 系统占用 > 5% 的主机及其产品线
dbexplain execute -env --dsl '
  SELECT p.instance, p.value, i.product, i.subproduct
  FROM @my-prom.promql(rate(node_cpu_seconds_total{mode="system"}[5m]) / rate(node_cpu_seconds_total[5m]) * 100 > 5) p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  ORDER BY p.value DESC LIMIT 10' --human

# promql() + 普通 Prometheus 指标 + MySQL 三源联邦：CPU 系统占用率 + up 状态 + 产品线
dbexplain execute -env --dsl '
  SELECT p.instance, p.value AS cpu_system_pct, u.value AS up_status, i.product
  FROM @my-prom.promql(rate(node_cpu_seconds_total{mode="system"}[5m]) / rate(node_cpu_seconds_total[5m]) * 100) p
  JOIN @my-prom.up u ON p.instance = u.instance AND u.value = 1
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  ORDER BY p.value DESC LIMIT 10' --human
```

> **物化机制**：联邦查询将 Prometheus 指标全量物化为内存表，再通过 file query 引擎执行 JOIN/UNION。大指标注意内存用量。

---

## 能力矩阵

| 属性 | 值 |
|------|-----|
| 类别 | 时序数据库 (Time Series DB) |
| 协议 | `prometheus://` |
| Schema 采集 | ✅ (labels/metrics) |
| SQL 查询 | — (使用 PromQL) |
| REPL 交互 | ✅ |
| DSL 单源 | ✅ (SELECT */列名/WHERE/GROUP BY+聚合/ORDER BY/LIMIT/OFFSET/别名) |
| DSL 单源 promql() | ✅ (任意 PromQL 表达式透传) |
| DSL 联邦 | ✅ (单源 + 跨源 JOIN/UNION, 含 promql() 联邦) |
| 文件引擎 | — |
| Capabilities | `row_count`, `promql` |
| 构建标签 | `prometheus` (属于 `full` 标签) |

---

## 已知限制

- **范围查询参数**：暂不支持通过 DSN 参数配置 `start`/`end`/`step`，可在 PromQL 中直接指定（如 `up[5m]`）
- **联邦物化开销**：跨源联邦查询会全量物化 Prometheus 指标数据到内存，大指标注意内存用量。当 DSL 含 ORDER BY 时单源路径也会全量拉取再排序
- **无写入支持**：仅只读 PromQL 查询，不向 Prometheus 写入任何数据
- **采集数据量**：labels/metadata 全量采集。无 per-metric 循环，恒定 2 次 API 调用
- **promql() WHERE 限制**：`promql()` 模式不支持 DSL 的 WHERE 子句，过滤条件需在 PromQL 表达式中内联（如 `promql(up{job="prom"})`）
- **promql() 范围查询**：`promql()` 支持 PromQL 范围查询语法（如 `up[5m]`），返回 matrix 类型结果
