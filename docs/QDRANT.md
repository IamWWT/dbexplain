# Qdrant 结构采集与排障手册

本文档详细说明 `dbexplain` 工具中 Qdrant 连接器（`connector/qdrant.go`）的实现机制，帮助理解其如何通过 gRPC 客户端安全地获取向量数据库的集合列表、向量配置信息和点数统计，同时提供常见问题的排障方法。Qdrant 作为向量数据库与传统关系型数据库在数据模型上有本质差异，工具适配了这一特殊性。

---

## 一、代码中的重要机制

### 1.1 连接建立与安全 Ping

```
client, err := qdrant.NewClient(d.Host, d.Port, d.User, d.Password, d.UseTLS)

pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
health, err := client.HealthCheck(pingCtx)
if err != nil { ... }
```

- **客户端选择**：使用官方 `github.com/qdrant/go-client`（go-qdrant），基于 gRPC 协议与 Qdrant 服务端通信。默认端口为 6334（gRPC 端口），HTTP REST API 端口为 6333，工具仅使用 gRPC 协议。
- **DSN 格式**：`qdrant://:api-key@host:6334?label=xxx`。用户名部分留空（`:` 前为空），API Key 作为密码字段。若使用未开启认证的 Qdrant 实例，则为 `qdrant://@host:6334?label=xxx`。
- **认证**：API Key 通过 gRPC 元数据（`api-key` header）传递，在每次请求时由 go-qdrant 客户端自动附加，代码无需显式处理 Token 刷新。
- **TLS 支持**：通过 DSN 查询参数 `tls=true` 启用 TLS。默认不使用 TLS 适用于内网环境。
- **超时控制**：Ping 操作使用独立的 5 秒超时，通过 HealthCheck API 验证服务端可达。
- **错误包装**：所有 error 均通过 `schema.NewDBError` 返回，记录脱敏 DSN 和操作上下文。

### 1.2 集合列表获取与元数据采集

```
collections, err := client.ListCollections(ctx)

for _, col := range collections {
    info, err := client.GetCollectionInfo(ctx, col.Name)
    ...
}
```

- **ListCollections**：获取所有集合的名称列表。此操作对应 gRPC `ListCollections` RPC，仅返回集合名和 ID，不传输数据。
- **GetCollectionInfo**：对每个集合获取详细元数据，包括：
  - 向量配置：维度（`size`）、距离算法（`distance`：Cosine/Euclid/Dot/Manhattan）、数据类型
  - 索引配置：HNSW 参数（`m`、`ef_construct`）、量化设置
  - 分片信息：分片数、分片分布方式
- **点数统计**：`GetCollectionInfo` 返回 `points_count` 字段，表示集合中已索引的向量总数。这是精确计数，非估算值。
- **无传统"表"概念**：Qdrant 的数据模型是 Collection（集合）而非 Table（表）。每个 Collection 包含向量数据（vectors）和可选的 payload（结构化元数据）。工具将每个 Collection 映射为一个逻辑"表"，其向量维度、距离算法等信息作为特殊字段输出。

### 1.3 向量配置信息采集

- **向量维度**：从 `GetCollectionInfo` 的 `VectorParams.Size` 获取，表示向量维度（如 768、1536）。
- **距离算法**：从 `VectorParams.Distance` 获取，可能值：`Cosine`、`Euclid`、`Dot`、`Manhattan`。
- **多向量支持**：Qdrant 支持命名向量（named vectors），即一个 Collection 可有多个向量字段。工具将每个命名向量的配置作为独立属性输出。
- **量化配置**：若启用了标量量化（Scalar Quantization）或乘积量化（Product Quantization），工具记录量化类型和参数。

### 1.4 安全设计：严格只读

```
// Qdrant 采集模式仅使用以下两个 API：
client.ListCollections(ctx)       // 列出所有集合
client.GetCollectionInfo(ctx, n)  // 获取集合元数据

// 以下 API 从不调用：
// client.Scroll() / client.Search() / client.Upsert() / client.Delete()
// client.CreateCollection() / client.UpdateCollection()
```

- **API 白名单**：采集模式下仅调用 `ListCollections` 和 `GetCollectionInfo` 两个元数据 API，完全不接触 Collection 内的向量数据或 payload 数据。
- **无数据泄露风险**：`GetCollectionInfo` 返回的是配置级信息（维度、距离算法、索引参数、点数），不包含任何用户的向量数据或 payload 内容。
- **无需特殊权限**：`ListCollections` 和 `GetCollectionInfo` 是任何认证用户都可访问的基础 API，无需管理员权限。

### 1.5 Schema 映射

由于 Qdrant 与传统关系型数据库模型不同，工具的 Schema 映射策略如下：

| Qdrant 概念 | Schema 映射 | 说明 |
|------------|------------|------|
| Collection | Table | 集合名映射为表名 |
| Points Count | Table.RowCount | 精确的点数，非估算 |
| Vector Dimension | Column（特殊） | 作为虚拟列输出，类型为 `VECTOR(dim)` |
| Distance Metric | Table 属性 | Cosine/Euclid/Dot 等信息 |
| Shards Count | Table 属性 | 分片配置信息 |
| Index Config | Table 属性 | HNSW 参数信息 |

### 1.6 错误处理与进度日志

- 所有操作均通过 `schema.NewDBError` 包装，记录 DSN（脱敏）和操作类型（`list collections`、`get collection info`）。
- 使用 `logf` 输出进度，包括集合采集数量和采集结果。
- 单个集合的 `GetCollectionInfo` 失败不影响其他集合的采集。
- API Key 在日志中自动脱敏（由 `Redacted()` 函数处理）。

---

## 二、常见问题与排障

### 2.1 连接被拒绝或认证失败

**错误示例**：
```
skip qdrant://...: qdrant ping: rpc error: code = Unavailable desc = connection error: desc = "transport: error while dialing: dial tcp x.x.x.x:6334: connect: connection refused"
```

**可能原因与解决方案**：
- **端口混淆**：确认使用 gRPC 端口（默认 6334）而非 HTTP REST 端口（默认 6333）。这两个端口是独立的，工具仅通过 gRPC 连接。
- **服务未运行**：确认 Qdrant 服务已启动：`docker ps | grep qdrant` 或 `systemctl status qdrant`。
- **API Key 错误**：确认 DSN 中的 API Key 正确。格式为 `qdrant://:api-key@host:6334?label=xxx`（密码字段为 API Key）。
- **TLS 不匹配**：如果服务端启用了 TLS 但 DSN 中未指定 `tls=true`，gRPC 客户端会使用明文连接并失败。反之，如果服务端未启用 TLS 但客户端指定了 TLS，同样会失败。

### 2.2 集合列表为空

- Qdrant 新安装时默认无集合。只有当用户通过 API 或 SDK 创建了 Collection 后才会出现在列表中。
- 确认使用的是正确的 Qdrant 实例（检查 host 和 port）。
- `ListCollections` API 不需要特殊权限，任何认证用户均可调用。

### 2.3 点数统计为 0

- 如果 `GetCollectionInfo` 返回 `points_count = 0`，表示 Collection 中确实没有数据（或所有数据已被删除）。
- Qdrant 的点数统计是实时的、精确的，不是估算值。0 就是 0。

### 2.4 向量维度或配置信息缺失

- 如果 Collection 创建时使用了默认参数，部分配置字段可能为空。工具会如实输出，不进行猜测或填充。
- 对于使用命名向量（named vectors）的 Collection，工具会输出所有向量字段的配置。若 Collection 使用匿名向量（默认），则输出单一的向量配置。

### 2.5 性能与超时

- Qdrant 的 `ListCollections` 和 `GetCollectionInfo` 都是轻量级元数据操作，即使集合数量较多（>100），采集时间通常也很短（<10 秒）。
- 如果遇到超时，首先检查网络延迟，而非增加超时时间。Qdrant 的元数据 API 设计为毫秒级响应。
- 可通过 `-timeout 30s` 增加全局超时，但通常不需要。

---

## 三、核心命令速查（HTTP API 验证）

由于 Qdrant 同时提供 gRPC 和 HTTP REST API，以下使用 `curl` 命令通过 HTTP API 进行等价验证（REST 端口 6333）：

| 目的 | curl 命令 |
|------|----------|
| 测试连接（健康检查） | `curl -H "api-key: YOUR_API_KEY" http://host:6333/healthz` |
| 列出所有集合 | `curl -H "api-key: YOUR_API_KEY" http://host:6333/collections` |
| 获取集合详情 | `curl -H "api-key: YOUR_API_KEY" http://host:6333/collections/collection_name` |
| 查看集合点数 | `curl -H "api-key: YOUR_API_KEY" http://host:6333/collections/collection_name \| jq '.result.points_count'` |
| 查看向量配置 | `curl -H "api-key: YOUR_API_KEY" http://host:6333/collections/collection_name \| jq '.result.config.params.vectors'` |
| 查看索引配置 | `curl -H "api-key: YOUR_API_KEY" http://host:6333/collections/collection_name \| jq '.result.config.hnsw_config'` |

---

## 四、核心命令速查（execute 子命令）

工具支持对 Qdrant 执行只读查询（execute 子命令）：

### Qdrant execute 查询格式

- **格式**：JSON（非 SQL）。Qdrant 不使用 SQL 语法，查询以 JSON 对象形式传入。
- **只读操作白名单**：
  - `{"scroll": "collection_name"}` — 滚动获取集合中的点（支持可选参数 `limit`、`offset`、`filter`）
  - `{"count": "collection_name"}` — 获取集合的点数（精确计数）

**示例**：
```bash
# 获取集合的点数
dbexplain execute -env --label qdrant-test '{"count":"documents"}'

# 滚动获取集合中的前 20 个点
dbexplain execute -env --label qdrant-test '{"scroll":"documents","limit":20}'

# 带过滤条件的滚动
dbexplain execute -env --label qdrant-test \
  '{"scroll":"documents","limit":50,"filter":{"must":[{"key":"category","match":{"value":"news"}}]}}'
```

### execute 安全机制

- **内部白名单校验**：仅允许 `scroll` 和 `count` 操作，拒绝所有其他操作（如 `upsert`、`delete`、`create`、`update` 等）。
- **不走 SQL 校验器**：Qdrant 的 JSON 格式不经过 `sqlguard.Validate()`，而是由 `isSQLKind()` 路由判断为非 SQL 数据库，通过连接器内部的只读白名单进行验证。
- **数据泄露防护**：`scroll` 返回的数据仅包含 vector ID 和 payload（用户定义的结构化元数据），不返回原始向量值（除非显式配置）。若需返回向量数据，Qdrant 服务端需设置 `with_vector=true`，但工具默认不启用。
- **无 AutoLimit 保护**：由于不是 SQL，不会执行自动 LIMIT 注入。`scroll` 操作的 `limit` 参数由用户显式指定，无默认上限。
- **超时保护**：应用层通过 `context.WithTimeout` 控制整体查询超时。Qdrant 没有数据库端超时设置（因为 Qdrant 没有类似 `statement_timeout` 的机制）。
- **并发控制**：通过 `query.QueryLock` 的 `TryLock` 机制确保同一 label 的 Qdrant 实例同时只有一个查询在执行。

---

## 五、经验总结

1. **数据模型差异**：Qdrant 是向量数据库，数据以 Collection 为单位组织，而非传统的关系表。每个 Collection 的核心是向量维度 + 距离算法，而非列约束和范式。理解这一差异是正确使用工具的前提。
2. **API 安全边界**：采集模式仅使用 `ListCollections` 和 `GetCollectionInfo`，不触及任何向量数据或业务 payload。execute 模式的白名单严格限定为 `scroll` 和 `count`，杜绝任何写入或修改操作。
3. **权限最小化**：Qdrant 的 API Key 认证模式下，采集和 execute 操作均不需要管理员权限。任何有读取权限的 API Key 均可使用完整功能。
4. **精确的点数**：与其他数据库（如 MySQL InnoDB 的 `TABLE_ROWS` 估算）不同，Qdrant 提供的 `points_count` 是精确值，AI Agent 可完全信赖此数字进行容量规划。
5. **无列级注释推断**：Qdrant 没有列概念，因此无列注释推断。向量维度、距离算法等信息直接来源于系统元数据，无需推断。
6. **gRPC 协议注意事项**：gRPC 使用 HTTP/2，若企业环境中存在 HTTP/2 不兼容的代理或负载均衡器，可能导致连接失败。此时需确保代理支持 HTTP/2 透传，或使用直连方式（跳过代理）。
7. **execute 的 scroll 限制**：`scroll` 操作不保证结果顺序，Qdrant 使用随机访问模式。若需要有序结果，应在应用层对返回数据进行排序。
8. **版本兼容**：测试于 Qdrant v1.3+ 版本，`ListCollections` 和 `GetCollectionInfo` API 在 1.x 系列中保持稳定。go-qdrant 客户端版本需与服务端版本匹配（建议使用最新版本）。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 Qdrant 向量数据库实例进行结构探查与数据查询。
