# dbexplain v0.1.7 发布：GaussDB Oracle 兼容 + 安全加固 + LLM Agent 数据眼睛

> v0.1.7 解决了两个 P0 阻塞问题：一个阻塞安全合规（错误密码导致进程永久卡死），一个阻塞产品集成（Agent 盲猜 metric 名）。外加国内企业高频场景的 GaussDB Oracle 兼容适配。

---

![dbexplain 架构全景](assets/DBEXPLAIN-ARCH.png)

---

## 太长不看版

v0.1.7 的变更按**轻重**优先级排列：

| 优先级 | 变更 | 影响 |
|--------|------|------|
| 🔴 **CRITICAL** | SanitizeErr 死循环修复 | 28P01 认证失败导致**进程永久卡死**，任何有 GaussDB 的用户都可能触发 |
| 🟠 **HIGH** | CTE 写检测加固 | 安全漏洞：WITH 伪装的 SQL 可绕过只读校验 |
| 🟠 **HIGH** | 代码审计 45 项 | 3 个真实修复（MongoDB $facet 写绕过、Hive TLS、凭证泄露） |
| 🟠 **HIGH** | GaussDB Oracle 兼容 | 独立连接器 + 自动降级，企业高频场景 |
| 🟡 **MEDIUM** | Prometheus meta 表 rows | VeinMap NL→PromQL 集成 P0 阻塞解除 |

> **诚实地说**：功能变更不大。但 CRITICAL 的那一项，**不修的话用户不知道工具是卡死了还是在跑**。

---

## 1. 🔴 CRITICAL: SanitizeErr 死循环修复 — GaussDB 错误密码卡死 (ISSUE-095)

### 现象

`dbexplain check` 连接 GaussDB 输入错误密码时（28P01 报错），进程**永久卡死**，Ctrl+C 都救不了。用户等了一个小时后问："到底是网络问题还是工具问题？"

### 排查过程

层层过滤耗时两天：

```
[连接超时？]  → 排查 lib/pq context 取消 → 不是
[TCP 卡住？]  → 排查 goroutine 泄漏     → 不是
[mutex 死锁？] → 排查 sync.Mutex         → 不是
[日志卡住？]  → 加日志跟踪执行路径       → 定位 SanitizeErr！✅
```

### 根因

两层脱敏函数**互相冲突**：

| 函数 | 职责 | 输出格式 |
|------|------|---------|
| `d.Redacted()` | 显示时脱敏 | `gaussdb://{dbuser}:{dbpassword}@host:port/db` |
| `config.SanitizeErr()` | 错误消息中脱敏 | 查找 `://user:pass@host` 模式并替换 |

`Redacted()` 产生了一个**长得像真实 DSN 的占位符**。`SanitizeErr()` 看到它，以为还有密码未脱敏，尝试替换 `{dbpassword}` → `***`。替换后的字符串仍然匹配 URL 模式，于是继续替换 `***` → `***`，无限循环：

```
第1轮：{dbpassword} → ***  ✅ 替换成功
第2轮：*** → ***           ❌ 字符串没变但循环不退出
第3轮：*** → ***           ❌ 无限循环...
```

两个函数单独都对，但放在一起就互咬了。

### 修复

一行代码：**替换后无变化 → 退出循环**

```go
newMsg := msg[:passStart] + "***" + msg[passStart+atIdx:]
if newMsg == msg {
    break  // 替换前后字符串没变 → 已脱敏完毕
}
msg = newMsg
```

### 教训：多层脱敏必须幂等

这不是 SanitizeErr 的循环逻辑问题，而是**两个独立设计的脱敏函数之间没有约定接口协议**：

- `Redacted()` 的占位符格式不应该匹配 `SanitizeErr()` 的匹配模式
- 更通用的解法：**每层脱敏必须是幂等的**（第二次执行不产生副作用）

`newMsg == msg → break` 就是幂等性保证——**不管输入是什么，执行一次后字符串不再变化就退出**。这样即使将来再加第三层脱敏，也不会出同样的问题。

> 项目宪法 CLAUDE.md 已记录此陷阱作为安全红线的必修课。

### 防御加固

除了根本原因修复，还做了 3 项防御性加固（这三个在重现环境中不会触发，但同级风险）：

| # | 潜在风险 | 修复 |
|---|---------|------|
| 1 | `db.Close()` 在 lib/pq 异常时可能阻塞 | GaussDB/PostgreSQL 改 goroutine 关闭 `defer func() { go db.Close() }()` |
| 2 | collect for 循环中 `defer cancel()` 堆积 | 改为显式 `cancel()`，select 后立即执行 |
| 3 | `[DEBUG]` 日志默认写到 stderr | 移入 dbexplain.log，通过 `Logf()` 输出，预留 `--verbose` 控制（ISSUE-096） |

---

## 2. 🟠 HIGH: CTE 写检测加固

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

> 项目宪法 CLAUDE.md 已记录此 Go 陷阱作为所有开发者的必修课。

---

## 3. 🟠 HIGH: 代码审计 — 45 个发现，3 个真实问题

对全部 95 个 Go 文件做了系统性代码审计，覆盖 bug、错误处理、异常处置、安全策略 4 个维度。

### 发现分布

```
CRITICAL: 4  →  1 真问题（MongoDB $facet）
HIGH:     12 →  2 真问题（凭证泄漏、Hive TLS）
MEDIUM:   20 →  0 真问题（误报或理论攻击面）
LOW:      9  →  0 真问题（风格/可维护性）
```

45 个发现中只修了 3 个——不是因为别的不管，而是反复验证后确认其他 42 个是误报或理论问题：

| 误报示例 | 审计说 | 实际 |
|---------|--------|------|
| C4 nil store | `LoadStore` 失败 → `store.Diff()` 空指针 | `LoadStore` 返回 `s, err`，`s` 永远初始化 |
| M2 Redis SORT...STORE | SORT 写操作绕过 | `SORT` 不在 `redisReadOps`，已被拒绝 |
| C1 sqlguard AST 绕过 | AST 返回 SelectStmt 漏过写操作 | 已有 `isSelectInto` + CTE 预检测覆盖 |

**诚实比数量重要。** 42 个标注"已验证不修复"。

### 3 个真实修复

| # | 问题 | 严重 | 修复 |
|---|------|------|------|
| C3 | MongoDB `$facet` 子管道写操作绕过 | CRITICAL | 递归遍历 `$facet` 值，检查所有嵌套子管道 |
| H2 | 凭证泄露到 metrics collector | HIGH | 使用 `config.SanitizeErr()` 脱敏后记录 |
| H3 | Hive TLS 自动跳过证书验证 | HIGH | 仅显式配置 `tls-skip-verify` 时禁用验证 |

---

## 4. 🟠 HIGH: GaussDB Oracle 兼容模式适配

### 为什么 GaussDB 值得单独做？

在国内企业级市场，华为 GaussDB 是 PostgreSQL 兼容生态里最特殊的那一个。Oracle 兼容模式下差异多：

| 差异 | 影响 |
|------|------|
| `::regclass` 类型转换不可用 | PG 标准写法，Oracle 模式报错 |
| EXPLAIN 不支持 `BUFFERS` 选项 | PG 连接器的 EXPLAIN 查询直接失败 |
| `statement_timeout` GUC 不识别 | 每次连接都报错到日志 |
| `pg_database.datistemplate` 列缺失 | 数据库列表查询报错 |
| 业务表不在 `public` schema | 每个表需 `schema.tablename` 全限定 |

以前这些差异被各种 `COALESCE` 或 `defer` 吞掉了，用户不知道 GaussDB 的支持其实有问题。

### 解决：独立连接器 + 自动降级

架构决策：**不给 PG 连接器加补丁，给 GaussDB 单独开一个**。

```
postgresConnector  →  Register("postgres")  →  EXPLAIN(BUFFERS) ✅
                                                     ↓
gaussdbConnector   →  Register("gaussdb")   →  EXPLAIN(无BUFFERS) ✅
                    (新文件 gaussdb.go)       + ::regclass 自动规避
                                             + datistemplate 自动回退
                                             + 无 statement_timeout 噪音
```

**关键设计**：复用 `collectPGDB()` 等包级函数，零代码重复。但 EXPLAIN 格式、超时设置、日志前缀各自独立。

---

## 5. 🟡 MEDIUM: Prometheus meta 表 rows 输出

### 背景

dbexplain 的 `--json` 输出一直有一个盲区：Prometheus 的 `_labels` 和 `_metrics` 表只有列信息和行数，**没有实际数据**。

LLM Agent 要做到 NL→PromQL 语义匹配，需要知道 metric 名、type、help 描述。没有这些数据，Agent 只能**猜 metric 名**。

### 解决

`schema.Table.SampleRows []map[string]any` 通用扩展点 → JSON 渲染为 `rows`：

```json
{
  "name": "_metrics",
  "rows": [
    {"metric": "up", "type": "gauge", "help": "The up scrape metric"},
    {"metric": "node_cpu_seconds_total", "type": "counter",
     "help": "Seconds the CPUs spent in each mode"}
  ]
}
```

**代码量不到 20 行，但对 LLM Agent 集成来说是 P0 阻塞解除。**

### 影响

```
VeinMap NL→PromQL:   ❌ 盲猜 metric 名  →  ✅ 直接查 _metrics.rows
                     ❌ 不知道 type 含义 →  ✅ type=gauge 做适当聚合
                     ❌ 没有 help 描述   →  ✅ help 做语义匹配
```

---

## 6. 📊 版本演进

```
v0.0.2: 5 种数据源起步
v0.1.0: 9 种，CapSQL + 文件查询引擎
v0.1.3: + DuckDB，双版本构建
v0.1.4: + Prometheus 时序数据库
v0.1.5: + Oracle + Hive，15 种，六层安全管道
v0.1.6: Prometheus DSL 升级 + Bug Bash 21 项修复
v0.1.7:  CRITICAL SanitizeErr 死循环 + HIGH CTE 写检测 + 代码审计
         + HIGH GaussDB Oracle 兼容 + MEDIUM Prometheus meta 表 rows
```

**dbexplain — 15 种异构数据源的确定性上下文编译器：Schema 采集、只读查询、联邦 JOIN、安全审计，All in one 单二进制。**

---

## 7. 快速试用

```bash
# 1. 查看 Prometheus _metrics 的 rows 数据
dbexplain -dsn 'prometheus://your-host:9090?label=prom' --json \
  | jq '.instances[].databases[].tables[] | select(.name == "_metrics") | {rows: .rows[:3]}'

# 2. GaussDB Schema 采集（Oracle 兼容模式）
dbexplain -dsn 'gaussdb://user:pass@host:25308/db?label=gauss-db&sslmode=disable' --human

# 3. 联邦查询：Prometheus + MySQL + 文件
dbexplain execute -env --dsl "
  SELECT p.instance, p.value, i.product, c.region
  FROM @my-prom.up p
  JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip
  JOIN @my-csv.nodes c ON p.hostip = c.ip
" --human

# 4. 检查配置连通性
dbexplain check -env
```

---

## 写在最后

v0.1.7 没有眼前一亮的新功能。它做的是：**把坑填了，把路通了**。

CRITICAL 的那个 SanitizeErr 死循环，如果不修，用户永远不知道工具是卡死了还是在跑——这是最不能接受的事情。一个诊断工具如果连自己都诊断不了，还怎么相信它诊断的结果？

做工具就是这样：**90% 的工作是让那 10% 的功能不出问题**。dbexplain 的目标不是"功能最多"，而是"能用、敢用"。

---

*项目开源协议：Apache 2.0*
*版本：v0.1.7 (2026-06-16)*
*ISSUE-095 (SanitizeErr 死循环) + ISSUE-096 (--verbose 日志级别控制 计划)*
