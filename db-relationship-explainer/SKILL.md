name: db-relationship-explainer
description: >
  零依赖探查数据库结构，支持 MySQL, PostgreSQL, ClickHouse, SQLite, Redis, MongoDB,
  Elasticsearch, Qdrant 等，自动生成表卡片、字段注释、跨库关系图谱及健康报告。
  适用于解释表结构、分析数据库关系、数据库巡检、跨库依赖等场景。
user-invocable: true
tools:
  - path: tools/dbexplain-{platform}
    description: |
      静态编译的数据库探查二进制。接受 DSN 列表或 .env 文件，输出 Markdown 格式报告。
      参数：-dsn, -env, -config, -include, -exclude, -json, -o, -timeout, -version。密码自动脱敏。
trigger:
  - "解释表结构"
  - "分析数据库关系"
  - "跨库依赖"
  - "生成 ER 图"
  - "理解数据库"
  - "数据库巡检"
  - "数据库健康检查"
---
## 核心原则
- **只读安全**：工具仅执行 SELECT / SHOW / SCAN 等只读操作，绝不写入或修改数据。
- **隐私保护**：Agent **不得**查看、记录或要求用户提供明文密码。用户应通过 `.env` 文件传递密码（工具会自动脱敏），或使用单引号包裹的 DSN 时自行确保 shell 安全。
- **职责边界**：Agent 只能调用工具，**不得自行创建、修改或读取 `.env` 文件的内容**。
- **工作目录**：使用 `-env` 参数时，**必须 `cd` 到本 Skill 目录**后再执行命令，因为工具只从当前工作目录加载 `.env`。Shell 命令的首条语句始终是 `cd <skill-dir>`。

## 使用方式

### 方式一：用户直接提供 DSN（适合临时分析少量库）
用户说出连接信息（如"分析 MySQL 192.168.1.1:3306 的 testdb"），Agent 据此构造 DSN 并调用工具。命令示例：
```bash
cd <skill-dir> && ./tools/dbexplain-{platform} -dsn 'mysql://user:password@host:3306/db?label=别名'
```
⚠️ 若连接需要密码，Agent 应提示："为避免密码泄露，建议您在 skill 的 `.env` 文件中配置连接，或直接在命令中键入密码（需用单引号包裹）。"

### 方式二：使用 `.env` 文件（推荐，适合多库或需保护密码时）
1. **提醒用户**在 Skill 目录下创建 `.env` 文件，内容格式如下（编号可任意，无需连续）：
   ```ini
   DB1=mysql://user:password@host:3306/db?label=库1
   DB2=redis://:password@host:6379/0?label=缓存
   ```
2. 用户确认已配置后，Agent **必须先 `cd` 到 Skill 目录**再执行：
   ```bash
   cd <skill-dir> && ./tools/dbexplain-{platform} -env
   ```
3. Agent 绝不能查看或编辑 `.env` 文件。如果用户反馈 `.env` 不存在，Agent 应准确回复路径和格式，等待用户完成。

### 方式三：使用 JSON 配置文件
用户准备一个包含 DSN 数组的 JSON 文件并告知路径，Agent 使用 `-config <文件路径>` 参数调用（`-config` 接受绝对或相对路径，无需 cd）。

### 其他常用参数
- `-json`：输出 JSON 格式，便于程序消费。
- `-o <文件名>`：将报告写入指定文件。
- `-timeout <时长>`：设置每个 DSN 的采集超时（默认 20s）。
- `-include <过滤条件>`：仅采集匹配的数据库，按**类型**(如 `mysql,redis`)、**标签**(如 `aiops-mysql`)、**实例编号**(如 `DB1,DB3`)过滤，逗号分隔。
- `-exclude <过滤条件>`：排除匹配的数据库，支持同样的匹配维度。与 `-include` 同时使用时，`-include` 优先。
- `--version`：输出版本号并退出。

### DSN 高级参数
- **Redis 集群**：在 Redis DSN 中追加 `?cluster=true` 启用集群模式。
  - 示例：`redis://:password@host:7000/0?cluster=true&label=集群缓存`
- **Elasticsearch TLS**：使用 `elasticsearchs://` 协议前缀或 `?tls=true` 参数启用 HTTPS。
- **PostgreSQL SSL**：在 PostgreSQL DSN 中通过 `?sslmode=<模式>` 指定 SSL 模式，支持 `disable`、`require`、`verify-ca`、`verify-full`。

## Agent 执行流程
1. 识别用户意图，选择合适调用方式。
2. 若用户未直接提供 DSN：
   - **必须先询问**用户是否已在 Skill 目录的 `.env` 中配置了数据库连接（.env 文件存在不等于已配置）。
   - 若用户确认已配置 → **`cd` 到 Skill 目录**，执行 `-env`。
   - 若用户未配置 → 引导用户填写 `.env`，等待完成后重新执行。
3. 若用户直接提供了连接信息（IP、端口、库名），构造 DSN 用 `-dsn` 调用。
4. 若工具返回 `no DSNs provided`：
   - 最常见原因是**未 `cd` 到 Skill 目录**，导致 `.env` 找不到。
   - 请确认 shell 命令以 `cd <skill-dir> &&` 开头，然后重试。
   - 若 `cd` 后仍报错，则引导用户检查 `.env` 内容（Agent 仍不得读取）。
5. 执行成功后，将工具输出呈现给用户，并可根据报告提出简要建议。

## 注意事项
- 使用 `-env` 或 `-dsn` 时，命令必须以 `cd <skill-dir> &&` 开头，因为工具从当前工作目录加载 `.env`。
- 密码中的特殊字符（如 `!`）在命令行中需用**单引号**包裹整个 DSN，在 `.env` 文件中则无需转义。
- 工具运行时会在 stderr 显示进度信息（"采集中… 完成"），不影响最终报告。
- 分析 MongoDB 时，DSN 中必须包含数据库名和 `authSource` 参数（详见 `docs/MONGO.md`）。
- 如需支持其他平台，请将命令中的二进制名称替换为对应的 `dbexplain-{os}-{arch}` 文件。
- PostgreSQL 连接器自动采集所有非系统 schema（排除 `pg_*` 和 `information_schema`）。`public` schema 下的表名不带前缀，其他 schema 的表名格式为 `schema.table`。
