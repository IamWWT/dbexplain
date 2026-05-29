# Redis 键空间安全采集与风险诊断手册

本文档详细说明 `dbexplain` 工具中 Redis 连接器（`connector/redis.go`）的实现机制，帮助理解其如何在不影响线上服务的前提下，进行键空间分析、模式聚合、字段采样以及健康检查，并提供常见问题的排障方法。

---

## 一、代码中的重要机制（优化版）

### 1.1 流式扫描与实时聚合

```
func streamScanAndGroup(ctx context.Context, rdb *redis.Client) []familyAgg {
    iter := rdb.Scan(ctx, 0, "*", scanBatchSize).Iterator()
    aggregates := map[string]*familyAgg{}
    for iter.Next(ctx) {
        key := iter.Val()
        pat := normalize(key)
        // 边扫描边计数，不存储完整 key 列表
        ...
    }
}
```

- **零内存驻留**：扫描过程中仅保留键模式（pattern）及计数，不会把实际 key 全量加载到内存切片中。  
- **上限控制**：通过 `maxScanKeys`（默认 2000）限制扫描总数，避免在生产 Redis 上执行时间过长。  
- **实时进度**：每 100 个 key 输出一条日志，缓解等待焦虑。

### 1.2 模式归一化（normalize）

```
var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-...`)
var hexRe  = regexp.MustCompile(`[0-9a-fA-F]{8,}`)
var numRe  = regexp.MustCompile(`\d{2,}`)
```

- 将 UUID、十六进制串、连续数字替换为 `{uuid}`、`{hex}`、`{id}`，从而把 `user:1001:profile` 归类为 `user:{id}:profile`。  
- 这是识别 Redis 键空间业务域的核心，不依赖配置文件或事先声明。

### 1.3 Pipeline 批量查询（关键性能优化）

```
pipe := rdb.Pipeline()
typeCmd := pipe.Type(ctx, fam.Example)
ttlCmd  := pipe.TTL(ctx, fam.Example)
memCmd  := pipe.MemoryUsage(ctx, fam.Example, 0)
_, err := pipe.Exec(ctx)
```

- 一次网络往返获取 key 的类型、剩余生存时间、内存占用。  
- 避免逐个 key 发送命令导致的 N 次往返延迟，大幅提升分析速度。

### 1.4 安全采样（防止大 key 引爆内存）

- **String**：使用 `GETRANGE` 仅读取前 512 字节，而非 `GET` 整个值。  
- **Hash**：使用 `HSCAN` 迭代 5 个字段即停止，绝不调用 `HGETALL`。  
- **Stream**：使用 `XRANGE` 带 `COUNT` 限制（默认 10），不拉取全量消息。  
- **List/Set/ZSet**：仅获取长度/基数，不采样元素内容。  

这些措施确保即使遇到几 MB 的 hash 或 10MB 的 JSON 字符串，也不会使客户端崩溃或给 Redis 造成明显阻塞。

### 1.5 风险检测与自动诊断

在 `buildFamilyTable` 中，对每个键模式自动检查：

- **无 TTL 的安全敏感键**（session、token、auth 等） → 标注“no TTL on security‑sensitive key”。  
- **超大容器**：Hash 字段数 >1000、Sorted Set 成员数 >10000、List 长度 >10000 等。  
- **Stream 无消费者组**：可能积压未消费消息。  
- **超大字符串**：值长度 >1MB 时警告。  

所有风险会写入表注释，最终在终端报告中以 `⚠️` 形式呈现。

### 1.6 进度日志与错误处理

- 使用 `logf` 从上下文获取专用 logger，写入 `logs/<label>.log`。  
- 扫描阶段每 100 键输出“已扫描 XX 键，Y 个模式”。  
- 对每个键模式的分析也输出序号和名称，以便定位故障。  
- 所有错误均包装为 `schema.NewDBError`，包含脱敏 DSN 和操作名称。

---

## execute 只读查询

`dbexplain` 提供 `execute` 子命令，支持对 Redis 实例执行只读原生命令，安全地将结果输出到终端。

### 查询格式

空格分隔的原生 Redis 命令，例如：
- `GET mykey`
- `HGETALL myhash`
- `SCAN 0 MATCH user:* COUNT 100`
- `LRANGE mylist 0 10`
- `ZRANGE myzset 0 -1 WITHSCORES`

### 校验机制

- **内置只读命令白名单**：工具内部维护了 30+ 个 Redis 只读命令的白名单，包括 `GET`、`HGET`、`HGETALL`、`HMGET`、`HKEYS`、`HVALS`、`HLEN`、`HEXISTS`、`SCAN`、`LRANGE`、`LLEN`、`LINDEX`、`ZRANGE`、`ZCARD`、`ZSCORE`、`ZRANK`、`SMEMBERS`、`SCARD`、`SISMEMBER`、`TTL`、`PTTL`、`TYPE`、`EXISTS`、`STRLEN`、`PING`、`MGET`、`ZCOUNT`、`ZREVRANGE`、`ZREVRANK`、`SRANDMEMBER`、`XLEN`、`XRANGE`、`XREVRANGE`、`XREAD`、`KEYS` 等。
- **写命令拒绝**：任何写命令（`SET`、`DEL`、`HSET`、`LPUSH`、`SADD`、`ZADD` 等）将被拒绝，返回 `READ_ONLY_VIOLATION` 错误。
- **注意**：Redis 校验不经过 `sqlguard` 模块，使用内部独立的白名单校验。

### 自动 LIMIT 追加

不适用。原生 Redis 命令各有自己的数据量控制机制（如 `SCAN` 的 `COUNT` 参数、`LRANGE` 的起止索引），工具不会修改命令本身。

### 超时控制

通过 `go-redis` 客户端上下文超时（Go `context.WithTimeout`）控制整体执行时长。

### 执行方式

使用 `go-redis` 的 `Do()` 方法，将用户输入的命令作为第一个参数传入，后续参数由 `go-redis` 自动解析和处理。

### 输出格式

所有结果统一以单列 `result` 展示，值格式化为字符串。对于 Hash 或 List 等多值类型，结果以简洁的字符串形式呈现。

### 最大行数控制

由 `--limit` 命令行标志控制，默认值为 1000。截断在客户端侧完成，对超出的结果行直接丢弃。

---

## 二、常见问题与排障

### 2.1 连接超时或认证失败

**错误日志示例**：
```
skip redis://...: connect: dial tcp x.x.x.x:6379: i/o timeout
```

**可能原因与解决方案**：
- **网络不可达**：检查主机地址和端口是否正确，防火墙是否开放。  
- **密码错误**：密码中的特殊字符（`#`、`!`、`@`）在 URL 中需编码（`%23`、`%21`、`%40`）。在命令行传递时建议用单引号包裹整个 DSN。  
- **数据库索引错误**：确认 DSN 路径部分（如 `/0`）正确，Redis 的 DB 编号范围为 0~15。  
- **连接超时**：`DialTimeout` 默认 5 秒，可通过 `-timeout` 参数调整全局采集超时，但连接自身的超时需修改源码增加。  

### 2.2 扫描耗时较长但程序“未卡死”

- 工具内置上限 2000 个 key，正常扫描 1 万以内 key 的库通常在 1~3 秒内完成。  
- 如果 Redis 延迟较高或 key 数量极大，可降低 `maxScanKeys`（重新编译）或使用 `-timeout` 缩短整体采集时间。  
- 终端会实时输出进度，同时 `logs/<label>.log` 会记录详细扫描过程。

### 2.3 键模式识别不准确或过于宽泛

- 例如 `user:20260101:profile` 和 `user:20260102:profile` 被归并为 `user:{id}:profile`（正确），但 `user:{id}1` 和 `user:{id}2` 可能未被区分。  
- 此时可调整 `normalize` 函数中的正则表达式，增加更细粒度的模式匹配。  
- 对于极端复杂场景，可考虑 token 化路径并按规则映射，但当前实现已覆盖 90% 以上的生产案例。

### 2.4 Redis 版本兼容性

- 需要 Redis ≥ 5.0（支持 `MEMORY USAGE`、`XINFO` 等命令）。  
- 如果使用更老版本，某些命令会失败，但不影响扫描与类型识别，只是缺少内存统计和 Stream 检测。  
- 工具会记录命令失败日志，不会影响整体流程。

### 2.5 集群模式 (Cluster) 支持

v0.0.3 起已支持 Redis Cluster。使用方法：

- **启用集群模式**：在 DSN 中添加 `?cluster=true` 参数，如 `redis://:password@host:6379/0?label=mycluster&cluster=true`
- **工作原理**：自动使用 `NewClusterClient` 创建客户端，通过 `ForEachMaster` 在所有分片上执行 SCAN，聚合各分片的 keyspace 统计数据。
- **注意事项**：
  - 集群模式仅支持 db0，DSN 中指定的其他数据库编号会被忽略并输出警告。
  - 扫描上限（maxScanKeys）在所有分片间共享，避免单次扫描开销过大。
  - 单机模式完全向后兼容，不加 `?cluster=true` 即为原有单机行为。

---

## 三、核心命令速查

| 目的 | 命令示例 |
|------|---------|
| 测试连接与认证 | `redis-cli -h host -p port -a password PING` |
| 查看服务器信息 | `INFO server` |
| 查看 key 总数 | `DBSIZE` 或 `INFO keyspace` |
| 扫描 key（测试时慎用） | `SCAN 0 MATCH * COUNT 100` |
| 获取 key 类型 | `TYPE key` |
| 查看 hash 字段数 | `HLEN key` |
| 查看 stream 长度 | `XLEN key` |
| 查看列表长度 | `LLEN key` |
| 查看 key 的内存占用 | `MEMORY USAGE key` |
| 查看 TTL | `TTL key` |

---

## 四、经验总结

1. **Redis 分析不是“模拟 SQL 表”**，而是理解键空间和识别潜在风险。工具输出的“表卡片”本质上是键模式及其推断字段，而非真正的 schema。  
2. **扫描上限 2000 key 足以发现绝大多数模式**，且不会给生产 Redis 造成负载。如需更全面分析，可增加上限并配合 `-timeout` 使用。  
3. **安全采样是必须遵守的红线**：绝不 `GET` 大值、绝不 `HGETALL`、绝不 `LRANGE` 全量。当前实现已完全遵循该原则。  
4. **风险诊断是最高价值部分**：无 TTL 的 session 键、持续增长的 list、未消费的 stream 等，往往比单纯列出键模式更有运维价值。  
5. 生产环境中，建议使用只读权限的 Redis 用户（如 `-@all +@read`），并配合 `-timeout 15s` 控制采集时长。  
6. 如发现“卡住”但日志无输出，可能是 `SCAN` 迭代超时或网络延迟，此时可适当增大 `ReadTimeout` 或减少 `scanBatchSize` 重新编译。  
7. `MEMORY USAGE` 命令在部分云服务 Redis 中被禁用，届时会返回错误并跳过内存统计，不影响其他分析。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 Redis 实例进行键空间探查与健康评估。 