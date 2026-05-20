# dbexplain – 零依赖多数据库结构探查与关系分析工具

`dbexplain` 是一个**静态编译、无外部运行时依赖**的命令行工具，只需提供数据库连接串，即可：

- 🔍 自动导出表结构、列信息、索引、外键
- 🧩 分析跨库、跨实例的表关系（显式外键 + 命名推断）
- 🗺️ 生成聚类关系图与问题诊断
- 📄 支持终端美化输出和 JSON 格式（含完整列/索引/外键元数据）
- 📁 各数据库采集日志独立写入 `logs/` 目录
- 🧠 对无注释字段自动推断语义（首行数据 + 规则引擎）
- 🏷️ 支持 `-include`/`-exclude` 按类型、标签、实例编号过滤 DSN
- 🔐 PostgreSQL 多 Schema 自动采集、SSL 可配置、行数统计
- ⚡ 实时进度输出，避免大库等待焦虑

无需安装任何数据库客户端或驱动——所有逻辑编译进单个二进制文件，可运行在 **Linux / macOS / Windows（x86_64 / arm64）** 上，**只读安全**。

---

## 支持的数据库

| 数据库         | 连接方式           | 备注                                                         |
|----------------|--------------------|--------------------------------------------------------------|
| MySQL          | `mysql://`         | 系统表 + SHOW，支持字段注释推断                              |
| PostgreSQL     | `postgres://`      | pg_catalog，多 Schema 采集，行数统计，SSL 可配（?sslmode=）    |
| GaussDB        | `gaussdb://`       | 兼容 PostgreSQL 协议，自动适配                               |
| SQLite         | `sqlite://`        | 纯 Go 驱动，无 CGO                                           |
| ClickHouse     | `clickhouse://`    | HTTP 接口，支持 MergeTree 排序键/分区键                      |
| Redis          | `redis://`         | 流式扫描键模式，自动识别无 TTL、大 key，支持集群（?cluster=true） |
| Qdrant         | `qdrant://`        | 向量数据库，获取集合与点数量                                 |
| Elasticsearch  | `elasticsearch://` | 获取索引映射，支持 HTTPS（`elasticsearchs://` 或 `?tls=true`） |
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
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.3/dbexplain-linux-amd64
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

**`.env` 文件**（推荐，自动脱敏密码）：
```ini
DB1=mysql://user:pass@localhost:3306/shop?label=my-mysql
DB3=redis://:password@localhost:6379/0?label=my-redis
DB5=postgres://user:pass@localhost:5432/warehouse?sslmode=disable&label=my-pg
```
> `.env` 中 `DB` 编号无需连续或从 1 开始，程序会自动按数字升序读取。
> 运行：`./dbexplain -env`
> 完整的各数据库 `.env` 配置模板和参数说明见上方 [DSN 详解与配置模板](#dsn-详解与配置模板)。`src/.env.example` 也提供了可直接复用的示例。

### 输出选项

- `-o report.md`：将结果写入文件
- `-json`：输出 JSON 格式，便于程序消费
- `-timeout 30s`：设置每个 DSN 的采集超时（默认 20s）
- `-include` / `-exclude`：按类型、标签、实例编号过滤 DSN（逗号分隔）
- `--version`：输出版本号并退出
- `-h`：查看所有参数说明

---

## DSN 详解与配置模板

### DSN 格式

所有数据库统一采用 URL 格式：

```
scheme://[用户[:密码]@]主机[:端口][/库名][?label=别名&其他参数...]
```

### 参数速查表

| 参数 | 类型 | 适用数据库 | 说明 |
|------|------|-----------|------|
| `label` | string | 全部 | 实例别名，决定日志文件名 (`logs/<label>.log`) 和报告中显示的实例名 |
| `cluster` | bool | Redis | `?cluster=true` 启用 Redis Cluster 模式，自动扫描所有分片、聚合统计 |
| `tls` | bool | ES, Redis | `?tls=true` 启用 TLS/HTTPS 加密连接。ES 也可用 `elasticsearchs://` 协议前缀 |
| `sslmode` | string | PostgreSQL | SSL 连接模式，可选值：`disable`（默认）、`require`、`verify-ca`、`verify-full` |
| `authSource` | string | MongoDB | 认证数据库名，即用户创建所在的库（如 `admin`、`openim_v3`） |

### 各数据库 .env 配置模板

以下模板可直接复制到 `db-relationship-explainer/.env` 文件，修改实际地址和密码即可使用。
**编号可任意，无需连续**，程序会自动按数字升序加载。

```ini
# ────────── MySQL ──────────
# 格式: mysql://用户:密码@主机:端口/库名?label=别名
# 必填: 库名。程序自动采集该库（或全部非系统库，如果不指定库名）
DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql

# ────────── PostgreSQL ──────────
# 格式: postgres://用户:密码@主机:端口/库名?label=别名&sslmode=disable
# 必填: 库名（可选，不填则采集所有非系统库）
# sslmode: disable | require | verify-ca | verify-full（默认 disable）
# 程序自动采集所有非系统 schema（pg_catalog / information_schema 除外）
DB2=postgres://postgres:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable

# ────────── GaussDB ──────────
# 格式: gaussdb://用户:密码@主机:端口/库名?label=别名
# 与 PostgreSQL 协议兼容，使用相同的连接器
DB3=gaussdb://user:password@192.168.0.1:25308/mydb?label=my-gauss

# ────────── ClickHouse ──────────
# 格式: clickhouse://用户:密码@主机:端口/库名?label=别名
# 默认端口 8123（HTTP），无需指定库名时可省略
DB4=clickhouse://default:password@127.0.0.1:8123/default?label=my-clickhouse

# ────────── SQLite ──────────
# 格式: sqlite:///绝对路径?label=别名
# 密码和用户留空，路径必须为绝对路径
DB5=sqlite:///home/user/data/app.db?label=my-sqlite

# ────────── Redis（单机）──────────
# 格式: redis://:密码@主机:端口/数据库编号?label=别名
# 数据库编号默认为 0，密码为空时写 redis://host:port/0 即可
DB6=redis://:password@127.0.0.1:6379/0?label=my-redis

# ────────── Redis（集群）──────────
# 格式: redis://:密码@任意节点:端口/0?cluster=true&label=别名
# 集群模式仅支持 db0，自动扫描所有分片
DB7=redis://:password@10.0.0.1:7000/0?cluster=true&label=my-redis-cluster

# ────────── Elasticsearch（HTTP）──────────
# 格式: elasticsearch://用户:密码@主机:端口?label=别名
# 默认端口 9200
DB8=elasticsearch://elastic:password@127.0.0.1:9200?label=my-es

# ────────── Elasticsearch（HTTPS / TLS）──────────
# 方式一：使用 elasticsearchs:// 协议前缀
DB9=elasticsearchs://elastic:password@127.0.0.1:9200?label=my-es-secure
# 方式二：使用 elasticsearch:// 并追加 ?tls=true
# DB9=elasticsearch://elastic:password@127.0.0.1:9200?label=my-es-secure&tls=true

# ────────── MongoDB ──────────
# 格式: mongodb://用户:密码@主机:端口/库名?authSource=认证库&label=别名
# 必填: 库名 和 authSource（用户创建所在的数据库名）
DB10=mongodb://admin:password@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo

# ────────── Qdrant ──────────
# 格式: qdrant://:api密钥@主机:端口?label=别名
# 默认端口 6334，用户名为空（Qdrant 只用 API Key 认证）
DB11=qdrant://:my-api-key@127.0.0.1:6334?label=my-qdrant
```

### 特殊字符注意事项

- **密码含 `!`**：`.env` 文件中无需转义，直接写即可。命令行中需用**单引号**包裹整个 DSN。
- **密码含 `@`**：需 URL 编码为 `%40`（如 `pass@word` → `pass%40word`）。
- **密码含 `#`**：需 URL 编码为 `%23`（如 `Pwd1Open2%23IMD`）。
- **密码在输出和日志中自动脱敏**（替换为 `***`），无需担心泄露。

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

本项目完全适配 **Claude Code Skill** 体系，兼容 DeepSeek、AixCoding、Agents 等多个 AI 平台。`db-relationship-explainer/` 目录提供了完整的 Skill 定义、预编译二进制和**一键安装/卸载脚本**。

### 一键安装

```bash
cd db-relationship-explainer
bash install_skill_for_all_platform.sh
```

交互选择安装目标：全部平台（symlink 共享）、单个平台、项目本地目录、或自定义路径。脚本自动检测当前 OS/Arch 并选择对应二进制。

> **Windows 用户**：请在 Git Bash 或 MSYS2 终端中运行该脚本，CMD/PowerShell 不支持 Bash 语法。

**前置条件与边界处理**：

| 场景 | 行为 |
|------|------|
| `tools/` 缺少当前平台的二进制 | 报错退出，列出已有二进制，提示到 GitHub Releases 下载并指明放置路径 `tools/` 后重试 |
| 源目录无 `.env` 文件 | 静默跳过，仅复制 `.env.example` 模板 |
| 源目录有 `.env` 文件 | 交互询问是否复制（含密码泄露风险提示），确认后 `chmod 600` |
| `.env.example` 不存在 | 静默跳过 |

安装完成后闭环验证：

```bash
bash install_skill_for_all_platform.sh --verify
```

![Skill 安装管理界面](docs/assets/skill_install_mgr.png)

### 更新 Skill

当 SKILL.md 或二进制有新版本时，一条命令更新所有已安装位置，`.env` 文件不会被覆盖：

```bash
bash install_skill_for_all_platform.sh --update              # 扫描标准位置，全部更新
bash install_skill_for_all_platform.sh --update /path/to/skills  # 更新指定目录
```

### 卸载

```bash
bash uninstall_skill_for_all_platform.sh           # 交互选择要移除的安装
bash uninstall_skill_for_all_platform.sh --list    # 列出所有已安装位置
bash uninstall_skill_for_all_platform.sh --all     # 移除全部安装
```

> 卸载前会检测 `.env` 文件并弹出凭据警告，确认后 `.env` 随目录一并删除。如需保留数据库连接配置，请先备份 `.env`。

### 手动部署

1. 将编译后的二进制放入 `tools/` 目录
2. 编写 `SKILL.md` 定义触发词和指令
3. AI 助手即可直接调用该工具理解数据库结构

> 详细部署步骤、各平台集成方式和 `.env` 配置说明见 [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md)。

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
- 构建：`CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v0.0.3"`
- 测试：`go test ./...`（DSN 解析 + 字段推断已覆盖）
- CI/CD：`.github/workflows/ci.yml`（push/PR 触发 go build/vet/test）

提交 PR 前请确保：
- 新连接器为只读且安全转义
- 通过 `go vet` 和 `go test ./...`

---

## License

Apache 2.0 © 2025-2026 WWT 