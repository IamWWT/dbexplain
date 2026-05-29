# dbexplain Architecture Vision

## 1. 项目定位

`dbexplain` 正式定义为 **Database Context Compiler**（数据库上下文编译器），而非"数据库分析工具"。

这个重新定义意味着：

| 旧定义 | 新定义 |
|--------|--------|
| 数据库分析工具 | Database Context Compiler |
| 输出报告给人看 | 输出 IR 给 AI 消费 |
| 按数据库类型组织 | 按通用图原语组织 |
| report-first | graph-first |

长期战略目标：

```
成为 AI Runtime 的 Database Ground Truth Layer
```

即：为 AI Agent 提供真实、确定性的数据库上下文信息层，而非用 LLM 替代 ground truth。

---

## 2. 核心哲学

### 唯一原则

```
dbexplain 保持 deterministic
LLM 在外部消费 IR 做推理
```

这意味着：
- dbexplain **只输出可证实的事实**
- 语义理解、总结、推理全部交给外部的 LLM
- dbexplain 的职责是成为 LLM 的"眼睛"，而非"大脑"

### 输出边界

| 允许（Deterministic Facts） | 禁止（AI Semantic） |
|---|---|
| 外键关系（DDL 声明） | "这是订单系统" |
| 列名、类型、可空性 | "status 表示支付状态" |
| 索引结构 | AI 关系猜测 |
| 命名推断的关系（_id 模式匹配） | LLM 生成的总结 |
| Redis 键 TTL 统计 | embedding-first 分析 |

---

## 3. 目标架构

### 当前目录结构（v0.1.0，实际代码）

```
src/
  main.go               # CLI 入口 + 配置加载
  execute.go            # 只读查询执行（sqlguard + policy + AutoLimit）
  capabilities/         # Capability 枚举（含 CapSQL/CapFile v0.1.0 新增）
  connector/            # 连接器自注册（11 种数据源）
  sqlguard/             # SQL 只读校验（P0 安全边界）
  policy/               # 细粒度访问控制（DENY_TABLES/COLUMNS/STATEMENTS）
  query/                # Queryable 接口 + 执行控制
  cache/                # Schema 指纹 + 增量扫描
  dsn/                  # DSN 解析 + 凭据脱敏
  schema/               # 通用数据模型（Instance/Table/Column/ForeignKey）
  ir/                   # IR v1 图原语（Node/Column/Edge）
  graph/                # 内部图模型
  core/                 # 公共 API（Collect/CollectToGraph/CollectToJSON）
  analyze/              # 聚类分析 + 重要性排序
  diagnostics/          # 统一诊断层
  context/              # AI Agent 上下文压缩
  render/               # Markdown/JSON 输出
  crypto/               # XChaCha20-Poly1305 加密
```

### 未来目录结构（规划中）

`internal/` 重构推迟到 v1.0 之后。包边界稳定后再做结构迁移：

```
internal/
  connectors/       # 仅负责：连接数据库
  capabilities/     # 声明数据库能力枚举
  extractors/       # 按 capability 工作，提取元数据
  analyzers/        # 通用分析（relation inference, clustering）
  diagnostics/      # 统一诊断层（从 connector 中抽离）
  graph/            # 图模型（Node, Column, Edge）
  ir/               # IR v1 定义和序列化
  renderers/        # Markdown / JSON / HTML 输出
  cache/            # Schema fingerprint + delta scan
  diff/             # Schema diff (未来)
```

### Capability Architecture（v0.1.0 已落地）

Capability 架构已落地在 `execute.go`（v0.1.0），`isSQLKind()` 硬编码 switch 已被删除。

当前架构（反模式）：

```go
if mysql { ... }
if postgres { ... }
if redis { ... }
```

当前架构（Capability-driven，v0.1.0 已落地）：

```go
type Capability string

const (
    CapForeignKey   Capability = "foreign_key"
    CapSampling     Capability = "sampling"
    CapTTL          Capability = "ttl"
    CapPartition    Capability = "partition"
    CapVector       Capability = "vector"
    CapRowCount     Capability = "row_count"
    CapIndex        Capability = "index"
    CapSQL          Capability = "sql"    // v0.1.0 新增
    CapFile         Capability = "file"   // v0.1.0 新增
)

// Connector 声明自己支持哪些能力
type Connector interface {
    Collect(ctx, dsn) (*Instance, error)
    Capabilities() []Capability
}

// execute.go 使用 CapSQL 路由
caps := capabilities.FromProvider(c)
if caps.Has(capabilities.CapSQL) {
    sqlguard.Validate(sqlArg)       // SQL 校验
    policies.CheckSQL(sqlArg)        // SQL 策略
    sql = sqlguard.AutoLimit(sql)    // 自动 LIMIT
} else {
    policies.CheckNative(sqlArg)     // 原生校验
}
```

**关键收益**：新增数据库类型不需要修改 pipeline。只需实现 Connector + 声明 CapSQL（若支持 SQL）。

**关键改进（v0.1.0）**：`isSQLKind()` 硬编码 switch 已被 `capabilities.FromProvider().Has(CapSQL)` 替代。宪法第 10 条已落地。

---

## 4. IR v1 设计（最高优先级）

Internal Representation (IR) 是项目最重要的资产。设计为通用图原语，独立于数据库类型。

### Node

```json
{
  "id": "mysql.prod.orders",
  "kind": "table",
  "engine": "mysql",
  "name": "orders",
  "metadata": {
    "row_count": 42000,
    "size_bytes": 1572864
  }
}
```

### Column

```json
{
  "id": "mysql.prod.orders.user_id",
  "kind": "column",
  "data_type": "bigint",
  "nullable": false,
  "is_primary": false,
  "is_unique": false,
  "comment": "用户标识符"
}
```

### Edge

```json
{
  "source": "mysql.prod.orders.user_id",
  "target": "mysql.prod.users.id",
  "edge_type": "declared_fk",
  "confidence": 100
}
```

### Edge Types

| Type | 含义 | 来源 |
|------|------|------|
| `declared_fk` | DDL 声明的外键 | 显式 FK 约束 |
| `inferred_ref` | 命名推测的引用 | `*_id` → `*` 模式匹配 |
| `index_edge` | 索引关系 | 索引列 |
| `cluster_edge` | 聚类关系 | 多表共享引用链 |

### 设计原则

- **不允许** AI semantic 进入 IR
- **长期兼容** — IR 格式可持续演进，但 v1 必须稳定
- **类型无关** — 同一个 IR 表示 MySQL、PostgreSQL、Redis、MongoDB 的结构

---

## 5. Context Compression

AI Agent 最大的瓶颈是 context window。需要输出多层上下文，让 Agent 按需加载。

### 输出层次

#### 1. `summary.json` — 快速概览

```json
{
  "total_tables": 500,
  "total_instances": 9,
  "core_tables": ["orders", "users", "payments"],
  "largest_tables": [
    {"name": "logs", "rows": 12000000}
  ],
  "highly_connected_tables": [
    {"name": "users", "degree": 23}
  ],
  "hot_tables": []
}
```

#### 2. `topology.json` — 拓扑结构

```json
{
  "subgraphs": [
    {"name": "order-flow", "tables": ["orders", "payments", "shipments"]}
  ],
  "isolated_tables": ["config_cache"],
  "cycles": []
}
```

#### 3. `diagnostics.json` — 问题清单

```json
{
  "missing_pk": [],
  "unindexed_fk": [
    {"table": "orders", "column": "user_id"}
  ],
  "redis_no_ttl": [
    {"key_pattern": "session:{hex}"}
  ]
}
```

#### 4. `retrieval_chunks/` — 单表上下文

```
chunks/orders.md
chunks/users.md
chunks/payments.md
```

每份文件包含单个"核心表"的完整上下文，供 Agent retrieval 使用。

### Importance Ranking

全部使用 deterministic 算法计算：

| 维度 | 计算方法 |
|------|----------|
| graph_degree | 图出度 + 入度 |
| fk_centrality | 被外键引用的次数 |
| row_count | 量化的表大小 |
| index_density | 索引数 / 列数 |
| join_frequency | pg_stat_statements / query_log (未来) |
| write_intensity | keyspace stats (未来) |

输出：

```json
{
  "table": "orders",
  "importance": 0.98,
  "factors": {
    "graph_degree": 0.8,
    "fk_centrality": 0.95,
    "row_count": 0.9
  }
}
```

---

## 6. 增量扫描

企业库全量扫描不可行。必须支持 delta scan。

### Schema Fingerprint

```go
type TableFingerprint struct {
    Name      string
    Columns   string // hash of column names + types
    Indexes   string // hash of index definitions
    FKs       string // hash of FK definitions
    ScannedAt time.Time
}
```

两次扫描相同 fingerprint = 跳过。

---

## 7. Query-Aware Metadata（Operational Semantics）

不依赖 AI 推理，而是采集真实的**行为事实**：

| 数据库 | 数据来源 | 提取信息 |
|--------|----------|----------|
| PostgreSQL | `pg_stat_statements` | join 频率、查询模式 |
| MySQL | `performance_schema` | 查询统计 |
| ClickHouse | `system.query_log` | 查询日志 |
| Redis | keyspace stats | 读写比例 |

提取结果不属于"AI 推理"，而是**可观测的行为事实**。对 LLM 推理极为重要。

---

## 8. 产品拆分

### 1. CLI Product（当前已有）

- 定位：数据库巡检工具
- 用户：DBA / 后端 / 运维 / AI Agent
- 输出：Markdown + Diagnostics + JSON
- 交互特性：
  - `-h` 7 组分栏帮助（数据源/过滤/输出控制/显示格式/AI 上下文/性能/帮助），中英双语
  - `all` / `<dbtype>` 完整手册约 600 行，`--filter <关键字>` 按行过滤快速查找
  - `--human` 上下文标记输出（`[table=]`/`[pattern=]`/`[database=]` 等按数据库类型自适应）
  - `-o` 文件输出自动添加 UTF-8 BOM（Windows 兼容）
  - 终端直接渲染含 ANSI 颜色高亮

### 2. IR Product（未来核心）

- 定位：AI Agent Context Compiler
- 输出：Graph + Summary + Retrieval Chunks + Diagnostics + Topology
- 消费方：AI Agent

---

## 9. 安全性

作为工具与外部系统之间的边界层，安全性是架构的硬约束，不是可选特性。

### 第一要义：密码绝不泄漏

**任何代码路径都不得将数据库密码写入标准输出、标准错误、日志文件或任何持久化存储。**

| 路径 | 要求 |
|------|------|
| DSN 解析 | `DSN.Redacted()` 在所有日志/输出中替代原始 DSN |
| 错误消息 | 过滤 DSN 时的 skip 消息必须使用脱敏后的 DSN |
| 日志文件 | `logs/<label>.log` 中不得出现原始密码 |
| JSON 输出 | 永不包含连接字符串 |
| 终端输出 | 所有 DSN 回显均经过 `Redacted()` |

**正确示例：**
```go
// 正确：使用脱敏后的 DSN
log.Printf("skipping %s (not matched by include filter)", parsed.Redacted())

// 错误：直接使用原始 DSN（密码泄漏！）
log.Printf("skipping %s (not matched by include filter)", e.raw)
```

### 其他安全原则

- **只读操作**：所有 Connector 仅执行 `SELECT`/`SHOW`/`SCAN`/`PRAGMA` 等只读操作
- **参数化查询**：使用参数化查询或标准 API，防止 SQL/命令注入
- **采样上限**：Redis 限制 2000 键、5 字段、512 字节、10 条流消息
- **元数据优先**：MongoDB 使用 `EstimatedDocumentCount`，不做全表扫描
- **独立隔离**：每实例独立日志文件，单实例 panic 不影响其他实例采集

### 配置文件加密 (v0.0.6)

通过硬件绑定加密保护 `.env` 中的敏感配置，防止明文密码泄露。

**两种模式：**
| 模式 | 密钥派生 | 使用方式 |
|------|----------|----------|
| 机器指纹（默认） | `SHA-256(硬件特征)` | `dbexplain encrypt` |
| 密码增强 | `PBKDF2-HMAC-SHA256(密码, 指纹, 100k)` | `dbexplain encrypt --password` |

**运行时透明解密：**
- `loadEnvFile()` 自动检测加密文件（首字节 0x00/0x01）
- 密码模式通过 `~/.config/dbexplain/.encryption_key` 文件提供解密密码（`APP_ENCRYPTION_KEY` 环境变量作为可选覆盖）
- 不使用加密文件时行为与 v0.0.5 完全一致（向后兼容）

**密钥层次：**
```
Machine-Only Mode:
  硬件特征 → SHA-256 → [32-byte key] → XChaCha20-Poly1305

Password Mode:
  密码 ──────────────────┐
                         ├─ PBKDF2-HMAC-SHA256(100k) → [32B key] → XChaCha20-Poly1305
  硬件特征 (salt 成分) ──┘
```

**文件格式：**
```
[1B mode][16B salt?][24B nonce][ciphertext + 16B tag]
```

**平台指纹来源：**

| 平台 | 核心数据源 | 备用 |
|------|-----------|------|
| Linux | `/etc/machine-id`, `/sys/class/dmi/id/product_uuid` | `/proc/cpuinfo`, hostname |
| macOS | `sysctl hw.uuid`, `hw.model`, `hw.machine` | `hw.memsize`, hostname |
| Windows | `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid` | hostname |

> 全面的安全检查清单见 [SECURITY_CHECKLIST.md](./SECURITY_CHECKLIST.md)，发布前必须逐项确认。

---

## 10. 明确不做

这些方向与 deterministic 哲学冲突，明确排除：

1. **AI 总结** — 不做"这是订单系统"类总结
2. **业务语义推测** — 不做"status 表示支付状态"类猜测
3. **AI 关系猜测** — 不做 LLM-based relation inference
4. **Embedding-first** — 先做 deterministic graph，不做向量化优先级

### 已知限制（v0.1.0）

这些是当前实现层面的已知限制，不改变核心定位：

1. **Streaming 输出** — 全量 Schema 在内存组装后输出，不支持流式。v1.0 前不实现
2. **Schema Diff** — `diff/` 包尚未实现，`cache.Delta` 提供基础差分能力
3. **MCP Server / IDE 集成** — 未来考虑独立仓库实现，dbexplain 不内嵌 serve 模式
4. **CSV/XLSX 定位** — 视为"文件数据源"而非"数据库"，不扩展更多文件格式（Parquet/Avro）
5. **数据库层只读双保险** — 工具层 sqlguard + policy 为第一道防线，强烈建议配合数据库 GRANT SELECT ONLY 使用

---

## 11. 发展路线

### Phase 1（已完成 — v0.0.4）

- [x] IR v1 定义：Node / Column / Edge schema
- [x] Graph model：内部统一图模型
- [x] Capability system：从 `if mysql` 重构为 `if Has(CapFK)`
- [x] Deterministic diagnostics 抽离到统一层
- [x] IR 序列化为 JSON 输出

### Phase 2（已完成 — v0.0.4）

- [x] Context Compression：summary / topology / diagnostics JSON
- [x] Importance Ranking（deterministic 算法）
- [x] Retrieval Chunks：单表上下文文件
- [x] Delta Scan：Schema fingerprint + 增量更新

### Phase 3（已完成 — v0.0.4）

- [x] Query-Aware Metadata：pg_stat_user_tables / query_log / INFO stats
- [x] Operational Graph：基于真实查询的关系图
- [x] 兜底机制：不可用数据源静默跳过，因子权重自动重新归一化

### v0.1.0 安全加固里程碑（2026-05-29）

- [x] sqlguard P0 绕过修复：WITH CTE 写操作 + SELECT INTO
- [x] sqlguard P1 加固：ANALYZE/REINDEX 移至黑名单、连接池竞态修复
- [x] policy 引擎双修复：matchStarSelect 全线检测、配置不再泄漏到 os.Environ
- [x] postgres 正确性双修复：FK schema JOIN、索引字符串解析
- [x] cache 原子写入：temp file + os.Rename
- [x] **Capability 架构落地**：isSQLKind() 删除 → capabilities.FromProvider().Has(CapSQL)
- [ ] MCP Server（战略级，独立项目）
- [ ] Cursor / OpenHands / Aider 集成（独立项目）
- [ ] 企业级 diff / lineage / governance
- [ ] Cloud scan orchestration

---

## 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-05-29 | v4 | v0.1.0: CapSQL 架构落地、P0/P1 安全加固；新增已知限制章节 |
| 2026-05-20 | v3 | 新增安全性章节，密码防泄漏为第一要义 |
| 2026-05-20 | v2 | Phase 1-3 已完成，更新路线图状态 |
| 2026-05-20 | v1 | 初始架构愿景，基于架构评审建议 |
