# L5: SQL 只读沙箱验证

> 验证 sqlguard 三层校验：动词白名单 + 多语句检测 + 自动 LIMIT。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

## 6.1 读操作允许

```bash
# SELECT
$BIN execute -env --db 1 "SELECT 1" 2>&1 | jq .
# EXPLAIN
$BIN execute -env --db 1 "EXPLAIN SELECT 1" 2>&1 | jq .
# WITH (CTE)
$BIN execute -env --db 1 "WITH t AS (SELECT 1) SELECT * FROM t" 2>&1 | jq .
# SHOW
$BIN execute -env --db 1 "SHOW DATABASES" 2>&1 | jq .
# DESCRIBE
$BIN execute -env --db 1 "DESCRIBE testdb.iplist" 2>&1 | jq .
```

## 6.2 写操作拒绝

```bash
for cmd in INSERT UPDATE DELETE DROP ALTER CREATE TRUNCATE RENAME REPLACE GRANT REVOKE KILL SHUTDOWN FLUSH SET RESET CALL PURGE; do
  echo "=== $cmd ==="
  $BIN execute -env --db 1 "$cmd test" 2>&1 | head -1
done
# 预期: 全部返回 READ_ONLY_VIOLATION
```

## 6.3 多语句检测

```bash
# 多 SELECT (保守拒绝)
$BIN execute -env --db 1 "SELECT 1; SELECT 2"
# 预期: READ_ONLY_VIOLATION: multiple statements detected (2)

# 写注入绕过
$BIN execute -env --db 1 "SELECT 1; DROP TABLE users"
# 预期: READ_ONLY_VIOLATION: multiple statements detected (2)
```

## 6.4 自动 LIMIT 注入

```bash
# 无 LIMIT 自动追加
$BIN execute -env --db 1 "SELECT * FROM testdb.iplist" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'rows={d[\"row_count\"]}, truncated={d[\"truncated\"]}')
# 预期: rows <= 1000 or truncated=true
"

# 已有 LIMIT 不追加
$BIN execute -env --db 1 "SELECT * FROM testdb.iplist LIMIT 5" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'rows={d[\"row_count\"]}')  # 预期 rows=5
"

# LIMIT( 紧凑语法（v0.1.2+ 支持）
$BIN execute -env --db 1 "SELECT * FROM testdb.iplist LIMIT(3)" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'rows={d[\"row_count\"]}')  # 预期 rows=3（不重复追加 LIMIT）
"
```

## 6.5 EXPLAIN 不走自动 LIMIT

```bash
$BIN execute -env --db 1 --explain "SELECT * FROM testdb.iplist WHERE device_type = 'PHY'" 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
print(f'columns={[c[\"name\"] for c in d[\"columns\"]]}')
# 预期: EXPLAIN 返回计划列（如 id/select_type/table 等）
"
```

## 6.6 EXPLAIN 分数据库类型格式化 (v0.1.2+)

```bash
# MySQL — FORMAT=JSON（human 输出用 render.FormatExplainJSON）
$BIN execute -env --db 1 --explain "SELECT * FROM testdb.iplist WHERE device_type = 'PHY'" --human 2>/dev/null | head -5
# 预期: MySQL JSON explain plan 可读渲染（含 select_type/table/access_type）

# SQLite — EXPLAIN QUERY PLAN
$BIN execute -env --db 3 --explain "SELECT * FROM rules WHERE rule_id = 1" --human 2>/dev/null | head -5
# 预期: 含 SCAN TABLE / SEARCH TABLE 等查询计划信息

# PostgreSQL — EXPLAIN (ANALYZE, BUFFERS)
$BIN execute -env --db 6 --explain "SELECT 1" --human 2>/dev/null | head -5
# 预期: 含 Seq Scan / Result 等执行计划
```

## 6.7 空查询拒绝

```bash
$BIN execute -env --db 1 ""
# 预期: READ_ONLY_VIOLATION: empty query

$BIN execute -env --db 1 "   "
# 预期: READ_ONLY_VIOLATION: empty query
```
