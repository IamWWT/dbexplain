# dbexplain — 数据库上下文编译器

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` 是一个**零依赖、静态编译**的命令行工具。给定数据库连接串，自动提取表结构、列、索引、外键，输出确定性、可证实的关系信息——不包含任何 AI 推理或语义猜测。

AI 时代数据库的“真值基座”。

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

```bash
# 下载预编译二进制
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.4/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64

# 创建 .env 文件
cat > .env << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
DB3=postgres://user:pass@127.0.0.1:5432/mydb?label=my-pg
EOF

# 运行
./dbexplain-linux-amd64 -env

# 输出 JSON
./dbexplain-linux-amd64 -env -json -o report.json

# 查看完整手册
./dbexplain-linux-amd64 --manual
```

> 从源码编译：`cd src && go mod tidy && bash build.sh`

---

## 使用方式

```bash
# 单个数据库
./dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'

# 多个异构数据库
./dbexplain \
  -dsn 'mysql://root:pwd@host1:3306/orders' \
  -dsn 'postgres://u:p@host2:5432/users' \
  -dsn 'redis://:pwd@host3:6379/0?label=cache'

# 从 .env 加载，使用 include/exclude 过滤
./dbexplain -env -include 'mysql,postgres'
./dbexplain -env -exclude 'mongodb,qdrant'

# 输出到文件（Windows 中文系统自动 GBK，其他系统 UTF-8 BOM，记事本/CMD 均兼容）
./dbexplain -env -o report.md
./dbexplain -env -json -o report.json

# 从 JSON 配置文件加载 DSN 数组
./dbexplain -config dbs.json

# 生成 AI 上下文文件（适合喂给 Agent）
./dbexplain -env --context ./context
# → context/summary.json      全局摘要（实例列表、表排行、重要性评分）
# → context/topology.json      关系拓扑图（跨库引用、集群）
# → context/diagnostics.json   问题诊断清单（严重度、表、消息）
# → context/chunks/*.md        每表单独的检索友好 Markdown

# 增量变更检测（配合 cron 定时任务）
./dbexplain -env --cache schema_cache.json
# 首次：生成 schema_cache.json（指纹快照）
# 后续：对比差异 → 输出 schema_cache_delta.json（added/removed/changed）

# 人类友好格式（带 [table=] [pattern=] 上下文标记）
./dbexplain -env --human

# 自定义超时（默认 20s）
./dbexplain -env -timeout 60s

# 按关键字查找手册内容
./dbexplain --manual --filter redis
./dbexplain --manual --language en --filter "SSL mode"
```

### 参数速查

| 参数 | 说明 |
|------|------|
| `-dsn <string>` | 数据库连接串，可多次使用 |
| `-env` | 从 `.env` 文件加载 DSN（格式 `DB<n>=<DSN>`） |
| `-config <file>` | 从 JSON 文件读取 DSN 数组 |
| `-include <filter>` | 仅包含匹配的 DSN（按类型/标签/编号，逗号分隔） |
| `-exclude <filter>` | 排除匹配的 DSN |
| `-json` | 输出 JSON 格式 |
| `-o <file>` | 写入文件 |
| `-timeout <duration>` | 每 DSN 超时（默认 20s） |
| `--version` | 输出版本号 |
| `--manual` | 完整帮助手册（`--language en` 英文） |
| `--filter <keyword>` | 过滤 `--manual` 输出（忽略大小写） |
| `--human` | 人类友好输出（含上下文标记） |
| `--context <dir>` | 写入 AI 上下文文件到目录（summary.json / topology.json / diagnostics.json / chunks/） |
| `--cache <file>` | Schema 指纹缓存。首次生成快照，后续输出 `<file>_delta.json` 增量差异 |
| `--language zh|en` | 手册语言（默认 zh） |

---

## 使用场景

### 给 AI Agent 用

```bash
# 输出 JSON 供程序或 AI Agent 解析
./dbexplain -env -json -o report.json

# 生成 AI 上下文文件（适合嵌入 Agent 提示词）
./dbexplain -env --context ./context
# 生成: summary.json / topology.json / diagnostics.json / chunks/*.md

# 增量变更检测（配合 cron 定时任务）
./dbexplain -env --cache schema_cache.json
# 首次运行生成缓存，后续运行输出 schema_cache_delta.json
```

### 给人看

```bash
# 终端直接渲染（默认文本格式，含颜色高亮）
./dbexplain -env

# 人类友好格式（带 [table=] [pattern=] 上下文标记）
./dbexplain -env --human

# 写入 Markdown 文件（带 UTF-8 BOM，兼容 Windows 记事本）
./dbexplain -env --human -o report.md

# 查找手册内容
./dbexplain --manual --filter redis
```

### 不同数据库用法

**MySQL**
```bash
./dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
```

**PostgreSQL**
```bash
./dbexplain -dsn 'postgres://user:pwd@127.0.0.1:5432/warehouse?label=my-pg&sslmode=disable'
```

**Redis（集群）**
```bash
./dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
```

**ClickHouse**
```bash
./dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
```

**SQLite**
```bash
./dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-db'
```

**MongoDB**
```bash
./dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
```

**Elasticsearch（HTTPS）**
```bash
./dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
```

**Qdrant**
```bash
./dbexplain -dsn 'qdrant://:api-key@127.0.0.1:6334?label=my-qdrant'
```

**GaussDB**
```bash
./dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
```

> 更多数据库细节: `./dbexplain --manual [--filter <关键字>]`

---

## DSN 格式

```
scheme://[用户:密码@]主机[:端口][/库名][?label=别名&参数...]
```

**通用参数：**

| 参数 | 适用 | 说明 |
|------|------|------|
| `label=<别名>` | 全部 | 实例别名，决定日志文件 `logs/<label>.log` |
| `cluster=true` | Redis | 集群模式，自动扫描所有分片 |
| `tls=true` | ES, Redis | 启用 TLS |
| `sslmode=<mode>` | PostgreSQL | SSL 模式：`disable`/`require`/`verify-ca`/`verify-full` |
| `authSource=<db>` | MongoDB | 认证数据库名 |

**.env 配置模板：**

```ini
# MySQL
DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql

# PostgreSQL
DB2=postgres://postgres:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable

# ClickHouse
DB3=clickhouse://default:password@127.0.0.1:8123/default?label=my-ch

# SQLite（绝对路径）
DB4=sqlite:///home/user/data/app.db?label=my-sqlite

# Redis 单机
DB5=redis://:password@127.0.0.1:6379/0?label=my-redis

# Redis 集群
DB6=redis://:password@10.0.0.1:7000/0?cluster=true&label=my-redis-cluster

# Elasticsearch
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

## 安全性

所有操作为**只读**：MySQL/PostgreSQL 仅 `SELECT`/`SHOW`/`PRAGMA`；Redis 仅 `SCAN`/`TYPE`/`HSCAN`（严格采样上限）；MongoDB 仅 `ListCollectionNames`/`EstimatedDocumentCount`。绝不执行写、改、删操作。

- 密码在输出和日志中自动脱敏
- 每 DSN 独立日志（`logs/<label>.log`）
- 过滤跳过记录写入 `logs/filter.log`，不污染终端输出
- 参数化查询防注入
- Redis 采样上限：2000 键、5 字段、512 字节、10 条流消息

---

## 作为 Skill 集成到 AI 助手

```bash
cd db-relationship-explainer

# 一键安装（交互选择目标平台）
bash install_skill_for_all_platform.sh

# 更新已安装的 Skill
bash install_skill_for_all_platform.sh --update

# 验证安装
bash install_skill_for_all_platform.sh --verify

# 卸载
bash uninstall_skill_for_all_platform.sh
```

![Skill 安装管理](docs/assets/skill_install_mgr.png)

> 支持 Claude Code、DeepSeek、AixCoding、Agents 等平台。详见 [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md)。

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
- **构建**：`CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v0.0.4"`
- **测试**：`go test ./...`（DSN 解析 + 字段推断）
- **交叉编译**：`bash build.sh`（linux/darwin/windows × amd64/arm64）

---

## 文档索引

| 文档 | 内容 |
|------|------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | 项目宪法（核心原则、开发约束） |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 架构愿景与发展路线 |
| [`docs/MYSQL.md`](docs/MYSQL.md) | MySQL 字段推断、索引/外键采集 |
| [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md) | PostgreSQL pg_catalog、SSL、多 Schema |
| [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md) | ClickHouse HTTP、排序键/分区键 |
| [`docs/REDIS.md`](docs/REDIS.md) | Redis 键空间分析、风险诊断 |
| [`docs/MONGO.md`](docs/MONGO.md) | MongoDB 认证排障、只读元数据 |
| [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md) | Elasticsearch 索引映射、HTTPS |
| [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md) | Skill 部署指南 |
| [`docs/DEPLOY_SRC.md`](docs/DEPLOY_SRC.md) | 源码编译部署 |
| [`CHANGELOG.md`](CHANGELOG.md) | 版本变更记录（中文） |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | 版本变更记录（英文） |
| [`README_EN.md`](README_EN.md) | English README |
| [`issues.json`](issues.json) | 问题跟踪 |

---

## License

Apache 2.0 © 2025-2026 WWT
