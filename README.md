# dbexplain — 数据库上下文编译器

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` 是一个**单二进制、零运行时依赖**的命令行工具。下载一个文件即可运行——不需 Python、Node、JDK 或任何外部动态库。给定数据库连接串，自动提取表结构、列、索引、外键，输出确定性、可证实的关系信息——不包含任何 AI 推理或语义猜测。

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
  - [加密配置文件](#加密配置文件)
- [DSN 格式与配置](#dsn-格式与配置)
- [使用方式](#使用方式)
  - [Schema 采集](#schema-采集)
  - [只读查询执行](#只读查询执行)
  - [列出数据库](#列出数据库)
  - [数据库参考手册](#数据库参考手册)
  - [参数速查](#参数速查)
  - [子命令](#子命令)
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
| CSV/TSV | `csv://` `tsv://` | 本地文件，单文件/目录/Glob，编码自动检测 |
| Excel | `xlsx://` | Excel 文件，每个 Sheet 作为表，标准构建即包含 |

> 各数据库详细机制、安全策略、排障指南见 [`docs/`](docs/) 专项手册。

---

## 核心原则

**只输出可证实的事实。** 外键来源于 DDL 声明。关系来源于命名模式匹配。风险诊断来源于可观测数据。没有 AI 总结，没有业务语义猜测，没有 LLM 推理。

更多架构愿景见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) 和 [`CONSTITUTION.md`](CONSTITUTION.md)。

![dbexplain 架构总览](docs/assets/architecture.drawio.png)
*4 阶段流水线：INPUT（连接配置）→ COLLECT（9 种 DB + CSV/XLSX 文件模式抽取）→ ANALYZE（FK 推断/排序/诊断/IR Graph）→ OUTPUT（Markdown/JSON/上下文文件）*

---

## 快速开始

### Linux / macOS

#### 在线安装（推荐）

一条命令完成工具全局安装 + AI Skill 部署：

```bash
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
bash dbexplain-skill/scripts/install.sh          # 中文 Skill
bash dbexplain-skill/scripts/install.sh --lang en  # English skill
```

脚本会自动检测系统和架构（`uname -s`/`uname -m`），从 GitHub Releases 下载对应二进制。

可用的平台标识：`linux-amd64`、`linux-arm64`、`darwin-amd64`、`darwin-arm64`。

#### 离线安装

预先下载对应平台的二进制文件，用 `--offline` 指定路径：

```bash
# 在有网络的机器上下载（以 Linux amd64 为例）
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-linux-amd64

# 复制到离线环境后安装
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64
```

仅安装工具、不部署 Skill：

```bash
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64 --no-skill
```

#### 手动下载二进制

```bash
# Linux amd64
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64
sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain

# macOS Apple Silicon
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-darwin-arm64
chmod +x dbexplain-darwin-arm64
sudo mv dbexplain-darwin-arm64 /usr/local/bin/dbexplain

dbexplain --version
```

### Windows

#### 在线安装（推荐）

在 PowerShell 中运行：

```powershell
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
.\dbexplain-skill\scripts\install.ps1           # 中文 Skill
.\dbexplain-skill\scripts\install.ps1 -Lang en   # English skill
```

脚本会自动下载 `dbexplain-windows-amd64.exe` 到 `%LOCALAPPDATA%\dbexplain\`，并添加到用户 PATH。

#### 离线安装

```powershell
# 在有网络的机器上下载
Invoke-WebRequest -Uri "https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-windows-amd64.exe" -OutFile "dbexplain-windows-amd64.exe"

# 复制到离线环境后，放到 %LOCALAPPDATA%\dbexplain\dbexplain.exe
# 然后把目录添加到用户 PATH
```

#### 手动下载

从 [GitHub Releases](https://github.com/IamWWT/dbexplain/releases) 下载 `dbexplain-windows-amd64.exe`，放到合适目录并添加到 PATH。

### 从源码编译

```bash
cd src && go mod tidy && bash build.sh
```

编译产物在 `release/` 目录（linux/darwin/windows × amd64/arm64 共 5 个）。

![dbexplain 部署拓扑](docs/assets/deployment.drawio.png)
*三步安装：GitHub Releases → install.sh → 三个目标（二进制 /usr/local/bin、配置 ~/.config、Skill ~/.agents）*

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

Windows 用户将配置文件放在 `%USERPROFILE%\.config\dbexplain\.env.dbexplain`。

运行验证：

```bash
dbexplain -env                  # 终端格式化报告
dbexplain --version             # 查看版本
dbexplain all                   # 完整手册
dbexplain mysql                 # MySQL 专用参考手册
dbexplain redis                 # Redis 专用参考手册
```

### 加密配置文件

`dbexplain` 支持使用机器指纹加密 `.env.dbexplain` 文件，加密后仅能在同一台机器上解密。

```bash
# 使用机器指纹加密（默认，无需密码）
dbexplain encrypt

# 使用密码 + 机器指纹双重保护
dbexplain encrypt --password

# 指定输入/输出文件
dbexplain encrypt .env.dbexplain -o config.enc
```

加密后，将 `.env.dbexplain.enc` 放在 `~/.config/dbexplain/` 或当前目录，`dbexplain -env` 会自动发现并解密：

```bash
# 加密后直接运行（无需手动设置环境变量）
dbexplain -env

# 如果使用了 --password 加密，将密码写入密钥文件：
echo "your-password" > ~/.config/dbexplain/.encryption_key
chmod 600 ~/.config/dbexplain/.encryption_key
dbexplain -env
```

> 也可通过 `DBPROBE_ENV_FILE` 环境变量显式指定加密文件路径（可选覆盖），通过 `APP_ENCRYPTION_KEY` 环境变量提供密码（可选覆盖）。
>
> **加密算法**: XChaCha20-Poly1305 (AEAD)。机器指纹模式无需密码，配置文件只能在加密时的机器上使用。
>
> **核心优势**:
> - 无需记忆密码，加密文件即用即解，对用户透明
> - 文件脱离机器即失效，即使被窃取也无法在其他机器解密
> - 纵深防御：弥补防火墙/ACL 之后的落盘加密防线
> - 合规友好：满足等保/GDPR 对敏感凭证静态加密要求
>
> **注意**: 加密后务必删除明文配置文件。更换硬件后需重新加密。

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
| `tls-skip-verify=true` | ES | 跳过 TLS 证书验证（诊断环境） |
| `authSource=<db>` | MongoDB | 认证数据库名 |

### 配置文件搜索优先级（`-env` 模式）

1. `DBPROBE_ENV_FILE` 环境变量指向的路径（可选覆盖）
2. 当前目录 `.env.dbexplain`
3. 当前目录 `.env.dbexplain.enc`（加密文件，自动解密）
4. `~/.config/dbexplain/.env.dbexplain`（Linux/macOS）或 `%USERPROFILE%\.config\dbexplain\.env.dbexplain`（Windows）
5. `~/.config/dbexplain/.env.dbexplain.enc`（加密文件，自动解密）
6. 当前目录 `.env`（向下兼容旧版）

> 详细说明（搜索规则与二进制路径无关、CWD 决定行为）见 [docs/CONFIG_SEARCH.md](docs/CONFIG_SEARCH.md)。

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

## 使用方式

### Schema 采集

```bash
# 单个数据库
dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'

# 多个异构数据库
dbexplain \
  -dsn 'mysql://root:pwd@host1:3306/orders' \
  -dsn 'postgres://u:p@host2:5432/users' \
  -dsn 'redis://:pwd@host3:6379/0?label=cache'

# 从配置文件加载（自动搜索，见 DSN 格式章节）
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
```

### 只读查询执行

`execute` 子命令在沙箱保护下执行只读查询，输出结构化数据。默认 JSON 格式（供 AI Agent 消费），`--human` 切换为 ASCII 表格。

```bash
# SQL 数据库（sqlguard 三层校验：动词白名单 + 多语句检测 + 自动 LIMIT）
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --db 1 'SHOW INDEX FROM users'
dbexplain execute -env --label my-pg --explain 'SELECT * FROM orders WHERE user_id=42'

# 非 SQL 数据库原生查询
dbexplain execute -env --label es-test 'SHOW TABLES'                    # ES SQL
dbexplain execute -env --label mongo '{"find":"users","filter":{}}'     # MongoDB JSON
dbexplain execute -env --label redis 'GET user:1001'                    # Redis 命令
dbexplain execute -env --label qdrant '{"count":"docs"}'                # Qdrant JSON

# 人类可读表格输出
dbexplain execute -env --db 3 --human "SELECT * FROM users LIMIT 5"
```

![list + execute --human 示例](docs/assets/install-offline-verify-2.png)

![dbexplain 使用示例](docs/assets/usages.png)

> 更多查询案例见 [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md)，安全机制详见 [`docs/EXECUTE.md`](docs/EXECUTE.md)。

### 列出数据库

```bash
# 零凭证暴露，加密 .env 自动解密
dbexplain list -env
```

### 数据库参考手册

```bash
# 完整手册（支持关键字过滤和语言切换）
dbexplain all --filter redis
dbexplain all --language en --filter "SSL mode"

# 各数据库专项手册
dbexplain mysql               # MySQL
dbexplain postgres            # PostgreSQL (别名: pg, postgresql)
dbexplain gaussdb             # GaussDB
dbexplain clickhouse          # ClickHouse (别名: ch)
dbexplain sqlite              # SQLite (别名: sqlite3)
dbexplain redis               # Redis
dbexplain elasticsearch       # Elasticsearch (别名: es)
dbexplain mongodb             # MongoDB
dbexplain qdrant              # Qdrant
dbexplain csv                 # CSV/TSV 文件处理（含 DSN 格式、编码、查询限制）
dbexplain xlsx                # Excel 文件处理（含构建要求）
```

### 参数速查

| 参数 | 说明 |
|------|------|
| `-dsn <string>` | 数据库连接串，可多次使用 |
| `-env` | 从配置文件加载 DSN（自动搜索 6 级路径，无需额外配置） |
| `-config <file>` | 从 JSON 文件读取 DSN 数组 |
| `-include <filter>` | 仅包含匹配的 DSN（按类型/标签/编号，逗号分隔） |
| `-exclude <filter>` | 排除匹配的 DSN |
| `-json` | 输出 JSON 格式 |
| `-o <file>` | 写入文件（文本模式自动添加 UTF-8 BOM） |
| `--log-dir <dir>` | 日志输出目录（默认 `/var/log/dbexplain`） |
| `-timeout <duration>` | 每 DSN 超时（默认 20s） |
| `--conn N` | Schema 采集最大并发连接数（默认 10） |
| `--version` | 输出版本号 |
| `--human` | 人类友好输出（含上下文标记） |
| `--context <dir>` | 写入 AI 上下文文件到目录（summary.json / topology.json / diagnostics.json / chunks/） |
| `--cache <file>` | Schema 指纹缓存。首次生成快照，后续输出 `<file>_delta.json` 增量差异 |
| `--language zh|en` | 手册语言（默认 zh） |

### 子命令

| 命令 | 说明 |
|------|------|
| `dbexplain list` | 列出所有已配置数据库的 INDEX/LABEL/KIND/HOST:PORT/DATABASE 映射（零凭证暴露） |
| `dbexplain execute <SQL>` | **只读查询执行**（沙箱保护）。SQL 类走 sqlguard 校验；非 SQL 类走原生格式。`--human` 切换表格输出 |
| `dbexplain encrypt <file>` | 加密 `.env` 配置文件（机器指纹 / 密码双重模式） |
| `dbexplain all` | 完整参考手册（支持 `--filter`、`--language`） |
| `dbexplain <dbtype>` | 数据库/文件参考手册。10 种类型：mysql, postgres/pg/postgresql, gaussdb, clickhouse/ch, sqlite/sqlite3, redis, mongodb, elasticsearch/es, qdrant, csv, xlsx |
| `dbexplain -h` | 显示简洁结构化帮助概览 |

---

## 作为 AI Skill 使用

`install.sh` 默认同时安装工具和 Skill，支持 `--lang zh|en` 选择语言。也可分开操作：

```bash
# 一键安装（工具 + Skill，在线）
bash dbexplain-skill/scripts/install.sh
bash dbexplain-skill/scripts/install.sh --lang en   # 英文 Skill

# 一键安装（工具 + Skill，离线）
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64

# 仅安装工具，不部署 Skill
bash dbexplain-skill/scripts/install.sh --no-skill

# 仅部署 Skill（工具已安装时）
# --lang zh 安装中文版，--lang en 安装英文版
bash dbexplain-skill/scripts/install-skill.sh
bash dbexplain-skill/scripts/install-skill.sh --lang en

# 更新已安装的 Skill
bash dbexplain-skill/scripts/install-skill.sh --update

# 验证安装
bash dbexplain-skill/scripts/install-skill.sh --verify

# 卸载 Skill
bash dbexplain-skill/scripts/uninstall-skill.sh

# 卸载工具
bash dbexplain-skill/scripts/uninstall.sh
```

![skill和工具安装](docs/assets/install-offline-1.png)

![AI Agent + dbexplain 交互流程](docs/assets/skill-interaction.drawio.png)
*5 步交互流程：① 用户提问 → ② AI 加载 SKILL.md → ③ AI 调用 dbexplain 采集模式 → ④ dbexplain 输出确定性报告 → ⑤ AI 解释给用户*

> 支持 Claude Code、DeepSeek、AixCoding、Agents 等平台。详见 [`docs/DEPLOY.md`](docs/DEPLOY.md)。

---

## 安全性

### Schema 采集模式
所有操作为**只读**：MySQL/PostgreSQL 仅 `SELECT`/`SHOW`/`PRAGMA`；Redis 仅 `SCAN`/`TYPE`/`HSCAN`（严格采样上限）；MongoDB 仅 `ListCollectionNames`/`EstimatedDocumentCount`。绝不执行写、改、删操作。

- 密码在输出和日志中自动脱敏（`Redacted()`）
- 每 DSN 独立日志（`logs/<label>.log`）
- 过滤跳过记录写入 `logs/filter.log`，不污染终端输出
- 参数化查询防 SQL 注入
- Redis 采样上限：2000 键、5 字段、512 字节、10 条流消息

### 只读查询执行 (`execute`)
`execute` 子命令在沙箱保护下执行用户 SQL/原生查询，与 schema 采集完全分离：

- **SQL 只读校验** (`sqlguard`)：动词白名单 + 多语句检测 + 自动 LIMIT 注入，拒绝 DROP/INSERT/UPDATE/DELETE
- **非 SQL 白名单**：Redis 30+ 命令白名单，MongoDB find/aggregate 白名单，Qdrant scroll/count 白名单
- **查询路由**：`isSQLKind()` 按数据库类型分流校验，SQL 类走 sqlguard，非 SQL 类各连接器内部验证
- **细粒度访问控制**：表级/列级/语句级拒绝策略 (`DENY_TABLES`/`DENY_COLUMNS`/`DENY_STATEMENTS`)；列值脱敏 (`MASK_COLUMNS`)
- **并发互斥**：per-label `TryLock`，同一 label 同时只有一个查询执行
- **双超时**：应用层 context + 数据库层语句超时
- **输出安全**：终端输出剥离 ANSI 转义和控制字符；列宽上限 256 字符
- **凭据保护**：查询结果 JSON 不包含任何连接信息或密码

> 详见 [`docs/EXECUTE.md`](docs/EXECUTE.md)

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
- **构建**：`CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v0.0.9"`
- **测试**：`go test ./...`（DSN 解析 + 字段推断）
- **交叉编译**：`bash build.sh`（linux/darwin/windows × amd64/arm64）

---

## 文档索引

| 文档 | 内容 |
|------|------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | 项目宪法（核心原则、开发约束） |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 架构愿景与发展路线 |
| [`docs/EXECUTE.md`](docs/EXECUTE.md) | 只读查询执行（安全架构、9-DB 验证） |
| [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) | CLI 查询案例库（13 条实测命令） |
| [`docs/DEPLOY.md`](docs/DEPLOY.md) | 部署指南（源码编译 + 工具安装 + Skill 部署） |
| [`docs/MYSQL.md`](docs/MYSQL.md) | MySQL 字段推断、索引/外键采集 |
| [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md) | PostgreSQL pg_catalog、SSL、多 Schema |
| [`docs/GAUSSDB.md`](docs/GAUSSDB.md) | GaussDB PostgreSQL 协议兼容 |
| [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md) | ClickHouse HTTP、排序键/分区键 |
| [`docs/SQLITE.md`](docs/SQLITE.md) | SQLite INTEGER PRIMARY KEY、CGO-free |
| [`docs/REDIS.md`](docs/REDIS.md) | Redis 键空间分析、风险诊断 |
| [`docs/MONGO.md`](docs/MONGO.md) | MongoDB 认证排障、只读元数据 |
| [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md) | Elasticsearch 索引映射、HTTPS |
| [`docs/QDRANT.md`](docs/QDRANT.md) | Qdrant 向量集合元数据 |
| [`docs/POLICY.md`](docs/POLICY.md) | 细粒度访问控制策略（表/列/语句级） |
| [`docs/FILE_PROCESSING.md`](docs/FILE_PROCESSING.md) | CSV/TSV/XLSX 文件处理（DSN 格式、编码、类型推断） |
| [`docs/ISSUE-062.md`](docs/ISSUE-062.md) | v0.0.9 策略引擎修复记录（全字段查询绕过） |
| [`docs/SECURITY_CHECKLIST.md`](docs/SECURITY_CHECKLIST.md) | 安全检查手册（发布前必读） |
| [`CHANGELOG.md`](CHANGELOG.md) | 版本变更记录（中文） |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | 版本变更记录（英文） |
| [`README_EN.md`](README_EN.md) | English README |
| [`issues.json`](issues.json) | 问题跟踪 |

---

## License

Apache 2.0 © 2026 WWT
