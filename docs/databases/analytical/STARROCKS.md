# StarRocks 连接器

> StarRocks 是 MySQL 协议兼容的 OLAP 数据库。dbexplain 通过 go-sql-driver/mysql 驱动连接，复用 MySQL 连接器的采集逻辑，追加 StarRocks 特有的 OLAP 元数据采集。

## DSN 格式

```
starrocks://用户:密码@主机:端口/库名?label=别名
sr://用户:密码@主机:端口/库名?label=别名  (简写)
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 端口 | 9030 | FE MySQL 协议端口 |
| label | 自动生成 | 实例别名，用于多实例区分 |
| dbname | (可选) | 不指定时自动 SHOW DATABASES 采集全部库 |

## 示例

```bash
# Schema 采集
dbexplain collect -dsn 'starrocks://root:@127.0.0.1:9030/test_db?label=my-sr' --human

# 只读查询
dbexplain execute -dsn 'starrocks://root:@127.0.0.1:9030/test_db?label=my-sr' \
  --sql 'SELECT * FROM test_table LIMIT 10' --human

# 连通性检查
dbexplain check -dsn 'starrocks://root:@127.0.0.1:9030/information_schema?label=my-sr'

# Docker 端口映射场景 (宿主机 9413 → 容器 9030)
dbexplain collect -dsn 'starrocks://root:@127.0.0.1:9413/test_db?label=my-sr' --human
```

## 采集机制

### 连接层
- **驱动**: `github.com/go-sql-driver/mysql` (与 MySQL 连接器共享)
- **协议**: MySQL wire protocol (FE 端口 9030)
- **连接函数**: 复用 `openMySQL(d)` — 同一 `connector` 包内包级函数

### Schema 采集
| 维度 | 来源 | 说明 |
|------|------|------|
| 库列表 | `SHOW DATABASES` | 跳过 `information_schema`, `_statistics_`, `_sys`, `_independent_stats`, `starrocks` |
| 表元数据 | `information_schema.TABLES` | 名称、引擎(OLAP/MySQL)、行数、体积 |
| 列信息 | `information_schema.COLUMNS` | 名称、类型、默认值、注释、nullable |
| 索引 | `SHOW INDEX FROM table` | 主键、二级索引 |
| OLAP 分区键 | `SHOW CREATE TABLE` 解析 | `PARTITION BY (...)` → `partition_key` |
| OLAP 分布键 | `SHOW CREATE TABLE` 解析 | `DISTRIBUTED BY HASH(...)` → `order_by_key` |
| 采样 | `SELECT * ... LIMIT 1` | 注释推断 (需 `--sample`) |

### 采集函数共享
StarRocks 连接器与 MySQL 连接器的函数共享关系，完全参照 GaussDB ↔ PostgreSQL 的先例：

| 函数 | 定义位置 | StarRocks 使用 |
|------|---------|---------------|
| `openMySQL(d)` | mysql.go | ✅ 直接调用 |
| `collectMySQLDB()` | mysql.go | ✅ 直接调用 |
| `executeSQLQuery()` | query.go | ✅ 直接调用 |
| `buildStarRocksDSN()` | starrocks.go | 独立 (不需要，openMySQL 接受 *DSN) |
| `enrichStarRocksTable()` | starrocks.go | StarRocks 独有 |

## 能力标签

| 能力 | 支持 | 说明 |
|------|------|------|
| CapSQL | ✅ | MySQL 协议 SQL 查询 |
| CapRowCount | ✅ | information_schema.TABLES.TABLE_ROWS |
| CapIndex | ✅ | SHOW INDEX FROM |
| CapPartition | ✅ | PARTITION BY 解析 |
| CapSampling | ✅ | SELECT * LIMIT 1 |
| CapForeignKey | ❌ | OLAP 数据库不强制外键约束 |

## 安全机制

- **只读管道**: 与 MySQL 连接器共享 sqlguard (只读校验 + 多语句检测 + AutoLimit)
- **策略引擎**: DENY_TABLES / DENY_COLUMNS / MASK_COLUMNS 全部生效
- **密码脱敏**: DSN 在日志和输出中自动脱敏 (`{dbpassword}`)
- **超时**: `SET SESSION max_execution_time` 尝试设置但忽略错误 (StarRocks 兼容性)，依赖 context 超时兜底

## 架构一致性

StarRocks 连接器遵循 dbexplain 的协议兼容数据库连接器模式：

```
MySQL 连接器 (mysql.go)        → go-sql-driver/mysql → 注册 "mysql"
StarRocks 连接器 (starrocks.go) → go-sql-driver/mysql → 复用 "mysql" 驱动
                                                         注册 "starrocks" kind
```

完全对应 GaussDB 模式：
```
PostgreSQL 连接器 (postgres.go) → pgx/v5 → 注册 "postgres"
GaussDB 连接器 (gaussdb.go)     → gaussdb-go → 注册 "gaussdb"
```

### Build Tag
```go
//go:build mysql || full
```
StarRocks 随 MySQL tag 编译，无独立 `starrocks` tag。与 GaussDB 随 `postgres` tag 编译的先例一致。

## 已知局限

1. **无外键约束**: OLAP 数据库不强制 FK，`collectMySQLDB()` 的外键查询返回空
2. **分布键存储位置**: `DISTRIBUTED BY HASH(col)` 存入 `order_by_key` 字段 (复用现有 schema 字段，避免新增)
3. **max_execution_time**: StarRocks 可能不支持此 MySQL 系统变量，设置失败时忽略错误
4. **EXPLAIN**: 使用标准 `EXPLAIN <sql>` (与 MySQL 语法相同)
5. **系统库过滤**: StarRocks 系统库 (`_statistics_`, `_sys` 等) 会被自动跳过
