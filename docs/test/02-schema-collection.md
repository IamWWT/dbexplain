# L2: Schema 采集测试

> 验证全部 10 种数据源 + CSV/TSV/XLSX 的 Schema 采集功能，使用 `-env` 加载 `.env` 文件（共 15 个 DSN）。

## 前置条件

```bash
cd src
# 确保 .env 文件存在（含 DB1-DB15）
ls -la .env
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
```

## 2.1 全部数据源采集

```bash
# 采集所有已配置的数据源（超时 30s）
dbexplain -env -timeout 30s --json
# 预期: 返回 JSON 格式的 Schema 实例列表，包含 DB1-DB15
# 验证点: 每个 DSN 应有 databases[].tables[] 数组
# 验证点: DB11-DB12 (xlsx) 应显示 Sheet 作为表
# 验证点: DB13-DB15 (csv/tsv) 应显示文件内容作为表
```

实际结果: 15 个 DSN 全部采集成功。

## 2.2 按标签包含筛选

```bash
dbexplain -env --include "aiops-mysql,aiops-es" --json
# 预期: 仅返回 MySQL 和 Elasticsearch 两个数据源
```

注意: `--include` 按 label 匹配（逗号分隔），`--exclude` 同理。

## 2.3 按排除筛选

```bash
dbexplain -env --exclude "redis" --json
# 预期: 不含 Redis 类型的数据源
```

## 2.4 逐数据源采集验证

### MySQL (DB1 - aiops-mysql)

```bash
dbexplain -env -timeout 10s --label aiops-mysql --human
# 预期: 显示 testdb 的表结构、列名、类型、外键
# 实际: 2 tables (iplist 12 rows, port 30 rows)
```

### ClickHouse (DB2 - aiops-clickhouse)

```bash
dbexplain -env -timeout 10s --label aiops-clickhouse --human
# 预期: 显示 ClickHouse 数据库、排序键/分区键/主键信息
# 实际: 2 databases, 含系统库和用户库
```

### SQLite (DB3 - intentapparatus-sqlite / rules.db)

```bash
dbexplain -env -timeout 10s --label intentapparatus-sqlite --human
# 预期: 显示 rules.db 的表结构和行数
# 实际: 5+ tables
```

### Qdrant (DB4 - aiops-qdrant)

```bash
dbexplain -env -timeout 10s --label aiops-qdrant --human
# 预期: 显示 Qdrant 向量集合元数据（维度、距离类型等）
# 实际: 2 collections (mcp_tools, runbooks 480 pts)
```

### Elasticsearch (DB5 - aiops-es)

```bash
dbexplain -env -timeout 10s --label aiops-es --human
# 预期: 显示 ES 索引映射和字段类型
# 实际: 17 索引/视图
```

### PostgreSQL (DB6 - video-pg)

```bash
dbexplain -env -timeout 10s --label video-pg --human
# 预期: 显示 videomon 数据库的多 Schema 信息
# 实际: 5+ tables
```

### Redis (DB7 - openim-redis)

```bash
dbexplain -env -timeout 10s --label openim-redis --human
# 预期: 显示 Redis key 模式推断和集群/风险诊断
```

### Redis (DB8 - video-redis)

```bash
dbexplain -env -timeout 10s --label video-redis --human
# 预期: 显示 video Redis 的 key 分析
# 实际: 1 table (_server_info)
```

### MongoDB (DB9 - openim-mongo)

```bash
dbexplain -env -timeout 10s --label openim-mongo --human
# 预期: 显示 MongoDB 集合名称和近似文档数
# 实际: 5+ collections
```

### SQLite (DB10 - veinmap-sqlite)

```bash
dbexplain -env -timeout 10s --label veinmap-sqlite --human
# 预期: 显示 veinmap.db 的表结构
# 实际: 4 tables
```

### XLSX (DB11 - tsf-xlsx)

```bash
dbexplain -env -timeout 10s --label tsf-xlsx --human
# 预期: 显示 Excel 文件的 Sheet 列表和列信息
# 实际: 3 sheets (45+14+6 rows)
```

### XLSX (DB12 - tdmq-xlsx)

```bash
dbexplain -env -timeout 10s --label tdmq-xlsx --human
# 预期: 显示 TDMQ Excel 的 Sheet 内容
# 实际: 1 sheet
```

### CSV 单文件 (DB13 - csv-users)

```bash
dbexplain -env -timeout 10s --label csv-users --human
# 预期: 显示 users.csv 的表结构，列名 name/age/city
# 实际: 1 table (users, 3 rows)
```

### CSV 目录 (DB14 - csv-test-data)

```bash
dbexplain -env -timeout 10s --label csv-test-data --human
# 预期: 同时采集目录下所有 .csv 文件
# 实际: 3 tables (users, products, types)
```

### TSV (DB15 - tsv-test-data)

```bash
dbexplain -env -timeout 10s --label tsv-test-data --human
# 预期: 正确解析制表符分隔的文件
# 实际: 1 table (data, 2 rows)
```

## 2.5 JSON 输出验证

```bash
dbexplain -env -timeout 10s --label aiops-mysql --json 2>/dev/null | python3 -m json.tool --no-ensure-ascii | head -30
# 预期: 合法的 JSON 输出，包含 kind、label、databases、tables 等字段
```

## 2.6 输出到文件

```bash
dbexplain -env -timeout 30s -o /tmp/dbexplain-output.txt
# 预期: 文件内容与终端输出一致
```

## 2.7 并发控制

```bash
dbexplain -env -timeout 30s --conn 1 --json
# 预期: 串行采集（--conn 1），全部完成无错误
```

## 2.8 GaussDB 测试（如可用）

GaussDB 未在 `.env` 中配置。如需测试，可通过 `-dsn` 直接传入：

```bash
dbexplain -timeout 10s -dsn 'gaussdb://user:pass@host:port/db?label=gauss-test' --human
# 预期: 显示 GaussDB 的 Schema 信息（需替换实际连接串）
```
