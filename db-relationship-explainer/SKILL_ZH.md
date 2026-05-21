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

## 5. DSN 高级参数

- **Redis 集群**：`redis://:password@host:7000/0?cluster=true&label=集群`
- **Elasticsearch TLS**：使用 `elasticsearchs://` 前缀或 `?tls=true`
- **PostgreSQL SSL**：`?sslmode=disable|require|verify-ca|verify-full`

## 6. Agent 执行流程

1. **确保工具可用**：若 `dbexplain --version` 报错，执行 `bash scripts/install.sh`。
2. **识别意图**：
   - 用户提供连接信息 → 构造 DSN 用 `-dsn` 调用。
   - 用户未提供连接信息 → 询问是否已配置 `~/.config/dbexplain/.env.dbexplain`。
     - 已配置 → `dbexplain -env`
     - 未配置 → 引导创建配置文件，等待完成后执行。
3. **错误排查**：
   - `dbexplain` 未找到 → `bash scripts/install.sh`
   - 配置文件未找到 → 检查 `~/.config/dbexplain/.env.dbexplain` 或加密文件 `~/.config/dbexplain/.env.dbexplain.enc`
4. **呈现结果**：将工具输出展示给用户，可基于报告提出建议。

## 7. 注意事项

- 如果 `dbexplain` 不在 PATH，先运行 `bash scripts/install.sh`。
- 密码含 `!` 等特殊字符，命令行用**单引号**包裹整个 DSN；`.env.dbexplain` 文件中无需转义。
- 工具运行时 stderr 显示进度信息（"采集中… 完成"），不影响最终报告。
- MongoDB 的 DSN 必须包含数据库名和 `authSource` 参数。
- 完整文档：`dbexplain --manual`
- 卸载工具：`bash scripts/uninstall.sh`；卸载 Skill：`bash scripts/uninstall-skill.sh`
