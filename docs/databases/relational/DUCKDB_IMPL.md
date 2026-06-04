# DuckDB 实现边界与架构

> **版本**: v0.1.3 | **更新**: 2026-06-03
> **摘要**: DuckDB 作为可选分析引擎的引入动机、构建策略、安全模型与已知限制。

---

## 目录

1. [引入动机](#1-引入动机)
2. [构建边界：CGO 例外](#2-构建边界cgo-例外)
3. [连接器架构](#3-连接器架构)
4. [文件分析安全模型](#4-文件分析安全模型)
5. [DSL 集成](#5-dsl-集成)
6. [与文件查询引擎的对比](#6-与文件查询引擎的对比)
7. [EXPLAIN 支持](#7-explain-支持)
8. [已知限制与 Phase 路线图](#8-已知限制与-phase-路线图)

---

## 1. 引入动机

DuckDB 被称为"嵌入式分析的 SQLite"，定位为**OLAP 分析型嵌入式引擎**。引入 DuckDB 主要解决以下场景：

| 场景 | 原有方案 | DuckDB 方案 |
|------|---------|-------------|
| Parquet 文件查询 | 不支持 | `read_parquet('/path/file.parquet')` |
| JSON 文件分析 | 不支持 | `read_json('/path/file.json')` |
| CSV 文件增强分析 | 内置文件引擎（有限功能） | DuckDB 完整 SQL + 类型推演 |
| 多格式 JOIN | 不支持 | `SELECT * FROM read_csv('a.csv') JOIN read_parquet('b.parquet') ...` |
| DuckDB 数据库连接 | 不支持 | `duckdb:///path/to/db` 标准 SQL 接口 |

### 为什么不选其他引擎？

| 引擎 | 不选原因 |
|------|---------|
| SQLite | 已支持（纯 Go 驱动），但 SQLite 不适合 OLAP 场景 |
| ClickHouse 本地 | 需独立服务进程，违背"单二进制"原则 |
| Polars (Go bindings) | 社区不成熟，Go 绑定不稳定 |
| Velox/DataFusion | Go 无原生绑定，需 CGO |

**DuckDB 的独特价值**：单文件嵌入式、标准 SQL、原生 Parquet/JSON/CSV 支持、OLAP 优化的向量化引擎。

---

## 2. 构建边界：CGO 例外

### 2.1 核心矛盾

项目宪法第 4 条要求**零 CGO 依赖**，保持跨平台纯 Go 编译。但 DuckDB 的 Go 驱动 (`github.com/duckdb/duckdb-go/v2`) 内嵌 DuckDB C++ 引擎，必须启用 CGO。

### 2.2 解决方案：可选构建标签

```
┌─────────────────────────────────────────────────┐
│                 构建产物                          │
├───────────────────────┬─────────────────────────┤
│  标准版 (-std)         │  DuckDB 版 (-duckdb)    │
│  CGO_ENABLED=0        │  CGO_ENABLED=1          │
│  tags=full            │  tags=duckdb,mysql,...  │
│  无 DuckDB             │  含全部驱动 + DuckDB    │
│  5 平台交叉编译         │  仅当前平台原生编译      │
│  ~9MB UPX             │  ~23MB UPX              │
└───────────────────────┴─────────────────────────┘
```

### 2.3 文件结构

| 文件 | 构建标签 | 作用 |
|------|---------|------|
| `connector/duckdb.go` | `//go:build duckdb` | DuckDB 连接器实现 |
| `manual/manual_duckdb.go` | `//go:build duckdb` | 帮助文本：DuckDB 正常显示 |
| `manual/manual_noduckdb.go` | `//go:build !duckdb` | 帮助文本：显示构建提示 |
| `connector/registry.go` | 无条件 | 注册表中对 duckdb 特殊错误提示 |

关键点：`duckdb` 标签**不包含在 `full` 中**，因此 `bash build.sh` 产出的标准版不含 DuckDB。

### 2.4 构建命令

```bash
# 标准版（纯 Go，5 平台交叉编译）
bash build.sh prod

# DuckDB 版（当前平台，需 C 工具链）
bash build.sh minimal duckdb,mysql,postgres,sqlite,clickhouse,redis,mongodb,elasticsearch,qdrant,csv,xlsx

# 或直接用 release.sh 双版发布
bash release.sh

# 手动编译
CGO_ENABLED=1 go build -tags duckdb -o dbexplain ./cmd/dbexplain
```

### 2.5 运行时依赖

DuckDB 版二进制在 Linux 上**不**静态链接 libstdc++/libc，运行时需要：
- Linux: `libstdc++.so.6`, `libc.so.6`（几乎每台 Linux 都有）
- macOS: 系统自带
- Windows: 需要 `VCRUNTIME140.dll`（通常已预装）

这不是传统意义上的"安装依赖"——C 运行时在所有主流系统上预装。**单文件部署模式不变**。

---

## 3. 连接器架构

### 3.1 DSN 格式

```
duckdb:///:memory:?label=alias             内存模式（三斜杠）
duckdb:///absolute/path/to/db?label=alias   文件数据库模式
```

> **重要**：必须使用 `duckdb:///:memory:`（三斜杠），而非 `duckdb://:memory:`。后者的 `:memory:` 会被 Go `url.Parse` 误解析为端口号。

### 3.2 连接串构建流程 (`buildDuckDBConnStr`)

```
DSN 原始串
    │
    ▼
提取 :// 后的路径部分
    │
    ▼
剥离 ?query 参数
    │
    ▼
判断是否为内存模式
  → ":memory:" / "" / "/:memory:" → 返回 ":memory:"
    │
    ▼
URL 解码路径
    │
    ▼
Windows 特殊处理: 去掉起始 /
    │
    ▼
返回文件路径
```

### 3.3 Schema 采集流程 (`Collect`)

```
Collect(ctx, dsn)
    │
    ▼
sql.Open("duckdb", connStr)
    │
    ▼
Ping 验证连接（3s 超时）
    │
    ▼
information_schema.tables → 枚举用户表/视图
    │
    ▼
对每张表:
  ├─ pragma_table_info() → 列名、类型、可空、默认值、主键
  ├─ SELECT COUNT(*)    → 行数
  ├─ LIMIT 1 采样       → 注释推断（DuckDB 无原生列注释）
  └─ duckdb_constraints() → 主键/唯一/外键约束
    │
    ▼
返回 schema.Instance
```

### 3.4 注册机制

通过 `connector.Register()` 注册，无额外依赖：

```go
func init() {
    Register("duckdb", func() Connector { return duckdbConnector{} })
}
```

### 3.5 Capabilities

```go
func (duckdbConnector) Capabilities() []capabilities.Capability {
    return []capabilities.Capability{
        capabilities.CapSQL,       // SQL 查询
        capabilities.CapRowCount,  // 行数统计
        capabilities.CapSampling,  // 采样推断
    }
}
```

---

## 4. 文件分析安全模型

DuckDB 的文件分析能力（`read_parquet`、`read_csv_auto`、`read_csv`、`read_json`）是**双刃剑**——允许查询任何文件路径。

### 4.1 安全控制机制

```
DSN 参数:
  duckdb:///:memory:?allowed_path=/data/csv/,/data/parquet/

用户 SQL:
  SELECT * FROM read_parquet('/data/parquet/sales.parquet')

验证流程:
  1. 扫描 SQL 中的 read_*() 函数调用
  2. 无文件函数 → 直接放行
  3. 有文件函数 + 无 allowed_path → 拒绝并提示
  4. 有文件函数 + allowed_path 已设置:
     ├─ 提取文件路径参数
     ├─ filepath.Clean() 标准化路径
     └─ strings.HasPrefix 检查是否在白名单内
```

### 4.2 拒绝策略

| 场景 | 返回信息 |
|------|---------|
| 使用读文件函数但未设置 `allowed_path` | `FILE_ACCESS_DENIED: read_parquet/read_csv_auto/read_json requires allowed_path` |
| 路径超出 `allowed_path` 范围 | `FILE_ACCESS_DENIED: path "/etc/passwd" is not within allowed_path "/data/"` |
| DSN 未配置 `allowed_path` 的正常查询 | 正常执行，不受影响 |

### 4.3 路径验证实现

```go
func validateFileAccess(opts query.ExecuteOpts) error {
    allowedPath := opts.DSN.DSNParam("allowed_path")
    // ...
    cleanPath := filepath.Clean(filePath)
    allowed := false
    for _, ap := range allowedPaths {
        if strings.HasPrefix(cleanPath, ap) {
            allowed = true
            break
        }
    }
    if !allowed {
        return fmt.Errorf("FILE_ACCESS_DENIED: path %q is not within allowed_path %q", ...)
    }
}
```

**防御深度**：
- `filepath.Clean()` 消除 `../` 路径遍历
- `strings.HasPrefix` 要求路径是白名单的子目录
- 多个路径用逗号分隔: `allowed_path=/data/csv,/data/parquet`

---

## 5. DSL 集成

DuckDB 在 DSL 模式中归类为 `SourceSQL`（SQL 数据源），与其他 SQL 数据库共享同一处理路径。

### 5.1 分类映射 (`binder.go`)

```go
func classifySource(kind string) SourceType {
    switch kind {
    case "mysql", "postgres", "clickhouse", "sqlite", "duckdb":
        return SourceSQL
    // ...
    }
}
```

### 5.2 DSL 查询示例

```bash
# DuckDB 数据源 DSL 查询
dbexplain execute -env --label my-duckdb --dsl "SELECT * FROM @my-duckdb.users LIMIT 5" --human

# DSL 联邦查询（DuckDB ↔ MySQL）
dbexplain execute -env --dsl "
  SELECT * FROM @duck-db.sales
  JOIN @mysql-db.users ON sales.user_id = users.id
" --human
```

---

## 6. 与文件查询引擎的对比

| 维度 | 文件查询引擎 (filequery) | DuckDB 文件分析 |
|------|------------------------|-----------------|
| 实现语言 | 纯 Go | DuckDB C++ 引擎 + Go 绑定 |
| CGO | 无需 | 需要 |
| 数据格式 | CSV/TSV/XLSX（内存加载） | Parquet/CSV/JSON（DuckDB 原生读取） |
| 查询能力 | 有限 SQL（WHERE/GROUP BY/JOIN/窗口函数） | 完整 DuckDB SQL |
| 性能 | 小文件（MB 级） | 大文件（GB 级，向量化执行） |
| 安全性 | 无文件路径验证 | `allowed_path` 白名单控制 |
| 构建方式 | 始终包含在 `full` 中 | 需 `-tags duckdb` 可选构建 |

**共存策略**：两者不替代，分别服务不同场景。文件查询引擎保持零依赖，DuckDB 提供增强分析能力。

---

## 7. EXPLAIN 支持

DuckDB 的 `EXPLAIN` 输出格式为文本计划树，与 MySQL/PostgreSQL 格式不同。

### 7.1 实现

```go
// executor.go 中的 wrapExplain() 适配
case "duckdb":
    return fmt.Sprintf("EXPLAIN %s", sql), nil
```

### 7.2 输出示例

```
┌───────────────────────────┐
│         EXPLAIN           │
├───────────────────────────┤
│ plan                      │
│ ───────────────────────── │
│ ┌─────────────────────────┐
│ │    PROJECTION           │
│ │    ──────────────────   │
│ │    → #1 AS id           │
│ │    → #2 AS name         │
│ └─────────────────────────┘
│ ...
└───────────────────────────┘
```

---

## 8. 已知限制与 Phase 路线图

### 8.1 当前限制 (v0.1.3)

| 限制 | 说明 | 影响评估 |
|------|------|---------|
| **不支持交叉编译** | DuckDB C++ 引擎需每平台原生编译 | 中：release.sh 已自动处理 |
| **C 运行时依赖** | 运行时需 libstdc++/libc | 低：所有主流系统预装 |
| **非静态链接** | DuckDB 版不生成纯静态二进制 | 中：CGO 不可避免 |
| **不支持 `full` 标签** | 必须显式指定 `-tags duckdb` | 低：设计如此 |
| **不支持 S3/云存储** | 未加载 httpfs 扩展 | 低：Phase 2 计划 |
| **不支持 Arrow/Iceberg/Delta** | 暂未注册扩展 | 低：Phase 2 计划 |
| **仅当前平台发布** | release.sh 的 DuckDB 版只产当前平台 | 低：设计如此 |

### 8.2 已知 Bug / 边界

| 问题 | 状态 | 说明 |
|------|------|------|
| `:memory:` DSN 三斜杠要求 | 已修复 | `duckdb:///:memory:` 非直觉但被迫 |
| `duckdb` 子命令未注册 | 已修复 | v0.1.3 早期版本缺失，已补 |
| 路径验证 SQL 注入 | 已评估 | `validateFileAccess` 只扫描字符串字面量，不作为安全边界 |

### 8.3 Phase 路线图

| Phase | 版本 | 内容 |
|-------|------|------|
| Phase 1 | v0.1.3 | 基础连接器 + 文件分析 + 安全控制 |
| Phase 2 | TBD | `INSTALL`/`LOAD` 扩展支持、httpfs（S3/GCS/OSS）、Arrow 格式 |

### 8.4 宪法例外

CONSTITUTION.md 第 4 条新增：

> **例外：`-tags duckdb` 可选编译**：DuckDB 连接器需 CGO 和 C 工具链（gcc/clang/mingw），仅在编译期依赖。构建命令 `CGO_ENABLED=1 go build -tags duckdb`。所有不含 `duckdb` 标签的构建仍保持零 CGO。DuckDB 是项目唯一 CGO 例外。

---

## 附录：关键决策记录

| 决策 | 选项 | 选择 | 原因 |
|------|------|------|------|
| DuckDB vs SQLite 文件分析 | SQLite + 扩展 vs DuckDB | DuckDB | Parquet 原生支持、OLAP 优化 |
| CGO 策略 | 全量 CGO vs 标签隔离 | 标签隔离 | 保持宪法零 CGO 原则 |
| 构建标签名 | `duckdb` vs `cgo` vs `duckdb_full` | `duckdb` | 直观、自文档化 |
| 文件分析安全 | SQL 解析 vs allowed_path | allowed_path | 轻量、零依赖、足够防御 |
| 双版命名 | `-std`/`-duckdb` vs `-pure`/`-cgo` | `-std`/`-duckdb` | 功能导向命名 |
