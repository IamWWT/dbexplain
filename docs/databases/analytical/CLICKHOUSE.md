# ClickHouse 结构采集与排障手册

本文档详细说明 `dbexplain` 工具中 ClickHouse 连接器（`connector/clickhouse.go`）的实现机制，帮助理解其如何通过 HTTP 接口安全地获取数据库列表、表结构、列属性（含排序键、分区键），并对无注释字段进行语义推断，同时提供常见问题的排障方法。

---

## 一、代码中的重要机制

### 1.1 HTTP 连接与健康检查

```
cli := &chHTTP{
    base:    fmt.Sprintf("http://%s:%s", host, port),
    user:    d.User,
    pass:    d.Password,
    httpCli: &http.Client{Timeout: 10 * time.Second},
}
if err := cli.ping(ctx); err != nil { ... }
```

- **纯 HTTP 接口**：ClickHouse 原生支持 HTTP 查询（默认 8123 端口），无需安装驱动或 CGO。  
- **认证**：通过请求头 `X-ClickHouse-User` 和 `X-ClickHouse-Key` 传递凭据，避免 HTTP 日志/Referer 头中泄露密码。  
- **超时设置**：HTTP 客户端整体超时 10 秒，每次查询均受此限制，避免长时间阻塞。  
- **安全 ping**：执行 `SELECT 1` 验证连通性，失败则终止采集并返回错误。

### 1.2 数据库列表获取

```
if d.DBName != "" && d.DBName != "default" {
    dbNames = []string{d.DBName}
} else {
    rows, err := cli.queryRows(ctx, 
        "SELECT name FROM system.databases WHERE name NOT IN ('system','information_schema','INFORMATION_SCHEMA') ORDER BY name")
}
```

- **指定库优先**：如果 DSN 中提供了非 `default` 的数据库名，则只采集该库。  
- **系统库过滤**：排除 `system`、`information_schema` 等内部库，仅保留用户库。  
- **特殊处理**：若 DSN 中库名为 `default` 或为空，则采集所有非系统库，因为 ClickHouse 的 `default` 库经常被使用。

### 1.3 表信息采集

```
rows, err := cli.queryRows(ctx, fmt.Sprintf(`
    SELECT name, engine, toUInt64(total_rows), toUInt64(total_bytes), comment
    FROM system.tables WHERE database='%s' AND engine NOT LIKE '%%View%%'
    ORDER BY name`, escCH(dbName)))
```

- **仅查询物理表**：通过 `engine NOT LIKE '%%View%%'` 过滤掉各类视图（MergeTree、ReplacingMergeTree 等属于物理表）。  
- **表大小与行数**：从 `system.tables` 读取 `total_rows` 和 `total_bytes`，直接作为 `RowCount` 和 `SizeBytes` 使用，数据准确。  
- **表引擎信息**：记录引擎名称（如 `MergeTree`、`ReplacingMergeTree`），呈现在卡片标题中。  
- **表注释**：取自 `comment` 字段，若为空则后续可能用采样行推断。

### 1.4 列信息与特殊键采集

```
rows, err := cli.queryRows(ctx, fmt.Sprintf(`
    SELECT name, type, default_kind, default_expression, comment,
           is_in_primary_key, is_in_sorting_key, is_in_partition_key
    FROM system.columns WHERE database='%s' AND table='%s'
    ORDER BY position`, escCH(dbName), escCH(t.Name)))
```

- **完整列属性**：包括列名、类型、默认值（种类+表达式）、注释以及是否属于主键、排序键、分区键。  
- **字段标志映射**：
  - `IsPrimary`：来自 `is_in_primary_key`
  - `IsSortKey`：来自 `is_in_sorting_key`
  - `IsPartitionKey`：来自 `is_in_partition_key`
- **无注释推断**：同其他 SQL 连接器，若列注释为空，工具会尝试获取一行样本数据，调用 `schema.InferComment` 生成语义注释。

**表额外元数据**：
```
meta, err := cli.queryRows(ctx, fmt.Sprintf(`
    SELECT partition_key, sorting_key, primary_key
    FROM system.tables WHERE database='%s' AND name='%s'`, escCH(dbName), escCH(t.Name)))
```
- 读取分区键表达式、排序键表达式，显示在卡片中（`PARTITION BY`、`ORDER BY`）。

### 1.5 首行数据采样与注释推断（注意已知局限）

```
sample, err := fetchCHSampleRow(ctx, cli, dbName, t.Name)
if err == nil {
    for _, c := range colsWithoutComment {
        if val, ok := sample[c.Name]; ok {
            c.Comment = schema.InferComment(c.Name, c.Type, val)
        }
    }
}
```

- **采样查询**：`SELECT * FROM <db>.<table> LIMIT 1`，由 `queryRows` 统一追加 `FORMAT JSONCompact`。
- **v0.0.3 已修复**：此前 `fetchCHSampleRow` 和 `queryRows` 各自追加了一次 `FORMAT JSONCompact`，导致实际 SQL 为 `SELECT ... LIMIT 1 FORMAT JSONCompact FORMAT JSONCompact` 的语法错误。修复方式：移除 `fetchCHSampleRow` 中多余的 FORMAT 追加，仅由 `queryRows` 统一追加。
- **安全性**：采样失败仅影响注释推断，不会导致采集中断。

### 1.6 错误处理与进度日志

- 所有 `queryRows` 的错误均被捕获，部分错误（如数据库不存在或表不存在）直接返回错误导致整个数据库采集跳过，但不会影响其他数据库。  
- 进度输出通过 `logf` 写入 `logs/<label>.log`，包括数据库名、表采集序号（`采集表 X/总数`）、采样失败警告。  
- 错误被包装为 `schema.NewDBError`，包含脱敏 DSN、数据库名、操作名称。

### 1.7 SQL 转义与注入防护

- 使用 `escCH(s string)` 将单引号替换为 `\'`，防止在拼接 SQL 时破坏字符串。  
- 尽管不是参数化查询，但对 ClickHouse HTTP 接口而言，拼接是常见且可控的做法，只要对输入严格转义即可。

---

## execute 只读查询

`dbexplain` 提供 `execute` 子命令，支持对 ClickHouse 实例执行只读 SQL 查询，安全地将结果以表格形式输出到终端。

### 查询格式

标准 SQL 语句，支持 `SELECT`、`EXPLAIN`、`WITH`（CTE）等只读操作。

### 校验机制

- **SQLGuard 动词白名单**：所有查询经过 `sqlguard` 模块校验，仅允许 `SELECT`、`EXPLAIN`、`WITH`、`SHOW`、`DESCRIBE`、`DESC`、`PRAGMA`、`CHECK` 八类只读动词通过。任何写操作语句将被拒绝。
- **多语句检测**：禁止分号分隔的多条 SQL 语句，防止注入绕过。

### 自动 LIMIT 追加

- `SELECT`、`WITH`、`EXPLAIN` 语句未显式包含 `LIMIT` 时，自动追加 `LIMIT 1000`，防止大结果集。

### 超时控制

- **数据库层**：在查询末尾追加 `SETTINGS max_execution_time=N`（N 为秒数），由 ClickHouse 内核限制单次查询最大执行时间。
- **应用层**：HTTP 客户端整体超时兜底。

### 执行方式

通过 HTTP POST 发送查询到 ClickHouse `/` 端点，请求体标记 `FORMAT JSON` 以获取结构化 JSON 响应。响应解析为标准的 `meta`（列元数据）、`data`（行数据）、`rows`（行数统计）结构。

### 最大行数控制

由 `--limit` 命令行标志控制，默认值为 1000。截断在客户端侧完成，对超出部分直接丢弃。

---

## 二、常见问题与排障

### 2.1 连接超时或 HTTP 错误

**错误示例**：
```
clickhouse connect: clickhouse HTTP 401: Authentication failed
```
**可能原因与解决方案**：
- **认证信息错误**：检查用户名和密码是否正确。ClickHouse 默认用户为 `default`，密码可为空或通过 `<users>` 配置。  
- **端口错误**：默认 HTTP 端口为 8123，确认服务是否监听该端口。  
- **防火墙/网络策略**：确保客户端 IP 被允许访问 8123 端口。  
- **HTTP 超时**：可在源码中调整 `http.Client{Timeout: ...}` 或通过 `-timeout` 延长整体采集时间。

### 2.2 表或数据库采集为空

- **权限不足**：用户需具有 `SHOW DATABASES` 和 `SELECT ON system.tables`、`system.columns` 的权限。  
- **数据库名称大小写**：ClickHouse 库名大小写敏感，确保 DSN 中的库名与系统一致。  
- **`default` 库的特殊处理**：若 DSN 中指定 `default`，只采集该库；若未指定库名，则采集所有非系统库。

### 2.3 采样行失败导致注释为空

- **v0.0.3 已修复**：此前因 `fetchCHSampleRow` 和 `queryRows` 重复追加 `FORMAT JSONCompact` 导致语法错误。已移除重复的格式化子句。
- 若采样仍失败，可能是权限不足（需 `SELECT` 权限）或表为空（无数据行）。
- 此问题不影响列类型、主键、排序键等关键信息的采集。

### 2.4 ClickHouse 版本兼容性

- 使用 `system.tables` 和 `system.columns` 的标准字段，适用于 ClickHouse 20.x 及以上。  
- 极旧版本（18.x 之前）可能缺少 `is_in_sorting_key` 等字段，会导致查询失败。建议使用 ClickHouse 21.8+。

### 2.5 分布式表 (Distributed) 的展示

- 物理表（如本地表）会被采集，分布式表引擎（`Distributed`）因其 `engine NOT LIKE '%%View%%'` 仍会被列出（因为它是表不是视图），其 `total_rows` 可能为 0 或聚合值。用户需注意区分。

---

## 三、核心命令速查（clickhouse-client 验证）

| 目的 | 命令 |
|------|------|
| 连接测试 | `clickhouse-client --host host --port 9000 --user user --password pass --query "SELECT 1"` |
| HTTP 查询 | `curl -u user:pass 'http://host:8123/?query=SELECT+1'` |
| 列出数据库 | `clickhouse-client --query "SELECT name FROM system.databases"` |
| 查看所有表 | `clickhouse-client --query "SELECT database, name, engine FROM system.tables WHERE engine NOT LIKE '%View%'"` |
| 查看表结构 | `clickhouse-client --query "DESCRIBE TABLE database.table"` |
| 查看表大小 | `clickhouse-client --query "SELECT name, total_rows, total_bytes FROM system.tables WHERE name='table'"` |
| 获取排序键/分区键 | `clickhouse-client --query "SELECT sorting_key, partition_key FROM system.tables WHERE database='db' AND name='table'"` |

---

## 四、经验总结

1. **HTTP 接口友好**：无需原生 TCP 驱动，`curl` 即可测试，集成轻量。  
2. **权限最小化**：建议创建专用只读用户，授予 `SELECT ON system.*` 和对应业务库的元数据读取权限，避免数据泄露。  
3. **采样注释**：工具从 ClickHouse 获取列注释时，系统表中预填的 `COMMENT` 注释会优先展示。采样推断作为补充，可为无注释列提供语义提示。
4. **大库采集策略**：ClickHouse 表数量和列数量可能极大，采集时间与表数成正比。可通过 `-timeout` 调大超时，或分 DSN 指定不同库分批采集。  
5. **MergeTree 家族展示**：工具将 `partition_key`、`sorting_key` 作为卡片信息输出，是理解 ClickHouse 数据组织的重要参考。  
6. **日志详尽**：所有查询失败记录在 `logs/<label>.log`，便于快速定位权限或语法问题。  
7. **已知改进点**：修正采样查询的 `FORMAT` 位置可完全激活字段语义推断，提升无注释表的可读性。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 ClickHouse 实例进行结构探查。 