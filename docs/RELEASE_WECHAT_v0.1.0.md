
# 那个给 AI 用的数据库工具，现在能直接分析 Excel 了：v0.1.0发布

> 纯 Go 内存 SQL 引擎 + 11 种数据源 + 三层安全防护。
> 不需要数据库就能跑 SQL 查询，AI Agent 直接分析 CSV/Excel —— 一个二进制，零依赖。

---

如果你还没听说过 **dbexplain**：它是一个专为 AI Agent 设计的数据库 Schema 上下文生成工具。但这次 v0.1.0 发布之后，它的能力已经远远超出了"Schema 采集"的范畴。

最核心的变化是：**现在它能直接对 CSV 和 Excel 文件执行 SQL 查询了**。不是调用外部引擎，而是内置了一个纯 Go 实现的内存 SQL 引擎。

```
数据源类型:    11 种
引擎单元测试:  66+
QA 场景:      15 个
外部依赖:     0
```

---

## 一、内置 SQL 引擎：从此 CSV 不再是二等公民

这是本次发布最重磅的能力。v0.1.0 新增了**纯 Go 实现的内存 SQL 查询引擎**，支持对 CSV/TSV/XLSX 文件执行完整的 SELECT 查询：

```bash
# WHERE 过滤 + 列投影
dbexplain execute -env --label my_data \
  'SELECT employee_id, completion_rate FROM sales_data WHERE completion_rate < 60' --human

# GROUP BY + 聚合
dbexplain execute -env --label my_data \
  'SELECT department, AVG(rate) AS avg_rate FROM data GROUP BY department ORDER BY avg_rate DESC' --human

# 跨文件 JOIN（CSV ↔ XLSX）
dbexplain execute -env --label my_data \
  'SELECT o.branch_name, AVG(t.completion_rate) FROM sales_data t JOIN org_info o ON t.dept_id = o.dept_id GROUP BY o.branch_name' --human

# 列间算术 + CAST 类型转换
dbexplain execute -env --label my_data \
  'SELECT employee_id, CAST(channel_cnt AS FLOAT) / total_cnt * 100 AS pct FROM data WHERE total_cnt > 0' --human
```

### 支持的语法全貌

| 类别 | 能力 |
|------|------|
| WHERE 过滤 | `=` `!=` `<` `>` `LIKE` `IN` `BETWEEN` `IS NULL` `AND`/`OR`/`NOT` |
| GROUP BY + 聚合 | `SUM`/`AVG`/`COUNT`/`MAX`/`MIN` + `HAVING` + `COUNT(DISTINCT col)` |
| 跨文件 JOIN | CSV↔CSV、XLSX↔XLSX、CSV↔XLSX，`LEFT`/`RIGHT JOIN` |
| 表达式 + 函数 | 列间算术、`CAST`、`ABS`、`ROUND`、子查询 `IN`/`NOT IN` |
| UNION + DISTINCT ON | 跨表合并结果，每组保留首行 |
| NULLS FIRST/LAST | 空值排序位置精确控制 |

> **为什么这很重要？** 业务数据大量以 CSV/Excel 形式存在，但传统 CLI 工具面对结构化查询力不从心。现在 AI Agent 可以用标准 SQL 直接分析这些文件，不需要导入数据库、不需要写 Python 脚本。

引擎底层是递归下降解析器 + 哈希 JOIN 引擎 + 哈希聚合器，全部用 Go 手写，零外部依赖。66 个单元测试覆盖所有语法路径和边界条件。

---

## 二、数据源矩阵：从 9 到 11

v0.0.9 新增了 CSV/TSV 和 XLSX 两类文件数据源，使总数据源数达到 11 种：

| 数据源 | 说明 |
|--------|------|
| MySQL | 外键、索引、字段注释推断 |
| PostgreSQL | 多 Schema、行数统计 |
| GaussDB | 兼容 pg 协议 |
| ClickHouse | 排序键/分区键 |
| SQLite | 纯 Go，零 CGO |
| Redis | 键模式推断，集群 |
| MongoDB | 近似文档数 |
| Elasticsearch | 索引映射，HTTPS |
| Qdrant | 向量集合元数据 |
| **CSV/TSV** 🆕 | 文件查询引擎：SQL 完整子集 |
| **XLSX Excel** 🆕 | 多 Sheet JOIN，跨格式 JOIN |

---

## 三、三层安全防护的只读查询

`execute` 子命令经过三个版本打磨，提供了一个**三层安全防护**的只读查询环境：

1. **sqlguard 只读校验**：动词白名单（SELECT/EXPLAIN/SHOW/DESC/PRAGMA），多语句检测，自动 LIMIT 1000 注入
2. **策略引擎细粒度访问控制**：表级拒绝（DENY_TABLES）、列级屏蔽（MASK_COLUMNS）、语句级黑名单
3. **双超时保护**：应用层 context 超时 + 数据库层语句超时

非 SQL 数据库各有专属查询方式：

```bash
# MongoDB：JSON 格式查询
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}'

# Redis：原生命令（30+ 只读命令白名单）
dbexplain execute -env --label redis 'GET user:1001'

# ES：通过 _sql 端点执行标准 SQL
dbexplain execute -env --label es 'SELECT * FROM my_index LIMIT 5'
```

---

## 四、策略引擎：细粒度访问控制

v0.0.8 引入的策略引擎，在 v0.1.0 中得到了充分验证和安全加固。4 种策略对所有 11 种数据源一致生效：

| 策略 | 作用 | 示例 |
|------|------|------|
| **DENY_TABLES** | 表级禁止访问 | `DENY_TABLES=sensitive,audit_log` |
| **DENY_COLUMNS** | 列级禁止访问 | `DENY_COLUMNS=users.password_hash` |
| **MASK_COLUMNS** | 列值替换不阻断 | `MASK_COLUMNS=email=REDACTED,card_no=****` |
| **DENY_STATEMENTS** | 语句级黑名单 | `DENY_STATEMENTS=DROP TABLE,FLUSHALL` |

v0.1.0 还修复了多个 P0 级别的安全绕过漏洞：

- **WITH CTE 写操作绕过**：`WITH ... INSERT/UPDATE/DELETE` 深层扫描修复
- **SELECT INTO 绕道**：`SELECT * INTO new_table` DDL 检测修复
- **SET SESSION 连接池竞态**：MySQL/PG 超时设置跨连接失效修复
- **策略配置泄漏**：`loadEnvFile` env 泄漏到 `/proc/[pid]/environ` 修复

配合 v0.0.6 引入的硬件绑定加密，现在整个链路从凭证存储 → 传输 → 使用都有了保障。

---

## 五、第三方分发包：AI Agent 开箱即用

这次发布首次测试了**完整的业务包**（`testdata/account-manager-skill/`，该技能为特定场景，未开源），包含：

- **5 平台预编译二进制**：Linux amd64/arm64、macOS Intel/Silicon、Windows
- **完整的 AI Agent Skill**：SKILL.md 包含语法速览、排障指南、可追溯分析规范
- **一键离线安装**：`bash scripts/install.sh --offline ./assets/dbexplain-linux-amd64`

> **macOS 用户注意**：安装脚本已自动处理 Gatekeeper 拦截问题，无需手动操作。

---

## 六、一个真实场景：客户经理触达分析

想象一下：市场部发来一个 CSV 文件——客户经理触达运营数据，40 列、2000 行。你的 AI Agent 需要分析哪个省份触达率最高、哪个渠道最有效。

以前你需要：写 Python 脚本 → pandas 处理 → 调格式 → 出报告。现在：

```bash
# 配置数据源
echo 'DB1=csv:///path/to/touch_data.csv?label=touch-ops' > .env.dbexplain

# 跨省排行
dbexplain execute -env --label touch-ops \
  'SELECT province, AVG(reach_rate) AS avg_rate FROM data GROUP BY province ORDER BY avg_rate DESC' --human

# 渠道效能（企微占比排名）
dbexplain execute -env --label touch-ops \
  'SELECT employee_id, CAST(wecom_cnt AS FLOAT) / total_cnt * 100 AS wecom_pct FROM data WHERE total_cnt > 0 ORDER BY wecom_pct DESC LIMIT 5' --human

# 跨文件 JOIN：关联组织架构做二级行排名
dbexplain execute -env --label touch-ops \
  'SELECT o.branch_name, AVG(t.reach_rate) FROM data t JOIN org o ON t.dept_id = o.dept_id GROUP BY o.branch_name ORDER BY avg_rate DESC' --human
```

**全部在命令行完成，零脚本，零依赖。**

---

## 七、CapSQL 架构：新增数据库不再改 execute.go

v0.1.0 完成了一项重要的架构改进：**CapSQL 能力声明**。每个 Connector 现在声明自己的能力标签（CapSQL / CapFile），execute 子命令根据能力标签自动路由，不再需要硬编码的类型分支：

```go
// 之前：新增数据库要改 execute.go 的 switch
// 现在：Connector 声明 CapSQL → execute 自动识别
func (c *MySQLConnector) Capabilities() capabilities.Set {
    return capabilities.NewSet(capabilities.CapSQL)
}
```

这意味着**新增数据库连接器不再需要修改 execute.go**，架构更加清晰、可扩展。

---

## 八、完整变更清单

### v0.0.9（文件处理 + 安全修复）

| 类别 | 内容 |
|------|------|
| 数据源 | CSV/TSV/XLSX 文件 Schema 采集 |
| 编码 | GBK/GB2312 编码支持，自定义分隔符 |
| 单二进制 | XLSX 合并到主模块，零运行时依赖 |
| 日志 | 采集进度日志解耦，stderr 不再被污染 |
| CLI | `--human` 放查询语句后也生效 |
| 安全修复 | schema 前缀匹配、`SELECT *` 绕过、MongoDB 原生查询绕过 |

### v0.1.0（SQL 引擎 + 安全加固 + 架构升级）

| 类别 | 内容 |
|------|------|
| 安全加固 | CTE/SELECT INTO 绕过、SET SESSION 竞态、策略泄漏修复 |
| 正确性 | PG FK Schema JOIN、索引解析、cache 原子写入、ORDER BY 别名排序 |
| SQL 引擎 | 纯 Go SQL 引擎 + GROUP BY/JOIN/聚合/表达式/UNION/子查询/NULLS/HAVING |
| 架构 | CapSQL 能力声明，新增数据库不再改 execute.go |
| JSON | `instances` 包装格式、groups/issues/refs 顶级键 |
| 分发包 | account-manager-skill 第三方分发包定型 |
| macOS 兼容 | Gatekeeper quarantine 自动移除 |
| 最佳实践 | 可追溯分析、9+3 错误分类、references/ 独立文档 |

---

## 写在最后

从 initial commit 到 v0.1.0，dbexplain 在两周内完成了从"Schema 采集工具"到"数据库上下文编译器 + 文件查询引擎"的蜕变。

这个项目的核心理念一直没变：**Make Databases AI-Readable**。但 v0.1.0 迈出了关键的一步——不再只是把数据库结构喂给 AI，而是让 AI 能**直接对文件数据和数据库执行只读查询**，获取真实的业务洞察。

接下来的路线：MCP Server 协议支持、企业级 SSO 集成、更多 SQL 方言兼容。

欢迎试用、Star、提 Issue。

---

**项目信息**

- GitHub: [github.com/IamWWT/dbexplain](https://github.com/IamWWT/dbexplain)
- 完整 CHANGELOG: [CHANGELOG.md](https://github.com/IamWWT/dbexplain/blob/main/CHANGELOG.md)
- 文件查询引擎 QA: [docs/test/13-file-query-engine.md](https://github.com/IamWWT/dbexplain/blob/main/docs/test/13-file-query-engine.md)

*dbexplain — Make Databases AI-Readable. Apache 2.0 © 2026 WWT.*
