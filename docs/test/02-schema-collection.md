# L3: Schema 采集验证

> 验证全部数据源的 Schema 采集能力，含基础输出和带参数输出。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
# 或使用 go run: BIN="go run ."
```

> **配置优先级**：确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../setup-guide/CONFIG_SEARCH.md)。

## 2.1 全量 Schema 采集 (JSON)

```bash
$BIN -timeout 30s --json 2>/dev/null | python3 -c "
import json, sys
data = json.load(sys.stdin)
instances = data.get('instances', data if isinstance(data, list) else [data])
print(f'Total data sources: {len(instances)}')
for d in instances:
    kind = d.get('kind', '?')
    label = d.get('label', '?')
    db_count = len(d.get('databases', []))
    tables = []
    for db in d.get('databases', []):
        for t in db.get('tables', []):
            tables.append(t.get('name', '?'))
    print(f'  [{kind:15s}] {label:25s} databases={db_count} tables={tables}')
"
# 预期: 全部 19 个 DSN 采集成功
```

## 2.2 全量 Schema 采集 (Human)

```bash
$BIN -timeout 30s --human 2>/dev/null | head -50
```

## 2.3 按类型过滤

```bash
# 仅 SQL 数据库
$BIN -include mysql,postgres,clickhouse,sqlite,elasticsearch --json 2>/dev/null | python3 -c "
import json,sys; data=json.load(sys.stdin)
instances = data.get('instances', data if isinstance(data, list) else [data])
print(f'SQL DBs: {len(instances)}')
"
```

```bash
# 仅 NoSQL
$BIN -include redis,mongodb,qdrant --json 2>/dev/null | python3 -c "
import json,sys; data=json.load(sys.stdin)
instances = data.get('instances', data if isinstance(data, list) else [data])
print(f'NoSQL DBs: {len(instances)}')
"
```

```bash
# 仅文件
$BIN -include csv,tsv,xlsx --json 2>/dev/null | python3 -c "
import json,sys; data=json.load(sys.stdin)
instances = data.get('instances', data if isinstance(data, list) else [data])
print(f'Files: {len(instances)}')
"
```

## 2.4 按 label 过滤

```bash
$BIN --label aiops-mysql --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
inst = d.get('instances', [d])[0]
print(f'kind={inst.get(\"kind\")} label={inst.get(\"label\")} dbs={len(inst.get(\"databases\",[]))}')
"
```

## 2.5 JSON 结构验证

```bash
$BIN --label aiops-mysql --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
# v0.1.0: top-level envelope with instances wrapper
assert 'instances' in d, 'missing top-level: instances'
assert 'refs' in d, 'missing top-level: refs'
assert 'groups' in d, 'missing top-level: groups'
print('  ✓ top-level envelope (instances/refs/groups)')
inst = d['instances'][0]
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

## 2.6 各数据库类型逐一验证

```bash
# SQL 数据库
for label in aiops-mysql aiops-clickhouse intentapparatus-sqlite aiops-es video-pg veinmap-sqlite; do
  echo "=== $label ==="
  $BIN --label "$label" --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
inst = d.get('instances', [d])[0]
tables = sum(len(db.get('tables',[])) for db in inst.get('databases',[]))
print(f'  kind={inst.get(\"kind\")} tables={tables}')
"
done
```

```bash
# NoSQL 数据库
for label in aiops-qdrant openim-redis video-redis openim-mongo; do
  echo "=== $label ==="
  $BIN --label "$label" --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
inst = d.get('instances', [d])[0]
tables = sum(len(db.get('tables',[])) for db in inst.get('databases',[]))
print(f'  kind={inst.get(\"kind\")} tables={tables}')
"
done
```

```bash
# 文件数据源
for label in tsf-xlsx tdmq-xlsx ops-data-csv test-data-csv tsv-test-data; do
  echo "=== $label ==="
  $BIN --label "$label" --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
inst = d.get('instances', [d])[0]
tables = sum(len(db.get('tables',[])) for db in inst.get('databases',[]))
print(f'  kind={inst.get(\"kind\")} tables={tables}')
"
done
```

## 2.7 Schema 采集性能优化 (v0.1.7+)

### 2.7.1 --no-sample flag

```bash
$BIN --help 2>&1 | grep no-sample
# 预期: --no-sample  Skip sample row fetching during schema collection

$BIN collect --help 2>&1 | grep no-sample
# 预期: --no-sample  Skip sample row fetching during schema collection
```

### 2.7.2 --skip-opstats flag

```bash
$BIN --help 2>&1 | grep skip-opstats
# 预期: --skip-opstats  Skip MySQL performance_schema op stats
```

### 2.7.3 CSV 流式读取验证

```bash
echo -e "id,name\n1,alice\n2,bob\n3,charlie\n4,dave\n5,eve" > /tmp/test_stream.csv

$BIN execute -dsn "csv:///tmp/test_stream.csv?label=csvtest" -limit 3 "SELECT * FROM t LIMIT 2"
# 预期: 返回 2 行（id=1,alice; id=2,bob）

$BIN execute -dsn "csv:///tmp/test_stream.csv?label=csvtest" -limit 10 "SELECT * FROM t"
# 预期: 返回全部 5 行

rm -f /tmp/test_stream.csv
```

### 2.7.4 XLSX 流式读取验证

```bash
python3 -c "
from openpyxl import Workbook
wb = Workbook()
ws = wb.active
ws.title = 'Sheet1'
ws.append(['id', 'name'])
for i in range(1, 101):
    ws.append([i, f'user{i}'])
wb.save('/tmp/test_stream.xlsx')
" 2>/dev/null || echo "skip (no python/openpyxl)"

$BIN execute -dsn "xlsx:///tmp/test_stream.xlsx?label=xlsxtest" -limit 5 "SELECT * FROM Sheet1 LIMIT 3"
# 预期: 返回 3 行

$BIN execute -dsn "xlsx:///tmp/test_stream.xlsx?label=xlsxtest" -limit 5 "SELECT COUNT(*) AS cnt FROM Sheet1"
# 预期: 返回 COUNT = 100

rm -f /tmp/test_stream.xlsx
```

### 2.7.5 inferRefs name index 不退化

```bash
go test -tags full ./internal/analyze/ -v -count=1
# 预期: PASS（或 [no test files] — analyze 包编译即验证）
```
