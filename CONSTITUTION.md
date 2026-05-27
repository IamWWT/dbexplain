# dbexplain 项目宪法

## 项目定位

`dbexplain` 是一个 **Database Context Compiler**（数据库上下文编译器），为 AI Agent 提供**确定性、可证实**的数据库结构信息层。

核心哲学：**dbexplain 只输出 deterministic facts，LLM 在外部消费 IR 做推理**。

### 消费方

- **AI Agent**：通过 `dbexplain-skill/SKILL.md` 定义的技能接口调用，Agent 读取 stdout 中的 Markdown 报告或 `-json` 输出的结构化数据
- **人类运维/DBA**：直接在终端执行，阅读格式化报告或 JSON 输出进行数据库巡检和结构分析

### 核心交付物

一个**单文件静态二进制**，无运行时依赖（无 CGO、无 libc 版本依赖、无外部进程），可跨 5 平台直接运行。

最终交付物包括：
- **CLI Product**：Markdown + Diagnostics（DBA/运维向）
- **IR Product**：Graph + Summary + Retrieval Chunks + Topology（AI Agent 向）

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

- 工具**仅执行**只读操作：SELECT、SHOW、DESCRIBE、PRAGMA、SCAN、INFO、_cat、_mapping 等
- **严禁**任何写操作：INSERT、UPDATE、DELETE、DROP、CREATE、ALTER 等
- MongoDB 连接器连 `find()` 都不执行，仅使用 `ListCollectionNames` + `EstimatedDocumentCount`
- Redis 连接器仅用 GETRANGE/HSCAN/XRANGE 安全采样，不读全量数据

### 4. 零 CGO

- 构建命令：`CGO_ENABLED=0 go build -ldflags="-s -w"`
- 所有依赖必须是纯 Go 实现
- SQLite 驱动使用 `github.com/glebarez/go-sqlite`（纯 Go，无 CGO）
- 交叉编译覆盖 5 个平台：linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64

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

---

## 构建与发布

- 构建脚本：`src/build.sh`（交叉编译 + `file` 命令校验架构）
- 输出目录：`release/`
- 无 CI/CD，手动构建发布
- 版本号遵循 semver，CHANGELOG.md 维护

---

## 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-05-19 | v1 | 初始宪法，基于 v0.0.2 代码库提取 |
| 2026-05-20 | v2 | 项目重新定义为 Database Context Compiler；新增第 8-10 条：Deterministic Only、Graph First、Capability Architecture |
