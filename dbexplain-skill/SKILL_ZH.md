name: dbexplain-skill
description: >
  当用户需要探查数据库结构、分析跨库关系、执行只读查询或检查数据库健康时使用此技能。
  支持 16+ 数据源（MySQL/PG/ClickHouse/Redis/MongoDB/ES/Prometheus/CSV 等）。
  输入：DSN 连接串或 .env.dbexplain 配置文件（自动发现）。输出：表结构/字段类型/健康评分/跨库拓扑的 JSON 或表格。
user-invocable: true
trigger:
  - "解释表结构"
  - "分析数据库关系"
  - "跨库依赖"
  - "跨库查询"
  - "生成 ER 图"
  - "理解数据库"
  - "数据库巡检"
  - "数据库健康检查"
  - "执行只读查询"
  - "检查表结构"
  - "数据源概览"
  - "数据库问题排查"
  - "表结构分析"
  - "字段查询"
  - "数据质量检查"
  - "连通性检查"
---
## 1. 工具概述

`dbexplain` 是一个 Go 二进制 CLI，已安装到系统 PATH。两种独立模式：

- **Schema 采集**（`dbexplain` 或 `dbexplain collect`）：探查表结构/字段类型/注释/跨库外键/健康评分，输出 `instances[]` + `refs[]`（JSON）
- **只读查询**（`dbexplain execute`）：采集后在 SQL/文件/Mongo/Redis 等数据源上执行只读 SELECT，输出 `columns[]` + `rows[]`

还支持：增量变更检测（`--cache`）、DSL 模式（`--dsl`）、配置加密（`encrypt`）、连通性检查（`check`）。

## 2. 输入定义

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| DSN 配置 | 文件路径 或 连接串 | ✅ | `.env.dbexplain`（自动发现），或 `-dsn 'scheme://...'` 直接提供 |
| 查询语句 | string | 按需 | `execute` 模式下提供 SELECT 查询。不提供则只做 Schema 采集 |
| `--human` | flag | ❌ | 终端表格输出（默认 JSON），适用于给用户展示 |
| `--label name` | string | ❌ | execute 模式按 label 选数据源；collect 模式是 `-include` 别名（按 kind/label 过滤） |
| `--db N` | int | ❌ | execute 模式按序号选数据源（DB1=1, DB2=2） |

> 若上述参数缺失，Agent 应主动询问用户补充，不自行猜测默认值。

## 3. 核心规则

- **只读安全**：仅执行 SELECT/SHOW/SCAN。绝不对数据库做任何写入操作。
- **隐私保护**：Agent **绝不**查看/记录/要求明文密码。让用户自行在 `.env` 配置，工具自动脱敏。
- **职责边界**：Agent 只调用工具命令。**绝不**创建/修改/读取用户配置文件的内容。
- **配置文件不存在**：`findConfigFile()` 搜索 7 级优先级路径后找不到 → 输出 "No config file found" → exit 1。此时引导用户创建 `.env.dbexplain` 或使用 `-dsn` 直接提供连接串。
- **裸命令行为**：`dbexplain`（无参数）输出帮助手册，非报错。`dbexplain collect`（无 DSN）同等待。
- **失败处理**：命令报错时如实报告错误信息，**不猜测**替代参数，不假装查询成功。当 `--label` 缺失导致多数据源冲突时，指导用户加 `--label` 参数。

## 4. 用户意图 → 命令速查

用户需求是非线性的，按意图查第一条命令。**P0 场景（最常用）放最前面：**

### P0 — 数据库巡检 / 表结构 / 连通性

| 用户说 | AI 执行 | 参考 |
|--------|---------|------|
| "帮我巡检数据库 / 看看有啥问题" | `dbexplain --context ./ctx` → **读 diagnostics.json** 里的 issues[] | §5.4 |
| "看下表结构 / 有哪些字段" | `dbexplain collect`（默认）或 `collect --tables`（仅表名） | §5.4 |
| "连不通 / 检查连接" | `dbexplain check` 验证连通性 | §5.3 |
| "有多少张表 / 总览" | `dbexplain collect --tables`（紧凑列表） | §5.4 |
| "数据源概览" | `dbexplain list` 看可用 DSN 列表 | §5.3 |

### P1 — 数据查询 / 统计 / 拓扑

| 用户说 | AI 执行 | 参考 |
|--------|---------|------|
| "查下数据 / 统计一下" | **先 collect** 了解字段，再 execute 查询 | §5.5 |
| "帮我查一下上个月的订单量" | collect 了解字段 → 构造 `SELECT COUNT(*) FROM t WHERE month = ?` | §5.5 |
| "跨库关系 / 拓扑图" | `dbexplain --context ./ctx` → 读 **topology.json** refs[] | §5.4 |
| "看下数据质量" | `dbexplain --context ./ctx` → 读 **diagnostics.json** | §5.4 |

### P2 — 高级场景

| 用户说 | AI 执行 | 参考 |
|--------|---------|------|
| "和上周比改了啥" | `dbexplain diff --cache cache.json --since v1.0 --human` | §5.8 |
| "跨库查一下数据" | `dbexplain execute --dsl 'SELECT * FROM @a.t JOIN @b.t ON ...'` | §5.6 |
| "分析 CSV/XLSX 文件" | `dbexplain execute -dsn 'csv:///path/file.csv?label=d' 'SELECT ...'` | §5.7 |

> 裸敲 `dbexplain`（无参数）输出帮助手册，不是报错。

## 5. 标准工作流

### Before you start — ALWAYS ask
1. 确认用户是否有 `.env.dbexplain` 配置文件
2. 确认目标数据源类型（SQL / NoSQL / 文件）
3. 确认分析目的（Schema 预览 / 数据查询 / 健康检查 / 连通性）

### 5.1 确认安装 + 获取帮助

```bash
dbexplain --version                  # 检查是否已安装
dbexplain all                        # 查看完整帮助手册
dbexplain all --filter execute       # 按关键字过滤帮助
```
失败 → 运行 `bash dbexplain-skill/scripts/install.sh`（在项目根目录）。

### 5.2 配置数据库连接

问用户：已有 `.env.dbexplain` 配置文件吗？

- **有**（推荐）→ 直接下一步。`.env.dbexplain` 会被自动发现，无需任何参数。
- **没有但有连接信息** → 引导用户在 `~/.config/dbexplain/.env.dbexplain` 创建。Agent **不能**替用户编辑此文件，告知路径和格式后等用户完成。
- **直接提供 DSN** → `dbexplain -dsn 'scheme://u:pass@host:port/db?label=x'`

### 5.3 连通性检查（P0 场景，先于采集）

用户说"连不通"或想确认配置是否正确时先做此步：

```bash
dbexplain check                        # 验证所有 DSN 连通性
dbexplain check --label mysql          # 只检查特定数据源
dbexplain list                         # 查看可用 DSN 列表
```
→ verify: exit code 0，无 ACCESS_DENIED 或 dial tcp 错误
失败 → 引导用户检查 DSN 配置或网络连通性

### 5.4 Schema 采集 + 数据库巡检

```bash
# 推荐：输出 AI 上下文目录
dbexplain --context ./ctx
```

#### --context 输出文件消费方式

| 文件 | 内容 | AI 用途 |
|------|------|---------|
| `summary.json` | 所有 DSN 的 instances 列表，含 table_count、field_count、health_score | 总览：几个库、几张表、健康评分 |
| `topology.json` | refs[] 跨库外键关系 | 跨库依赖分析、ER 图 |
| `diagnostics.json` | issues[] 问题列表（无主键表、字段类型异常等） | 数据库巡检结论 |
| `chunks/*.json` | 分块上下文（按表/库拆分） | 按需加载到 Agent context |

→ verify: summary.json 包含所有 DSN 的采集结果，instances 非空

#### collect 常用参数

| 参数 | 作用 |
|------|------|
| `--tables` | 紧凑表列表模式（仅 name/engine/row_count） |
| `--sample` | 采样行获取注释推断（默认关闭） |
| `--conn N` | 并发采集数（默认 10） |
| `--timeout 30s` | 单 DSN 超时（默认 20s） |
| `-include mysql,postgres` | 只采集指定类型 |
| `-exclude redis` | 排除指定类型 |

增量变更：首次采集后加 `--cache schema.cache`，后续自动输出 `changes` 字段。

### 5.5 执行只读查询

先采集 Schema，明确字段含义后再查询。Agent 自行构造查询，不依赖用户提供 SQL。

```bash
# SQL 数据库
dbexplain execute --label mysql 'SELECT COUNT(*) FROM orders' --human

# MongoDB（JSON 格式）
dbexplain execute --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}' --human

# Redis（原生命令）
dbexplain execute --label redis 'GET user:1001' --human
```
→ verify: 返回 columns + rows，row_count > 0，无 ACCESS_DENIED 错误

自动 LIMIT 1000。明确拒绝 DROP/INSERT/UPDATE/DELETE。

NoSQL 命令参考见 [`references/nosql-commands.md`](references/nosql-commands.md)，Prometheus 查询见 [`references/prometheus-queries.md`](references/prometheus-queries.md)，完整场景示例见 [`references/examples.md`](references/examples.md)。

### 5.6 DSL 模式

```bash
dbexplain execute --dsl --label mysql 'SELECT * FROM @mysql.users WHERE status = "active"'
```
DSL 编译流程：预处理 → AST 解析 → 符号绑定 → 后端路由，全程确定性。完整 DSL 语法见 [`references/dsl-syntax.md`](references/dsl-syntax.md)。

### 5.7 文件查询

CSV/XLSX 文件支持完整 SELECT 子集。**完整语法**见 [`references/sql-syntax.md`](references/sql-syntax.md)。

```bash
# 聚合分析
dbexplain execute --label my_data \
  'SELECT department, AVG(rate) AS avg_rate FROM data GROUP BY department ORDER BY avg_rate DESC' --human

# 跨文件 JOIN
dbexplain execute --label my_data \
  'SELECT o.branch_name, AVG(t.rate) FROM data t JOIN org o ON t.dept_id = o.dept_id GROUP BY o.branch_name' --human
```
→ verify: 聚合结果包含所有分组，空值已处理

> **`SELECT *` 只用于预览，不得作为分析结论的数据来源。** 任何统计值必须通过聚合查询计算。

**最佳实践**：
- 模糊需求先澄清（汇总 vs 明细？分组对比？），一次问 2-3 个关键问题
- 每次聚合查询显式用 WHERE 限定范围，不依赖对话上下文隐式过滤
- 默认 JSON（Agent 分析），加 `--human` 给用户看终端表格

### 5.8 Schema Diff（P2 — 用户问"和上周比改了啥"时用）

```bash
# 首次：建立基线缓存
dbexplain --cache schema.cache
# 后续：对比差异
dbexplain diff --cache schema.cache --since v1.0 --human
```
→ verify: 输出 changes[]，包含 added/removed/modified 字段级变更

完整 Diff 语法见 `references/dsl-syntax.md`。

## 6. 边界与安全策略

### boundaries
- ❌ 绝不执行 DROP/INSERT/UPDATE/DELETE
- ❌ 绝不绕过错信息重试被安全策略拒绝的查询
- ❌ 绝不读取或修改用户配置文件内容
- ✅ 连接失败时如实报告具体错误，不猜测原因

### 安全策略（`ACCESS_DENIED` 时）

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

## 7. 可追溯分析

### 严禁编造数据

**分析输出中的每一个数字必须来自实际的 SQL 查询输出。** 禁止以下行为：
- **编造统计值**：AVG/MAX/MIN/COUNT 精确计算，不许从 SELECT * 结果中目测估计
- **编造不存在的功能**：引擎不支持的功能（窗口函数、STDDEV、中位数等）不得编造计算结果。报错则如实报告
- **排序错误**：排名表按指标值严格排序
- **概念混淆**：区分行数（`COUNT(*)`）和时间天数（`COUNT(DISTINCT date_col)`）等不同维度计数

> **执行原则**：需要什么数字，就写什么 SQL 去查。

### 引用规范

- 量化结论末尾标注来源 SQL 和执行行数
- **逐条标注**，禁止笼统归因（如"所有数据来自 XXX 查询"）
- 查询结果原文引用，`--human` 表格可截取关键行，但不能概括为不同数值

### 好的例子

> 部门 A 平均完成率最高，为 95.2%；部门 B 最低，为 82.1%。
> 来源：`SELECT department, AVG(completion_rate) AS avg_rate FROM sales_data GROUP BY department ORDER BY avg_rate DESC`（6 行）

### 不好的例子

> 部门 A 表现最好。← ✗ 无来源、无具体数值
> 所有数据均来自 SELECT * 查询。← ✗ 笼统归因

### eval
- Schema 采集：所有 DSN 返回 instances 非空 → ✅
- 只读查询：返回 columns + rows，row_count > 0 → ✅
- 安全拦截：DROP/INSERT/UPDATE 被明确拒绝 → ✅
- 分析结论：每个数字有来源 SQL 标注 → ✅
- 任何一条不满足 → 标记降级

### fallback
- [DSN 连接失败] → 报告具体错误，不猜测原因
- [查询返回空结果] → 确认 WHERE 条件，换表重试
- [查询超时] → 建议 `--timeout` 或简化查询
- [ACCESS_DENIED] → 换表/去掉禁用列重试

## 8. 注意事项

- **密码含特殊字符**：命令行用**单引号**包裹整个 DSN；`.env` 文件里无需转义
- **MongoDB**：DSN 必须含 `authSource`（如 `?authSource=admin`），且需指定数据库名
- **加密配置文件**：用户执行 `dbexplain encrypt`（机器指纹加密）。**加密后务必删除明文文件**。Agent 绝不能参与加密过程
- **ES 查询限制**：不支持数组字段，只选标量字段
- **ClickHouse**：查询不要加分号，否则被判定为多语句
- **完整帮助**：`dbexplain all`（支持 `--filter keyword`）
- **排障指南**：DB 连接/文件查询错误见 [`references/troubleshooting.md`](references/troubleshooting.md)
