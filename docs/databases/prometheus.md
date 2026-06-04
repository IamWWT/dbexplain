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

`dbexplain -dsn 'prometheus://...' --json` 将从 Prometheus HTTP API v2.x 采集三类信息：

### 1. Targets（采集目标）
- **API**: `GET /api/v1/targets`
- 按 `scrapePool` / `job` 分组，每组为一个表
- 表名 = job 名称（如 `node_exporter`、`blackbox_exporter-http`）
- 列: `instance`、`health`、`last_scrape_duration_ms`、`last_error`
- 每个表的 RowCount = 该 job 下的 target 数
- 超过 50 个 job 时自动截断（日志输出警告）

### 2. Labels（标签名）
- **API**: `GET /api/v1/labels`
- 表名 `_labels`，列 `name`
- RowCount = label 总数

### 3. Metric 元数据
- **API**: `GET /api/v1/metadata`
- 表名 `_metrics`，列 `metric`、`type`、`help`、`unit`
- RowCount = metric 总数

### 示例输出

```bash
$ dbexplain -dsn 'prometheus://192.168.0.127:9440?label=my-prom' --human
> DSN mapping:
  DB16 → my-prom              prometheus://***@192.168.0.127:9440

kind     label    db           table                          columns              rows
───────  ───────  ───────────  ─────────────────────────────  ───────────────────  ──────
promethe my-prom  prometheus   node_exporter                  instance, health...  5
promethe my-prom  prometheus   blackbox_exporter-http         instance, health...  8
promethe my-prom  prometheus   _labels                        name                 206
promethe my-prom  prometheus   _metrics                       metric, type...      657
```

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

## 能力矩阵

| 属性 | 值 |
|------|-----|
| 类别 | 时序数据库 (Time Series DB) |
| 协议 | `prometheus://` |
| Schema 采集 | ✅ (targets/labels/metadata) |
| SQL 查询 | — (使用 PromQL) |
| REPL 交互 | ✅ |
| DSL 联邦 | — (暂不支持) |
| 文件引擎 | — |
| Capabilities | `row_count`, `promql` |
| 构建标签 | `prometheus` (属于 `full` 标签) |

---

## 已知限制

- **范围查询参数**：暂不支持通过 DSN 参数配置 `start`/`end`/`step`，可在 PromQL 中直接指定（如 `up[5m]`）
- **DSL 联邦**：Prometheus 不走 SQL 接口，暂不支持 DSL `@label` 引用
- **无写入支持**：仅只读 PromQL 查询，不向 Prometheus 写入任何数据
- **采集数据量**：targets 超过 50 个时自动截断；labels/metadata 全量采集
