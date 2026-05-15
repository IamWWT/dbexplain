# dbexplain – 零依赖多数据库结构探查与关系分析工具

`dbexplain` 是一个**静态编译、无外部运行时依赖**的命令行工具，只需提供数据库连接串，即可：

- 🔍 自动导出表结构、列信息、索引、外键
- 🧩 分析跨库、跨实例的表关系（显式外键 + 命名推断）
- 🗺️ 生成聚类关系图与问题诊断
- 📄 支持终端美化输出和 JSON 格式
- 📁 各数据库采集日志独立写入 `logs/` 目录
- 🧠 对无注释字段自动推断语义（首行数据 + 规则引擎）
- ⚡ 实时进度输出，避免大库等待焦虑

无需安装任何数据库客户端或驱动——所有逻辑编译进单个二进制文件，可运行在 **Linux / macOS / Windows（x86_64 / arm64）** 上，**只读安全**。

---

## 支持的数据库

| 数据库         | 连接方式           | 备注                                                         |
|----------------|--------------------|--------------------------------------------------------------|
| MySQL          | `mysql://`         | 系统表 + SHOW，支持字段注释推断                              |
| PostgreSQL     | `postgres://`      | pg_catalog，支持 pgvector                                     |
| GaussDB        | `gaussdb://`       | 兼容 PostgreSQL 协议，自动适配                               |
| SQLite         | `sqlite://`        | 纯 Go 驱动，无 CGO                                           |
| ClickHouse     | `clickhouse://`    | HTTP 接口，支持 MergeTree 排序键/分区键                      |
| Redis          | `redis://`         | 流式扫描键模式，自动识别无 TTL、大 key 等风险                |
| Qdrant         | `qdrant://`        | 向量数据库，获取集合与点数量                                 |
| Elasticsearch  | `elasticsearch://` | 获取索引映射与字段信息                                       |
| MongoDB        | `mongodb://`       | 元数据采集，仅获取集合与近似文档数，零数据风险               |

> 新增数据库只需实现一个 `Connector` 接口并调用 `Register`，无需修改核心代码。  
> 各数据库的**详细机制、安全策略、排障指南**请参阅 [`docs/`](docs/) 目录下的专项手册：
> - [`docs/MYSQL.md`](docs/MYSQL.md)   MySQL 字段推断、索引/外键采集
> - [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md)   pg_catalog 查询、注释推断
> - [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md)   HTTP 查询、排序键/分区键、采样局限
> - [`docs/REDIS.md`](docs/REDIS.md)   流式键空间分析、风险诊断、安全采样
> - [`docs/MONGO.md`](docs/MONGO.md)   强制库名、认证排障、只读元数据
> - [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md)   索引映射、系统过滤、安全机制

---

## 快速开始

### 1. 下载预编译二进制

从 [GitHub Releases](https://github.com/IamWWT/understand_dbs_skills/releases) 下载对应平台的最新版本，解压后直接运行。

```bash
# 示例：下载 Linux amd64 版本
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.2/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64
./dbexplain-linux-amd64 -env
```

### 2. 从源码编译

```bash
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills/src
go mod tidy
bash build.sh   # 生成多平台二进制到当前目录
```

---

## 使用方法

### 运行

```bash
./dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'
```

输出将包含：
- 实例概览
- 每张表的列、类型、约束、索引、外键、行数估算
- 自动推断的列注释（若无原始 comment）
- 显式外键关系（实线）与推断关系（虚线）
- 表聚类（如 `orders* cluster`）
- 潜在问题（缺主键、未索引外键、Redis 风险等）

### 同时分析多个数据库

```bash
./dbexplain \
  -dsn 'mysql://root:123@prod-db:3306/orders' \
  -dsn 'postgres://admin:pass@crm-db:5432/customers' \
  -dsn 'redis://:foobared@cache:6379/0?label=session-cache'
```

> 密码含 `!` 等特殊字符时，请使用**单引号**包裹整个 DSN，避免 Shell 历史展开。

### 使用配置文件或 `.env`

**JSON 配置文件** (`dbs.json`)：
```json
[
  "mysql://user:pass@localhost:3306/shop?label=shop",
  "postgres://user:pass@localhost:5432/warehouse"
]
```
运行：`./dbexplain -config dbs.json`

**`.env` 文件**（推荐）：
```ini
DB1=mysql://user:pass@localhost:3306/shop
DB3=redis://:password@localhost:6379/0?label=cache
DB5=postgres://user:pass@localhost:5432/warehouse
```
> `.env` 中 `DB` 编号无需连续或从 1 开始，程序会自动按数字升序读取。  
运行：`./dbexplain -env`

### 输出选项

- `-o report.md`：将结果写入文件
- `-json`：输出 JSON 格式，便于程序消费
- `-timeout 30s`：设置每个 DSN 的采集超时（默认 20s）
- `-h`：查看所有参数说明

---

## DSN 格式

统一采用 URL 格式：

```
scheme://[user[:password]@]host[:port][/dbname][?label=alias&其他参数]
```

示例：
- MySQL：`mysql://root:123@127.0.0.1:3306/mydb?label=本地库`
- PostgreSQL / pgvector：`postgres://postgres:pass@localhost:5432/testdb`
- GaussDB：`gaussdb://user:pass@host:25308/db`
- SQLite：`sqlite:///绝对路径/data.db`
- ClickHouse：`clickhouse://default:@localhost:8123/default`
- Redis：`redis://:foobared@localhost:6379/0`
- Qdrant：`qdrant://:api-key@localhost:6334?label=qdrant-test`
- Elasticsearch：`elasticsearch://elastic:pass@localhost:9200?label=es`
- MongoDB：`mongodb://user:pass@localhost:27017/mydb?authSource=admin&label=mongo`

密码在输出和日志中自动脱敏。

---

## 输出示例（截取）

```
▸ Instances (2)
  shop-db                    mysql    1 db(s), 5 tables
  cache                      redis    1 db(s), 3 tables

▸ shop-db  /  mydb

  orders [InnoDB] ~42,000 rows  核心订单表
────────────────────────────────────────────
  name          type           flags         comment
  ────────────  ─────────────  ────────────  ────────────────────
  id            int(11)        PK NN
  user_id       int(11)        NN            标识符
  total         decimal(10,2)  NN            金额/数量
  status        tinyint(4)     NN            示例: 1
  created_at    datetime       NN            时间
  indexes: IDX(user_id)  IDX(status,created_at)

▸ cache  /  db0
  session:{hex} ~120 keys  type=hash  ⚠️ no TTL on security‑sensitive key

▸ Relationships (3 explicit FK, 2 inferred)
  shop-db/mydb.orders(user_id) ──FK──▶ shop-db/mydb.users(id)

▸ Issues (4)
  ⚠ shop-db/mydb/orders  FK column "user_id" has no index
  ℹ cache/db0/session:{hex}  no TTL on security‑sensitive key
```

更多运行截图请见 [`docs/assets/`](docs/assets/) 目录。
![dbexplain 终端运行示例](docs/assets/explain-test-dsn+env.png)
![多 DSN 同时采集](docs/assets/explain-test-mysql-2dsn.png)

---

## 安全性

- **只读操作**：所有 SQL 均为 `SELECT`、`SHOW`、`PRAGMA`；Redis 仅 `SCAN`/`TYPE`/`STRLEN`/`HSCAN`（限制采样）；MongoDB 仅 `ListCollections`/`EstimatedDocumentCount`。**绝不会执行写、改、删操作**。
- **大 key 保护**：Redis 的 hash 只用 `HSCAN` 采样 5 字段，string 仅取前 512 字节，stream 只取 10 条消息。详见 [`REDIS.md`](docs/REDIS.md)。
- **SQL 注入防护**：所有查询均使用参数化，标识符严格转义。
- **超时控制**：每个连接有独立的超时（可配置），避免挂起。
- **密码脱敏**：输出、日志中的 DSN 均隐藏密码。
- **日志分离**：每个数据库的采集日志写入 `logs/<label>.log`，终端只显示最终报告。

---

## 扩展新数据库

1. 在 `src/connector/` 下新建文件（如 `hbase.go`）
2. 实现 `Connector` 接口：
   ```go
   type Connector interface {
       Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error)
   }
   ```
3. 在文件 `init()` 中调用 `Register("hbase", func() Connector { return hbaseConnector{} })` 完成自注册
4. 重新编译

无需修改其他任何文件，完全符合开闭原则。

---

## 作为 Skill 集成到 AI 助手

本项目完全适配 **Claude Code Skill** 体系，详细部署步骤见 [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md)。

1. 将编译后的二进制放入 `tools/` 目录
2. 编写 `SKILL.md` 定义触发词和指令
3. AI 助手即可直接调用该工具理解数据库结构

---

## 常见问题

**Q: 全量运行多个 DSN 时程序好像“卡住”了？**  
A: 终端会实时显示每个 DSN 的采集进度和耗时，最后一行是 `全部采集完成，总耗时 XXms`。若未看到该行，可能因管道死锁（已修复）或网络超时，请检查 `logs/` 目录下对应日志。

**Q: 密码中含 `!` 等特殊字符，命令行报错？**  
A: 使用单引号包裹整个 DSN，例如 `-dsn 'redis://:pass!word@host:6379/0'`。`.env` 文件中无需转义。

**Q: `.env` 中注释掉 DB1 后，后面的 DB2 会失效吗？**  
A: 不会。程序会扫描所有 `DB<n>` 键并按数字排序，编号无需连续。

**Q: Redis 扫描会拖慢服务吗？**  
A: 使用 `SCAN` 非阻塞迭代，且限采 2000 个 key，仅对每个模式执行一次安全采样，详见 [`REDIS.md`](docs/REDIS.md)。

**Q: MongoDB 连接提示认证失败？**  
A: 检查 `authSource` 是否正确（用户创建所在的库），详见 [`MONGO.md`](docs/MONGO.md)。

**Q: 想了解某个数据库的详细实现和排障指南？**  
A: 参见 [`docs/`](docs/) 目录下对应数据库的专题文档。

---

## 开发

- 语言：Go 1.26+
- 依赖：仅标准库 + 数据库驱动（编译后静态链接）
- 构建：`CGO_ENABLED=0 go build -ldflags="-s -w"`

提交 PR 前请确保：
- 新连接器为只读且安全转义
- 通过 `go vet` 和 `golint`

---

## License

MIT © 2026 WWT 