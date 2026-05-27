name: dbexplain-skill
description: >
  数据库结构探查工具，支持 MySQL/PG/ClickHouse/SQLite/Redis/MongoDB/ES/Qdrant。
  自动生成表结构/字段注释/跨库关系图谱/健康报告。
  支持只读查询（execute）与访问控制（policy）。
user-invocable: true
trigger:

  - "解释表结构"
  - "分析数据库关系"
  - "跨库依赖"
  - "生成 ER 图"
  - "数据库巡检"
  - "执行只读查询"
---
## 工具概述

`dbexplain` 是一个 Go 二进制 CLI，已安装到系统 PATH。两种独立模式：

- **Schema 采集**（`dbexplain -env`）：探查表/字段/类型/注释/跨库外键/健康评分，输出 `instances[]` + `refs[]`（JSON）
- **只读查询**（`dbexplain execute`）：采集后执行 SELECT 验证数据，输出 `columns[]` + `rows[]`（JSON）

此外支持：帮助手册（`dbexplain all`）、配置文件加密（`encrypt`）、增量变更检测（`-cache`）。

## 1. 核心规则

- **只读安全**：仅执行 SELECT/SHOW/SCAN。绝不写入数据。
- **隐私保护**：Agent **绝不**查看/记录/要求明文密码。让用户自行在 `.env` 配，工具自动脱敏。
- **职责边界**：Agent 只调用工具。**绝不**创建/修改/读取配置文件内容。
- **全局 PATH**：`dbexplain` 已在 PATH，任意目录直接调用。

## 2. 标准工作流

### 2.1 确认安装 + 获取帮助

```bash
dbexplain --version                  # 检查是否已安装
dbexplain all                        # 查看完整帮助手册
dbexplain all --filter execute       # 按关键字过滤帮助
dbexplain execute -h                 # 子命令帮助
```
失败 → 告诉用户运行 `bash scripts/install.sh`（在项目根目录）。

### 2.2 配置数据库连接

问用户：已有 `.env` 配置文件吗？

- **有**（推荐）→ 直接下一步。
- **没有但有连接信息** → 引导用户在 `~/.config/dbexplain/.env.dbexplain` 创建：
  ```ini
  DB1=mysql://user:pass@host:3306/mydb?label=my-mysql
  DB2=redis://:pass@host:6379/0?label=my-redis
  ```
  Agent **不能**替用户编辑此文件。告知路径和格式，等用户弄好。
- **直接提供 DSN** → `dbexplain -dsn 'scheme://u:pass@host:port/db?label=x'`

### 2.3 查看已配数据库

```bash
dbexplain list -env
```
输出 INDEX / LABEL / KIND / HOST:PORT / DATABASE 映射。后续 `--db N` 或 `--label xxx` 用此定位。

### 2.4 采集 Schema

```bash
# 采集全部
dbexplain -env

# 按类型过滤（只采 MySQL 和 PG）
dbexplain -env -include mysql,postgres

# 排除某类型
dbexplain -env -exclude redis

# 输出 AI 上下文目录（推荐）
dbexplain -env --context ./ctx
```

`--context ./ctx` 生成的文件结构及解读：

| 文件 | 内容 | 用途                 |
|------|------|-----------|
| `ctx/summary.json` | 总览：DB 列表、表数、字段总数、健康评分 | 快速了解整体情况 |
| `ctx/topology.json` | 拓扑：跨库外键连接、引用关系、数据流向 | 分析跨库依赖和关系链 |
| `ctx/diagnostics.json` | 诊断：缺失索引、字段类型异常、注释缺失项 | 输出巡检问题和风险 |
| `ctx/chunks/*.json` | 每张表的详细结构（字段名/类型/注释） | 逐表分析字段语义 |

Agent 应将分析结果呈现给用户：有哪些表、字段含义、外键关系、健康评分和风险项。

### 2.5 Schema 增量变更检测

首次采集生成指纹缓存，后续自动检测变更：

```bash
# 首次：建立缓存
dbexplain -env --context ./ctx --cache ./schema.cache

# 后续：检测与上次相比的差异
dbexplain -env --context ./ctx --cache ./schema.cache
```
输出包含 `changes` 字段（新增/删除/变更的表和字段）。适用于定期巡检、版本对比。

### 2.6 执行只读查询（先采集后查询）

采集完 schema 后，字段含义不明或想验证数据时再用。Agent 自行构造查询，不依赖用户提供 SQL。

**SQL 数据库（MySQL/PG/ClickHouse/SQLite/ES）：**
```bash
dbexplain execute -env --label mysql 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --label pg --explain 'SELECT * FROM users WHERE id=42'
dbexplain execute -env --db 3 --human "SELECT * FROM events LIMIT 5"
```
自动 LIMIT 1000。拒绝 DROP/INSERT/UPDATE/DELETE。ES 使用标准 SQL 通过 `_sql` 端点。

**MongoDB（JSON 格式）：**
```bash
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}'
dbexplain execute -env --label mongo '{"aggregate":"orders","pipeline":[{"$match":{"status":"done"}}]}'
```

**Redis（原生命令）：**
```bash
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --label redis 'HGETALL session:abc'
```

**输出：** 默认 JSON（Agent 继续分析），加 `--human` 给用户看终端表格。

## 3. 安全策略（`ACCESS_DENIED` 时）

管理员在 `.env` 配置了敏感数据保护。Agent **不应绕过**，如实告知用户：

```env
DENY_TABLES=sensitive,audit_log               # 表级禁止
DENY_COLUMNS=users.password_hash               # 列级禁止（须 table.column 格式）
DENY_STATEMENTS=DROP TABLE,ALTER TABLE         # 语句禁止
MASK_COLUMNS=email=REDACTED,card_no=****       # 不阻断，替换列值输出
```

| 输出 | 含义 | Agent 处理 |
|------|------|-----------|
| `ACCESS_DENIED: table "xxx"` | 表被禁用 | 换表重试 |
| `ACCESS_DENIED: column "xxx"` | 列被禁用 | 去掉该列重试 |
| `READ_ONLY_VIOLATION` | 写操作 | 改成 SELECT |
| `CONCURRENT_LIMIT` | 并发冲突 | 稍后重试 |
| `QUERY_ERROR: ...` | 连接或 SQL 错 | 修 DSN 或 SQL |

## 4. 常用参数

| 参数 | 作用域 | 说明 |
|------|--------|------|
| `--label/--db N` | execute | 按标签或编号选库 |
| `--human` | execute | 表格输出（默认 JSON） |
| `--limit/--timeout` | execute | 行数(1000)/超时(30s) |
| `--explain` | execute | 输出查询计划 |
| `--context dir` | 采集 | AI 上下文目录（summary/topology/diagnostics/chunks） |
| `--cache file` | 采集 | Schema 指纹缓存，增量变更检测 |
| `-include/-exclude` | 采集 | 按 DB 类型过滤采集 |
| `-json/-o file` | 采集 | JSON 输出/写入文件 |

## 5. 注意事项

- **密码含特殊字符**：命令行用**单引号**包裹整个 DSN；`.env` 文件里无需转义
- **MongoDB**：DSN 必须含 `authSource`（如 `?authSource=admin`），且需指定数据库名
- **加密配置文件**：
  1. 用户自行执行：`dbexplain encrypt .env.dbexplain`（机器指纹加密，无需密码）
  2. **加密后务必删除明文文件**（否则工具优先加载明文）
  3. 加密后 `.env` 自动发现 `.enc` 文件并解密，无需改命令
  4. Agent **绝不能**读取加密密钥或参与加密过程
- **ES 查询限制**：不支持数组字段（如 `SELECT *` 可能报错），只选标量字段
- **ClickHouse**：查询不要加分号，否则被判定为多语句
- **安装/卸载**：`bash scripts/install.sh` / `bash scripts/uninstall.sh`
- **完整帮助**：`dbexplain all`（支持 `--filter keyword`）
