# L3: Schema 采集验证

> 验证全部数据源的 Schema 采集能力，含基础输出和带参数输出。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain-linux-amd64"
# 或使用 go run: BIN="go run ."
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [test/README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../docs/CONFIG_SEARCH.md)。

## 2.1 全量 Schema 采集 (JSON)

```bash
$BIN -env -timeout 30s --json 2>/dev/null | python3 -c "
import json, sys
data = json.load(sys.stdin)
if isinstance(data, list):
    print(f'Total data sources: {len(data)}')
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
    print(f'Single: kind={data.get(\"kind\",\"?\")} label={data.get(\"label\",\"?\")}')
"
# 预期: 全部 15 个 DSN 采集成功
```

## 2.2 全量 Schema 采集 (Human)

```bash
$BIN -env -timeout 30s --human 2>/dev/null | head -50
```

## 2.3 按类型过滤

```bash
# 仅 SQL 数据库
$BIN -env -include mysql,postgres,clickhouse,sqlite,elasticsearch --json 2>/dev/null | python3 -c "
import json,sys; data=json.load(sys.stdin)
if isinstance(data): print(f'SQL DBs: {len(data)}')
"
```

```bash
# 仅 NoSQL
$BIN -env -include redis,mongodb,qdrant --json 2>/dev/null | python3 -c "
import json,sys; data=json.load(sys.stdin)
if isinstance(data): print(f'NoSQL DBs: {len(data)}')
"
```

```bash
# 仅文件
$BIN -env -include csv,tsv,xlsx --json 2>/dev/null | python3 -c "
import json,sys; data=json.load(sys.stdin)
if isinstance(data): print(f'Files: {len(data)}')
"
```

## 2.4 按 label 过滤

```bash
$BIN -env --label aiops-mysql --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
if isinstance(d, list) and len(d) == 1:
    d = d[0]
print(f'kind={d.get(\"kind\")} label={d.get(\"label\")} dbs={len(d.get(\"databases\",[]))}')
"
```

## 2.5 JSON 结构验证

```bash
$BIN -env --label aiops-mysql --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
if isinstance(d, list) and len(d) == 1:
    d = d[0]
required = ['kind', 'label', 'dsn', 'databases']
for field in required:
    assert field in d, f'missing field: {field}'
    print(f'  ✓ {field}')
for db in d['databases']:
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
  $BIN -env --label "$label" --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
if isinstance(d, list) and len(d) == 1:
    d = d[0]
tables = sum(len(db.get('tables',[])) for db in d.get('databases',[]))
print(f'  kind={d.get(\"kind\")} tables={tables}')
"
done
```

```bash
# NoSQL 数据库
for label in aiops-qdrant openim-redis video-redis openim-mongo; do
  echo "=== $label ==="
  $BIN -env --label "$label" --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
if isinstance(d, list) and len(d) == 1:
    d = d[0]
tables = sum(len(db.get('tables',[])) for db in d.get('databases',[]))
print(f'  kind={d.get(\"kind\")} tables={tables}')
"
done
```

```bash
# 文件数据源
for label in tsf-xlsx tdmq-xlsx ops-data-csv test-data-csv tsv-test-data; do
  echo "=== $label ==="
  $BIN -env --label "$label" --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
if isinstance(d, list) and len(d) == 1:
    d = d[0]
tables = sum(len(db.get('tables',[])) for db in d.get('databases',[]))
print(f'  kind={d.get(\"kind\")} tables={tables}')
"
done
```
