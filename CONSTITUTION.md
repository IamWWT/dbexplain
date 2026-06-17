# dbexplain 项目宪法

## 项目定位

`dbexplain` 是一个 **Database Context Compiler**（数据库上下文编译器），为 AI Agent 提供**确定性、可证实**的数据库结构信息层。

核心哲学：**dbexplain 只输出 deterministic facts，LLM 在外部消费 IR 做推理**。

### 消费方

- **AI Agent**：通过 `dbexplain-skill/SKILL_ZH.md` / `SKILL_EN.md` 定义的技能接口调用，Agent 读取 stdout 中的 Markdown 报告或 `-json` 输出的结构化数据
- **人类运维/DBA**：直接在终端执行，阅读格式化报告或 JSON 输出进行数据库巡检和结构分析

### 核心交付物

一个**单文件静态二进制**，无运行时依赖（无 CGO、无 libc 版本依赖、无外部进程），可跨 5 平台直接运行。

最终交付物包括：
- **JSON schema 输出**（`--json`）：结构化 IR，供 AI Agent 消费
- **Markdown / 人类可读报告**（`--human` / 默认终端输出）：DBA/运维向
- **查询执行结果**（`execute` 子命令）：只读查询的标准化输出
- **AI 上下文导出**（`--context`）：为 LLM 预处理的上下文文件

> 注意：内部图模型（Graph/Node/Edge）是分析 pipeline 的实现细节，不作为独立交付物暴露。

---

## 核心原则

### 1. Connector 自注册（开闭原则）

- 所有数据库连接器通过 `init()` + `Register()` 向注册表自注册（`connector/registry.go`）
- **绝对不修改** `connector.go` 中的 switch-case 来新增数据库
- 新增数据库类型 = 新建一个文件 + 实现 `Connector` 接口 + `init()` 中调用 `Register()`
- 注册表是线程安全的 `map[string]func() Connector`

### 2. Panic 隔离

- 每个 `Collect()` 调用必须经过 `CollectSafe()` 包装（`connector/runner.go`）
- `CollectSafe` 使用 `defer`/`recover()` 捕获 panic，转换为 error（附带调用栈）
- 一个数据库连接器的崩溃**绝不**导致整体进程退出

### 3. 只读安全

整个工具**仅执行**只读操作，不区分 Collect（Schema 采集）和 Query（用户查询）：

**Collect 阶段** — 每个 Connector 仅采集元数据：
- SQL 数据库：SELECT FROM information_schema / SHOW / DESCRIBE / PRAGMA
- MongoDB：仅 ListCollectionNames + EstimatedDocumentCount（不执行用户查询）
- Redis：仅 GETRANGE/HSCAN/XRANGE 安全采样（不读全量数据）
- Elasticsearch：仅 _cat/_mapping 端点
- Qdrant：仅 grpc 健康检查 + 集合信息

**Query 阶段**（`execute` 子命令）— 用户查询通过安全管道：
- SQL 数据库：sqlguard AST 级只读校验（8 个读动词放行、11 个写动词拒绝）+ policy 引擎 + AutoLimit
- NoSQL 数据库：各自的原生命令白名单或查询校验器
- **严禁**任何写操作：INSERT、UPDATE、DELETE、DROP、CREATE、ALTER 等（无论 Collect 还是 Query 阶段）

### 4. 零 CGO

- 构建命令：`CGO_ENABLED=0 go build -ldflags="-s -w"`
- 所有依赖必须是纯 Go 实现
- SQLite 驱动使用 `github.com/glebarez/go-sqlite`（纯 Go，无 CGO）
- 交叉编译覆盖 5 个平台：linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64

> **例外：`-tags duckdb` 可选编译**：DuckDB 连接器需 CGO 和 C 工具链（gcc/clang/mingw），仅在编译期依赖。构建命令 `CGO_ENABLED=1 go build -tags duckdb`。所有不含 `duckdb` 标签的构建仍保持零 CGO。DuckDB 是项目唯一 CGO 例外。

### 5. 统一错误处理

- 所有数据库错误使用 `schema.NewDBError()` 包裹（`schema/errors.go`）
- 错误链携带完整上下文：脱敏 DSN、数据库名、表名、操作名称
- 使用 `%w` 实现 `Unwrap()` 兼容 `errors.Is()`/`errors.As()`
- 所有查询必须 `defer rows.Close()`

### 6. 无状态设计

- 每个 `Connector.Collect()` 方法自包含，不依赖外部可变状态
- 多 DSN 并发采集，结果通过 `sync.Mutex` 保护的共享 slice 聚合
- 每个 DSN 独立日志文件（`logs/<label>.log`）

### 7. 代码风格

- Go 1.26+，标准 `gofmt`
- 包命名遵循 Go 惯例（小写、单字母单词）
- 领域特有描述使用中文注释
- 导出符号使用英文命名
- 密码自动脱敏：`DSN.Redacted()` 将 `user:password@` 替换为 `{dbuser}:{dbpassword}@`

### 8. Deterministic Only（确定性输出）

- 工具**绝对不输出** AI 推断、业务语义猜测、LLM 生成的总结
- 只输出**可证实的事实**：DDL 声明的外键、列名/类型/可空性、索引结构、命名模式匹配的结果
- 语义理解、上下文总结、业务推理全部交给外部 LLM
- 命名推断的关系必须标注 `inferred=true` 和置信度，与显式 FK 明确区分

### 9. Graph First（图优先）

- 所有数据库对象内部统一建模为**通用图原语**：Node（表/集合）、Column（列/字段）、Edge（关系/引用）
- 终端 Markdown、JSON、HTML 输出**仅是渲染层**
- 内部分析 pipeline 操作图模型，不操作渲染格式
- 图模型**独立于数据库类型**——MySQL、PostgreSQL、Redis、MongoDB 共用同一套图原语

### 10. Capability Architecture（能力驱动架构）

- Connector 声明其支持的 Capability 集合（如 `CapForeignKey`、`CapTTL`、`CapPartition`）
- Extractor 按 Capability 工作，不按数据库类型分支
- 新增数据库类型**不需要修改 pipeline**——只需实现 Connector + 声明已有 Capabilities
- 禁止出现 `if mysql { ... } else if postgres { ... }` 的 pipeline 分支
- **v0.1.0 已落地**：`execute.go` 中 `isSQLKind()` 已被 `capabilities.FromProvider().Has(CapSQL)` 替代

### 11. 项目结构（`cmd/` + `internal/` 布局）

- 入口统一在 `src/cmd/dbexplain/`（`main.go` + 子命令 handler）
- 业务逻辑在 `src/internal/` 下按职责分包，不对外暴露
- **v0.1.1 已完成全量 internal 迁移**：所有 14 个业务包已移至 `src/internal/`，`src/` 仅保留 `cmd/` + 构建文件
- **禁止在 `internal/` 外创建业务逻辑**

### 12. DSL 确定性（编译型查询入口）

- DSL 是 `dbexplain execute --dsl` 的可选入口，与原生 SQL 通道共存
- DSL 解析/编译/规划必须确定：相同的输入 + 相同的 DSN 环境 → 相同输出
- 虚拟表模型统一所有数据源（SQL 数据库 / 文件 / 原生引擎）
- DSL 语法错误直接报错，不退回到原生通道（避免用户以为在跑 DSL 实际跑了原生 SQL）
- DSL 通道安全能力与原生通道一致：sqlguard + policy + AutoLimit + ApplyMask

---

## 项目边界 (v0.1.3)

### 数据源范围
- **核心**: MySQL, PostgreSQL, GaussDB, ClickHouse, SQLite, Redis, Elasticsearch, MongoDB, Qdrant
- **文件数据源**（非"数据库"）: CSV, TSV, XLSX — 可做 Schema 采集和只读查询，但不扩展更多文件格式
- **可选连接器**（需 `-tags duckdb` 构建）: DuckDB — 嵌入式 SQL 引擎，支持本地数据库查询和 Parquet/JSON/CSV 文件分析
- **不支持的**: Avro, Google Sheets — 超出"数据库上下文编译器"定位

### DSL 查询
- `--dsl` flag 提供统一 DSL 查询入口，支持 `@label.table` 语法引用数据源
- DSL 编译流程：预处理 → sqlast 解析 → 符号绑定 → 后端路由
- v0.1.2 支持跨源联邦查询（SQL ↔ 文件 JOIN/UNION），原生源（Redis/Mongo/Qdrant/ES）仍不支持 DSL 模式
- DSL 是可选入口，原生 SQL/原生命令通道完全保留

### 集成策略
- MCP Server, Cursor, OpenHands, Aider 集成 → **独立项目/仓库**，dbexplain 不内嵌 serve 模式
- dbexplain 是 CLI 工具，消费方通过 stdout/文件/JSON 获取数据

### 安全边界
- **sqlguard + policy 是第一道防线**，建议配合数据库层 GRANT SELECT ONLY 作为第二道防线
- 容器/VM 环境：机器指纹加密可能失效（无 `/etc/machine-id`），推荐使用密码模式或 `--machine-id-override`
- DSL 通道安全能力与原生通道一致：sqlguard AST 级校验 + policy AST 级表/列提取 + AutoLimit + ApplyMask

---

## 构建与发布

见 [docs/setup-guide/DEPLOY.md](docs/setup-guide/DEPLOY.md) — 构建方式、产物目录、部署方式、发布流程在该文档中完整维护。

宪法层面仅约束：
- **单二进制交付**：无动态依赖、无外部进程、`CGO_ENABLED=0`
- **版本号**：遵循 semver，版本一致性子命令 `--version` 和 CHANGELOG.md 对齐
- **无 CI/CD**：手动构建发布，发布前必须执行 `docs/security-policies/SECURITY_CHECKLIST.md` 所有检查项

---

## 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-06-03 | v7 | 第 4 条新增 DuckDB CGO 例外；项目边界更新至 v0.1.3，DuckDB 可选连接器 + Parquet 间接支持 |
| 2026-06-03 | v6 | 核心交付物更新（去除未实现的 IR Product 概念）；Principle 3 区分 Collect/Query 阶段并更新 MongoDB 描述；构建与发布章节精简为 DEPLOY.md 引用 |
| 2026-06-03 | v5 | 新增 DSL 联邦查询、REPL 模式、Build Tags 能力；项目边界更新至 v0.1.2 |
| 2026-05-29 | v3 | 第 10 条确认落地；新增"项目边界"章节，定义数据源范围和集成策略 |
| 2026-05-19 | v1 | 初始宪法，基于 v0.0.2 代码库提取 |
| 2026-05-20 | v2 | 项目重新定义为 Database Context Compiler；新增第 8-10 条：Deterministic Only、Graph First、Capability Architecture |
