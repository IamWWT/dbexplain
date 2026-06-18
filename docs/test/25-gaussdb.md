# GaussDB 连接器测试 (v0.1.9+)

> 验证 GaussDB 连接器的 DSN 解析、CLI 帮助、Schema 采集、Oracle 兼容模式、EXPLAIN 路由、构建隔离。
> GaussDB 使用独立 DSN 构建器 `buildGaussDBDSN()`（`gaussdb://` 协议头），共享 `collectPGDB()`/`executeSQLQuery()` 包级函数。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

## 测试项

### T1: DSN 解析

```bash
# GaussDB PG 兼容模式
$BIN -dsn 'gaussdb://user:pass@host:5432/mydb?label=test-gaussdb-pg' --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)['instances'][0]
print(f'kind={d[\"kind\"]} label={d[\"label\"]}')
# 预期: kind=gaussdb, label=test-gaussdb-pg
"

# GaussDB Oracle 兼容模式
$BIN -dsn 'gaussdb://user:pass@host:5432/mydb?oracleCompatible=true&label=test-gaussdb-ora' --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)['instances'][0]
print(f'kind={d[\"kind\"]} label={d[\"label\"]}')
# 预期: kind=gaussdb, label=test-gaussdb-ora
"
```

**验证点**: scheme `gaussdb://` → kind=`gaussdb`；`oracleCompatible=true` 作为 DSN 参数保留。

### T2: CLI 子命令帮助

```bash
$BIN gaussdb 2>&1 | head -5
# 预期: 显示 GaussDB 专用手册（含版本号）
```

### T3: 全量手册中包含 GaussDB

```bash
$BIN all 2>&1 | grep -i gaussdb
# 预期: 输出包含 GaussDB 章节
```

### T4: Schema 采集

GaussDB 连接器复用 `collectPGDB()` 包级函数，行为与 PostgreSQL 一致：

```bash
# 需配置可连接的 GaussDB DSN
$BIN --label my-gaussdb --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
inst = d.get('instances', [d])[0]
print(f'kind={inst.get(\"kind\")} dbs={len(inst.get(\"databases\",[]))}')
# 预期: kind=gaussdb, dbs>=1
"
```

### T5: Oracle 兼容模式 — datistemplate 回退

Oracle 兼容模式下 `datistemplate` 列可能缺失，连接器自动回退到 `SELECT datname FROM pg_database WHERE datallowconn`：

```bash
# 无需实际数据库，检查代码路径
grep -n "datistemplate" src/internal/connector/gaussdb.go
# 预期: 显示 Oracle 兼容模式跳过 datistemplate 的逻辑
```

### T6: EXPLAIN 路由 — 无 BUFFERS

```bash
# 检查 executor.go 中 GaussDB EXPLAIN 路由
grep -n "gaussdb" src/internal/executor/executor.go
# 预期: GaussDB 使用 EXPLAIN (ANALYZE, FORMAT TEXT)（无 BUFFERS 选项）
```

### T7: 构建隔离

```bash
# 全量编译包含 gaussdb（在 postgres build tag 中）
go build -tags full ./... && echo "build: OK"

# gaussdb 在子命令列表中
$BIN -h 2>&1 | grep gaussdb
# 预期: 包含 gaussdb
```

### T8: Redacted 脱敏与 SanitizeErr

GaussDB 的 `gaussdb://user:pass@host/db` 格式曾触发 SanitizeErr 死循环 (ISSUE-095)：

```bash
# 验证 Redacted() 输出安全
$BIN -dsn 'gaussdb://u:p@h:5432/d?label=test' --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
inst = d.get('instances', [d])[0] if isinstance(d, dict) else d[0]
dsn = inst.get('dsn', '')
assert '{dbpassword}' in dsn or '***' in dsn, f'password leaked: {dsn}'
print('Redacted: OK ✓')
"
```

---

## 测试总结

| 编号 | 测试项 | 类型 | 无需外部 DB |
|------|--------|------|-----------|
| T1 | DSN 解析 | 功能验证 | ✅ |
| T2 | CLI 手册 | 功能验证 | ✅ |
| T3 | 全量手册 | 功能验证 | ✅ |
| T4 | Schema 采集 | 功能验证 | ❌（需 GaussDB 实例） |
| T5 | datistemplate 回退 | 静态分析 | ✅ |
| T6 | EXPLAIN 路由 | 静态分析 | ✅ |
| T7 | 构建隔离 | 编译检查 | ✅ |
| T8 | 密码脱敏 | 功能验证 | ✅ |
