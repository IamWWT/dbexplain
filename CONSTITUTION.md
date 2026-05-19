# dbexplain 项目宪法

## 项目定位

`dbexplain` 是一个**零依赖、静态编译**的命令行工具，用于探索和分析数据库结构。它只读连接各种数据库，提取元数据（表、列、索引、外键），推断关系，生成报告。

### 消费方

- **AI Agent**：通过 `db-relationship-explainer/SKILL.md` 定义的技能接口调用，Agent 读取 stdout 中的 Markdown 报告或 `-json` 输出的结构化数据
- **人类运维/DBA**：直接在终端执行，阅读格式化报告或 JSON 输出进行数据库巡检和结构分析

### 核心交付物

一个**单文件静态二进制**，无运行时依赖（无 CGO、无 libc 版本依赖、无外部进程），可跨 5 平台直接运行。

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
- 密码自动脱敏：`DSN.Redacted()` 将 `:password@` 替换为 `:***@`

---

## 构建与发布

- 构建脚本：`src/build.sh`（交叉编译 + `file` 命令校验架构）
- 输出目录：`db-relationship-explainer/tools/`
- 无 CI/CD，手动构建发布
- 版本号遵循 semver，CHANGELOG.md 维护

---

## 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-05-19 | v1 | 初始宪法，基于 v0.0.2 代码库提取 |
