# dbexplain — 数据库上下文编译器

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` 是一个**零依赖、静态编译**的命令行工具。给定数据库连接串，自动提取表结构、列、索引、外键，输出确定性、可证实的关系信息——不包含任何 AI 推理或语义猜测。

AI 时代数据库的"真值基座"。

---

## 目录

- [支持的数据库](#支持的数据库)
- [核心原则](#核心原则)
- [快速开始](#快速开始)
  - [Linux / macOS](#linux--macos)
  - [Windows](#windows)
  - [从源码编译](#从源码编译)
  - [安装后配置](#安装后配置)
- [使用方式](#使用方式)
  - [基本用法](#基本用法)
  - [参数速查](#参数速查)
  - [各数据库用法](#各数据库用法)
- [DSN 格式与配置](#dsn-格式与配置)
- [输出示例](#输出示例)
- [作为 AI Skill 使用](#作为-ai-skill-使用)
- [安全性](#安全性)
- [扩展新数据库](#扩展新数据库)
- [开发](#开发)
- [文档索引](#文档索引)

---

## 支持的数据库

| 数据库 | 连接方式 | 亮点 |
|--------|----------|------|
| MySQL | `mysql://` | 外键、索引、字段注释推断 |
| PostgreSQL | `postgres://` | 多 Schema、行数统计、SSL 可配 |
| GaussDB | `gaussdb://` | 兼容 PostgreSQL 协议 |
| ClickHouse | `clickhouse://` | 排序键/分区键/主键 |
| SQLite | `sqlite://` | 纯 Go 驱动，无 CGO |
| Redis | `redis://` | 键模式推断、集群、风险诊断 |
| Elasticsearch | `elasticsearch://` | 索引映射、HTTPS |
| MongoDB | `mongodb://` | 近似文档数、零数据风险 |
| Qdrant | `qdrant://` | 向量集合元数据 |

> 各数据库详细机制、安全策略、排障指南见 [`docs/`](docs/) 专项手册。

---

## 核心原则

**只输出可证实的事实。** 外键来源于 DDL 声明。关系来源于命名模式匹配。风险诊断来源于可观测数据。没有 AI 总结，没有业务语义猜测，没有 LLM 推理。

更多架构愿景见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) 和 [`CONSTITUTION.md`](CONSTITUTION.md)。

---

## 快速开始

### Linux / macOS

#### 在线安装（推荐）

一条命令完成工具全局安装 + AI Skill 部署：

```bash
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills
bash db-relationship-explainer/scripts/install.sh          # 中文 Skill
bash db-relationship-explainer/scripts/install.sh --lang en  # English skill
```

脚本会自动检测系统和架构（`uname -s`/`uname -m`），从 GitHub Releases 下载对应二进制。

可用的平台标识：`linux-amd64`、`linux-arm64`、`darwin-amd64`、`darwin-arm64`。

#### 离线安装

预先下载对应平台的二进制文件，用 `--offline` 指定路径：

```bash
# 在有网络的机器上下载（以 Linux amd64 为例）
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-linux-amd64

# 复制到离线环境后安装
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64
```

仅安装工具、不部署 Skill：

```bash
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64 --no-skill
```

#### 手动下载二进制

```bash
# Linux amd64
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64
sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain

# macOS Apple Silicon
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-darwin-arm64
chmod +x dbexplain-darwin-arm64
sudo mv dbexplain-darwin-arm64 /usr/local/bin/dbexplain

dbexplain --version
```

### Windows

#### 在线安装（推荐）

在 PowerShell 中运行：

```powershell
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills
.\db-relationship-explainer\scripts\install.ps1           # 中文 Skill
.\db-relationship-explainer\scripts\install.ps1 -Lang en   # English skill
```

脚本会自动下载 `dbexplain-windows-amd64.exe` 到 `%LOCALAPPDATA%\dbexplain\`，并添加到用户 PATH。

#### 离线安装

```powershell
# 在有网络的机器上下载
Invoke-WebRequest -Uri "https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-windows-amd64.exe" -OutFile "dbexplain-windows-amd64.exe"

# 复制到离线环境后，放到 %LOCALAPPDATA%\dbexplain\dbexplain.exe
# 然后把目录添加到用户 PATH
```

#### 手动下载

从 [GitHub Releases](https://github.com/IamWWT/understand_dbs_skills/releases) 下载 `dbexplain-windows-amd64.exe`，放到合适目录并添加到 PATH。

### 从源码编译

```bash
cd src && go mod tidy && bash build.sh
```

编译产物在 `release/` 目录（linux/darwin/windows × amd64/arm64 共 5 个）。

### 安装后配置

创建全局配置文件（任意目录均可运行）：

```bash
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
DB3=postgres://user:pass@127.0.0.1:5432/mydb?label=my-pg
EOF
```

Windows 用户将配置文件放在 `%USERPROFILE%\.dbexplain\.env.dbexplain`。

运行验证：

```bash
dbexplain -env                  # 终端格式化报告
dbexplain --version             # 查看版本
dbexplain --manual              # 完整手册
```

---

## 使用方式

### 基本用法

```bash
# 单个数据库
dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'

# 多个异构数据库
dbexplain \
  -dsn 'mysql://root:pwd@host1:3306/orders' \
  -dsn 'postgres://u:p@host2:5432/users' \
  -dsn 'redis://:pwd@host3:6379/0?label=cache'

# 从配置文件加载（自动搜索，见下方 DSN 格式章节）
dbexplain -env
dbexplain -env -include 'mysql,postgres'       # 按类型/标签过滤
dbexplain -env -exclude 'mongodb,qdrant'       # 排除指定项

# 输出到文件（Windows 中文系统自动 GBK，其他 UTF-8 BOM）
dbexplain -env -o report.md
dbexplain -env -json -o report.json

# 从 JSON 配置文件加载 DSN 数组
dbexplain -config dbs.json

# 生成 AI 上下文文件（适合喂给 Agent）
dbexplain -env --context ./context
# → context/summary.json      全局摘要（实例列表、表排行、重要性评分）
# → context/topology.json      关系拓扑图（跨库引用、集群）
# → context/diagnostics.json   问题诊断清单（严重度、表、消息）
# → context/chunks/*.md        每表单独的检索友好 Markdown

# 增量变更检测（配合 cron 定时任务）
dbexplain -env --cache schema_cache.json
# 首次：生成 schema_cache.json（指纹快照）
# 后续：对比差异 → 输出 schema_cache_delta.json（added/removed/changed）

# 人类友好格式（带 [table=] [pattern=] 上下文标记）
dbexplain -env --human

# 自定义超时（默认 20s）
dbexplain -env -timeout 60s

# 按关键字查找手册内容
dbexplain --manual --filter redis
dbexplain --manual --language en --filter "SSL mode"
```

### 参数速查

| 参数 | 说明 |
|------|------|
| `-dsn <string>` | 数据库连接串，可多次使用 |
| `-env` | 从配置文件加载 DSN（搜索: `DBPROBE_ENV_FILE` → `.env.dbexplain` → `~/.config/dbexplain/.env.dbexplain` → `.env`） |
| `-config <file>` | 从 JSON 文件读取 DSN 数组 |
| `-include <filter>` | 仅包含匹配的 DSN（按类型/标签/编号，逗号分隔） |
| `-exclude <filter>` | 排除匹配的 DSN |
| `-json` | 输出 JSON 格式 |
| `-o <file>` | 写入文件（自动添加 UTF-8 BOM） |
| `--log-dir <dir>` | 日志输出目录（默认 `/var/log/dbexplain`） |
| `-timeout <duration>` | 每 DSN 超时（默认 20s） |
| `--version` | 输出版本号 |
| `--manual` | 完整帮助手册（`--language en` 英文） |
| `--filter <keyword>` | 过滤 `--manual` 输出（忽略大小写） |
| `--human` | 人类友好输出（含上下文标记） |
| `--context <dir>` | 写入 AI 上下文文件到目录（summary.json / topology.json / diagnostics.json / chunks/） |
| `--cache <file>` | Schema 指纹缓存。首次生成快照，后续输出 `<file>_delta.json` 增量差异 |
| `--language zh|en` | 手册语言（默认 zh） |

### 各数据库用法

**MySQL**
```bash
dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
```

**PostgreSQL（多 Schema / SSL）**
```bash
dbexplain -dsn 'postgres://user:pwd@127.0.0.1:5432/warehouse?label=my-pg&sslmode=disable'
```

**Redis 单机 / 集群**
```bash
# 单机
dbexplain -dsn 'redis://:pwd@127.0.0.1:6379/0?label=my-redis'
# 集群
dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
```

**ClickHouse**
```bash
dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
```

**SQLite（绝对路径）**
```bash
dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-db'
```

**MongoDB**
```bash
dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
```

**Elasticsearch（HTTP / HTTPS）**
```bash
# HTTP
dbexplain -dsn 'elasticsearch://elastic:pwd@127.0.0.1:9200?label=my-es'
# HTTPS
dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
```

**Qdrant**
```bash
dbexplain -dsn 'qdrant://:api-key@127.0.0.1:6334?label=my-qdrant'
```

**GaussDB**
```bash
dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
```

> 更多细节: `dbexplain --manual [--filter <关键字>]`

---

## DSN 格式与配置

```
scheme://[用户:密码@]主机[:端口][/库名][?label=别名&参数...]
```

### 通用参数

| 参数 | 适用 | 说明 |
|------|------|------|
| `label=<别名>` | 全部 | 实例别名，决定日志文件 `logs/<label>.log` |
| `cluster=true` | Redis | 集群模式，自动扫描所有分片 |
| `tls=true` | ES, Redis | 启用 TLS |
| `sslmode=<mode>` | PostgreSQL | SSL 模式：`disable`/`require`/`verify-ca`/`verify-full` |
| `authSource=<db>` | MongoDB | 认证数据库名 |

### 配置文件搜索优先级（`-env` 模式）

1. `DBPROBE_ENV_FILE` 环境变量指向的路径
2. 当前目录 `.env.dbexplain`
3. `~/.config/dbexplain/.env.dbexplain`（Linux/macOS）或 `%USERPROFILE%\.dbexplain\.env.dbexplain`（Windows）
4. 当前目录 `.env`（向下兼容旧版）

### 配置模板

```ini
# MySQL
DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql

# PostgreSQL
DB2=postgres://postgres:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable

# ClickHouse
DB3=clickhouse://default:password@127.0.0.1:8123/default?label=my-ch

# SQLite（绝对路径）
DB4=sqlite:///home/user/data/app.db?label=my-sqlite

# Redis 单机 / 集群
DB5=redis://:password@127.0.0.1:6379/0?label=my-redis
DB6=redis://:password@10.0.0.1:7000/0?cluster=true&label=my-redis-cluster

# Elasticsearch HTTP / HTTPS
DB7=elasticsearch://elastic:password@127.0.0.1:9200?label=my-es
# HTTPS: elasticsearchs:// 或 elasticsearch://...?tls=true

# MongoDB
DB8=mongodb://admin:password@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo

# Qdrant
DB9=qdrant://:api-key@127.0.0.1:6334?label=my-qdrant
```

---

## 输出示例

```
> Instances (2)
  shop-db                    mysql    1 db(s), 5 tables
  cache                      redis    1 db(s), 3 tables

> shop-db  /  mydb
  orders [InnoDB] ~42,000 rows  核心订单表
────────────────────────────────────────────
  name       type          flags    comment
  ─────────  ────────────  ───────  ────────────
  id         int(11)       PK NN
  user_id    int(11)       NN       标识符
  total      decimal(10,2) NN       金额/数量
  created_at datetime      NN       时间
  indexes: IDX(user_id)

> Relationships (3 explicit FK, 2 inferred)
  shop-db/mydb.orders(user_id) ──FK──> shop-db/mydb.users(id)

> Issues (2)
  [!] shop-db/mydb/orders  FK column "user_id" has no index
  [i] cache/db0/session:{hex}  no TTL on security-sensitive key
```

![终端运行示例](docs/assets/explain-test-dsn+env.png)

---

## 作为 AI Skill 使用

`install.sh` 默认同时安装工具和 Skill，支持 `--lang zh|en` 选择语言。也可分开操作：

```bash
# 一键安装（工具 + Skill，在线）
bash db-relationship-explainer/scripts/install.sh
bash db-relationship-explainer/scripts/install.sh --lang en   # 英文 Skill

# 一键安装（工具 + Skill，离线）
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64

# 仅安装工具，不部署 Skill
bash db-relationship-explainer/scripts/install.sh --no-skill

# 仅部署 Skill（工具已安装时）
# --lang zh 安装中文版，--lang en 安装英文版
bash db-relationship-explainer/scripts/install-skill.sh
bash db-relationship-explainer/scripts/install-skill.sh --lang en

# 更新已安装的 Skill
bash db-relationship-explainer/scripts/install-skill.sh --update

# 验证安装
bash db-relationship-explainer/scripts/install-skill.sh --verify

# 卸载 Skill
bash db-relationship-explainer/scripts/uninstall-skill.sh

# 卸载工具
bash db-relationship-explainer/scripts/uninstall.sh
```

![Skill 安装管理](docs/assets/skill_install_mgr.png)

> 支持 Claude Code、DeepSeek、AixCoding、Agents 等平台。详见 [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md)。

---

## 安全性

所有操作为**只读**：MySQL/PostgreSQL 仅 `SELECT`/`SHOW`/`PRAGMA`；Redis 仅 `SCAN`/`TYPE`/`HSCAN`（严格采样上限）；MongoDB 仅 `ListCollectionNames`/`EstimatedDocumentCount`。绝不执行写、改、删操作。

- 密码在输出和日志中自动脱敏（`Redacted()`）
- 每 DSN 独立日志（`logs/<label>.log`）
- 过滤跳过记录写入 `logs/filter.log`，不污染终端输出
- 参数化查询防 SQL 注入
- Redis 采样上限：2000 键、5 字段、512 字节、10 条流消息

> 详细安全审查与数据库权限指南见各数据库专项文档（[`docs/`](docs/)）。

---

## 扩展新数据库

1. 在 `src/connector/` 下新建文件
2. 实现 `Connector` 接口的 `Collect(ctx, *dsn.DSN) (*schema.Instance, error)`
3. 在 `init()` 中调用 `Register("kind", func() Connector { ... })`
4. 重新编译

无需修改核心代码，完全符合开闭原则。

---

## 开发

- **语言**：Go 1.26+
- **构建**：`CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v0.0.5"`
- **测试**：`go test ./...`（DSN 解析 + 字段推断）
- **交叉编译**：`bash build.sh`（linux/darwin/windows × amd64/arm64）

---

## 文档索引

| 文档 | 内容 |
|------|------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | 项目宪法（核心原则、开发约束） |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 架构愿景与发展路线 |
| [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md) | Skill 部署指南（多平台集成） |
| [`docs/DEPLOY_SRC.md`](docs/DEPLOY_SRC.md) | 源码编译部署 |
| [`docs/MYSQL.md`](docs/MYSQL.md) | MySQL 字段推断、索引/外键采集 |
| [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md) | PostgreSQL pg_catalog、SSL、多 Schema |
| [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md) | ClickHouse HTTP、排序键/分区键 |
| [`docs/REDIS.md`](docs/REDIS.md) | Redis 键空间分析、风险诊断 |
| [`docs/MONGO.md`](docs/MONGO.md) | MongoDB 认证排障、只读元数据 |
| [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md) | Elasticsearch 索引映射、HTTPS |
| [`CHANGELOG.md`](CHANGELOG.md) | 版本变更记录（中文） |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | 版本变更记录（英文） |
| [`README_EN.md`](README_EN.md) | English README |
| [`issues.json`](issues.json) | 问题跟踪 |

---

## License

Apache 2.0 © 2025-2026 WWT
