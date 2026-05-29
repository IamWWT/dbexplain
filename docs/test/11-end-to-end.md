# L4: 全量集成测试

> 一次运行所有数据源的 Schema 采集和查询执行，验证整体管道完整性。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain-linux-amd64"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../docs/CONFIG_SEARCH.md)。

## 11.1 全部数据源 Schema 采集

```bash
$BIN -env -timeout 60s --json 2>/dev/null | python3 -c "
import json, sys
data = json.load(sys.stdin)
if isinstance(data, list):
    print(f'Total data sources collected: {len(data)}')
    for d in data:
        kind = d.get('kind', '?')
        label = d.get('label', '?')
        db_count = len(d.get('databases', []))
        tables = []
        for db in d.get('databases', []):
            for t in db.get('tables', []):
                tables.append(t.get('name', '?'))
        print(f'  [{kind:15s}] {label:25s} databases={db_count} tables={tables}')
else:
    kind = data.get('kind', '?')
    label = data.get('label', '?')
    print(f'Single: kind={kind} label={label}')
"
```

预期输出示例:
```
Total data sources collected: 15
  [mysql]          aiops-mysql           databases=1 tables=['iplist', 'port']
  [clickhouse]     aiops-clickhouse      databases=2 tables=[...]
  ...
  [csv]            ops-data-csv          databases=1 tables=['ops_data']
  [tsv]            tsv-test-data         databases=1 tables=['currency_minor_singular']
```

## 11.2 全部数据源 Execute 验证

```bash
# SQL 数据库
echo "=== MySQL ===" && $BIN execute -env --db 1 "SELECT 1" --human 2>/dev/null
echo "=== PostgreSQL ===" && $BIN execute -env --db 6 "SELECT 1" --human 2>/dev/null
echo "=== ClickHouse ===" && $BIN execute -env --db 2 "SELECT 1" --human 2>/dev/null
echo "=== SQLite (rules) ===" && $BIN execute -env --db 3 "SELECT 1" --human 2>/dev/null
echo "=== SQLite (veinmap) ===" && $BIN execute -env --db 10 "SELECT 1" --human 2>/dev/null
echo "=== Elasticsearch ===" && $BIN execute -env --db 5 "SHOW TABLES" --human 2>/dev/null

# NoSQL 数据库
echo "=== Redis (openim) ===" && $BIN execute -env --db 7 "PING" --human 2>/dev/null
echo "=== Redis (video) ===" && $BIN execute -env --db 8 "PING" --human 2>/dev/null
echo "=== MongoDB ===" && $BIN execute -env --db 9 '{"count":"system.users"}' --human 2>/dev/null
echo "=== Qdrant ===" && $BIN execute -env --db 4 '{"count":"runbooks"}' --human 2>/dev/null

# 文件处理
echo "=== CSV (ops-data) ===" && $BIN execute -env --db 13 "SELECT * LIMIT 3" --human 2>/dev/null
echo "=== CSV (test-data) ===" && $BIN execute -env --db 14 "SELECT * LIMIT 3" --human 2>/dev/null
echo "=== TSV ===" && $BIN execute -env --db 15 "SELECT * LIMIT 3" --human 2>/dev/null
echo "=== XLSX (TSF) ===" && $BIN execute -env --label tsf-xlsx "SELECT * LIMIT 3" --human 2>/dev/null
echo "=== XLSX (TDMQ) ===" && $BIN execute -env --label tdmq-xlsx "SELECT * LIMIT 3" --human 2>/dev/null
```

## 11.3 Schema JSON 结构验证

```bash
$BIN -env --label aiops-mysql --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
insts = d.get('instances', [d])
inst = insts[0] if isinstance(insts, list) else d
# Note: v0.1.0 wraps in {"instances": [...]}, no top-level dsn field
required = ['kind', 'label', 'databases']
for field in required:
    assert field in inst, f'missing field: {field}'
    print(f'  ✓ {field}')
for db in inst['databases']:
    assert 'name' in db
    assert 'tables' in db
    for t in db['tables']:
        assert 'name' in t
        assert 'columns' in t
        assert 'row_count' in t
print('All required fields present ✓')
"
```

## 11.4 Execute JSON 结构验证

```bash
$BIN execute -env --db 1 "SELECT 1 AS n, 'hello' AS s" 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
required = ['columns', 'rows', 'row_count', 'execution_time', 'truncated']
for field in required:
    assert field in d, f'missing field: {field}'
    print(f'  ✓ {field}')
assert d['row_count'] == 1
assert d['columns'][0]['name'] == 'n'
assert d['rows'][0][0] == 1
assert d['rows'][0][1] == 'hello'
print('Execute JSON structure OK ✓')
"
```

## 11.5 汇总报告

```bash
echo "=== dbexplain v0.1.0 集成测试报告 ==="
echo "日期: $(date '+%Y-%m-%d %H:%M')"
echo "Go 版本: $(go version)"
echo ""
echo "--- 编译 ---"
go build ./... && echo "编译: ✓" || echo "编译: ✗"
go vet ./... && echo "Vet:   ✓" || echo "Vet:   ✗"
echo ""
echo "--- 单元测试 ---"
go test ./... -count=1 2>&1 | grep -E '^(ok|FAIL|---)' | head -20
echo ""
echo "--- Schema 采集 ---"
$BIN -env -timeout 30s --json 2>/dev/null | python3 -c "
import json,sys
data = json.load(sys.stdin)
if isinstance(data, list):
    print(f'数据源: {len(data)}')
    for d in data:
        print(f'  {d.get(\"kind\",\"?\"):15s} {d.get(\"label\",\"?\"):25s} ✓')
else:
    print(f'数据源: 1 ({data.get(\"kind\",\"?\")})')
"
echo ""
echo "--- 版本 ---"
$BIN --version
```
