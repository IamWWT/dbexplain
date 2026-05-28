# dbexplain v0.0.9 测试文档

## 测试总览

本目录包含 dbexplain v0.0.9 的全量分层测试文档，覆盖全部 10 种数据源类型和所有历史版本功能。

## 测试层级

| 层级 | 文件 | 覆盖范围 |
|------|------|---------|
| L1 环境验证 | `01-environment.md` | Go 编译、vet、单元测试、交叉编译 |
| L2 Schema 采集 | `02-schema-collection.md` | 全部 10 种数据源的 Schema 采集 |
| L3 SQL 执行 | `03-execute-sql.md` | MySQL/PostgreSQL/ClickHouse/SQLite/Elasticsearch |
| L4 NoSQL 执行 | `04-execute-nosql.md` | Redis/MongoDB/Qdrant |
| L5 文件处理 | `05-file-processing.md` | CSV/TSV/XLSX |
| L6 安全沙箱 | `06-security-sqlguard.md` | sqlguard 只读校验 |
| L7 策略引擎 | `07-policy-engine.md` | DENY_TABLES/COLUMNS/STATEMENTS/MASK |
| L8 并发限制 | `08-concurrent-limit.md` | --conn 并发控制 |
| L9 CLI 帮助 | `09-cli-help.md` | 所有子命令、手册、--version |
| L10 回归测试 | `10-regression.md` | 历史版本功能回归 |
| L11 全量集成 | `11-end-to-end.md` | 一次运行所有数据源 |

## 前置条件

1. Go 1.26+
2. 配置 `.env` 文件（`src/.env`，包含 15 个 DSN 条目）
3. 所有数据库服务正常运行
4. 测试 CSV 文件准备（`/tmp/dbexplain-test/` 目录）

### 准备 CSV/TSV 测试数据

```bash
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

### 测试二进制

```bash
cd src

# 一键构建（所有数据库类型 + xlsx 支持）
bash build.sh

# 测试用二进制路径
BIN="../release/dbexplain-linux-amd64"
# 或使用 go run
BIN="go run ."
```

## 配置文件 (.env)

文件位置: `src/.env`

```
DB1=mysql://root:root@123456@localhost:9433/testdb?label=aiops-mysql
DB2=clickhouse://default:ClickHouse@2026!@localhost:9421?label=aiops-clickhouse
DB3=sqlite:///home/wwt/Downloads/aigc/proj/agents/aiops/intent-apparatus/data/rules.db?label=intentapparatus-sqlite
DB4=qdrant://:Qdrant@2026!@localhost:9426?label=aiops-qdrant
DB5=elasticsearch://elastic:Es@Pass2026!@localhost:9422?label=aiops-es
DB6=postgres://videomon_user:VideoMon@2026!Secure@localhost:5432/videomon?label=video-pg
DB7=redis://default:Pwd1Open2%23IMD@localhost:6389/0?label=openim-redis
DB8=redis://:Redis@2026!Secure@localhost:6379/0?label=video-redis
DB9=mongodb://openIM:Pwd1Open2%23IMD@192.168.0.127:27017/openim_v3?authSource=openim_v3&directConnection=true&label=openim-mongo
DB10=sqlite:///home/wwt/Downloads/aigc/proj/agents/aiops/veinmap/data/veinmap.db/?label=veinmap-sqlite
DB11=xlsx:///home/wwt/Documents/aigc/nmpaas/TSF/TSF模块进程管理&日志信息.xlsx?label=tsf-xlsx
DB12=xlsx:///home/wwt/Documents/aigc/nmpaas/TDMQ/消息队列 TDMQ V1.5 日常巡检说明 01 .xlsx?label=tdmq-xlsx
DB13=csv:///tmp/dbexplain-test/users.csv?label=csv-users
DB14=csv:///tmp/dbexplain-test/?label=csv-test-data
DB15=tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test-data
```

## DSN 速查表

| 编号 | 标签 | 类型 | 说明 | 采集结果 |
|------|------|------|------|---------|
| DB1 | aiops-mysql | MySQL | testdb | 2 tables (iplist 12 rows, port 30 rows) |
| DB2 | aiops-clickhouse | ClickHouse | default | 2 databases |
| DB3 | intentapparatus-sqlite | SQLite | rules.db | 5+ tables |
| DB4 | aiops-qdrant | Qdrant | 向量集合 | 2 collections (mcp_tools, runbooks 480 pts) |
| DB5 | aiops-es | Elasticsearch | 索引映射 | 17 索引/视图 |
| DB6 | video-pg | PostgreSQL | videomon | 5+ tables |
| DB7 | openim-redis | Redis | openim | key 模式推断 |
| DB8 | video-redis | Redis | video/cache | 1 table (_server_info) |
| DB9 | openim-mongo | MongoDB | openim_v3 | 5+ collections |
| DB10 | veinmap-sqlite | SQLite | veinmap.db | 4 tables |
| DB11 | tsf-xlsx | XLSX | TSF 巡检 | 3 sheets (45+14+6 rows) |
| DB12 | tdmq-xlsx | XLSX | TDMQ 巡检 | 1 sheet |
| DB13 | csv-users | CSV | users.csv | 1 table (3 rows) |
| DB14 | csv-test-data | CSV | 目录扫描 | 3 tables |
| DB15 | tsv-test-data | TSV | data.tsv | 1 table (2 rows) |

## 注意事项

- GaussDB 不在 .env 中，但代码完全支持；如需测试请单独通过 `-dsn` 传入
- 代理环境: 所有 `go mod tidy` 命令需加 `HTTPS_PROXY=http://127.0.0.1:7897/`
- 所有测试命令在 `src/` 目录下执行（`cd src`）
