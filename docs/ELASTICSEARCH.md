# Elasticsearch 索引映射采集与排障手册

本文档详细说明 `dbexplain` 工具中 Elasticsearch 连接器（`connector/elasticsearch.go`）的实现机制，帮助理解其如何在不读取任何文档数据的前提下，安全地获取索引列表、字段映射，并生成可读的结构卡片，同时提供常见问题的排障方法。

---

## 一、代码中的重要机制（优化版）

### 1.1 连接建立与健康检查

```
cfg := elasticsearch.Config{
    Addresses: []string{fmt.Sprintf("http://%s:%s", d.Host, d.Port)},
    Username:  d.User,
    Password:  d.Password,
    Transport: &http.Transport{MaxIdleConnsPerHost: 2},
}
client, err := elasticsearch.NewClient(cfg)

infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
res, err := client.Info(client.Info.WithContext(infoCtx))
```

- **超时控制**：所有与 ES 的交互均通过独立的 `context.WithTimeout` 派生子上下文，确保单次操作不会无限挂起。  
- **连接复用**：`MaxIdleConnsPerHost` 设为 2，避免在采集多个索引时耗尽文件描述符。  
- **健康检查**：使用 `client.Info` 获取集群信息，若返回错误或状态码异常，则终止采集并输出错误。  
- **TLS 支持**：通过 DSN scheme `elasticsearchs://` 或参数 `?tls=true` 启用 HTTPS。默认验证证书，诊断环境可追加 `?tls-skip-verify=true` 跳过验证。

### 1.2 索引列表获取与过滤

```
catCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
res, err = client.Cat.Indices(
    client.Cat.Indices.WithContext(catCtx),
    client.Cat.Indices.WithFormat("json"),
)
```

- **只读操作**：使用 `_cat/indices` API，仅获取索引元数据（名称、文档数、存储大小等），不读取任何实际数据。  
- **JSON 格式**：设置 `format=json` 便于直接解析为结构体。  
- **系统索引过滤**：索引名以 `.` 开头（如 `.kibana`、`.security`）会被自动跳过，避免生成无意义的“表”。  
- **进度输出**：过滤后的有效索引会输出 `[es] 采集索引 X/总数: 索引名`，让用户了解当前进度。

### 1.3 字段映射采集（零文档风险）

```
func getESMapping(ctx context.Context, client *elasticsearch.Client, indexName string) (map[string]map[string]interface{}, error) {
    res, err := client.Indices.GetMapping(
        client.Indices.GetMapping.WithContext(ctx),
        client.Indices.GetMapping.WithIndex(indexName),
    )
    // 解析 mappings.properties，提取每个字段的元数据
}
```

- **仅读取 Schema**：调用 `_mapping` 接口，获取索引的字段定义（名称、类型），绝不扫描任何文档。  
- **字段属性透传**：每个字段的类型直接取自 `props["type"]`，如 `text`、`keyword`、`date`、`nested` 等。  
- **容错处理**：若某个索引的映射获取失败，工具会记录日志并继续处理下一个索引，不会中断整个采集流程。  
- **避免大对象**：`_mapping` 返回数据通常很小（KB 级），不会造成内存压力。

### 1.4 输出模型映射

```
for field, props := range mapping {
    if field == "_all" || field == "_source" {
        continue
    }
    c := &schema.Column{
        Name:    field,
        Type:    fmt.Sprintf("%v", props["type"]),
        Comment: "es field",
    }
}
```

- **忽略内部字段**：跳过 `_all`、`_source` 等 ES 内部元数据字段，仅展示用户定义的字段。  
- **固定注释**：每个列的 Comment 均为 `"es field"`，因 ES 无原生列注释概念。后续可通过采样推断（但当前版本未实现，以保安全）。  
- **表结构统一**：所有索引归属同一个数据库（名称固定为 `"elasticsearch"`），引擎类型设为 `"elasticsearch"`。

### 1.5 错误处理与日志

- 所有可能返回 error 的操作均使用 `schema.NewDBError` 包装，提供脱敏 DSN 和具体操作上下文。  
- 通过 `logf` 将日志写入 `logs/<label>.log`，记录连接、索引列表、每个索引的映射请求状态。  
- 若集群不可用或某索引映射失败，不会 panic，仅记录并跳过。

---

## execute 只读查询

`dbexplain` 提供 `execute` 子命令，支持对 Elasticsearch 实例执行只读 SQL 查询（利用 ES `_sql` REST 端点，ES 6.3+ 支持），安全地将结果以表格形式输出到终端。

### 查询格式

标准 SQL 语句，支持 `SELECT`、`EXPLAIN`、`WITH`（CTE）等只读操作。SQL 查询会被转译为 ES 内部的 DSL 查询执行。

### 校验机制

- **SQLGuard 动词白名单**：所有查询经过 `sqlguard` 模块校验，按 SQL 语义进行只读动词白名单验证。
- **多语句检测**：禁止分号分隔的多条 SQL 语句。

### 自动 LIMIT 追加

- `SELECT`、`WITH`、`EXPLAIN` 语句未显式包含 `LIMIT` 时，自动追加 `LIMIT 1000`，防止大结果集。

### 超时控制

通过 HTTP 客户端超时控制整体执行时长。

### 执行方式

通过 HTTP POST 发送 JSON 请求体 `{"query": "...", "fetch_size": N}` 到 Elasticsearch 的 `/_sql` 端点。认证方式为 HTTP Basic Auth（若 DSN 中提供了凭据）。TLS 连接默认验证证书，诊断环境可追加 `?tls-skip-verify=true` 跳过验证。

### 最大行数控制

由 `--limit` 命令行标志控制，通过 `fetch_size` 参数传递给 ES `_sql` 端点。

---

## 二、常见问题与排障

### 2.1 连接超时或认证失败

**错误日志示例**：
```
skip es://...: es info: dial tcp x.x.x.x:9200: i/o timeout
```

**可能原因与解决方案**：
- **网络不可达**：确认主机和端口正确，防火墙开放 9200 端口。  
- **认证信息错误**：检查用户名、密码是否正确，ES 8+ 默认启用安全。  
- **HTTPS 要求**：许多 ES 集群启用 HTTPS 和自签名证书。使用 `elasticsearchs://` scheme 或 `?tls=true` 参数启用 HTTPS（证书验证默认跳过，适合内网诊断）。  
- **超时调整**：可适当增大 Info 的超时（代码中为 5 秒），或通过 `-timeout` 延长全局采集时间。

### 2.2 索引数量极大导致采集时间过长

- 工具会逐索引获取映射，若索引数上百，可能出现较长延迟。  
- 日志中会打印每个索引的采集进度，可通过 `tail -f logs/<label>.log` 观察。  
- 若需加速，可考虑仅在 DSN 中指定索引名称（通过 `index` 参数过滤，但当前代码未实现，需二次开发）。

### 2.3 索引映射为空或字段类型不准确

- 若索引的 `mappings` 为空（例如刚创建未定义字段），则 `getESMapping` 可能返回空 properties，导致表无列。工具仍会输出索引卡片，仅显示索引名和行数。  
- 动态映射生成的字段类型可能为 `text` 和 `keyword` 双字段，但在 `properties` 中只显示子字段的类型，工具会取顶层的 `type` 值（可能为 `text`），实际使用时需注意 `keyword` 子字段存在。  
- 若字段类型为 `nested` 或 `object`，`props["type"]` 可能为 `<nil>`，显示为空字符串，可进一步优化代码以展示嵌套结构（当前版本未递归解析，避免复杂）。

### 2.4 版本兼容性

- 使用的 Go 客户端 `go-elasticsearch/v8` 兼容 ES 8.x 及部分 7.x 版本。  
- 如果连接 ES 7.x，`_cat/indices` 和 `_mapping` 格式基本兼容，可正常工作。  
- 若为更旧版本（如 6.x），字段映射结构略有差异，但大概率仍可解析。

### 2.5 集群模式下连接

- 工具连接单个端点（`host:port`），由 ES 客户端自动发现其他节点并负载均衡，无需特殊配置。  
- 若集群前端有负载均衡器，直接填写均衡器地址即可。  
- 如果用户只想分析某个特定索引，可在 DSN 中增加 `?index=xxx` 参数（需扩展代码），当前版本会收集所有非系统索引。

---

## 三、核心命令速查（用 curl 验证）

| 目的 | curl 命令示例 |
|------|---------------|
| 检查集群健康 | `curl -u user:pass 'http://host:9200/_cluster/health?pretty'` |
| 获取所有索引 | `curl -u user:pass 'http://host:9200/_cat/indices?v'` |
| 获取特定索引映射 | `curl -u user:pass 'http://host:9200/my-index/_mapping?pretty'` |
| 查看索引设置 | `curl -u user:pass 'http://host:9200/my-index/_settings?pretty'` |
| 测试认证 | `curl -u user:pass 'http://host:9200/'` |

---

## 四、经验总结

1. **ES 分析的目标是获得索引 Schema**，而非数据内容。工具严格限定于 `_cat/indices` 和 `_mapping`，符合只读安全要求。  
2. **系统索引过滤**很重要，避免在报告中出现大量内部索引，使结果聚焦业务数据。  
3. **字段类型展示直观**，但注意 `text` 与 `keyword` 的区别在实际使用中很重要，工具未来可考虑展示多字段（fields）。  
4. **生产环境认证**务必开启，并使用最小权限用户（仅需 `monitor` 或 `view_index_metadata` 角色），避免误操作。  
5. 若索引数量过多，可通过 `-timeout` 延长全局超时，或分批次采集（每次连接不同索引模式）。  
6. 日志文件记录详尽，遇到问题时第一时间查看 `logs/<label>.log` 可快速定位故障点。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 Elasticsearch 集群进行索引结构探查。 