# dbexplain — Database Context Compiler

> **[English version →](README_EN.md)**

> **数据库上下文编译器** — 为 AI Agent 与工程团队提供确定性、可证实的数据结构信息。

`dbexplain` 是一个**单二进制、零运行时依赖**的命令行工具，支持 **15 种异构数据源**（含可选 DuckDB）的 Schema 采集与只读查询执行，所有操作在统一的安全沙箱下审计可追溯。

核心理念：**只输出可证实的事实，LLM 在外部消费结构化输出来做推理。**

---

## 架构层次

```
┌──────────────────────────────────────────────────────────────────┐
│                    CLI 命令层                                      │
│  Schema 采集  查询执行  REPL交互  Diff对比  配置管理  参考手册   │
│  collect     execute    repl      diff      list/encrypt  csv/xlsx│
├──────────────────────────────────────────────────────────────────┤
│                    查询执行层                                      │
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────────┐   │
│  │ 直接执行        │  │ DSL 模式       │  │ 联邦查询           │   │
│  │ --db 1 "SELECT" │  │ @label.table   │  │ 跨源 JOIN/UNION   │   │
│  │ --label redis   │  │ 自动解析绑定    │  │ 文件+SQL+PromQL   │   │
│  └────────────────┘  └────────────────┘  └───────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│                    安全防护层                                      │
│  ┌──────────┐    ┌────────────────────────┐    ┌────────────┐    │
│  │ sqlguard │ →  │ 策略引擎               │ →  │ AutoLimit  │    │
│  │ AST校验   │    │ DENY_TABLES/COLUMNS   │    │ LIMIT 1000 │    │
│  │ 只读保障  │    │ MASK_COLUMNS/DENY_SQL │    │ 防全表导出  │    │
│  └──────────┘    └────────────────────────┘    └────────────┘    │
├──────────────────────────────────────────────────────────────────┤
│                  连接器层 (15 种数据源)                            │
│  关系型: MySQL PG GaussDB SQLite DuckDB Oracle                 │
│  分析型: ClickHouse Hive                                        │
│  键值型: Redis                                                    │
│  文档型: MongoDB Elasticsearch                                    │
│  向量型: Qdrant                                                   │
│  文件型: CSV TSV XLSX (内置纯 Go SQL 引擎)                       │
│  时序型: Prometheus (PromQL 即时查询)                             │
├──────────────────────────────────────────────────────────────────┤
│                  Schema / IR 数据层                                │
│  Collect() → schema.Instance → IR → JSON / Human / Diff / Graph │
└──────────────────────────────────────────────────────────────────┘
```

### 层级说明

| 层级 | 职责 | 关键组件 |
|------|------|---------|
| **CLI 命令层** | 用户交互入口，子命令分发 | `cmd/dbexplain/` — `main.go`、`execute.go`、`repl.go`、`encode.go` |
| **查询执行层** | 三路径查询：直接/DSL/联邦 | `executor/`、`dsl/`（DSL 编译器）、`connector/filequery/`（文件 SQL 引擎） |
| **安全防护层** | AST 只读校验 + LIMIT 注入 + 策略拒绝 | `sqlguard/`、`policy/`、`query/`（并发锁） |
| **连接器层** | 15 种数据源统一接口 | `connector/` — 每数据源独立文件，`init()` 自注册到全局 registry |
| **Schema/IR 层** | 采集 → 内部表示 → 输出渲染 | `schema/`、`ir/`、`render/`、`output/`、`graph/`、`diff/` |

![dbexplain Architecture](docs/assets/DBEXPLAIN-ARCH.png)

> 完整模块映射见 [`docs/CODE_MAP.md`](docs/CODE_MAP.md)。

---

## 能力全景映射

> 数据源 × 能力模块矩阵。✅ 支持，— 不适用，⚠️ 有条件支持。

| 类别 | 数据源 | 协议 | Schema采集 | 查询 | REPL交互 | DSL | 亮点 |
|------|--------|------|:----------:|:----:|:--------:|:---:|------|
| **关系型** | MySQL | `mysql://` | ✅ | ✅ SQL | ✅ | ✅ | 外键、索引、字段注释推断 |
| | PostgreSQL | `postgres://` | ✅ | ✅ SQL | ✅ | ✅ | 多 Schema、行数统计、SSL 可配 |
| | GaussDB | `gaussdb://` | ✅ | ✅ SQL | ✅ | ✅ | 独立连接器，Oracle 兼容模式 |
| | SQLite | `sqlite://` | ✅ | ✅ SQL | ✅ | ✅ | 纯 Go 驱动，无 CGO |
| | Oracle | `oracle://` `oracles://` | ✅ | ✅ SQL | ✅ | ✅ | 外键/索引/主键、TLS、12c+ 需 FETCH FIRST |
| **分析型** | ClickHouse | `clickhouse://` | ✅ | ✅ SQL | ✅ | ✅ | 排序键 / 分区键 / 主键 |
| | Hive | `hive://` `hives://` | ✅ | ✅ SQL | ✅ | ✅ | DESCRIBE FORMATTED、Kerberos、TLS、无行数统计 |
| | DuckDB ¹ | `duckdb://` | ✅ | ✅ SQL | ✅ | ✅ | 嵌入式分析引擎，需 `-tags duckdb` 构建 |
| **键值型** | Redis | `redis://` `rediss://` | ✅ | — | ✅ | — | 键模式推断、集群、TTL 风险 |
| **文档型** | MongoDB | `mongodb://` | ✅ | — | ✅ | — | 近似文档数 |
| | Elasticsearch | `elasticsearch://` `elasticsearchs://` | ✅ | ⚠️ SQL+JSON | ✅ | — | 索引映射、TLS、原生 JSON _search |
| **向量型** | Qdrant | `qdrant://` | ✅ | — | ✅ | — | 向量集合元数据 |
| **时序型** | Prometheus ² | `prometheus://` | ✅ | ✅ PromQL | ✅ | ✅ | targets/labels/metrics 元数据 |
| **文件型** | CSV / TSV | `csv://` `tsv://` | ✅ | ✅ SQL ⁵ | ✅ | ✅ | 内置纯 Go SQL 引擎 ³ |
| | Excel | `xlsx://` | ✅ | ✅ SQL ⁵ | ✅ | ✅ | 内置纯 Go SQL 引擎 ³ |

> ¹ DuckDB 为可选构建：标准版(-std)不含 DuckDB，DuckDB 版(-duckdb)全驱动 + DuckDB 需 CGO 环境。<br>
> ² Prometheus 单源 DSL 和跨源联邦均支持：`SELECT * FROM @prom.up WHERE job="prometheus"`。支持 `promql()` 语法嵌入任意 PromQL 表达式：`FROM @prom.promql(rate(cpu[5m]) / rate(mem[5m]) * 100)`。<br>
> ³ CSV/TSV/XLSX 支持完整 SQL 子集（WHERE/GROUP BY/JOIN/窗口函数/UNION）和哈希索引优化。
> ⁵ 文件型数据源通过内置 SQL 引擎执行查询，不走 executor 路径。

![Prometheus DSL vs MySQL 查询映射对照](docs/assets/promtheus-mysql-dsl-1.png)
![Prometheus DSL 查询案例](docs/assets/promtheus-mysql-dsl-2.png)

---

## 核心能力

### Schema 采集
从所有数据源提取表结构、列、索引、外键、行数、分区键、引擎元数据。支持输出格式：

| 格式 | 命令 | 用途 |
|------|------|------|
| **JSON** | `dbexplain --json` | 机器消费的结构化数据 |
| **Human** | `dbexplain --human` | 人类可读渲染结果 |
| **增量缓存** | `dbexplain --cache /tmp/cache.json` | 仅 Schema 变化时重新采集 |
| **AI 上下文** | `dbexplain --context /tmp/ctx.md` | LLM 可直接消费的上下文文件 |
| **采集指标** | `dbexplain --metrics` | Prometheus 文本格式输出到 stderr |

### 只读查询执行 — 三路径架构

| 路径 | 命令 | 说明 |
|------|------|------|
| **直接执行** | `--db 1 "SELECT ..."` / `--label redis "PING"` | SQL 走原生驱动，NoSQL 走原生命令 |
| **DSL 模式** | `--dsl "SELECT * FROM @label.table"` | `@label.table` 引用数据源，自动解析绑定 |
| **联邦查询** | `--dsl "SELECT * FROM @a.t JOIN @b.t ON ..."` | 跨源 JOIN/UNION（SQL+文件+PromQL），filequery 引擎内存合并 |

三条路径共享同一安全管道：

```
sqlguard(AST 只读校验) → 策略引擎(DENY/MASK) → AutoLimit(LIMIT 1000)
```

```bash
# 直接执行
dbexplain execute --db 1 "SELECT COUNT(*) FROM orders"
dbexplain execute --label redis "PING"

# DSL 模式
dbexplain execute --dsl "SELECT * FROM @my-mysql.users LIMIT 10" --human

# 联邦查询
dbexplain execute --dsl "SELECT * FROM @ops-csv.data UNION ALL SELECT * FROM @xlsx.Sheet1" --human
```

### REPL 交互式查询

| 特性 | 说明 |
|------|------|
| **启动** | `dbexplain repl`（从配置加载）或 `--dsn` 直连 |
| **无配置启动** | 空 DSN 进入 `(disconnected)` 状态，`.connect <dsn>` 动态接入 |
| **命令** | `.conn`/`.dsn` 切换数据源、`.list` 列出全部、`.help`/`.exit`/`.quit` |
| **安全** | 同受 sqlguard + 策略引擎保护，写操作全部拒绝 |
| **ES JSON** | 原生 JSON 查询通过 `/_search` 端点执行，列名动态确定 |
| **DSL 模式** | REPL 内支持单源和联邦 DSL 查询（含 Prometheus PromQL） |

### 文件查询引擎
CSV / TSV / XLSX 由**内置纯 Go SQL 引擎**驱动，无需外部数据库：

| 能力 | 说明 |
|------|------|
| **基础** | WHERE / GROUP BY / HAVING / 聚合函数 (COUNT, SUM, AVG, MIN, MAX) |
| **进阶** | Hash JOIN（INNER/LEFT/RIGHT）/ ORDER BY (NULLS FIRST/LAST) / UNION / DISTINCT ON |
| **高级** | 子查询 IN / 窗口函数 (ROW_NUMBER, RANK, LEAD, LAG, NTILE 等) |
| **优化** | 哈希索引 — `WHERE col='literal'` 等值条件 O(1) 查找 |
| **跨格式** | CSV ↔ XLSX 跨格式 JOIN，跨文件 JOIN |

### Schema 变更对比
字段级差异追踪：检测列（新增/删除/类型/可空/默认/注释/主键）、索引、外键三级变更。

```bash
dbexplain diff --cache schema.json --since v1.0 --human
```

### 安全体系：六层防护管道

所有查询通过统一安全管道执行，按数据库类型自动路由到合适的校验路径。

```
                    ┌─ SQL 路径 ─────────────────────────────┐
                    │  sqlguard(AST 只读) → 策略引擎 CheckSQL │
                    │  → AutoLimit(1000)                     │
                    ├─ Native 路径 ──────────────────────────┤
                    │  策略引擎 CheckNative(命令白名单)       │
                    ├─ 文件路径 ─────────────────────────────┤
                    │  策略引擎 DenyTables(文件名校验)        │
                    └────────────────────────────────────────┘
                               ↓
                    并发锁(每标签) → 执行 → ApplyMask / StripDeniedColumns
```

| 层级 | 组件 | SQL 路径 | Native 路径 | 文件路径 |
|:----:|------|:--------:|:-----------:|:--------:|
| L1 | **sqlguard** — AST 只读校验（8 读/17 写动词） | ✅ | — | — |
| L2 | **AutoLimit** — 自动注入 LIMIT 1000 | ✅ | — | — |
| L3 | **策略引擎** — DENY_TABLES/COLUMNS/STATEMENTS | ✅ CheckSQL | ✅ CheckNative | ✅ DenyTables |
| L4 | **并发锁** — 每标签 QueryLock | ✅ | ✅ | — ⁴ |
| L5 | **ApplyMask** — 列值掩码（后执行） | ✅ | ✅ | ✅ |
| L6 | **StripDeniedColumns** — 列剥离（后执行） | ✅ | ✅ | ✅ |

#### 每数据库类型安全覆盖

| 类别 | 数据源 | 查询路径 | L1 sqlguard | L2 AutoLimit | L3 策略 | L4 Lock | L5 Mask | L6 Strip | 额外防护 |
|------|--------|---------|:-----------:|:------------:|:-------:|:-------:|:-------:|:--------:|----------|
| **关系型** | MySQL | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | PostgreSQL | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | GaussDB | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | SQLite | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | Oracle | executor.IsSQL=true | ✅ | ✅ ¹ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| **分析型** | ClickHouse | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | Hive | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | DuckDB ² | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard + 文件访问校验 |
| **键值型** | Redis | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | 42 命令白名单 |
| **文档型** | MongoDB | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | find/aggregate 白名单 |
| | Elasticsearch | executor.IsSQL ³ | ⚠️ SQL 查询 | ⚠️ | ✅ | ✅ | ✅ | ✅ | _search 端点 |
| **向量型** | Qdrant | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | scroll/count 白名单 |
| **时序型** | Prometheus | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | PromQL 只读 API |
| **文件型** | CSV / TSV | HandleFileExecute ⁴ | — | — | ✅ DenyTables | — | ✅ | ✅ | 文件只读 |
| | Excel | HandleFileExecute ⁴ | — | — | ✅ DenyTables | — | ✅ | ✅ | 文件只读 |

> ¹ Oracle AutoLimit: `LIMIT N` 自动转换为 `FETCH FIRST N ROWS ONLY`（Oracle 12c+）。
> ² DuckDB 额外文件访问校验：`read_parquet`/`read_csv`/`read_json` 函数受 `allowed_path` 参数限制。
> ³ ES 双模式：SQL 查询走 IsSQL=true（完整管道），JSON 原生查询走 IsSQL=false（无 sqlguard）。
> ⁴ 文件路径由 `queryutil.HandleFileExecute` 处理，绕过 executor 但保留策略引擎防护。L4 并发锁在文件路径中不适用（文件查询是单线程全内存操作）。

非 SQL 数据库拥有各自的命令白名单或原生查询校验器。密码在所有输出和日志中自动脱敏。

---

## 二进制变体

| 变体 | 构建命令 | CGO | 原始体积 | UPX 后 |
|------|---------|:---:|:--------:|:------:|
| **标准版 (-std)** | `bash build.sh prod`（默认） | ❌ 关闭 | 58 MB | 11 MB (81%) |
| **DuckDB 版 (-duckdb)** | `bash build.sh minimal duckdb,...`¹ | ✅ 开启 | 141 MB | 53 MB (62%) |

> ¹ DuckDB 版完整标签：`duckdb,mysql,postgres,sqlite,clickhouse,redis,mongodb,elasticsearch,qdrant,csv,xlsx,prometheus`。
>
> **启动速度（冷启动）**: UPX 版每次调用时增加约 435ms 的自解压开销（可执行文件在运行任何应用代码前需先将自身解压到内存），noUPX 版约 3ms。解压完成后运行时性能**完全一致**。
> **发布**: `bash release.sh` 零参数一键产出全部变体——5 平台 -std + 2 平台 -duckdb，每变体含 UPX/noUPX 双版本，共 12 个 tarball。

---

## CLI 常用参数

| 场景 | 命令 |
|------|------|
| Schema 采集 | `dbexplain / -dsn <url> / --json / --human / -o <file>` |
| 查询执行 | `dbexplain execute --db <N> / --label <name> / --dsl / --human` |
| REPL 交互 | `dbexplain repl` / `dbexplain repl --dsn <url>` / `.connect <dsn>` |
| 联邦查询 | `dbexplain execute --dsl "SELECT * FROM @a.t JOIN @b.t ON ..." --human` |
| 文件查询 | `dbexplain execute -dsn "csv://file.csv" "SELECT ..."` |
| Schema 对比 | `dbexplain diff --cache <file> --since <ver>` |
| 查看 DSN | `dbexplain list`（自动加载配置） |
| 采集指标 | `dbexplain --metrics`（Prometheus 格式到 stderr） |
| 加密配置 | `dbexplain encrypt`（自动搜索 .env.dbexplain，输出 .enc） |
| 参考手册 | `dbexplain mysql` / `dbexplain oracle` / `dbexplain hive` / `dbexplain all` |

---

## 快速开始

```bash
# 1. 构建（单平台全驱动，快速）
cd src && bash build.sh dev

# 或全平台发布版（5 GOOS/GOARCH + UPX 压缩）
cd src && bash build.sh

# 2. 配置（自动搜索 6 级路径）
mkdir -p ~/.config/dbexplain && cat > ~/.config/dbexplain/.env.dbexplain << EOF
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
EOF

# 3. 验证
./release/dbexplain                          # Schema 采集
./release/dbexplain execute --db 1 "SELECT 1" --human  # 查询
./release/dbexplain --version                     # 版本
```

---

## 文档导航

| 场景 | 文档 |
|------|------|
| 傻瓜用法手册（5 分钟上手） | [`docs/USAGE_GUIDE.md`](docs/USAGE_GUIDE.md) |
| 查询案例（20+ 示例，含 REPL/DSL/联邦） | [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) |
| 部署安装（源码/二进制/Skill） | [`docs/DEPLOY.md`](docs/DEPLOY.md) |
| 排障指南 | [`dbexplain-skill/references/troubleshooting.md`](dbexplain-skill/references/troubleshooting.md) |
| 安全策略配置 | [`docs/POLICY.md`](docs/POLICY.md) |
| 配置文件搜索规则 | [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md) |
| SQL 语法参考（文件查询引擎） | [`dbexplain-skill/references/sql-syntax.md`](dbexplain-skill/references/sql-syntax.md) |
| 代码模块映射 | [`docs/CODE_MAP.md`](docs/CODE_MAP.md) |
| 数据库使用手册（每数据源独立文档） | [`docs/databases/`](docs/databases/) |
| 测试报告（166+ 项） | [`docs/test/`](docs/test/) |

---

## 开发

```bash
cd src
go build ./...                              # 编译检查
go vet ./...                                # 静态分析
go test ./... -count=1                      # 单元测试
bash build.sh                               # 发布：5 平台 + 全驱动 + UPX
bash build.sh dev                           # 开发：当前平台 + 全驱动
bash release.sh                             # 正式发布：标准版 + DuckDB 版
bash build.sh minimal mysql,postgres        # 精简：按需驱动
bash build.sh --help                        # 查看全部参数
```

> **命名规范**: 标准版（纯 Go，无 DuckDB）后缀 `-std`；DuckDB 版（全驱动 + DuckDB）后缀 `-duckdb`。

---

## License

Apache 2.0 © 2026 WWT
