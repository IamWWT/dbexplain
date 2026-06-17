# L6: NoSQL 数据库查询执行

> 验证 Redis、MongoDB、Qdrant 的只读查询执行。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

## 4.1 Redis — PING

```bash
$BIN execute --db 7 "PING" --human
```

## 4.2 Redis — SCAN 遍历

```bash
$BIN execute --db 7 "SCAN 0 MATCH CONVERSATION:* COUNT 10" --human
```

## 4.3 Redis — 查询 key 类型

```bash
$BIN execute --db 7 "TYPE user:1001" --human
```

## 4.4 Redis — 写命令拒绝

```bash
$BIN execute --db 7 "SET foo bar"
# 预期: READ_ONLY_VIOLATION: redis command "SET" is not allowed
```

## 4.5 MongoDB — count

```bash
$BIN execute --db 9 '{"count":"system.users"}' --human
```

## 4.6 MongoDB — find

```bash
$BIN execute --db 9 '{"find":"user","filter":{},"limit":5}' --human
```

## 4.7 MongoDB — 带过滤 find

```bash
$BIN execute --db 9 '{"find":"user","filter":{"app_manger_level":{"$gte":3}},"limit":5}' --human
```

## 4.8 MongoDB — 写操作拒绝

```bash
$BIN execute --db 9 '{"insert":"user","documents":[{"x":1}]}'
# 预期: READ_ONLY_VIOLATION: mongo query must specify "find" or "aggregate"
```

## 4.9 Qdrant — count

```bash
$BIN execute --db 4 '{"count":"runbooks"}' --human
```

## 4.10 Qdrant — scroll

```bash
$BIN execute --db 4 '{"scroll":"runbooks","limit":20}' --human
```

## 4.11 Qdrant — 非法操作拒绝

```bash
$BIN execute --db 4 '{"upsert":"runbooks","points":[]}'
# 预期: READ_ONLY_VIOLATION: qdrant query must specify "scroll" or "count"
```

## 4.12 MongoDB — 聚合管道写阶段拒绝 (v0.1.2+)

```bash
# $out 写阶段拒绝
$BIN execute --db 9 '{"aggregate":"user","pipeline":[{"$out":"malicious_output"}]}'
# 预期: READ_ONLY_VIOLATION: write stage "$out" is not allowed in aggregation pipeline

# $merge 写阶段拒绝
$BIN execute --db 9 '{"aggregate":"user","pipeline":[{"$match":{"status":"active"}},{"$merge":{"into":"backup"}}]}'
# 预期: READ_ONLY_VIOLATION: write stage "$merge" is not allowed in aggregation pipeline

# 正常聚合管道放行
$BIN execute --db 9 '{"aggregate":"user","pipeline":[{"$match":{"app_manger_level":{"$gte":3}}},{"$limit":2}]}' --human
# 预期: 正常返回查询结果
```
