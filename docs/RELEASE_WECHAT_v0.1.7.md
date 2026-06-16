# dbexplain v0.1.7 发布：给 LLM 装上数据眼睛 + GaussDB Oracle 兼容 + 安全审计闭环

> **Prometheus meta 表 `rows` 输出**：LLM Agent 不再"盲猜" metric 名，直接消费语义化样本数据做 NL→PromQL 匹配。**GaussDB Oracle 兼容模式**：独立连接器让华为高斯数据库用户也能用上全部功能。同时完成全局代码审计，45 个发现，3 项修复。

---

![dbexplain 架构全景](assets/DBEXPLAIN-ARCH.png)

---

## 太长不看版

v0.1.7 做的不是"新功能"，而是**让现有功能真正可用**：

1. **👁️ Prometheus meta 表输出 `rows`** — VeinMap NL→PromQL 集成的 P0 阻塞解除。以前 `--json` 只能看到 `_labels` 和 `_metrics` 两张表的列名和行数，Agent 不知道 metric 名叫什么、type 是什么、help 描述是什么。现在全量数据直接写入 JSON 输出，Agent 拿来就能做语义匹配。

2. **🔒 CTE 写检测加固** — `WITH x AS (SELECT 1) INSERT INTO y VALUES(1)` 绕过只读校验的漏洞修复。一个 `break` 在 Go 的 `switch` + `for` 嵌套中只跳出 `switch` 的语义陷阱，排查了 3 小时。

3. **🔧 代码审计 45 个发现** — 3 个 real fix（MongoDB `$facet` 子管道写绕过、Hive TLS 自动跳过验证、凭证泄露到 metrics），42 个确认误报或理论问题。

4. **🇨🇳 GaussDB Oracle 兼容模式适配** — 华为高斯数据库在 Oracle 兼容模式下 `::regclass` 转换、EXPLAIN BUFFERS、statement_timeout 均不可用。以前勉强跑通，出错了也不知道为什么。现在：独立连接器 + 自动降级 + 完整文档。

> **诚实地说**：v0.1.7 功能变更不大。但它解决了两个 P0 阻塞问题——一个阻塞产品集成，一个阻塞安全合规，外加一个国内企业高频场景的兼容性痛点。

---

## 1. 👁️ Prometheus meta 表 rows 输出

### 背景：为什么这个功能重要？

dbexplain 的 `--json` 输出一直有一个"盲区"：Prometheus 的 `_labels` 和 `_metrics` 两张 meta 表只有列信息和行数，**没有实际数据**。

这对人类看没问题——看到 "row_count=657" 就知道有多少 metric。但 LLM Agent 不行。Agent 要做 NL→PromQL 语义匹配，它需要知道：

- 有哪些 metric 名？（`up`、`node_cpu_seconds_total`、`node_memory_MemTotal_bytes`...）
- 每个 metric 的 type 是什么？（counter / gauge / histogram / summary）
- help 描述是什么？（"The up scrape metric"、"CPU time in seconds"...）

没有这些数据，Agent 只能**猜 metric 名**，猜错了就出无效 PromQL。

### 解决：SampleRows 通用 Schema 扩展

```json
{
  "name": "_metrics",
  "columns": [
    {"name": "metric", "type": "text"},
    {"name": "type", "type": "text"},
    {"name": "help", "type": "text"},
    {"name": "unit", "type": "text"}
  ],
  "row_count": 657,
  "rows": [
    {"metric": "up", "type": "gauge", "help": "The up scrape metric", "unit": ""},
    {"metric": "node_cpu_seconds_total", "type": "counter",
     "help": "Seconds the CPUs spent in each mode", "unit": "seconds"}
  ]
}
```

两层映射：`schema.Table.SampleRows []map[string]any`（schema 层通用扩展点）→ JSON 渲染为 `rows`。

**关键设计**：`map[string]any` 保持通用性，`_labels` 只有 `name` 键，`_metrics` 有 `metric/type/help/unit` 四键。未来任意连接器都可以填充样本数据，消费端统一走 `rows` 路径。

### 影响

```
VeinMap NL→PromQL:   ❌ 盲猜 metric 名  →  ✅ 直接查 _metrics.rows
                     ❌ 不知道 type 含义 →  ✅ type=gauge 做适当聚合
                     ❌ 没有 help 描述   →  ✅ help 做语义匹配
```

---

## 2. 🔒 CTE 写检测加固

### 漏洞：WITH 伪装

```sql
WITH x AS (SELECT 1) INSERT INTO y VALUES(1)
```

这个 SQL 看起来有个 SELECT，实际上是个 INSERT。sqlast.Parse 可能会吞掉 WITH 前缀，把 `INSERT INTO y VALUES(1)` 当成一个有效 SELECT 解析，绕过只读校验。

### 修复：三层加固

```
1. WITH 检测提前到 AST 解析之前    — 不让 AST 有机会吞掉 WITH
2. 括号深度追踪扫描 CTE 体          — 只扫描真正在 CTE 内部的写动词
3. 主查询体写动词检查               — CTE 处理完后，剩下的主查询也要检查
```

### 血泪教训：labeled break

```go
// ❌ break 只跳出 switch，for 循环继续
for {                          // ← 想跳出这个
    switch {
    case ')':
        if depth == 1 { break } // ← 实际只跳出这个
    }
}
```

修这个 bug 花了 3 小时。调试过程：测试 `WITH x AS (SELECT 1) INSERT INTO y VALUES(1)` 发现 `lastParenEnd` 总是指向 `VALUES(1)` 的 `)` 而不是 CTE 体的 `)`。原因：`break` 跳出的是 `switch`，`for` 循环继续执行，读到 `VALUES(1)` 的 `)` 把正确的值覆盖了。

这个错误被写进项目宪法（CLAUDE.md）作为**所有 Go 开发者的必修课**。

---

## 3. 🔧 代码审计：45 个发现，3 个真问题

对全部 95 个 Go 文件做了系统性代码审计，覆盖 bug、错误处理、异常处置、安全策略 4 个维度。

### 审计方法论

| 维度 | 检查重点 | 扫描方式 |
|------|---------|---------|
| Bug | nil 指针、slice bound、竞态、逻辑错误 | 逐文件 + Go vet |
| 错误处理 | 静默吞错、漏检 | 逐文件 + grep `_ =` |
| 异常处置 | 缺 defer/recover、goroutine 泄漏 | 逐 goroutine 跟踪 |
| 安全策略 | sqlguard 绕过、凭证泄漏、policy 遗漏 | 安全工程师手动审查 |

### 发现分布

```
CRITICAL: 4  →  1 真问题（MongoDB $facet）
HIGH:     12 →  2 真问题（凭证泄漏、Hive TLS）
MEDIUM:   20 →  0 真问题（误报或理论攻击面）
LOW:      9  →  0 真问题（风格/可维护性）
```

45 个发现中最终只修了 3 个——不是因为别的不管，而是**反复验证后确认其他 42 个是误报或理论问题**。比如：

| 误报 | 审计说 | 实际 |
|------|--------|------|
| C4 nil store | `LoadStore` 失败 → `store.Diff()` 空指针 | `LoadStore` 返回 `s, err`，`s` 永远初始化 |
| M2 Redis SORT...STORE | SORT 写操作绕过 | `SORT` 不在 `redisReadOps`，已被拒绝 |
| C1 sqlguard AST 绕过 | AST 返回 SelectStmt 漏过写操作 | 已有 `isSelectInto` + CTE 预检测覆盖 |

**诚实比数量重要**。审计报告写了 45 个，我们就修了 3 个，42 个标注"已验证不修复"。

### 3 个修复

| # | 问题 | 严重程度 | 修复 |
|---|------|---------|------|
| C3 | MongoDB `$facet` 子管道写操作绕过 | **CRITICAL** | 递归遍历 `$facet` 值，检查所有嵌套子管道 |
| H2 | 凭证泄露到 metrics collector | **HIGH** | 使用 `config.SanitizeErr()` 脱敏后记录 |
| H3 | Hive TLS 自动跳过证书验证 | **HIGH** | 仅显式配置 `tls-skip-verify` 时禁用验证 |

### 验证闭环

```
go build -tags full ./cmd/dbexplain/  →  编译通过
go vet ./...                          →  静态分析通过
go test -tags full ./... -count=1     →  全部单元测试通过
bash build.sh                         →  5平台全量构建通过
```

---

## 4. 🇨🇳 GaussDB Oracle 兼容模式适配

### 背景：为什么 GaussDB 值得单独做？

在国内企业级市场，华为 GaussDB 是 PostgreSQL 兼容生态里**最特殊的那一个**。

说它"PG 协议兼容"没错——它确实能用 `lib/pq` 驱动连。但 Oracle 兼容模式下差异多得让人头疼：

| 差异 | 影响 |
|------|------|
| `::regclass` 类型转换不可用 | PG 标准写法，Oracle 模式报错 |
| EXPLAIN 不支持 `BUFFERS` 选项 | PG 连接器的 EXPLAIN 查询直接失败 |
| `statement_timeout` GUC 不识别 | 每次连接都报错到日志 |
| `pg_database.datistemplate` 列缺失 | 数据库列表查询报错 |
| 业务表不在 `public` schema | 每个表都要 `schema.tablename` 全限定 |

以前所有这些差异都被**捂着**——PG 连接器勉强能跑，但各种报错被 `COALESCE` 或 `defer` 吞掉了，用户不知道 GaussDB 的支持其实是有问题的。

### 解决：独立连接器 + 自动降级

v0.1.7 的架构决策：**不给 PG 连接器加补丁，给 GaussDB 单独开一个**。

```
postgresConnector  →  Register("postgres")  →  EXPLAIN(BUFFERS) ✅
                                                     ↓
gaussdbConnector   →  Register("gaussdb")   →  EXPLAIN(无BUFFERS) ✅
                    (新文件 gaussdb.go)       + ::regclass 自动规避
                                             + datistemplate 自动回退
                                             + 无 statement_timeout 噪音
```

**关键设计**：复用 `collectPGDB()`、`buildPGDSN()`、`executeSQLQuery()` 等包级函数，零代码重复。但 EXPLAIN 格式、超时设置、日志前缀各自独立。

### 文档

- 新增 `docs/databases/gaussdb.md` — 独立兼容性指南，包含 Oracle 模式说明、已验证兼容项列表、已知差异、分布式部署注意事项
- CLI 帮助（`dbexplain gaussdb`）现在正确标注 GaussDB 的已知限制，不再混在 PG 文档里

### 影响

```
GaussDB 用户：  ❌ 勉强跑通，报错被吞 →  ✅ 独立连接器，日志清晰
                ❌ EXPLAIN 查询失败    →  ✅ 自动降级 FORMAT TEXT
                ❌ 问题定位靠猜       →  ✅ 完整兼容性文档
```

---

## 5. 📊 版本演进

```
v0.0.2: 5 种数据源起步
v0.1.0: 9 种，CapSQL + 文件查询引擎
v0.1.3: + DuckDB，双版本构建
v0.1.4: + Prometheus 时序数据库
v0.1.5: + Oracle + Hive，15 种，六层安全管道
v0.1.6: Prometheus DSL 升级 + Bug Bash 21 项修复
v0.1.7: 👁️ Prometheus meta 表 rows + CTE 写检测加固 + GaussDB Oracle 兼容
```

**dbexplain — 15 种异构数据源的确定性上下文编译器：Schema 采集、只读查询、联邦 JOIN、安全审计，All in one 单二进制。**

---

## 6. 快速试用

```bash
# 1. 查看 Prometheus _metrics 的 rows 数据
dbexplain -dsn 'prometheus://your-host:9090?label=prom' --json | jq '.instances[].databases[].tables[] | select(.name == "_metrics") | {rows: .rows[:3]}'

# 2. 联邦查询：Prometheus + MySQL + 文件
dbexplain execute -env --dsl "
  SELECT p.instance, p.value, i.product, c.region
  FROM @my-prom.up p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  JOIN @my-csv.nodes c ON p.hostip = c.ip
" --human

# 3. 采集完整 Schema（含 meta 表 rows）
dbexplain -dsn 'prometheus://your-host:9090?label=prom' --json -o schema.json

# 4. GaussDB Schema 采集（Oracle 兼容模式）
dbexplain -dsn 'gaussdb://user:pass@host:25308/db?label=gauss-db&sslmode=disable' --human
```

---

## 写在最后

v0.1.7 没有"眼前一亮的新功能"。它做的是：**把欠的债还了，把堵的路通了**。

Prometheus meta 表 rows 这个改动，代码量不到 20 行。但对 LLM Agent 集成来说，**20 行代码 = P0 阻塞解除**。CTE 写检测修复也是——一个 labeled break，让一个隐藏了不知道多久的安全漏洞不再存在。

做工具就是这样：**90% 的工作是看不见的**。看得见的只有那 10% 的界面和功能。dbexplain 的目标一直不是"功能最多"，而是"能用、敢用"。

---

*项目开源协议：Apache 2.0*
*版本：v0.1.7 (2026-06-16)*
