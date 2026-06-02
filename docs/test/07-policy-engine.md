# L5: 安全策略引擎验证

> 验证 policy 包：表级/列级/语句级拒绝策略 + 非 SQL 数据库策略 + MASK_COLUMNS。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../docs/CONFIG_SEARCH.md)。

## 7.1 SQL 语句级拒绝

```bash
# sqlguard 先拦截（不需要策略）
$BIN execute -env --db 1 'DROP TABLE users'
# → READ_ONLY_VIOLATION

# 策略层额外拦截
DENY_STATEMENTS="DROP TABLE" $BIN execute -env --db 1 'SELECT 1'
# → 正常返回（不匹配策略）

DENY_STATEMENTS="FLUSHALL" $BIN execute -env --db 7 'FLUSHALL'
# → ACCESS_DENIED: query matches denied statement pattern
```

## 7.2 SQL 表级拒绝

```bash
DENY_TABLES=iplist $BIN execute -env --db 1 'SELECT * FROM testdb.iplist'
# → ACCESS_DENIED: table "iplist" is not allowed for query
```

## 7.3 SQL 列级拒绝

```bash
DENY_COLUMNS=testdb.iplist.hostip $BIN execute -env --db 1 'SELECT testdb.iplist.hostip FROM testdb.iplist'
# → ACCESS_DENIED: column "testdb.iplist.hostip" is not allowed for query
```

## 7.4 MongoDB 集合级拒绝

```bash
DENY_TABLES=user $BIN execute -env --db 9 '{"find":"user","filter":{},"limit":1}'
# → ACCESS_DENIED: table "user" is not allowed for query
```

## 7.5 Redis Key 级拒绝

```bash
DENY_TABLES="CONVERSATION:*" $BIN execute -env --db 7 'GET CONVERSATION:abc123'
# → ACCESS_DENIED: table "CONVERSATION:*" is not allowed for query
```

## 7.6 Qdrant 集合级拒绝

```bash
DENY_TABLES=runbooks $BIN execute -env --db 4 '{"count":"runbooks"}'
# → ACCESS_DENIED: table "runbooks" is not allowed for query
```

## 7.7 正常查询放行

```bash
# 无策略时正常
$BIN execute -env --db 1 --human "SELECT 1 AS test_val"
# → 正常返回
```

## 7.8 MASK_COLUMNS 列值屏蔽

```bash
MASK_COLUMNS=hostip=*** $BIN execute -env --db 1 --human "SELECT hostip, device_type FROM testdb.iplist LIMIT 3"
# → hostip 显示 ***，device_type 保持原值
```

## 7.9 策略链验证

策略链执行顺序：
```
sqlguard.Validate() → policy.CheckSQL/CheckNative() → AutoLimit() → ExecQuery()
```

验证策略拒绝在 sqlguard 之后、查询执行之前触发：

```bash
# sqlguard 放行但策略拒绝
DENY_TABLES=iplist $BIN execute -env --db 1 'SELECT 1 FROM testdb.iplist'
# → ACCESS_DENIED（非 READ_ONLY_VIOLATION）
```

## 7.10 防绕过验证

```bash
# 反引号包裹表名
DENY_TABLES=iplist $BIN execute -env --db 1 'SELECT * FROM `testdb`.`iplist`'
# → ACCESS_DENIED（归一化后匹配）

# 注释绕过
DENY_TABLES=iplist $BIN execute -env --db 1 'SELECT * FROM testdb.-- comment\niplist'
# → ACCESS_DENIED（注释剥离后匹配）

# 多余空白
DENY_STATEMENTS="DROP TABLE" $BIN execute -env --db 1 'DROP  TABLE  users'
# → ACCESS_DENIED（空白归一化后匹配）
```
