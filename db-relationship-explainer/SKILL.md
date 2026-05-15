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
      参数：-dsn, -env, -config, -json, -o, -timeout。密码自动脱敏。
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

## 使用方式

### 方式一：用户直接提供 DSN（适合临时分析少量库）
用户说出连接信息（如“分析 MySQL 192.168.1.1:3306 的 testdb”），Agent 可据此构造 DSN 并调用工具。  
⚠️ 若连接需要密码，Agent 应提示：“为避免密码泄露，建议您在本技能的 `.env` 文件中配置连接，或直接在命令中键入密码（需用单引号包裹，如 `'mysql://user:password@host:3306/db'`）。”  
命令示例：
```bash
./tools/dbexplain-linux-amd64 -dsn 'mysql://user:password@host:3306/db?label=别名'
```

### 方式二：使用 `.env` 文件（推荐，适合多库或需保护密码时）
1. **提醒用户**在 **`db-relationship-explainer/` 目录下**创建 `.env` 文件，内容格式如下（编号可任意，无需连续）：
   ```ini
   DB1=mysql://user:password@host:3306/db?label=库1
   DB2=redis://:password@host:6379/0?label=缓存
   ```
2. 用户确认已创建后，Agent 执行：
   ```bash
   ./tools/dbexplain-{platform} -env
   ```
3. **Agent 绝不能查看或编辑该文件**。如果用户反馈 `.env` 不存在，Agent 应准确回复放置路径和格式，并等待用户完成。

### 方式三：使用 JSON 配置文件
用户可准备一个包含 DSN 数组的 JSON 文件，告知 Agent 路径。Agent 使用 `-config` 参数调用，同样不应读取该文件内容。

### 其他常用参数
- `-json`：输出 JSON 格式，便于程序消费。
- `-o <文件名>`：将报告写入指定文件。
- `-timeout <时长>`：设置每个 DSN 的采集超时（默认 20s）。
- 可组合使用，如 `-env -json -o report.json`。

## Agent 执行流程
1. 识别用户意图，选择合适调用方式。
2. 若用户未提供 DSN 且未提及 `.env`，**主动询问**：“是否已在 `db-relationship-explainer/.env` 文件中配置数据库连接？如需帮助，我可以为您说明格式。”
3. 确认用户选择后，构建并执行命令。
4. 将工具输出（或保存的文件内容）呈现给用户，并可根据报告提出简要建议（如发现的无主键风险、Redis 无 TTL 键等）。

## 注意事项
- 密码中的特殊字符（如 `!`）在命令行中需用**单引号**包裹整个 DSN，在 `.env` 文件中则无需转义。
- 工具运行时会在 stderr 显示进度信息（“采集中… 完成”），不影响最终报告。
- 分析 MongoDB 时，DSN 中必须包含数据库名和 `authSource` 参数（详见 `docs/MONGO.md`）。
- 如需支持其他平台，请将命令中的二进制名称替换为对应的 `dbexplain-{os}-{arch}` 文件。 