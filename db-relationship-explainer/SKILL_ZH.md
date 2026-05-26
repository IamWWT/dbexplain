name: db-relationship-explainer
description: >
  零依赖探查数据库结构，支持 MySQL, PostgreSQL, ClickHouse, SQLite, Redis, MongoDB,
  Elasticsearch, Qdrant 等，自动生成表卡片、字段注释、跨库关系图谱及健康报告。
  适用于解释表结构、分析数据库关系、数据库巡检、跨库依赖等场景。
user-invocable: true
trigger:
  - "解释表结构"
  - "分析数据库关系"
  - "跨库依赖"
  - "生成 ER 图"
  - "理解数据库"
  - "数据库巡检"
  - "数据库健康检查"
---
## 1. 首次使用：安装工具

如果 `dbexplain` 尚未安装（`dbexplain --version` 报 command not found），执行：

```bash
bash scripts/install.sh
```

这会自动下载并安装 `dbexplain` 到 `/usr/local/bin`，并创建配置文件模板 `~/.config/dbexplain/.env.dbexplain`。

## 2. 核心原则

- **只读安全**：工具仅执行 SELECT / SHOW / SCAN 等只读操作，绝不写入或修改数据。
- **隐私保护**：Agent **不得**查看、记录或要求用户提供明文密码。用户应通过配置文件传递密码，工具会自动脱敏。
- **职责边界**：Agent 只能调用工具，**不得自行创建、修改或读取配置文件的内容**。
- **全局命令**：`dbexplain` 安装后位于系统 PATH，任意目录直接调用。

## 3. 使用方式

### 方式一：用户直接提供 DSN

用户说出连接信息（如"分析 MySQL 192.168.1.1:3306 的 testdb"），Agent 构造 DSN 并调用：

```bash
dbexplain -dsn 'mysql://user:password@host:3306/db?label=别名'
```

若需要密码，提示用户："为避免密码泄露，建议您在 `~/.config/dbexplain/.env.dbexplain` 中配置连接，或将密码直接键入命令（需单引号包裹）。"

### 方式二：配置文件（推荐，多库或需保护密码）

配置文件搜索优先级（`-env` 自动发现）：
1. `DBPROBE_ENV_FILE` 环境变量（可选覆盖）
2. `./.env.dbexplain`（当前目录）
3. `./.env.dbexplain.enc`（当前目录，加密文件自动解密）
4. `~/.config/dbexplain/.env.dbexplain`
5. `~/.config/dbexplain/.env.dbexplain.enc`（加密文件自动解密）

引导用户在 `~/.config/dbexplain/.env.dbexplain` 创建配置文件：

```ini
DB1=mysql://user:password@host:3306/db?label=库1
DB2=redis://:password@host:6379/0?label=缓存
```

用户确认后直接执行：

```bash
dbexplain -env
```

Agent 绝不能查看或编辑配置文件。用户反馈配置文件不存在时，Agent 回复正确路径和格式，等待用户操作。

### 加密配置文件 (v0.0.6)

用户可使用机器指纹加密配置文件，加密后仅能在同一台机器上解密。**Agent 绝不能查看、要求或记录用户密码。** 用户自行在终端执行以下命令：

```bash
# 加密配置文件（机器指纹模式，无需密码）
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain

# 加密后务必删除明文配置文件！
rm ~/.config/dbexplain/.env.dbexplain
```

如果用户选择密码增强模式：

```bash
# 用户自行执行（Agent 不能看到密码输入过程）
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain --password

# 删除明文，将密码写入密钥文件（用户自行操作，Agent 不能读取）
rm ~/.config/dbexplain/.env.dbexplain
echo "用户自选密码" > ~/.config/dbexplain/.encryption_key
chmod 600 ~/.config/dbexplain/.encryption_key
```

加密后，`dbexplain -env` 会自动发现 `.enc` 文件并解密（无需环境变量）。Agent 应提醒用户：
1. 加密后**务必删除明文配置文件**（否则工具优先匹配明文）
2. 密钥文件 `~/.config/dbexplain/.encryption_key` 权限应设为 600
3. Agent **永远不会**读取或操作这些文件

### 方式三：JSON 配置文件

用户提供 JSON 文件路径，Agent 使用 `-config <路径>` 调用。

### 列出可用数据库 (v0.0.7)

Agent 在执行任何操作前，应优先使用 `list` 子命令查看已配置的数据库列表：

```bash
dbexplain list
```

输出包含以下字段的映射表，帮助 Agent 正确选择 `--db N` 或 `--label <name>`：

| 字段 | 说明 |
|------|------|
| INDEX | DB 索引（对应 `--db N`，如 DB1=1, DB2=2） |
| LABEL | DSN 标签（对应 `--label`） |
| KIND | 数据库类型（mysql/redis/mongodb 等） |
| HOST:PORT | 主机与端口 |
| DATABASE | 数据库名 |

**安全提示**：`list` 命令仅显示元数据（标签/类型/主机/库名），**绝不输出 DSN 连接串、密码或任何凭证信息**。即使配置文件已加密（`.enc`），其内容也不会被解密显示。

### 方式四：只读查询执行 (v0.0.7)

Agent 在理解 schema 后，可通过 `execute` 子命令在沙箱保护下执行只读查询来验证假设或检查数据。**全部 9 种数据库均支持查询。**

#### SQL 数据库查询

```bash
# 基本 SELECT 查询（自动追加 LIMIT 1000）
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'

# EXPLAIN 查询计划
dbexplain execute -env --label my-pg --explain 'SELECT * FROM orders WHERE user_id=42'

# 自定义超时和行数限制
dbexplain execute -env --label shop-db --timeout 30 --limit 500 'SELECT * FROM events'
```

#### 非 SQL 数据库原生查询

```bash
# 使用 --db 编号（配合 dbexplain list 查看映射）
dbexplain execute -env --db 1 'SELECT * FROM users LIMIT 5'

# 使用 --label 名称
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'

# Elasticsearch (标准 SQL，通过 _sql 端点)
dbexplain execute -env --label es-test 'SHOW TABLES'
dbexplain execute -env --label es-test 'SELECT * FROM index_name WHERE status="active"'

# MongoDB (JSON 格式)
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}},"limit":100}'
dbexplain execute -env --label mongo '{"aggregate":"orders","pipeline":[{"$match":{"status":"done"}}]}'

# Redis (原生命令，30+ 命令白名单)
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --label redis 'HGETALL session:abc'
dbexplain execute -env --label redis 'SCAN 0 MATCH user:* COUNT 100'

# Qdrant (JSON 格式)
dbexplain execute -env --label qdrant '{"count":"documents"}'
dbexplain execute -env --label qdrant '{"scroll":"documents","limit":20}'
```

#### 安全约束

Agent 必须了解并遵守以下约束：

1. **只读强制执行**：所有查询受 sqlguard 白名单校验（SQL 类）或连接器内部白名单（非 SQL 类）保护，DROP/INSERT/UPDATE/DELETE 等写操作被拒绝
2. **多语句禁止**：分号拼接多个语句会被检测并拒绝（防止 SQL 注入逃逸）
3. **自动 LIMIT**：SELECT 查询无 LIMIT 时自动追加 `LIMIT 1000`，防止全表扫描
4. **并发互斥**：同一 label 同时只能执行一个查询
5. **输出格式分离**：查询结果 JSON（`columns + rows`）与 schema 采集 JSON（`instances + refs`）完全独立，不会混淆
6. **密码保护**：查询结果中不包含任何连接信息或密码

#### Agent 典型使用场景

- 采集 schema 后发现某字段含义不明 → 用 `execute` 查看该字段的实际数据样例
- 怀疑某外键关系 → 用 `execute` 验证引用完整性
- 需要确认表行数/数据分布 → 用 `execute` 执行 `SELECT COUNT(*)` 或分组统计
- 巡检时发现风险指标 → 用 `execute` 确认风险影响范围

## 4. 常用参数

| 参数 | 说明 |
|------|------|
| `-dsn <str>` | 直接指定连接串，可重复使用 |
| `-env` | 从配置文件加载 DSN（自动搜索多级路径） |
| `-config <file>` | 从 JSON 文件读取 DSN 数组 |
| `-json` | 输出 JSON 格式 |
| `-o <file>` | 将报告写入文件 |
| `--log-dir <dir>` | 日志输出目录（默认 /var/log/dbexplain） |
| `--context <dir>` | AI 上下文输出（summary.json / topology.json / diagnostics.json / chunks/） |
| `-cache <file>` | Schema 指纹缓存，用于增量变更检测 |
| `-timeout <dur>` | 每 DSN 采集超时（默认 20s） |
| `-include <f>` | 仅采集匹配的 DSN（按类型/标签/编号，逗号分隔） |
| `-exclude <f>` | 排除匹配的 DSN |
| `--human` | 人类友好输出，带 `[table=]`/`[pattern=]` 上下文标记 |
| `--version` | 输出版本号 |

### execute 子命令专用参数

| 参数 | 说明 |
|------|------|
| `execute <query>` | 只读查询（SQL 语句 / JSON / 原生命令），与 schema 采集格式分离 |
| `--label <name>` | 按 label 匹配 DSN |
| `--db <N>` | 按 DB 编号匹配（DB1=1, DB2=2） |
| `--limit <N>` | 最大返回行数（默认 1000） |
| `--timeout <N>` | 查询超时秒数（默认 30） |
| `--explain` | 包裹 EXPLAIN 返回查询计划（仅 SQL 数据库） |

## 5. DSN 高级参数

- **Redis 集群**：`redis://:password@host:7000/0?cluster=true&label=集群`
- **Elasticsearch TLS**：使用 `elasticsearchs://` 前缀或 `?tls=true`
- **PostgreSQL SSL**：`?sslmode=disable|require|verify-ca|verify-full`

## 6. Agent 执行流程

1. **确保工具可用**：若 `dbexplain --version` 报错，执行 `bash scripts/install.sh`。
2. **列出可用数据库**：若用户使用配置文件（典型场景），先执行 `dbexplain list` 查看所有已配置数据库的 INDEX/LABEL/KIND/HOST:PORT/DATABASE 映射，确认目标数据库及其编号。
3. **识别意图**：
   - 用户需要**理解数据库结构** → 采集 schema：
     - 用户提供连接信息 → 构造 DSN 用 `-dsn` 调用。
     - 用户未提供连接信息 → 询问是否已配置 `~/.config/dbexplain/.env.dbexplain`。
       - 已配置 → `dbexplain -env`
       - 未配置 → 引导创建配置文件，等待完成后执行。
   - 用户需要**验证假设/检查数据/确认细节** → 使用 `execute` 子命令：
     - 基于 `list` 输出选择目标数据库（`--db N` 或 `--label <name>`）。
     - 基于已采集的 schema 信息，构造安全查询验证。
     - Agent 应自行构造查询语句，不要依赖用户提供具体 SQL。
     - 查询结果用于验证字段语义、外键关系、数据分布等。
4. **错误排查**：
   - `dbexplain` 未找到 → `bash scripts/install.sh`
   - 配置文件未找到 → 检查 `~/.config/dbexplain/.env.dbexplain` 或加密文件 `~/.config/dbexplain/.env.dbexplain.enc`
   - `READ_ONLY_VIOLATION` → 查询包含不允许的写操作，修正 SQL 重试
   - `CONCURRENT_LIMIT` → 同一 label 已有查询在运行，稍后重试
   - `QUERY_ERROR` → 连接失败或 SQL 语法错误，检查 DSN 或修正查询
5. **呈现结果**：将工具输出展示给用户，可基于报告提出建议。

## 7. 注意事项

- 如果 `dbexplain` 不在 PATH，先运行 `bash scripts/install.sh`。
- 密码含 `!` 等特殊字符，命令行用**单引号**包裹整个 DSN；`.env.dbexplain` 文件中无需转义。
- 工具运行时 stderr 显示进度信息（"采集中… 完成"），不影响最终报告。
- MongoDB 的 DSN 必须包含数据库名和 `authSource` 参数。
- 完整文档：`dbexplain all`（替代旧的 `dbexplain --manual`）
- **列出数据库**：`dbexplain list` 显示所有已配置数据库的 INDEX/LABEL/KIND/HOST:PORT/DATABASE 映射。与 `-env` 一样，密码始终脱敏为 `{dbuser}`/`{dbpassword}` 占位符，绝不会泄露完整 DSN 或凭证。
- **凭证安全**：`list` 和 `-env` 输出均使用 `{dbuser}`/`{dbpassword}` 占位符替代真实凭证，确保敏感信息不在日志或终端中泄露。
- 卸载工具：`bash scripts/uninstall.sh`；卸载 Skill：`bash scripts/uninstall-skill.sh`

### execute 子命令注意事项

- **schema 采集与查询执行分离**：schema 采集输出 `instances/refs/groups/issues`；查询执行输出 `columns/rows/row_count/execution_time`。两者 JSON 格式完全不同，Agent 不应混用。
- **先采集后查询**：Agent 应先通过 `-env` 采集 schema 理解数据库结构，再根据具体需求决定是否执行查询。不要跳过 schema 采集直接查询。
- **Agent 自行构造查询**：Agent 应根据分析目标自行编写 SQL/原生查询，不要将用户的自然语言直接传递给 `execute`。
- **非 SQL 数据库查询格式**：Elasticsearch 使用标准 SQL；MongoDB/Qdrant 使用 JSON；Redis 使用原生命令。Agent 应根据数据库类型选择正确的查询格式。
- **查询结果受限**：默认最多 1000 行，超大结果自动截断（`truncated: true`）。Agent 应据此判断是否需要缩小查询范围。
- **并发限制**：同一数据库实例（label）同时只能运行一个查询。如果收到 `CONCURRENT_LIMIT` 错误，等待前一个查询完成后重试。
- **安全文档**：详见 `docs/EXECUTE.md`
