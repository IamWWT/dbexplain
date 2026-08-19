# L5: SQL 只读沙箱验证

> 验证 sqlguard 三层校验：动词白名单 + 多语句检测 + 自动 LIMIT。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../setup-guide/CONFIG_SEARCH.md)。

## 6.1 读操作允许

```bash
# SELECT
$BIN execute --db 1 "SELECT 1" 2>&1 | jq .
# EXPLAIN
$BIN execute --db 1 "EXPLAIN SELECT 1" 2>&1 | jq .
# WITH (CTE)
$BIN execute --db 1 "WITH t AS (SELECT 1) SELECT * FROM t" 2>&1 | jq .
# SHOW
$BIN execute --db 1 "SHOW DATABASES" 2>&1 | jq .
# DESCRIBE
$BIN execute --db 1 "DESCRIBE testdb.iplist" 2>&1 | jq .
```

## 6.2 写操作拒绝

```bash
for cmd in INSERT UPDATE DELETE DROP ALTER CREATE TRUNCATE RENAME REPLACE GRANT REVOKE KILL SHUTDOWN FLUSH SET RESET CALL PURGE; do
  echo "=== $cmd ==="
  $BIN execute --db 1 "$cmd test" 2>&1 | head -1
done
# 预期: 全部返回 READ_ONLY_VIOLATION
```

## 6.3 多语句检测

```bash
# 多 SELECT (保守拒绝)
$BIN execute --db 1 "SELECT 1; SELECT 2"
# 预期: READ_ONLY_VIOLATION: multiple statements detected (2)

# 写注入绕过
$BIN execute --db 1 "SELECT 1; DROP TABLE users"
# 预期: READ_ONLY_VIOLATION: multiple statements detected (2)
```

## 6.4 自动 LIMIT 注入

```bash
# 无 LIMIT 自动追加
$BIN execute --db 1 "SELECT * FROM testdb.iplist" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'rows={d[\"row_count\"]}, truncated={d[\"truncated\"]}')
# 预期: rows <= 1000 or truncated=true
"

# 已有 LIMIT 不追加
$BIN execute --db 1 "SELECT * FROM testdb.iplist LIMIT 5" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'rows={d[\"row_count\"]}')  # 预期 rows=5
"

# LIMIT( 紧凑语法（v0.1.2+ 支持）
$BIN execute --db 1 "SELECT * FROM testdb.iplist LIMIT(3)" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'rows={d[\"row_count\"]}')  # 预期 rows=3（不重复追加 LIMIT）
"
```

## 6.5 --explain 标志路径不走自动 LIMIT

> 仅 `--explain` 标志路径跳过 AutoLimit（EXPLAIN 包裹发生在校验之后）。用户手写 `EXPLAIN ...`（无 `--explain` 标志）仍会走 AutoLimit 追加 `LIMIT 1000`（v0.1.11 行为不变）。

```bash
$BIN execute --db 1 --explain "SELECT * FROM testdb.iplist WHERE device_type = 'PHY'" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'columns={[c[\"name\"] for c in d[\"columns\"]]}')
# 预期: EXPLAIN 返回计划列（如 id/select_type/table 等）
"
```

## 6.6 EXPLAIN 分数据库类型格式化 (v0.1.2+)

```bash
# MySQL — FORMAT=JSON（human 输出用 render.FormatExplainJSON）
$BIN execute --db 1 --explain "SELECT * FROM testdb.iplist WHERE device_type = 'PHY'" --human 2>/dev/null | head -5
# 预期: MySQL JSON explain plan 可读渲染（含 select_type/table/access_type）

# SQLite — EXPLAIN QUERY PLAN
$BIN execute --db 3 --explain "SELECT * FROM rules WHERE rule_id = 1" --human 2>/dev/null | head -5
# 预期: 含 SCAN TABLE / SEARCH TABLE 等查询计划信息

# PostgreSQL — EXPLAIN (ANALYZE, BUFFERS)
$BIN execute --db 6 --explain "SELECT 1" --human 2>/dev/null | head -5
# 预期: 含 Seq Scan / Result 等执行计划
```

## 6.7 空查询拒绝

```bash
$BIN execute --db 1 ""
# 预期: READ_ONLY_VIOLATION: empty query

$BIN execute --db 1 "   "
# 预期: READ_ONLY_VIOLATION: empty query
```

## 6.8 CTE 写检测加固 (v0.1.7+)

验证 CTE（WITH）查询中写操作检测的完备性。

### 6.8.1 CTE 体写操作拦截

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

### 6.8.2 CTE + 主查询写操作拦截

WITH 定义后主查询为 INSERT/UPDATE/DELETE 应被拒绝。

```bash
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH x AS (SELECT 1) INSERT INTO y VALUES (1)"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation

$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH x AS (SELECT 1), y AS (SELECT 2) DELETE FROM z"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation
```

### 6.8.3 合法 WITH 查询不被拦截

```bash
$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH cte AS (SELECT 1 AS n) SELECT * FROM cte"
# 预期: 正常返回结果，n=1

$BIN execute -dsn "sqlite:///:memory:?label=test" "WITH a AS (SELECT 1 AS x), b AS (SELECT 2 AS y) SELECT * FROM a JOIN b"
# 预期: 正常返回结果
```

### 6.8.4 单元测试验证

```bash
cd src && go test -tags full ./internal/sqlguard/ -v -run TestValidate_RejectedWriteOps
# 预期: PASS — 37 测试用例全部通过（含 CTE 体写 + 主查询写）
```

### 6.8.5 AutoLimit 不受影响

```bash
$BIN execute -dsn "sqlite:///:memory:?label=test" --human "WITH cte AS (SELECT 1 AS n) SELECT * FROM cte"
# 预期: 正常返回，自动追加 LIMIT 1000
```

### 6.9 EXPLAIN 内部语句写检测加固 (v0.1.11+)

> **背景**：EXPLAIN 在只读白名单内且 AST 解析器不识别，回退仅检查首词——`EXPLAIN ANALYZE INSERT ...` 等 EXPLAIN 包裹写语句可绕过只读校验（PostgreSQL/GaussDB/MySQL 8.0.18+/DuckDB 的 `EXPLAIN ANALYZE` 会真实执行）。v0.1.11 起剥离 EXPLAIN 前缀与方言选项后对内部语句递归校验。

#### 6.9.1 EXPLAIN 包裹写语句拦截

```bash
# EXPLAIN + INSERT
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN INSERT INTO t VALUES(1)"
# 预期: READ_ONLY_VIOLATION: write operation "INSERT" is not allowed

# EXPLAIN (ANALYZE) + INSERT —— 高危（pg/gaussdb/mysql8/duckdb 真实执行）
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN (ANALYZE) INSERT INTO t VALUES(1)"
# 预期: READ_ONLY_VIOLATION: write operation "INSERT" is not allowed

# EXPLAIN ANALYZE + UPDATE
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN ANALYZE UPDATE t SET a=1"
# 预期: READ_ONLY_VIOLATION: write operation "UPDATE" is not allowed

# 方言修饰符包裹（FORMAT=JSON / QUERY PLAN / PLAN FOR）
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN FORMAT=JSON INSERT INTO t VALUES(1)"
# 预期: READ_ONLY_VIOLATION: write operation "INSERT" is not allowed

$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN QUERY PLAN INSERT INTO t VALUES(1)"
# 预期: READ_ONLY_VIOLATION: write operation "INSERT" is not allowed

# EXPLAIN + CTE 写
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN WITH x AS (SELECT 1) INSERT INTO y VALUES (1)"
# 预期: READ_ONLY_VIOLATION: WITH CTE contains write operation

# EXPLAIN + 多语句
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN SELECT 1; DROP TABLE t"
# 预期: READ_ONLY_VIOLATION: multiple statements detected (2)

# hint/注释包裹写动词同样拒绝
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN /*+ hint */ INSERT INTO t VALUES(1)"
# 预期: READ_ONLY_VIOLATION: write operation "INSERT" is not allowed

# EXPLAIN 单独（无内部语句）
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN"
# 预期: READ_ONLY_VIOLATION: EXPLAIN without an inner statement
```

#### 6.9.2 合法 EXPLAIN 不被拦截

```bash
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN QUERY PLAN SELECT 1"
# 预期: 正常返回查询计划

$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN SELECT 1"
# 预期: 正常返回查询计划

# hint/注释形态（Oracle/MySQL hint 风格）
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN /*+ hint */ SELECT 1"
# 预期: 正常返回查询计划

# 字符串字面量含写动词 — 防误报
$BIN execute -dsn "sqlite:///:memory:?label=test" "EXPLAIN SELECT * FROM t WHERE note = 'INSERT'"
# 预期: 正常返回（INSERT 在字符串字面量内，非写操作）
```

> **冒烟范围说明**：本小节 sqlite DSN 仅能冒烟 sqlite 兼容形态（`EXPLAIN QUERY PLAN`、`EXPLAIN`）。postgres `(ANALYZE, BUFFERS, FORMAT TEXT)`、mysql `FORMAT=JSON`、oracle `PLAN FOR`、clickhouse `PLAN` 等方言形态的校验行为由单测 `TestStripExplainPrefix` 覆盖（§6.9.3），无需真实 DB 也可验证 sqlguard 层。

#### 6.9.3 单元测试验证

```bash
cd src && go test -tags full ./internal/sqlguard/ -v -run 'TestStripExplainPrefix|TestValidate_AllowedReadOps|TestValidate_RejectedWriteOps'
# 预期: PASS — 121 个子测试全部通过（各方言 EXPLAIN 形态、EXPLAIN 包裹写语句拒绝、hint/注释形态、字符串字面量防误报、多语句）
```
