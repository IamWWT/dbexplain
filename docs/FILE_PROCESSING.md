# CSV/TSV/XLSX 文件处理

> 对本地文件执行 Schema 采集与只读查询（无需数据库服务）。支持 CSV、TSV、Excel (.xlsx) 三种文件格式，通过统一 DSN 接口接入 dbexplain 管道。

---

## 1. DSN 格式

### CSV/TSV

```
csv:///文件绝对路径?label=别名[&encoding=gbk][&delimiter=,]
tsv:///文件绝对路径?label=别名[&delimiter=%09]
csv:///目录路径/?label=别名
csv:///通配符表达式/*.csv?label=别名
```

### XLSX

```
xlsx:///文件绝对路径?label=别名
```

### 各参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `label` | (自动) | 数据源别名，`-env` 模式下通过 `--label` 匹配 |
| `encoding` | `utf-8` | 文件编码。指定 `gbk`/`gb2312`/`gb18030` 时自动转码 |
| `delimiter` | `,` (csv) / `\t` (tsv) | 自定义分隔符。支持 `tab`、`pipe`、`semicolon`、URL 编码值 |

---

## 2. 路径解析（三态规则）

CSV/TSV 支持三种路径模式，按以下优先级互斥判定：

| 模式 | 判定条件 | 行为 |
|------|----------|------|
| **Glob** | 路径含 `*`/`?`/`[` 通配符 | 调用 `filepath.Glob()` 展开匹配 |
| **目录** | 路径为已存在的目录 | 扫描目录内全部 `.csv`/`.tsv` 文件 |
| **单文件** | 以上均不匹配 | 直接读取 |

XLSX 仅支持单文件模式（无目录/Glob）。

---

## 3. 编码支持

- **默认**: UTF-8
- **GBK 系**: `?encoding=gbk` 参数触发 `transform.NewReader(f, simplifiedchinese.GBK.NewDecoder())`
- 同时支持 `gb2312`、`gb18030` 别名
- 编码参数对单文件和目录模式均生效

---

## 4. 分隔符

| DSN Scheme | 默认分隔符 | 自定义 |
|------------|-----------|--------|
| `csv://` | `,` (逗号) | `?delimiter=` |
| `tsv://` | `\t` (制表符) | `?delimiter=` |

自定义分隔符支持关键字：
- `tab` → `\t`
- `pipe` → `|`
- `semicolon` → `;`
- URL 编码值（如 `%09`）
- 字面字符

---

## 5. 类型推断

每个文件/Sheet 独立采样前 100 行，按优先级判定列类型：

```
INTEGER → FLOAT → DATE → TEXT
```

| 类型 | 判定方法 |
|------|---------|
| `INTEGER` | `strconv.ParseInt()` 成功 |
| `FLOAT`  | `strconv.ParseFloat()` 成功 |
| `DATE`   | 匹配常见日期格式（12 种 pattern） |
| `TEXT`   | 以上均不匹配 |

空值/`NULL` 字符串在采样时跳过，不影响类型判定。

---

## 6. 采集行为

| 特性 | CSV/TSV | XLSX |
|------|---------|------|
| 每文件 = 一表 | ✅ | — |
| 每 Sheet = 一表 | — | ✅ |
| 首行作列名 | ✅ | ✅ |
| 类型推断 | ✅ | ✅ |
| 行数统计 | ✅ | ✅ |
| 容量限制 | 1000 行采样 | 1000 行采样 |

---

## 7. 查询执行

### 支持的语法

```
SELECT *                    — 返回全部行（受 MaxRows 限制）
SELECT * LIMIT N            — 返回前 N 行
SELECT * LIMIT N OFFSET M   — 分页，跳过 M 行后返回 N 行
```

### 限制

- **不支持**: 列选择、WHERE 条件、JOIN、ORDER BY、GROUP BY、子查询
- **不经过 sqlguard**: 文件查询绕过 SQL 沙箱校验（文件本身只读）
- **不经过 Policy 引擎**: 文件查询不执行策略检查
- **XLSX 默认查询第一个 Sheet**: 不支持按 Sheet 名选择

### 执行示例

```bash
# CSV 查询
dbexplain execute -dsn 'csv:///tmp/data.csv?label=test' 'SELECT * LIMIT 5'

# TSV 查询
dbexplain execute -dsn 'tsv:///tmp/data.tsv?label=test&delimiter=%09' 'SELECT *'

# XLSX 查询
dbexplain execute -dsn 'xlsx:///tmp/report.xlsx?label=report' 'SELECT * LIMIT 10'
```

---

## 8. 构建要求

### CSV/TSV

- **零外部依赖**: 仅使用 Go 标准库 (`encoding/csv`)
- **始终可用**: 无需编译标签，`go build ./...` 即包含
- **GBK 编码**: 依赖 `golang.org/x/text`（已存在于 go.mod）

### XLSX

- **内建于主模块**: `github.com/xuri/excelize/v2` 是 `src/go.mod` 的永久依赖
- **无需编译标签**: `go build ./...` 或 `bash build.sh` 即包含 xlsx 支持
- **无需额外构建步骤**: 标准构建产物即为全功能二进制

---

## 9. CLI 帮助

```bash
# CSV/TSV 参考手册
dbexplain csv

# XLSX 参考手册
dbexplain xlsx
```

两个子命令均输出中英双语帮助，涵盖 DSN 格式、采集机制、查询限制和示例。

---

## 10. 与核心管道的集成

```
.env DSN → dsn.ParseDSN() → connector.GetConnector("csv"|"xlsx")
                                ↓
                          Collect() → *schema.Instance
                                ↓
                          ExecQuery() → *query.QueryResult
```

- **Schema 采集**: 通过 `-env`/`-dsn` 加载，与数据库 DSN 完全对等
- **Execute 查询**: 通过 `dbexplain execute` 入口，专用分支 `handleFileExecute()` 跳过 sqlguard/Policy
- **json / --human**: 输出格式与数据库查询完全一致

---

## 参考文档

- `src/connector/csv.go` — CSV/TSV 连接器实现
- `src/connector/xlsx.go` — XLSX 连接器实现（含 excelize 封装）
- `src/connector/infer.go` — 类型推断共享逻辑
- `src/execute.go` — `handleFileExecute()` 独立执行路径
- `docs/test/05-file-processing.md` — 文件处理测试用例
