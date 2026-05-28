# L5: 文件处理测试

> 验证 CSV/TSV/XLSX 文件的 Schema 采集与查询执行。

## 前置条件

```bash
cd src
# 准备 CSV 测试数据
mkdir -p /tmp/dbexplain-test
echo 'name,age,city' > /tmp/dbexplain-test/users.csv
echo 'Alice,30,Beijing' >> /tmp/dbexplain-test/users.csv
echo 'Bob,25,Shanghai' >> /tmp/dbexplain-test/users.csv
echo 'Charlie,35,Guangzhou' >> /tmp/dbexplain-test/users.csv

echo 'id,product,price' > /tmp/dbexplain-test/products.csv
echo '1,Widget A,9.99' >> /tmp/dbexplain-test/products.csv
echo '2,Widget B,19.99' >> /tmp/dbexplain-test/products.csv

printf 'name\tage\tcity\nAlice\t30\tBeijing\nBob\t25\tShanghai\n' > /tmp/dbexplain-test/data.tsv

echo 'int_col,float_col,date_col,text_col' > /tmp/dbexplain-test/types.csv
echo '1,3.14,2024-01-01,hello' >> /tmp/dbexplain-test/types.csv
echo '2,2.718,2024-02-15,world' >> /tmp/dbexplain-test/types.csv

# xlsx 所有二进制均包含（无需特殊构建）
# export BIN=../release/dbexplain-linux-amd64
# 或使用 go run .
export BIN="go run ."
```

## 5.1 CSV 单文件 Schema 采集

```bash
dbexplain -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" --human
```

实际结果: 采集结果包含表名 "users"，列名 name/age/city，类型推断 TEXT/INTEGER/TEXT。

## 5.2 CSV 单文件查询执行

### 全部行

```bash
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human
```

实际结果: 3 行数据（Alice, Bob, Charlie）。

### LIMIT

```bash
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT * LIMIT 2" --human
# 实际: 仅显示前 2 行
```

### LIMIT + OFFSET

```bash
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT * LIMIT 1 OFFSET 1" --human
# 实际: 跳过第 1 行，显示第 2 行（Bob）
```

### JSON 输出

```bash
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *"
# 预期: JSON 格式的查询结果
```

## 5.3 CSV 目录采集

```bash
dbexplain -dsn "csv:///tmp/dbexplain-test/?label=csv-dir" --human
# 实际: 同时采集 users、products 两张表
```

## 5.4 CSV Glob 通配符

```bash
dbexplain -dsn "csv:///tmp/dbexplain-test/*.csv?label=csv-glob" --human
# 实际: 匹配全部 .csv 文件作为表（users, products, types）
```

## 5.5 TSV 文件

### Schema 采集

```bash
dbexplain -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" --human
# 实际: 正确解析制表符分隔的文件
```

### 查询执行

```bash
dbexplain execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT *" --human
# 实际: 显示 2 行数据（Alice, Bob）
```

## 5.6 GBK 编码文件（如有）

```bash
# 如果存在 GBK 编码的文件
dbexplain -dsn "csv:///path/to/gbk-file.csv?label=gbk-test&encoding=gbk" --human
# 预期: 正确显示中文字段名和数据
```

## 5.7 类型推断验证

```bash
dbexplain -dsn "csv:///tmp/dbexplain-test/types.csv?label=types-test" --human
```

实际推断结果:
| 列名 | 类型 |
|------|------|
| int_col | INTEGER |
| float_col | FLOAT |
| date_col | DATE |
| text_col | TEXT |

## 5.8 XLSX Schema 采集 (DB11 - TSF)

```bash
# XLSX Schema 采集 (DB11 - TSF)
$BIN -env --label tsf-xlsx --human
# 实际: 3 sheets (45+14+6 rows)，含中文列名
```

## 5.9 XLSX 查询执行 (DB11)

```bash
$BIN execute -env --label tsf-xlsx "SELECT * LIMIT 5" --human
# 实际: 显示前 5 行数据，列名为原 Excel 中文表头（如"进程名称"、"模块名称"等）
```

## 5.10 XLSX 多 Sheet (DB12 - TDMQ)

```bash
$BIN -env --label tdmq-xlsx --human
# 实际: 1 sheet
```

```bash
$BIN execute -env --label tdmq-xlsx "SELECT * LIMIT 3" --human
# 实际: 返回第一个 Sheet 的前 3 行
```

## 5.11 查询限制验证

### 不支持 WHERE

```bash
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT * WHERE age > 30" 2>&1
# 预期: 错误提示（不支持 WHERE）
```

### 不支持列选择

```bash
dbexplain execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT name FROM users" 2>&1
# 预期: 错误提示（仅支持 SELECT *）
```

## 5.12 边界条件

### 空文件

```bash
touch /tmp/dbexplain-test/empty.csv
dbexplain -dsn "csv:///tmp/dbexplain-test/empty.csv?label=empty" --human
# 预期: 可采集到空表结构（仅列头）
```

### 无头文件

```bash
echo 'val1,val2,val3' > /tmp/dbexplain-test/noheader.csv
echo 'a,b,c' >> /tmp/dbexplain-test/noheader.csv
dbexplain -dsn "csv:///tmp/dbexplain-test/noheader.csv?label=noheader" --human
# 预期: 首行仍然作列名处理
```

### 混合类型列

```bash
echo 'mixed' > /tmp/dbexplain-test/mixed.csv
echo 'hello' >> /tmp/dbexplain-test/mixed.csv
echo '42' >> /tmp/dbexplain-test/mixed.csv
echo '2024-01-01' >> /tmp/dbexplain-test/mixed.csv
dbexplain -dsn "csv:///tmp/dbexplain-test/mixed.csv?label=mixed" --human
# 预期: 因类型不一致推断为 TEXT
```
