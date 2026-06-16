# Schema 采集性能优化测试 (v0.1.7)

> 验证 PG/MySQL 批量查询、`--no-sample`、`--skip-opstats`、CSV/XLSX 流式读取、`inferRefs` name index 五项优化。

---

## 环境

```bash
cd src
```

---

## 测试项

### 1. 编译验证（全量 + 选择性）

```bash
# 全量编译
go build -tags full ./cmd/dbexplain/
echo $?  # 预期: 0

# 静态分析
go vet ./...
echo $?  # 预期: 0

# 选择性编译（验证 batch query + streaming 代码路径）
bash build.sh minimal postgres,mysql,csv,xlsx 2>&1 | grep "Success"
# 预期: Success: ../release/dbexplain-linux-amd64

# 单元测试
go test -tags full ./... -count=1
# 预期: 全部 ok
```

### 2. --no-sample flag

```bash
# --help 显示
./dbexplain --help 2>&1 | grep no-sample
# 预期: --no-sample  Skip sample row fetching during schema collection

# collect 子命令 help
./dbexplain collect --help 2>&1 | grep no-sample
# 预期: --no-sample  Skip sample row fetching during schema collection

# 带 --no-sample 执行（需可连接数据库）
# ./dbexplain -dsn 'mysql://user:pass@host:3306/db?label=test' --no-sample --human 2>&1
# 预期: 正常采集，日志无 sample row 查询
```

### 3. --skip-opstats flag

```bash
# --help 显示
./dbexplain --help 2>&1 | grep skip-opstats
# 预期: --skip-opstats  Skip MySQL performance_schema op stats

# 带 --skip-opstats 执行（需可连接 MySQL 数据库）
# ./dbexplain -dsn 'mysql://user:pass@host:3306/db?label=test' --skip-opstats --human 2>&1
# 预期: 正常采集，无 performance_schema 查询
```

### 4. CSV 流式读取验证

```bash
# 创建测试 CSV 文件
echo -e "id,name\n1,alice\n2,bob\n3,charlie\n4,dave\n5,eve" > /tmp/test_stream.csv

# LIMIT 查询 — 应使用流式路径
./dbexplain execute -dsn "csv:///tmp/test_stream.csv?label=csvtest" -limit 3 "SELECT * FROM t LIMIT 2"
# 预期: 返回 2 行（id=1,alice; id=2,bob），非调试模式无日志输出

# 比较小文件和流式查询结果一致
./dbexplain execute -dsn "csv:///tmp/test_stream.csv?label=csvtest" -limit 10 "SELECT * FROM t"
# 预期: 返回全部 5 行

rm -f /tmp/test_stream.csv
```

### 5. XLSX 流式读取验证

```bash
# 创建测试 XLSX 文件（需 python3 + openpyxl）
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

# LIMIT 查询 — 应使用流式迭代器路径
./dbexplain execute -dsn "xlsx:///tmp/test_stream.xlsx?label=xlsxtest" -limit 5 "SELECT * FROM Sheet1 LIMIT 3"
# 预期: 返回 3 行

# SQL 聚合查询 — 仍使用全量路径
./dbexplain execute -dsn "xlsx:///tmp/test_stream.xlsx?label=xlsxtest" -limit 5 "SELECT COUNT(*) AS cnt FROM Sheet1"
# 预期: 返回 COUNT = 100

rm -f /tmp/test_stream.xlsx
```

### 6. dbexplain check --env 默认 true

```bash
# 创建临时 .env.dbexplain
echo 'DB1=sqlite:///:memory:?label=test' > /tmp/.env.dbexplain

# 在临时目录下运行 check（无 --env）
cd /tmp && ../src/dbexplain check
# 预期: 自动加载 /tmp/.env.dbexplain，显示 DSN count: 1，Syntax OK ✅

# 验证 --env=false --dsn 显式跳过
cd /tmp && ../src/dbexplain check --env=false -dsn 'sqlite:///:memory:?label=test2'
# 预期: 仅显示 --dsn 指定的 DSN，不加载 .env

rm -f /tmp/.env.dbexplain
```

### 7. inferRefs name index 不退化

```go
// 单元测试自动验证 analyze 包不退化
go test -tags full ./internal/analyze/ -v -count=1
// 预期: PASS (或 [no test files] — analyze 包不含测试，编译即验证)
```

---

## 预期结果

| # | 测试项 | 预期状态 |
|---|--------|---------|
| 1 | 全量编译 + 静态分析 + 单元测试 | 全部通过 |
| 2 | --no-sample flag 定义显示 + context 注入 | 可编译/help 显示 |
| 3 | --skip-opstats flag 定义显示 | 可编译/help 显示 |
| 4 | CSV 流式读取 | 结果正确 |
| 5 | XLSX 流式读取 | 结果正确 |
| 6 | check --env 默认 true 加载 | 自动发现 |
| 7 | inferRefs name index | 编译不退化 |
