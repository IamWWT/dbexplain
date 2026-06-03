name: dbexplain-skill
description: >
  当用户需要探查数据库结构、分析跨库关系、执行只读查询或检查数据库健康时使用此技能。
  输入：DSN 连接串或 .env 配置文件。输出：表结构/字段类型/健康评分的 JSON 或表格。
user-invocable: true
trigger:
  - "解释表结构"
  - "分析数据库关系"
  - "跨库依赖"
  - "生成 ER 图"
  - "理解数据库"
  - "数据库巡检"
  - "数据库健康检查"
  - "执行只读查询"
---
## 1. 工具概述

`dbexplain` 是一个 Go 二进制 CLI，已安装到系统 PATH。两种独立模式：

- **Schema 采集**（`dbexplain -env`）：探查表结构/字段类型/注释/跨库外键/健康评分，输出 `instances[]` + `refs[]`（JSON）
- **只读查询**（`dbexplain execute`）：采集后在 SQL/文件/Mongo/Redis 等数据源上执行只读 SELECT，输出 `columns[]` + `rows[]`

还支持：增量变更检测（`--cache`）、DSL 模式（`--dsl`）、配置加密（`encrypt`）、帮助手册（`dbexplain all`）。

## 2. 输入定义

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| DSN 配置 | 文件路径 或 连接串 | ✅ | `.env` 文件路径（自动发现），或 `-dsn 'scheme://...'` 直接提供 |
| 查询语句 | string | 按需 | `execute` 模式下提供 SELECT 查询。不提供则只做 Schema 采集 |
| `--human` | flag | ❌ | 终端表格输出（默认 JSON），适用于给用户展示 |
| `--label/--db N` | string/int | ❌ | 多数据源时指定目标 |

> 若上述参数缺失，Agent 应主动询问用户补充，不自行猜测默认值。

## 3. 核心规则

- **只读安全**：仅执行 SELECT/SHOW/SCAN。绝不对数据库做任何写入操作。
- **隐私保护**：Agent **绝不**查看/记录/要求明文密码。让用户自行在 `.env` 配置，工具自动脱敏。
- **职责边界**：Agent 只调用工具命令。**绝不**创建/修改/读取用户配置文件的内容。
- **全局 PATH**：`dbexplain` 已在系统 PATH，任意目录直接调用。
- **失败处理**：命令报错时如实报告错误信息，**不猜测**替代参数，不假装查询成功。当 `--label` 缺失导致多数据源冲突时，指导用户加 `--label` 参数。

## 4. 标准工作流

### 4.1 确认安装 + 获取帮助

```bash
dbexplain --version                  # 检查是否已安装
dbexplain all                        # 查看完整帮助手册
dbexplain all --filter execute       # 按关键字过滤帮助
```
失败 → 告诉用户运行 `bash scripts/install.sh`（在项目根目录）。

### 4.2 配置数据库连接

问用户：已有 `.env` 配置文件吗？

- **有**（推荐）→ 直接下一步。
- **没有但有连接信息** → 引导用户在 `~/.config/dbexplain/.env.dbexplain` 创建。Agent **不能**替用户编辑此文件，告知路径和格式后等用户完成。
- **直接提供 DSN** → `dbexplain -dsn 'scheme://u:pass@host:port/db?label=x'`

### 4.3 Schema 采集

```bash
# 采集全部
dbexplain -env

# 输出 AI 上下文目录（推荐）
dbexplain -env --context ./ctx
```

`--context ./ctx` 生成的文件：

| 文件 | 用途 |
|------|------|
| `ctx/summary.json` | DB 列表、表数、字段总数、健康评分 |
| `ctx/topology.json` | 跨库外键连接、引用关系、数据流向 |
| `ctx/diagnostics.json` | 缺失索引、字段类型异常、注释缺失 |
| `ctx/chunks/*.json` | 每张表的详细结构（字段名/类型/注释） |

增量变更：首次采集后加 `--cache schema.cache`，后续自动输出 `changes` 字段（新增/删除/变更的表和字段），适用于定期巡检。

按类型过滤：`-include mysql,postgres` / `-exclude redis`。

### 4.4 执行只读查询

先采集 Schema，明确字段含义后再查询。Agent 自行构造查询，不依赖用户提供 SQL。

```bash
# SQL 数据库
dbexplain execute -env --label mysql 'SELECT COUNT(*) FROM orders' --human

# MongoDB（JSON 格式）
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}' --human

# Redis（原生命令）
dbexplain execute -env --label redis 'GET user:1001' --human
```

自动 LIMIT 1000。明确拒绝 DROP/INSERT/UPDATE/DELETE。

### 4.5 DSL 模式（v0.1.1+）

`--dsl` 启用 DSL 模式，使用 `@label.table` 语法引用数据源：

```bash
dbexplain execute -env --dsl --label mysql 'SELECT * FROM @mysql.users WHERE status = "active"'
```

DSL 编译流程：预处理 → AST 解析 → 符号绑定 → 后端路由，全程确定性。详见 `dbexplain all --filter dsl`。

### 4.6 文件查询

CSV/XLSX 文件支持完整 SELECT 子集。**完整语法**见 [`references/sql-syntax.md`](references/sql-syntax.md)。

```bash
# 数据预览（仅预览，不作为分析结论来源）
dbexplain execute -env --label my_data 'SELECT *' --limit 5 --human

# 聚合分析
dbexplain execute -env --label my_data \
  'SELECT department, AVG(rate) AS avg_rate FROM data GROUP BY department ORDER BY avg_rate DESC' --human

# 跨文件 JOIN
dbexplain execute -env --label my_data \
  'SELECT o.branch_name, AVG(t.rate) FROM data t JOIN org o ON t.dept_id = o.dept_id GROUP BY o.branch_name' --human
```

> **`SELECT *` 只用于预览，不得作为分析结论的数据来源。** 任何统计值必须通过聚合查询计算。

**最佳实践**：
- 模糊需求先澄清（汇总 vs 明细？分组对比？具体指标？），一次问 2-3 个关键问题
- 每次聚合查询显式用 WHERE 限定范围，不依赖对话上下文隐式过滤
- 默认 JSON（Agent 分析），加 `--human` 给用户看终端表格

## 5. 安全策略（`ACCESS_DENIED` 时）

管理员在 `.env` 配置了敏感数据保护。Agent **不应绕过**，如实告知用户。

```env
DENY_TABLES=sensitive,audit_log               # 表级禁止
DENY_COLUMNS=users.password_hash               # 列级禁止（须 table.column 格式）
MASK_COLUMNS=email=REDACTED,card_no=****       # 不阻断，替换列值输出
```

| 输出 | 含义 | Agent 处理 |
|------|------|-----------|
| `ACCESS_DENIED: table "xxx"` | 表被禁用 | 换表重试 |
| `ACCESS_DENIED: column "xxx"` | 列被禁用 | 去掉该列重试 |
| `READ_ONLY_VIOLATION` | 写操作 | 改成 SELECT |
| `CONCURRENT_LIMIT` | 并发冲突 | 稍后重试 |
| `QUERY_ERROR: ...` | 连接或 SQL 错 | 修 DSN 或 SQL |

## 6. 可追溯分析

### 严禁编造数据

**分析输出中的每一个数字必须来自实际的 SQL 查询输出。** 禁止以下行为：

- **编造统计值**：AVG/MAX/MIN/COUNT 精确计算，不许从 SELECT * 结果中目测估计
- **编造不存在的功能**：引擎不支持的功能（窗口函数、STDDEV、中位数等）不得编造计算结果。报错则如实报告
- **编造示例表格**：不自行画假表，引用查询结果时贴原始输出的列名和数值
- **排序错误**：排名表按指标值严格排序
- **隐瞒输出**：查询结果原文引用，`--human` 表格可截取关键行，但不能概括为不同数值
- **概念混淆**：区分行数（`COUNT(*)`）和时间天数（`COUNT(DISTINCT date_col)`）等不同维度计数

> **执行原则**：需要什么数字，就写什么 SQL 去查。

### 引用规范

- 量化结论末尾标注来源 SQL 和执行行数
- **逐条标注**，禁止笼统归因（如"所有数据来自 XXX 查询"）

### 好的例子

> 部门 A 平均完成率最高，为 95.2%；部门 B 最低，为 82.1%。
> 来源：`SELECT department, AVG(completion_rate) AS avg_rate FROM sales_data GROUP BY department ORDER BY avg_rate DESC`（6 行）

### 不好的例子

> 部门 A 表现最好。← ✗ 无来源、无具体数值
> 所有数据均来自 SELECT * 查询。← ✗ 笼统归因

## 7. 注意事项

- **密码含特殊字符**：命令行用**单引号**包裹整个 DSN；`.env` 文件里无需转义
- **MongoDB**：DSN 必须含 `authSource`（如 `?authSource=admin`），且需指定数据库名
- **加密配置文件**：
  1. 用户执行：`dbexplain encrypt .env.dbexplain`（机器指纹加密，无需密码）
  2. **加密后务必删除明文文件**（否则工具优先加载明文）
  3. Agent **绝不能**读取加密密钥或参与加密过程
- **ES 查询限制**：不支持数组字段，只选标量字段
- **ClickHouse**：查询不要加分号，否则被判定为多语句
- **技能目录结构**：安装后 `tools/dbexplain` 是全局二进制的符号链接，作为 AI Agent 在受限环境中的备选入口。系统 PATH 中有 `dbexplain` 时优先使用全局命令
- **完整帮助**：`dbexplain all`（支持 `--filter keyword`）
- **排障指南**：DB 连接/文件查询错误见 [`references/troubleshooting.md`](references/troubleshooting.md)
