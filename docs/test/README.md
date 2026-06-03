# dbexplain 测试框架

> 分层测试方法论，覆盖全部 12 种数据源 + 安全引擎 + 性能基准。

---

## E2E 验证标准

所有 E2E 测试（手动/半自动验证）必须遵循以下统一标准：

### 构建命令

```bash
cd src && go build -o ../release/dbexplain ./cmd/dbexplain
```

二进制输出到项目根目录的 `release/dbexplain`（无架构后缀，用于本地开发测试）。
交叉编译使用 `cd src && bash build.sh`，输出到 `release/dbexplain-{os}-{arch}`。

### 标准变量

```bash
cd src                           # 工作目录始终为 src/
BIN="../release/dbexplain"       # 所有测试文档统一使用此变量
```

### 测试文档模板

```bash
# === 每个测试文档的前置条件 ===
cd src
BIN="../release/dbexplain"

# === 执行测试 ===
$BIN -env --json ...
$BIN execute -env --db 1 "SELECT 1" --human
```

### 注意事项

| 场景 | 说明 |
|------|------|
| 需要 `.env.dbexplain` 的测试 | 确保 `src/` 下有 `.env.dbexplain`（优先级 2，击败全局加密配置） |
| 无需外部数据库的测试 | 使用 DSN 直连（`-dsn "csv:///..."`），不依赖 `-env` |
| 测试文件引用 | 从 `src/` 出发，项目根目录路径加 `../` 前缀（如 `../testdata/qa/.env.qa-touch-csv`） |
| 已安装系统级二进制 | 若 `dbexplain` 已在 PATH 中，可直接使用 `dbexplain` 代替 `$BIN` |
| 仅编译检查（无需执行） | 使用 `go build ./...`（见 [01-environment.md](01-environment.md)） |

### 验证清单

每次 E2E 验证按以下顺序执行：

```bash
# 1. 构建
cd src && go build -o ../release/dbexplain ./cmd/dbexplain

# 2. 单元测试（全部）
cd src && go test ./... -count=1

# 3. Schema 采集（15 数据源）
cd src && $BIN -env --json | python3 -c "import json,sys;d=json.load(sys.stdin);print(f'{len(d[\"instances\"])} instances')"

# 4. 查询执行（抽样验证）
cd src && $BIN execute -env --db 1 "SELECT 1" --human

# 5. Schema Diff 验证（如有 cache 需要）
cd src && $BIN -env --cache /tmp/e2e.cache --json -o /tmp/e2e.json
cd src && $BIN diff --cache /tmp/e2e.cache --list-versions
```

详细测试步骤见对应分层文档（L1-L8）。

## 测试分层

| 层级 | 名称 | 覆盖范围 | 文档 |
|------|------|---------|------|
| L0 | 版本升级 | 跨版本构建对比、回归基线 | [10-regression.md](10-regression.md) |
| L1 | 静态分析 | go build/vet/test、交叉编译、安全审计、Shell 语法 | [01-environment.md](01-environment.md) |
| L2 | 单元测试 | 各包测试用例详细清单 | [01-environment.md](01-environment.md) §1.3 |
| L3 | 功能集成 | CLI 帮助、子命令、手册、别名解析 | [09-cli-help.md](09-cli-help.md) |
| L4 | 端到端回归 | 全部 DSN Schema 采集 | [10-regression.md](10-regression.md) |
| L5 | 安全专项 | sqlguard + policy 全链路 | [06-security-sqlguard.md](06-security-sqlguard.md)、[07-policy-engine.md](07-policy-engine.md) |
| L6 | 查询执行 | SQL + NoSQL + 文件 execute | [03-execute-sql.md](03-execute-sql.md)、[04-execute-nosql.md](04-execute-nosql.md)、[05-file-processing.md](05-file-processing.md) |
| L7 | 文档验证 | 版本一致性、文档引用正确性 | [10-regression.md](10-regression.md) |
| L8 | 能力架构 | CapSQL 路由、PostgreSQL 多 Schema、策略引擎增强、JSON 包装格式 | [12-capability-routing.md](12-capability-routing.md) |
| L8 | DuckDB 连接器 | DuckDB 采集/查询/安全/DSL/构建隔离 | [16-duckdb.md](16-duckdb.md) |

## 测试概览

| 维度 | 数据 |
|------|------|
| 数据源 | 16 (mysql/clickhouse/sqlite/qdrant/es/postgres/redis×2/mongo/xlsx×2/csv/tsv/duckdb) |
| 二进制架构 | 双版本: 标准版(-std 纯 Go) + DuckDB版(-duckdb CGO=1) |
| Go 版本 | 1.26 |
| 测试环境 | Linux x86-64 (amd64) |

## 配置优先级说明

> 详细搜索机制见 [docs/CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

`-env` 模式按以下顺序查找配置文件（命中即停）：

```
优先级 1: $DBPROBE_ENV_FILE 环境变量
优先级 2: CWD/.env.dbexplain
优先级 3: CWD/.env.dbexplain.enc
优先级 4: ~/.config/dbexplain/.env.dbexplain
优先级 5: ~/.config/dbexplain/.env.dbexplain.enc
优先级 6: CWD/.env
```

### 常见场景与应对

**场景一：全局有加密配置，本地测试用明文 `.env`**

本机 `~/.config/dbexplain/.env.dbexplain.enc` 优先级（5）高于 `CWD/.env`（6），
导致从 `src/` 运行 `-env` 时全局配置抢先匹配，本地 `.env` 不被读取。

**解决方案（三选一）：**

| 方法 | 命令 | 优先级 | 适用场景 |
|------|------|--------|---------|
| 创建 `.env.dbexplain` | `cp .env .env.dbexplain` | 2（最高） | 开发测试，一劳永逸 |
| 环境变量覆盖 | `DBPROBE_ENV_FILE=.env dbexplain -env` | 1（最高） | 临时切换，无需改文件 |
| 重命名全局配置 | `mv ~/.config/dbexplain/.env.dbexplain.enc ~/.config/dbexplain/.env.dbexplain.enc.bak` | — | 彻底禁用全局，影响所有项目 |

**推荐开发测试方式：**

```bash
cd src

# 方案 A：创建 .env.dbexplain（优先级 2，击败全局加密配置）
cp .env .env.dbexplain
dbexplain -env                    # 命中 .env.dbexplain ✓
dbexplain execute -env --db 1 "SELECT 1"  # 同上 ✓

# 方案 B：环境变量覆盖（不产生新文件）
DBPROBE_ENV_FILE=.env dbexplain -env                    # 显式指定 ✓
DBPROBE_ENV_FILE=.env dbexplain execute -env --db 1 "SELECT 1"  # ✓
```

**场景二：多项目切换，每个项目有独立配置**

```bash
# 每个项目目录下放 .env.dbexplain，自动命中（优先级 2）
cd ~/project-a && dbexplain -env   # 用 project-a 的配置
cd ~/project-b && dbexplain -env   # 用 project-b 的配置
# 互不干扰，无需环境变量
```

**场景三：专门测试某个 DSN**

```bash
# 直接 -dsn 绕过文件搜索，完全不依赖 -env
dbexplain -dsn 'csv:///tmp/test.csv?label=test'
dbexplain execute -dsn 'csv:///tmp/test.csv?label=test' "SELECT *"
```

## 最新测试结果

完整测试结果报告见 [RESULTS.md](RESULTS.md)。v0.1.3 测试结果: **全部单元测试通过**，含 DuckDB 连接器 20 测试、DSL 35 测试、Schema Diff 24 测试、窗口函数 33 测试。

## 快速导航

```bash
# 推荐执行顺序
01-environment.md    # 构建验证 + 单元测试
02-schema-collection.md  # Schema 采集
03-execute-sql.md    # SQL 查询执行
04-execute-nosql.md  # NoSQL 查询执行
05-file-processing.md    # 文件处理
06-security-sqlguard.md  # SQL 沙箱
07-policy-engine.md  # 安全策略
08-concurrent-limit.md   # 并发限制
09-cli-help.md       # CLI 帮助
10-regression.md     # 回归测试
11-end-to-end.md     # 全量集成
12-capability-routing.md  # 能力架构
```

## 测试充分性评估

| 模块 | 充分性 | 置信度 | 依据 |
|------|--------|--------|------|
| DSN 解析 | 高 | 95% | 35+ 用例 |
| 字段推断 | 高 | 95% | 44 用例 |
| 安全策略引擎 | 高 | 99% | 41 用例覆盖全部三层 + 12-DB 类型 + 防绕过 |
| SQL 只读校验 | 高 | 100% | 28 用例 + 实机 8 动词验证 |
| 查询引擎 | 高 | 100% | 15 DSN 实机执行 (SQL/NoSQL/文件) |
| 交叉编译 | 高 | 100% | 5/5 平台成功 |
| 文档同步 | 高 | 100% | 全部文件版本 v0.1.2 一致 |
| 文件处理 (CSV/TSV/XLSX) | 中 | 90% | 基本功能覆盖，边界场景需补充 |
| 能力架构 (CapSQL) | 高 | 100% | 全 15 连接器路由验证 |

### 测试边界与薄弱点

| 薄弱点 | 风险 | 说明 | 缓解措施 |
|--------|------|------|---------|
| analyze/connector/diagnostics 无单元测试 | 高 | 核心分析管线无 `*_test.go` | L1+L3+L4 全量覆盖 |
| policy 正则提取假阳性 | 低 | 可能误匹配注释中的 FROM | 安全设计：false positive 偏向拒绝 |
| Windows 实机未验证 | 中 | install.ps1 仅语法审查 | PowerShell 语法检查通过 |
| 大文件 CSV/XLSX 性能 | 中 | 全量读入内存 | 文档标注限制 |
| 超大结果集 human 输出 | 低 | maxColWidth=256 截断 | 防御性设计 |
| TSV kind 报告为 csv | 低 | csv.go 硬编码 Kind 为 csv | 仅标签问题，功能和查询正常 |
| Redis _server_info 无 columns | 低 | 元数据格式无列信息 | schema 采集正确，仅无列信息 |
| QueryLock CLI 跨进程 | 低 | 每次 execute 为独立进程 | 库模式正常，单元测试验证 |
