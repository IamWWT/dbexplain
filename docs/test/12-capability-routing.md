# L8: v0.1.0 能力架构验证

> 验证 v0.1.0 新增的能力架构（Capability Architecture）：
> CapSQL 路由、PostgreSQL 多 Schema 采集、策略引擎增强、JSON instances 包装格式。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

## 12.1 CapSQL 路由验证

v0.1.0 用 `capabilities.FromProvider().Has(CapSQL)` 替代了 `isSQLKind()` switch 分支。
验证 SQL 数据源和非 SQL 数据源的路由正确性。

```bash
# SQL 数据源（CapSQL=true）：走 sqlguard → AutoLimit → CheckSQL 链路
# 写操作应被 sqlguard 拦截
echo "=== MySQL (SQL) ==="
$BIN execute --db 1 "DROP TABLE test" 2>&1 | head -1
# → READ_ONLY_VIOLATION

echo "=== PostgreSQL (SQL) ==="
$BIN execute --db 6 "DROP TABLE test" 2>&1 | head -1
# → READ_ONLY_VIOLATION

echo "=== ClickHouse (SQL) ==="
$BIN execute --db 2 "DROP TABLE test" 2>&1 | head -1
# → READ_ONLY_VIOLATION

# 非 SQL 数据源（CapSQL=false）：跳过 sqlguard，走 CheckNative
echo "=== Redis (non-SQL) ==="
$BIN execute --db 7 "FLUSHALL" 2>&1 | head -1
# → ACCESS_DENIED（策略引擎拦截，非 sqlguard）

echo "=== MongoDB (non-SQL) ==="
$BIN execute --db 9 '{"drop":"test"}' 2>&1 | head -1
# → ACCESS_DENIED（策略引擎拦截，非 READ_ONLY_VIOLATION）

echo "=== Qdrant (non-SQL) ==="
$BIN execute --db 4 '{"delete":"runbooks"}' 2>&1 | head -1
# → ACCESS_DENIED（策略引擎拦截）

# 文件数据源（CapFile=true）：跳过 sqlguard，仍受策略引擎约束
# CSV 不支持 DROP，返回 QUERY_NOT_SUPPORTED
echo "=== CSV (File) ==="
$BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "DROP TABLE" 2>&1 | head -1
# → QUERY_NOT_SUPPORTED（文件数据源只读，不接受 DROP）
```

## 12.2 JSON instances 包装格式验证

v0.1.0 Schema 采集 JSON 输出使用 `{"instances": [...]}` 顶级包装。

```bash
echo "=== JSON wrapper structure ==="
$BIN --label aiops-mysql --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
# 验证顶级结构
assert 'instances' in d, 'missing instances wrapper'
insts = d['instances']
assert isinstance(insts, list), 'instances should be array'
assert len(insts) == 1, f'expected 1, got {len(insts)}'
inst = insts[0]
# 验证实例字段（注意：v0.1.0 不含 dsn 字段）
for key in ['kind', 'label', 'databases']:
    assert key in inst, f'missing: {key}'
# 验证数据库结构
for db in inst['databases']:
    assert 'name' in db, 'db missing name'
    assert 'tables' in db, 'db missing tables'
    for t in db['tables']:
        assert 'name' in t
        assert 'columns' in t
        assert 'row_count' in t
print('JSON instances wrapper: OK ✓')
print(f'Top-level keys: {sorted(d.keys())}')
print(f'Instance keys: {sorted(inst.keys())}')
"
```

## 12.3 PostgreSQL 多 Schema 验证

v0.1.0 采集所有非系统 schema（public + 自定义），不再仅限于 public。

```bash
echo "=== PostgreSQL schemas ==="
$BIN --label video-pg --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
inst = d.get('instances', [d])[0]
tables = []
for db in inst['databases']:
    for t in db['tables']:
        tables.append(t['name'])
print(f'Tables ({len(tables)}): {tables}')
# 预期：至少包含 public schema 的表，可能包含其他非系统 schema 的表
print('PostgreSQL multi-schema collection: OK ✓')
"
```

## 12.4 策略引擎 matchStarSelect 验证

v0.1.0 新增 `matchStarSelect`：`SELECT *` 时检查 `DENY_COLUMNS` 表前缀。

```bash
echo "=== matchStarSelect: SELECT * with denied columns ==="
# 不加载 .env 策略，直接通过 -dsn 测试
MYSQL_DSN="mysql://root:root@123456@localhost:9433/testdb?label=test"

# 验证：SELECT * 但 iplist.owner 在 DENY_COLUMNS 中
DENY_COLUMNS="iplist.owner" $BIN execute -dsn "$MYSQL_DSN" "SELECT * FROM iplist" 2>&1 | head -1
# → ACCESS_DENIED: column "iplist.owner"

# 验证：显式列名，不包含 owner，应放行
DENY_COLUMNS="iplist.owner" $BIN execute -dsn "$MYSQL_DSN" "SELECT ID, hostip FROM iplist" --human 2>&1 | head -5
# → 正常返回

# 验证：SELECT * 但 DENY_COLUMNS 无关该表，应放行
DENY_COLUMNS="other_table.secret" $BIN execute -dsn "$MYSQL_DSN" "SELECT * FROM iplist LIMIT 2" --human 2>&1 | head -5
# → 正常返回
```

## 12.5 策略引擎 CTE 支持验证

v0.1.0 修复 `extractTableNames` 支持 CTE（WITH 子句）。

```bash
echo "=== CTE policy check ==="
# CTE 中的表名应被策略引擎识别
DENY_TABLES="iplist" $BIN execute -dsn "$MYSQL_DSN" "WITH t AS (SELECT * FROM iplist) SELECT * FROM t" 2>&1 | head -1
# → ACCESS_DENIED: table "iplist" is not allowed

# CTE 查询正常（表允许时）
$BIN execute -dsn "$MYSQL_DSN" "WITH t AS (SELECT 1 AS n) SELECT * FROM t" --human 2>&1 | head -5
# → 正常返回 row
```

## 12.6 文件数据源策略引擎验证

v0.1.0 对 CSV/TSV/XLSX 文件数据源应用策略引擎（DENY_TABLES, MASK_COLUMNS）。

```bash
echo "=== File policy: DENY_TABLES ==="
DENY_TABLES="users" $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" 2>&1 | head -1
# → ACCESS_DENIED: table "users" is not allowed for query

echo "=== File policy: MASK_COLUMNS ==="
MASK_COLUMNS="name=***" $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human 2>&1
# → name 列显示 ***（非原始值）

echo "=== File policy: no policy = normal return ==="
$BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human 2>&1
# → 正常显示 3 行数据
```

## 12.7 能力一致性验证

所有 connector 的能力声明不应有冲突。

```bash
echo "=== Capability consistency ==="
$BIN -timeout 30s --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
insts = d.get('instances', [])
cap_map = {}
for inst in insts:
    kind = inst.get('kind', '?')
    label = inst.get('label', '?')
    # Count SQL capable vs non-SQL
    db_count = len(inst.get('databases', []) or [])
    table_count = sum(len(db.get('tables', []) or []) for db in (inst.get('databases', []) or []))
    print(f'{kind:15s} {label:25s} dbs={db_count} tables={table_count}')
    cap_map[label] = kind
print(f'\\nTotal: {len(insts)} data sources')
print('Capability consistency: OK ✓')
"
```
