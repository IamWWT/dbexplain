# NoSQL 命令范围

> 以下 NoSQL 数据源通过 `dbexplain execute` 执行。LLM 已知的命令用法不再赘述，只标注支持的语法范围。

---

## 1. Redis

**DSN**: `redis://[:password@]host:port[/db][?cluster=true&label=x]`

**支持的语法范围**：

| 类型 | 命令 | 说明 |
|------|------|------|
| String | `GET`, `MGET`, `STRLEN`, `GETRANGE` | 含 Cluster 模式 |
| Hash | `HGET`, `HMGET`, `HGETALL`, `HKEYS`, `HVALS`, `HLEN`, `HEXISTS` | |
| List | `LLEN`, `LRANGE`, `LINDEX`, `LPOS` | |
| Set | `SMEMBERS`, `SCARD`, `SISMEMBER`, `SRANDMEMBER` | |
| ZSet | `ZRANGE`, `ZREVRANGE`, `ZRANK`, `ZCARD`, `ZCOUNT`, `ZSCORE`, `ZRANGEBYSCORE` | |
| Stream | `XLEN`, `XRANGE`, `XREVRANGE`, `XINFO` | |
| Generic | `EXISTS`, `TYPE`, `TTL`, `KEYS`, `RANDOMKEY`, `PING` | |

**不支持的命令**：所有写命令（`SET`, `HSET`, `LPUSH`, `DEL`, `EXPIRE` 等）在只读模式下被 `READ_ONLY_VIOLATION` 拒绝。

**集群模式**：DSN 加 `cluster=true`。命令不变，驱动自动处理 `MOVED` 重定向。

---

## 2. MongoDB

**DSN**: `mongodb://user:pass@host:27017/db?authSource=admin&label=x`

查询以 JSON 格式传入，结构同 `db.collection.find()`：

```json
{"find": "<collection>", "filter": {<query>}, "projection": {}, "sort": {}, "limit": 100, "skip": 0}
```

**支持的操作符**：`$gt`, `$gte`, `$lt`, `$lte`, `$eq`, `$ne`, `$in`, `$nin`, `$regex`, `$and`, `$or`, `$not`

**聚合管道**：`[{"$match":{}}, {"$group":{}}, {"$sort":{}}]` — 支持 `$match`, `$group`, `$sort`, `$project`, `$limit`, `$unwind`。

---

## 3. Elasticsearch

**DSN**: `elasticsearch://user:pass@host:9200?label=x` 或 `elasticsearchs://`（TLS）

查询以 JSON 格式传入，使用 ES Query DSL：

```json
{"query": {<query>}, "aggs": {<aggregations>}}
```

**限制**：不支持数组字段，只选标量字段。

---

## 4. Qdrant

**DSN**: `qdrant://:api-key@host:6334?label=x`

搜索以 JSON 格式传入：

```json
{"search": "<collection>", "vector": [0.1, ...], "limit": 10, "filter": {}, "with_payload": true}
```

**Filter 支持**：`must`, `must_not`, `should`；`match`, `range` 条件。
