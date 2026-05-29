# dbexplain — 数据库上下文编译器

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` 是一个**单二进制、零运行时依赖**的命令行工具。给定数据库连接串，自动提取表结构、列、索引、外键，输出确定性、可证实的关系信息——不包含任何 AI 推理或语义猜测。

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
| MongoDB | `mongodb://` | 近似文档数 |
| Qdrant | `qdrant://` | 向量集合元数据 |
| CSV/TSV | `csv://` `tsv://` | 文件查询引擎: WHERE/GROUP BY/JOIN/聚合/表达式 |
| Excel | `xlsx://` | 文件查询引擎: WHERE/GROUP BY/JOIN/聚合/表达式 |

> 各数据库详细机制、安全策略见 [`docs/`](docs/) 专项手册。

---

## 核心原则

**只输出可证实的事实。** 没有 AI 总结，没有语义猜测，没有 LLM 推理。详见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) 和 [`CONSTITUTION.md`](CONSTITUTION.md)。

![dbexplain 架构总览](docs/assets/architecture.drawio.png)
*4 阶段流水线：INPUT → COLLECT（11 种数据源）→ ANALYZE（FK 推断/排序/诊断/IR Graph）→ OUTPUT*

---

## 快速开始

### 安装

```bash
# 在线安装（工具 + AI Skill）
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain && bash dbexplain-skill/scripts/install.sh

# 手动下载
wget https://github.com/IamWWT/dbexplain/releases/download/v0.1.0/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64 && sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain
```

> 详细安装步骤（离线安装、Windows、从源码编译）见 [`docs/DEPLOY.md`](docs/DEPLOY.md)。

### 配置

```bash
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
EOF
```

> 配置文件搜索规则（`-env` 模式自动搜索 6 级路径）详见 [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md)。

### 验证

```bash
dbexplain -env                          # Schema 采集
dbexplain --version                     # 查看版本（需 v0.1.0）
dbexplain all                           # 完整手册
```

---

## DSN 格式

```
scheme://[用户:密码@]主机[:端口][/库名][?label=别名&参数...]
```

**通用参数**：`label=<别名>`（实例标识，决定日志文件名）、`cluster=true`（Redis 集群）、`tls=true`（ES/Redis 启用 TLS）、`sslmode=<mode>`（PostgreSQL SSL）、`authSource=<db>`（MongoDB 认证库）。

---

## 使用方式

### Schema 采集

```bash
# 单数据源
dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'

# 多数据源（配置文件）
dbexplain -env                          # 全部
dbexplain -env --include 'mysql,redis'  # 按 label/kind 筛选
dbexplain -env -o report.md             # 输出到文件
dbexplain -env --context ./ctx          # AI 上下文文件（summary/topology/diagnostics/chunks）
dbexplain -env --cache schema.json      # 增量变更检测
```

### 只读查询执行

`execute` 子命令在沙箱保护下执行只读查询：

```bash
dbexplain execute -env --db 1 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --db 3 --human "SELECT * FROM users LIMIT 5"
```

> 13 条实测查询案例见 [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md)，安全机制见 [`docs/EXECUTE.md`](docs/EXECUTE.md)。

### 列出数据库

```bash
dbexplain list -env   # 显示 INDEX/LABEL/KIND/HOST:PORT 映射（零凭证暴露）
```

### 数据库参考手册

```bash
dbexplain all --filter redis              # 按关键字过滤
dbexplain all --language en               # 英文版
dbexplain mysql / redis / qdrant / csv    # 专项手册
```

### 参数速查

| 参数 | 说明 |
|------|------|
| `-dsn <string>` | 数据库连接串，可多次使用 |
| `-env` | 从配置文件加载 DSN（自动搜索） |
| `-config <file>` | 从 JSON 文件读取 DSN 数组 |
| `--include / --exclude` | 按 label/kind/编号过滤 DSN |
| `-json` | JSON 格式输出 |
| `-o <file>` | 写入文件（自动 UTF-8 BOM） |
| `-timeout <duration>` | 每 DSN 采集超时（默认 20s） |
| `--conn N` | 最大并发连接数（默认 10） |
| `--human` | 人类友好输出（含上下文标记） |
| `--context <dir>` | 写入 AI 上下文文件 |
| `--cache <file>` | Schema 指纹增量缓存 |
| `--log-dir <dir>` | 日志输出目录 |

---

## 安全性

### Schema 采集
所有操作为**只读**（仅 `SELECT`/`SHOW`/`SCAN`/`PRAGMA`），密码在输出和日志中自动脱敏，每 DSN 独立日志，参数化查询防注入。

### 只读查询执行
`execute` 子命令三层安全防护：**sqlguard** 只读校验 → **policy 引擎**细粒度访问控制 → **AutoLimit** 防全表扫描。非 SQL 数据库各自拥有命令白名单。

> 安全机制详解见 [`docs/EXECUTE.md`](docs/EXECUTE.md)，策略引擎见 [`docs/POLICY.md`](docs/POLICY.md)，发布前检查清单见 [`docs/SECURITY_CHECKLIST.md`](docs/SECURITY_CHECKLIST.md)。

---

## 作为 AI Skill 使用

`install.sh` 默认同时安装工具和 AI Skill（支持 Claude Code / DeepSeek / AixCoding / Agents 等平台）：

```bash
bash dbexplain-skill/scripts/install.sh            # 中文 Skill
bash dbexplain-skill/scripts/install.sh --lang en  # English
bash dbexplain-skill/scripts/install-skill.sh --verify  # 验证安装
```

> 部署与集成详情见 [`docs/DEPLOY.md`](docs/DEPLOY.md)。

---

## 开发

```bash
cd src && go mod tidy && bash build.sh   # 交叉编译 5 平台
go test ./...                            # 运行测试
```

> 分层测试框架见 [`docs/test/`](docs/test/) 目录，包含 12 个测试专题覆盖全部数据源和安全特性。

---

## 文档索引

| 文档 | 内容 |
|------|------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | 项目宪法（核心原则、开发约束） |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | 架构愿景与发展路线 |
| [`docs/ALGORITHMS.md`](docs/ALGORITHMS.md) | 算法文档（命名推断、图聚类、重要性评分等） |
| [`docs/EXECUTE.md`](docs/EXECUTE.md) | 只读查询执行安全架构 |
| [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) | 13 条实测查询案例 |
| [`docs/POLICY.md`](docs/POLICY.md) | 细粒度访问控制策略 |
| [`docs/DEPLOY.md`](docs/DEPLOY.md) | 部署与 Skill 集成指南 |
| [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md) | 配置文件搜索规则 |
| [`docs/SECURITY_CHECKLIST.md`](docs/SECURITY_CHECKLIST.md) | 安全检查手册 |
| [`docs/FILE_PROCESSING.md`](docs/FILE_PROCESSING.md) | CSV/TSV/XLSX 文件处理 |
| [`docs/MYSQL.md`](docs/MYSQL.md) ... | 各数据库专项手册 |
| [`docs/test/README.md`](docs/test/README.md) | 分层测试框架 |
| [`CHANGELOG.md`](CHANGELOG.md) | 版本变更记录 |
| [`issues.json`](issues.json) | 问题跟踪 |

---

## License

Apache 2.0 © 2026 WWT
