# dbexplain — Database Context Compiler

> **[English version →](README_EN.md)**

> **数据库上下文编译器** — 为 AI Agent 与工程团队提供确定性、可证实的数据结构信息。

`dbexplain` 是一个**单二进制、零运行时依赖**的命令行工具，支持 **11 种异构数据源**的 Schema 采集与只读查询执行，所有操作在统一的安全沙箱下审计可追溯。

核心理念：**只输出可证实的事实，LLM 在外部消费结构化输出来做推理。**

---

## 为什么用 dbexplain？

- **异构统一** — 一套工具管理 MySQL / PG / Redis / ES / Mongo / 文件 等 11 种数据源
- **确定性优先** — 相同输入 → 相同输出，无 AI 幻觉，所有推断关系明确标注 `inferred=true`
- **单二进制部署** — 零依赖，CGO_ENABLED=0，一条命令即可运行
- **安全三层防护** — AST 只读校验 + 策略引擎 + AutoLimit，适用于生产环境查询
- **DSL 模式** — 通过 `@label.table` 统一引用不同数据源，无需切换连接

---

## 确定性优先

**dbexplain 不做任何 AI 推理或语义猜测。** 同样的数据库、同样的工具版本、同样的查询语句 → 永远得到同样的结果，没有随机性，没有黑盒判断。

所有推断的关系（如命名模式匹配的外键）标注 `inferred=true` 和置信度分数，与 DDL 声明的显式外键严格区分。工具输出的是**可证实的事实**（列名、类型、约束、索引结构），语义理解留给外部 LLM / Agent。

---

## 支持的数据源

| 数据源 | 协议 | 亮点 |
|--------|------|------|
| MySQL | `mysql://` | 外键、索引、字段注释推断 |
| PostgreSQL | `postgres://` | 多 Schema、行数统计、SSL 可配 |
| GaussDB | `gaussdb://` | PostgreSQL 协议兼容 |
| ClickHouse | `clickhouse://` | 排序键 / 分区键 / 主键 |
| SQLite | `sqlite://` | 纯 Go 驱动，无 CGO |
| Redis | `redis://` | 键模式推断、集群、TTL 风险诊断 |
| Elasticsearch | `elasticsearch://` | 索引映射、HTTPS |
| MongoDB | `mongodb://` | 近似文档数 |
| Qdrant | `qdrant://` | 向量集合元数据 |
| CSV / TSV | `csv://` `tsv://` | 内置文件查询引擎 |
| Excel | `xlsx://` | 内置文件查询引擎 |

---

## 核心能力

### Schema 采集
从所有数据源提取表结构、列、索引、外键、行数、分区键、引擎元数据。支持输出格式：
- **JSON** — 机器可消费的结构化数据
- **Markdown / 人类可读** — 带上下文标记的渲染结果
- **增量指纹缓存** `--cache` — 仅当 Schema 变化时才重新采集
- **AI 上下文导出** `--context` — 直接生成供 LLM 使用的上下文文件

### 只读查询执行 — 双路径架构

两条路径共享同一安全管道：**sqlguard (AST 校验) → AutoLimit → 策略引擎**

| 路径 | 用法 | 说明 |
|------|------|------|
| **直接执行** | `--db 1 "SELECT ..."` | SQL 数据库走原生 SQL，NoSQL 走原生命令 |
| **DSL 模式** | `--dsl "SELECT * FROM @label.table"` | 通过 `@label.table` 引用数据源，系统自动解析绑定 |

```bash
# 直接执行
dbexplain execute -env --db 1 "SELECT COUNT(*) FROM orders"
dbexplain execute -env --label redis "PING"

# DSL 模式
dbexplain execute -env --dsl "SELECT * FROM @my-mysql.users LIMIT 10" --human
```

> v0.1.1 限制：DSL 模式仅支持单数据源。v0.1.2 已支持跨源 JOIN/UNION（联邦查询），仍不支持 Redis / Mongo / Qdrant / ES 原生数据源。

### 文件查询引擎
CSV / TSV / XLSX 由**内置纯 Go SQL 引擎**驱动，无需外部数据库。支持：
- **基础** — WHERE / GROUP BY / HAVING / 聚合函数 (COUNT, SUM, AVG, MIN, MAX)
- **进阶** — Hash JOIN / ORDER BY (NULLS FIRST/LAST) / UNION / DISTINCT ON
- **高级** — 子查询 IN / 窗口函数 (ROW_NUMBER, RANK, DENSE_RANK, NTILE, LAG, LEAD, FIRST_VALUE, LAST_VALUE, 聚合 OVER + ROWS/RANGE 窗口框架)

### Schema 变更对比
字段级差异追踪：检测列（新增/删除/类型/可空/默认/注释/主键）、索引、外键三级变更。支持版本基线、双文件、当前采集三种对比模式。

```bash
dbexplain diff --cache schema.json --since v1.0 --human
```

### 安全三层防护
1. **sqlguard** — AST 级只读校验：8 个读动词放行、11 个写动词拒绝、多语句检测、CTE 写检测。AST 解析失败时回退到字符串匹配
2. **策略引擎** — `DENY_TABLES`（表级封锁）、`DENY_COLUMNS`（列级封锁，含 `SELECT *` 星号展开检测）、`DENY_STATEMENTS`（语句模式封锁）、`MASK_COLUMNS`（结果脱敏）
3. **AutoLimit** — 无 LIMIT 查询自动注入 1000，已有 LIMIT 的不重复追加

非 SQL 数据库拥有各自的命令白名单或原生查询校验器。密码在所有输出和日志中自动脱敏。

---

## 文档导航

| 场景 | 文档 |
|------|------|
| 傻瓜用法手册（5 分钟上手） | [`docs/USAGE_GUIDE.md`](docs/USAGE_GUIDE.md) |
| 查询案例（13 个实测示例） | [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) |
| 部署安装（源码/二进制/Skill） | [`docs/DEPLOY.md`](docs/DEPLOY.md) |
| 排障指南（连接/查询/文件问题） | [`dbexplain-skill/references/troubleshooting.md`](dbexplain-skill/references/troubleshooting.md) |
| 安全策略配置（DENY_TABLES 等） | [`docs/POLICY.md`](docs/POLICY.md) |
| 配置文件搜索规则 | [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md) |
| SQL 语法参考（文件查询引擎） | [`dbexplain-skill/references/sql-syntax.md`](dbexplain-skill/references/sql-syntax.md) |
| 全部文档索引 | [`docs/CODE_MAP.md`](docs/CODE_MAP.md) |

---

## 快速开始

```bash
# 1. 构建
cd src && CGO_ENABLED=0 go build -o ../release/dbexplain ./cmd/dbexplain

# 2. 配置（自动搜索 6 级路径）
mkdir -p ~/.config/dbexplain && cat > ~/.config/dbexplain/.env.dbexplain << EOF
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
EOF

# 3. 验证
./release/dbexplain -env                          # Schema 采集
./release/dbexplain execute -env --db 1 "SELECT 1" --human  # 查询
./release/dbexplain --version                     # 版本
```

---

## CLI 常用参数

| 场景 | 命令 |
|------|------|
| Schema 采集 | `dbexplain -env / -dsn <url> / -json / -human / -o <file>` |
| | `dbexplain collect -env --human`（显式子命令，v0.1.2+） |
| 查询执行 | `dbexplain execute -env --db <N> / --label <name> / --dsl / --human` |
| 交互式查询 | `dbexplain repl --dsn <url>` 或 `dbexplain repl -env`（v0.1.2+，支持 11 种数据源，不支持 DSL 模式） |
| 文件查询 | `dbexplain execute -dsn "csv://file.csv" "SELECT ..."` |
| Schema 对比 | `dbexplain diff --cache <file> --since <ver>` |
| 查看 DSN | `dbexplain list -env` |
| 加密配置 | `dbexplain encrypt .env.dbexplain` |

---

## 开发

```bash
cd src
go build ./...                              # 编译检查
go vet ./...                                # 静态分析
go test ./... -count=1                      # 单元测试
bash build.sh                               # 交叉编译 5 平台
```

---

## License

Apache 2.0 © 2026 WWT
