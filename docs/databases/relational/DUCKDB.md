# DuckDB 连接器使用手册

> DuckDB 连接器是**可选连接器**，需使用 `-tags duckdb` 显式编译。不在 `full` 标签中。

## 一、前置条件

### 构建要求

DuckDB 需要 CGO 和 C 工具链：

| 平台 | 工具链 |
|------|--------|
| Linux | `gcc` 或 `clang` |
| macOS | Xcode Command Line Tools（`xcode-select --install`） |
| Windows | MinGW-w64（安装 `mingw-w64`） |

### 构建命令

```bash
# 仅 DuckDB
cd src && CGO_ENABLED=1 go build -tags duckdb ./cmd/dbexplain

# DuckDB + MySQL + PostgreSQL
cd src && bash build.sh minimal duckdb,mysql,postgres
```

> 注意：带 DuckDB 的构建是**当前平台原生编译**，不支持交叉编译。产物为单文件、零运行时依赖（CGO 静态编译）。

## 二、DSN 格式

```
duckdb://:memory:?label=名字          # 内存数据库
duckdb:///绝对/路径/数据库.db?label=名字  # 文件数据库
```

### 参数

| 参数 | 必需 | 说明 | 示例 |
|------|------|------|------|
| `label` | 推荐 | 别名，用于 CLI 引用 | `?label=my-duckdb` |
| `allowed_path` | 文件分析时需要 | 控制 `read_parquet`/`read_csv` 等文件函数可读取的目录 | `?allowed_path=/data/parquet/` |

### 示例

```
# 内存模式
duckdb://:memory:?label=analysis

# 文件数据库
duckdb:///home/user/analytics.db?label=warehouse

# 带 Parquet 文件访问权限
duckdb://:memory:?label=file-analysis&allowed_path=/data/parquet/

# 多路径（逗号分隔）
duckdb://:memory:?label=all-data&allowed_path=/data/csv/,/data/parquet/
```

## 三、Schema 采集

DuckDB 连接器通过以下方式采集表结构：

1. **表枚举** — `information_schema.tables` 过滤系统 schema
2. **列信息** — `pragma_table_info()` 获取名称、类型、是否可空、默认值、主键
3. **约束** — `duckdb_constraints()` 获取主键、外键、唯一约束
4. **行数** — `SELECT COUNT(*)` 精确计数
5. **采样** — `LIMIT 1` 采样一行数据用于列注释推断

采集结果示例：

```bash
dbexplain collect --dsn "duckdb:///tmp/analytics.db?label=my-duckdb" --human
```

## 四、查询执行

### SQL 查询

```bash
# 标准 SQL 查询
dbexplain execute --dsn "duckdb:///tmp/analytics.db?label=my-duckdb" \
  "SELECT * FROM users LIMIT 5" --human

# EXPLAIN 查询计划
dbexplain execute --dsn "duckdb:///tmp/analytics.db?label=my-duckdb" \
  "EXPLAIN SELECT * FROM users JOIN orders ON users.id = orders.user_id" --human
```

### DSL 模式

```bash
# 通过 label 引用
dbexplain execute --dsl "SELECT * FROM @my-duckdb.users LIMIT 5" --human

# 联邦查询：DuckDB JOIN PostgreSQL
dbexplain execute --dsl \
  "SELECT u.name, o.amount
   FROM @my-duckdb.users u
   JOIN @my-pg.orders o ON u.id = o.user_id
   WHERE o.amount > 100" --human
```

### 文件分析

通过 DuckDB 的文件读取函数进行 Parquet/CSV/JSON 文件查询。需要 `allowed_path` DSN 参数。

```bash
# Parquet 文件分析
dbexplain execute --dsn "duckdb://:memory:?label=analysis&allowed_path=/data/" \
  "SELECT region, SUM(amount) FROM read_parquet('/data/sales.parquet') GROUP BY region" --human

# CSV 文件分析
dbexplain execute --dsn "duckdb://:memory:?label=analysis&allowed_path=/data/" \
  "SELECT * FROM read_csv_auto('/data/report.csv') WHERE date > '2025-01-01'" --human

# JSON 文件分析
dbexplain execute --dsn "duckdb://:memory:?label=analysis&allowed_path=/data/" \
  "SELECT * FROM read_json('/data/events.json')" --human
```

## 五、安全机制

### 文件访问控制

`read_parquet()`、`read_csv_auto()`、`read_csv()`、`read_json()` 等文件读取函数受到 `allowed_path` DSN 参数的限制：

- **`allowed_path` 未设置**：任何文件读取函数调用都被拒绝
- **`allowed_path` 已设置**：文件路径必须在 `allowed_path` 前缀下，路径规范化后验证
- 多路径支持逗号分隔

安全验证由 Go 层的 `validateFileAccess()` 函数执行，是增强性安全措施。最终的安全保障来自数据库层的权限控制。

### 只读安全

- DuckDB 连接器通过 sqlguard 管道执行查询，只允许 SELECT/EXPLAIN/DESCRIBE/SHOW 等只读操作
- 连接字符串使用 DuckDB 默认读写模式（`:memory:` 模式不支持 READ_ONLY）

## 六、限制说明

- **DuckDB 不在 `full` 标签中** — 需通过 `-tags duckdb` 显式编译
- **CGO 必需** — 内嵌 DuckDB C++ 引擎，不支持纯 Go 交叉编译
- **文件分析是附加能力** — 不替代内置 filequery 引擎，两者独立
- **云存储不支持** — S3/GCS/OSS 访问需要额外安装 `httpfs` 扩展，不在当前范围
- **数据源特有能力** — 不采集分区信息、文件大小等 DuckDB 特有元数据
