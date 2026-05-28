# L11: 全量集成测试

> 一次运行所有数据源的 Schema 采集和查询执行，验证整体管道完整性。

## 11.1 全部数据源 Schema 采集

```bash
cd src

# 全量采集（所有 15 个 DSN）
dbexplain -env -timeout 60s --json 2>/dev/null | python3 -c "
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
    print(f'Single data source: kind={kind} label={label}')
"
# 预期: 全部 15 个 DSN 采集成功
```

实际结果 (2026-05):
```
Total data sources collected: 15
  [mysql]          aiops-mysql           databases=1 tables=['iplist', 'port']
  [clickhouse]     aiops-clickhouse      databases=2 tables=[...]
  [sqlite]         intentapparatus-sqlite databases=1 tables=[...]
  [qdrant]         aiops-qdrant          databases=1 tables=['mcp_tools', 'runbooks']
  [elasticsearch]  aiops-es              databases=1 tables=[17 indices]
  [postgres]       video-pg              databases=1 tables=[...]
  [redis]          openim-redis          databases=1 tables=[...]
  [redis]          video-redis           databases=1 tables=['_server_info']
  [mongodb]        openim-mongo          databases=1 tables=[...]
  [sqlite]         veinmap-sqlite        databases=1 tables=[...]
  [xlsx]           tsf-xlsx              databases=1 tables=[3 sheets]
  [xlsx]           tdmq-xlsx             databases=1 tables=[1 sheet]
  [csv]            csv-users             databases=1 tables=['users']
  [csv]            csv-test-data         databases=1 tables=['users', 'products', 'types']
  [tsv]            tsv-test-data         databases=1 tables=['data']
```

## 11.2 全部数据源 Execute 验证

```bash
# SQL 数据库
echo "=== MySQL ===" && dbexplain execute -env --db 1 "SELECT 1" --human
echo "=== PostgreSQL ===" && dbexplain execute -env --db 6 "SELECT 1" --human
echo "=== ClickHouse ===" && dbexplain execute -env --db 2 "SELECT 1" --human
echo "=== SQLite (rules) ===" && dbexplain execute -env --db 3 "SELECT 1" --human
echo "=== SQLite (veinmap) ===" && dbexplain execute -env --db 10 "SELECT 1" --human
echo "=== Elasticsearch ===" && dbexplain execute -env --db 5 "SHOW TABLES" --human

# NoSQL 数据库
echo "=== Redis (openim) ===" && dbexplain execute -env --db 7 "PING" --human
echo "=== Redis (video) ===" && dbexplain execute -env --db 8 "PING" --human
echo "=== MongoDB ===" && dbexplain execute -env --db 9 '{"count":"system.users"}' --human
echo "=== Qdrant ===" && dbexplain execute -env --db 4 '{"count":"runbooks"}' --human

# 文件处理
echo "=== CSV ===" && dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human
echo "=== TSV ===" && dbexplain execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT *" --human

# XLSX 文件
echo "=== XLSX (TSF) ===" && $BIN execute -env --label tsf-xlsx "SELECT * LIMIT 3" --human
echo "=== XLSX (TDMQ) ===" && $BIN execute -env --label tdmq-xlsx "SELECT * LIMIT 3" --human
```

## 11.3 Schema JSON 结构验证

```bash
dbexplain -env --label aiops-mysql --json 2>/dev/null | python3 -c "
import json, sys
d = json.load(sys.stdin)
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
# 预期: All required fields present ✓
```

## 11.4 Execute JSON 结构验证

```bash
dbexplain execute -env --db 1 "SELECT 1 AS n, 'hello' AS s" 2>/dev/null | python3 -c "
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
# 预期: Execute JSON structure OK ✓
```

## 11.5 安全审计日志

```bash
# 检查日志目录
ls -la logs/ 2>/dev/null && echo "Logs directory exists" || echo "No logs directory"
```

## 11.6 端到端耗时统计

```bash
# 全量采集耗时
time dbexplain -env -timeout 60s --json 2>/dev/null > /dev/null
# 预期: 所有数据源在 60s 内完成
```

## 11.7 汇总报告

```bash
# 输出汇总
echo "=== dbexplain v0.0.9 集成测试报告 ==="
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
dbexplain -env -timeout 30s --json 2>/dev/null | python3 -c "
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
dbexplain --version
```
