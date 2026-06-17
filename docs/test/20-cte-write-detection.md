# CTE 写检测加固测试 (v0.1.7)

> **已合并**: 本文件内容已合并到 [06-security-sqlguard.md §6.8](06-security-sqlguard.md#68-cte-写检测加固-v017)。请直接参考该文件。

验证 CTE（WITH）查询中写操作检测的完备性：CTE 体写操作 + 主查询写操作均被正确拦截。

---

## 环境

```bash
cd src
BIN="../release/dbexplain"
```

## 测试项

### 1. CTE 体写操作拦截

CTE 定义体中包含 INSERT/UPDATE/DELETE 应被拒绝。

```bash
# CTE 体中 INSERT ... RETURNING
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH ins AS (INSERT INTO t VALUES(1) RETURNING id) SELECT * FROM ins"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation

# CTE 体中 DELETE ... RETURNING
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH del AS (DELETE FROM orders WHERE id=1 RETURNING id) SELECT * FROM del"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation

# CTE 体中 UPDATE ... RETURNING
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH upd AS (UPDATE users SET status='banned' RETURNING id) SELECT * FROM upd"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation

# 多个 CTE，其中一个含写操作
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH a AS (INSERT INTO t VALUES(1)), b AS (SELECT 1) SELECT * FROM b"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation
```

### 2. CTE + 主查询写操作拦截

WITH 定义后主查询为 INSERT/UPDATE/DELETE 应被拒绝。

```bash
# WITH + INSERT 主查询
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH x AS (SELECT 1) INSERT INTO y VALUES (1)"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation

# WITH + DELETE 主查询
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH x AS (SELECT 1), y AS (SELECT 2) DELETE FROM z"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation
```

### 3. 合法 WITH 查询不被拦截

纯读取的 WITH 查询应正常执行。

```bash
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH cte AS (SELECT 1 AS n) SELECT * FROM cte"
# 预期: 正常返回结果，n=1

$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH a AS (SELECT 1 AS x), b AS (SELECT 2 AS y) SELECT * FROM a JOIN b"
# 预期: 正常返回结果
```

### 4. 单元测试验证

```bash
cd src && go test -tags full ./internal/sqlguard/ -v -run TestValidate_RejectedWriteOps
# 预期: PASS — 37 测试用例全部通过（含 CTE 体写 + 主查询写）
```

### 5. AutoLimit 不受影响

AutoLimit 对 WITH 查询正常追加 LIMIT。

```bash
$BIN execute -dsn "sqlite:///:memory:?label=test" --human "WITH cte AS (SELECT 1 AS n) SELECT * FROM cte"
# 预期: 正常返回，自动追加 LIMIT 1000
```

---

## 预期结果

| # | 测试项 | 预期状态 |
|---|--------|---------|
| 1 | CTE 体 INSERT/DELETE/UPDATE 拦截 | 拒绝写入 |
| 2 | WITH + 主查询 INSERT/DELETE 拦截 | 拒绝写入 |
| 3 | 合法 WITH 纯读取查询 | 正常执行 |
| 4 | 单元测试全部通过 | 37/37 PASS |
| 5 | AutoLimit 兼容 | 正常追加 |
