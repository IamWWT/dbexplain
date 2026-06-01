name: dbexplain-skill
description: >
  数据库结构探查工具，支持 MySQL/PG/ClickHouse/SQLite/Redis/MongoDB/ES/Qdrant、CSV/TSV/XLSX 文件。
  自动生成表结构/字段注释/跨库关系图谱/健康报告。
  支持只读查询（execute, 含 CSV/XLSX 文件查询引擎 WHERE/GROUP BY/JOIN/聚合/表达式）与访问控制（policy）。
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
## 工具概述

`dbexplain` 是一个 Go 二进制 CLI，已安装到系统 PATH。两种独立模式：

- **Schema 采集**（`dbexplain -env`）：探查表/字段/类型/注释/跨库外键/健康评分，输出 `instances[]` + `refs[]`（JSON）
- **只读查询**（`dbexplain execute`）：采集后执行 SELECT 验证数据，输出 `columns[]` + `rows[]`（JSON）

此外支持：帮助手册（`dbexplain all`）、配置文件加密（`encrypt`）、增量变更检测（`--cache`）。

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

| 文件 | 内容 | 用途 |
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

**CSV/XLSX 文件（v0.1.0+ 文件查询引擎）：**
文件数据源支持完整 SELECT 子集，无需外部工具即可进行业务分析。完整语法参考见 [`references/sql-syntax.md`](references/sql-syntax.md)。

| 语法 | 说明 |
|------|------|
| `SELECT ... FROM table` | 列投影，支持 `SELECT *`、别名、`DISTINCT ON` |
| `WHERE ... AND/OR/NOT` | 过滤，支持 `=`/`!=`/`<`/`>`/`LIKE`/`IN`/`BETWEEN`/`IS NULL` |
| `GROUP BY ... HAVING` | 分组聚合 + 分组后过滤 |
| `SUM/AVG/COUNT/MAX/MIN` | 聚合函数，支持 `COUNT(DISTINCT col)` |
| `ORDER BY ... NULLS FIRST/LAST` | 排序 |
| `JOIN / LEFT JOIN / RIGHT JOIN` | 跨文件哈希连接 |
| `UNION / UNION ALL` | 合并结果 |
| `CAST / ABS / ROUND` | 类型转换和数学函数 |
| `col IN (SELECT ...)` | 子查询 |

```bash
# WHERE 过滤 + 列投影
dbexplain execute -env --label my_data 'SELECT employee_id, completion_rate FROM sales_data WHERE completion_rate < 60' --human

# GROUP BY + 聚合
dbexplain execute -env --label my_data 'SELECT department, AVG(rate) AS avg_rate FROM data GROUP BY department ORDER BY avg_rate DESC' --human

# 跨文件 JOIN
dbexplain execute -env --label my_data \
  'SELECT o.branch_name, AVG(t.completion_rate) FROM sales_data t JOIN org_info o ON t.dept_id = o.dept_id GROUP BY o.branch_name' --human

# 列间算术 + 类型转换
dbexplain execute -env --label my_data \
  'SELECT employee_id, CAST(channel_cnt AS FLOAT) / total_cnt * 100 AS pct FROM data WHERE total_cnt > 0' --human
```
文件数据源只读（仅 SELECT），遇到 DROP/INSERT 会返回 parse error。

### 2.7 文件查询最佳实践

#### 数据预览

先用 `SELECT * --limit 5` 查看数据样例，再用聚合查询检查维度列的基数：

```bash
dbexplain execute -env --label my_data 'SELECT *' --limit 5 --human                # 看数据样例
dbexplain execute -env --label my_data 'SELECT DISTINCT department FROM data' --human  # 部门有几个
```

> **`SELECT *` 只用于预览数据结构，不得作为分析结论的数据来源。** 任何统计值（均值、极值、占比等）必须通过聚合查询计算。

#### 澄清需求

**用户问题模糊时，必须先澄清再分析。** 不要替用户假设分析维度。一次问 2-3 个关键问题：

- 需要看汇总统计（均值/总数）还是明细数据？
- 是否需要分组对比或排名？
- 关注哪个具体指标？是否需要时间趋势？

#### 业务分析

根据用户问题确定分析范围，**每次聚合查询显式用 WHERE 限定范围**，不依赖对话上下文隐式过滤。需要全部数据时不要加 LIMIT（默认上限 1000 行足够）。

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

## 4. 可追溯分析

每条量化结论必须标注来源 SQL，确保用户能验证数据的真实性。

### 严禁编造数据

**分析输出中的每一个数字必须来自实际的 SQL 查询输出。** 禁止以下行为：

- **编造统计值**：平均完成率、最大值、范围等必须通过 `AVG/MAX/MIN/COUNT` 等聚合查询精确计算，不许从 `SELECT *` 的结果中目测估计
- **编造不存在的功能**：文件查询引擎支持的功能见[语法速览](#26-执行只读查询先采集后查询)表格。**引擎不支持的功能（如窗口函数、STDDEV、中位数等）不得编造计算结果。** 如果引擎报错，如实报告
- **编造示例表格**：不要用中文翻译列名重新画一个假表。如果引用查询结果，直接贴原始输出的列名和数值
- **排序错误**：排名表必须按指标值严格排序
- **隐瞒输出**：查询结果必须原文引用到分析中。`--human` 输出的表格可以截取关键行，但不能自行概括为不同的数值

> **执行原则**：需要什么数字，就写什么 SQL 去查。`SELECT *` 只用于数据预览，不应作为分析结论的数据来源。

### 引用规范

- **排名、占比、均值等量化结论**：末尾标注来源 SQL 和执行行数
- **逐条标注，禁止笼统归因**：每条结论标注各自来源 SQL。**禁止在结尾统一写"所有数据来自 XXX 查询"**
- **定性判断**：需有具体数据对比支撑（如分组对比、时间序列）

### 好的例子

> 部门 A 平均完成率最高，为 95.2%；部门 B 最低，为 82.1%。
> 来源：`SELECT department, AVG(completion_rate) AS avg_rate FROM sales_data GROUP BY department ORDER BY avg_rate DESC`（6 行）

### 不好的例子

> 部门 A 表现最好。← ✗ 无来源、无具体数值
> 大多数部门完成率集中在 85-90% 之间。← ✗ 模糊表述替代具体数字
> 自定义表格：部门 | 完成率 | 说明 ← ✗ 数据可能是编造的
> 所有数据均来自 SELECT * 查询。← ✗ 笼统归因

### SQL 报错时

如实报告错误信息，不假装查询成功，不编造替代结果。

## 5. 错误处理

区分两类排障场景，完整排障指南见 [`references/troubleshooting.md`](references/troubleshooting.md)。

### 数据库连接问题（9 种数据库类型适用）

| 现象 | 常见原因 | 处理 |
|------|---------|------|
| `connection refused` | 服务未启动/端口错误/防火墙拦截 | 检查服务状态和端口号 |
| `i/o timeout` | 网络延迟/防火墙丢包 | 检查网络连通性或增大 `--timeout` |
| `access denied` | 用户名/密码错误 | 让用户检查 `.env` 中的凭据 |
| `no such host` | DNS 无法解析主机名 | 确认主机名拼写或用 IP 替代 |
| `unsupported protocol` | DSN scheme 缺失或拼错 | 确认 scheme 前缀正确 |
| `no scanners configured` | 连接器未编译 | 确认 `dbexplain` 版本包含所需连接器 |

### 文件查询问题（CSV/TSV/XLSX 适用）

| 现象 | 常见原因 | 处理 |
|------|---------|------|
| `parse error` | SQL 语法不支持；引号用反 | 检查语法和引号用法 |
| `table "xxx" not found` | FROM 表名用了 label | 用文件名（不含扩展名） |
| `multiple DSNs matched` | 多数据源时缺 `--label` | 加 `--label` 参数 |
| `file not found` / `Instances (0)` | DSN 文件路径非绝对路径 | 使用绝对路径 |

---

## 6. 常用参数

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

## 7. 注意事项

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
