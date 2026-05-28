# L10: 回归测试

> 验证从 v0.0.4 到 v0.0.9 的所有历史版本功能在 v0.0.9 中仍然正常工作。

## 10.1 DSN 解析回归 (v0.0.4)

### MySQL DSN

```bash
go run . -dsn "mysql://user:pass@localhost:3306/db?label=test"
# 预期: 正确解析 scheme、user、password、host、port、db、label
```

### PostgreSQL DSN

```bash
go run . -dsn "postgres://user:pass@localhost:5432/db?label=test&sslmode=require"
# 预期: 正确解析 SSL 模式参数
```

### SQLite DSN

```bash
go run . -dsn "sqlite:///tmp/test.db?label=test"
# 预期: 正确解析文件路径
```

### Redis DSN

```bash
go run . -dsn "redis://:pass@localhost:6379/0?label=test&cluster=true"
# 预期: 正确解析 cluster 参数
```

### MongoDB DSN

```bash
go run . -dsn "mongodb://user:pass@localhost:27017/db?authSource=admin&label=test"
# 预期: 正确解析 MongoDB 参数
```

### Elasticsearch DSN

```bash
go run . -dsn "elasticsearch://user:pass@localhost:9200?label=test&tls=true&tls-skip-verify=true"
# 预期: 正确解析 TLS 参数
```

### CSV DSN (v0.0.9 新增)

```bash
go run . -dsn "csv:///tmp/data.csv?label=test&encoding=gbk&delimiter=semicolon"
# 预期: 正确解析 encoding 和 delimiter 参数
```

## 10.2 加密功能回归 (v0.0.6)

```bash
# 加密测试文件
echo "DB1=mysql://root:pass@localhost:3306/test?label=test" > /tmp/test.env
go run . encrypt /tmp/test.env 2>&1
# 预期: 提示加密成功或选择加密模式
```

## 10.3 配置文件搜索回归 (v0.0.5)

```bash
# 验证 -env 从 src/.env 加载
go run . list -env 2>&1 | head -5
# 预期: 正确加载并显示 15 个 DSN
```

## 10.4 Schema 采集回归 (v0.0.4)

```bash
# 验证多数据源采集
go run . -env --label aiops-mysql --json 2>/dev/null | python3 -c "
import json,sys
d = json.load(sys.stdin)
assert d['kind'] == 'mysql', f'kind mismatch: {d[\"kind\"]}'
assert d['label'] == 'aiops-mysql', f'label mismatch'
assert len(d.get('databases',[])) > 0, 'no databases'
print(f'OK: kind={d[\"kind\"]}, label={d[\"label\"]}, databases={len(d[\"databases\"])}')
"
# 预期: OK: kind=mysql, label=aiops-mysql, databases=N
```

## 10.5 输出格式回归 (v0.0.4)

### --human 输出

```bash
go run . -env --label aiops-mysql --human 2>&1 | head -20
# 预期: 人类可读的格式化输出
```

### -o 输出到文件

```bash
go run . -env --label aiops-mysql -o /tmp/dbexplain-test-output.txt
cat /tmp/dbexplain-test-output.txt | head -10
# 预期: 文件内容与终端输出一致
```

## 10.6 并发采集回归 (v0.0.2)

```bash
go run . -env -timeout 30s --conn 5 --json 2>/dev/null | python3 -c "
import json,sys
d = json.load(sys.stdin)
if isinstance(d, list):
    print(f'Collected {len(d)} data sources')
else:
    print(f'Collected 1 data source')
"
# 预期: 全部 15 个数据源采集成功
```

## 10.7 Execute 回归 (v0.0.7)

### SQL 执行

```bash
go run . execute -env --db 1 "SELECT 1+1 AS result" --human
# 预期: result = 2
```

### NoSQL 执行

```bash
go run . execute -env --db 7 "PING" --human
# 预期: PONG
```

### 文件执行 (v0.0.9 新增)

```bash
go run . execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human
# 预期: 显示 3 行数据
```

### JSON 输出

```bash
go run . execute -env --db 1 "SELECT 1 AS n" | python3 -c "
import json,sys
d = json.load(sys.stdin)
assert d['row_count'] == 1
assert d['columns'][0]['name'] == 'n'
assert d['rows'][0][0] == 1
print('Execute JSON output OK')
"
# 预期: Execute JSON output OK
```

## 10.8 安全功能回归 (v0.0.8)

### sqlguard

```bash
go run . execute -env --db 1 "DROP TABLE test" 2>&1
# 预期: READ_ONLY_VIOLATION
```

### 策略引擎

```bash
# 测试 DENY_STATEMENTS（需在 .env 中配置）
go run . execute -env --db 7 "FLUSHALL" 2>&1
# 预期: READ_ONLY_VIOLATION 或 POLICY_VIOLATION
```

## 10.9 版本一致性

```bash
# 检查非历史文档中的版本号一致性
grep -rn "v0\.0\.8" --include="*.sh" --include="*.ps1" --include="*.go" .
# 预期: 无 v0.0.8 出现在非历史文档中（CHANGELOG 和 MEMORY.md 除外）
```

## 10.10 编译完整性

```bash
HTTPS_PROXY=http://127.0.0.1:7897/ go build ./...
HTTPS_PROXY=http://127.0.0.1:7897/ go vet ./...
go test ./... -count=1 2>&1 | tail -5
# 预期: 全部通过
```
