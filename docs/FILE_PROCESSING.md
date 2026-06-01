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

文件查询引擎提供纯 Go 内存 SQL 查询能力，CSV 和 XLSX 数据源支持完全相同的 SQL 语法。

### 支持的 SQL 语法

```sql
SELECT [DISTINCT [ON (col1, col2, ...)]] col1, col2, ...
FROM table_name [alias]
[JOIN / LEFT JOIN / RIGHT JOIN other_table [alias] ON condition]
[WHERE condition]
[GROUP BY col1, col2, ...]
[HAVING condition]
[ORDER BY col1 [ASC|DESC] [NULLS FIRST|LAST], ...]
[LIMIT n] [OFFSET m]
[UNION [ALL] SELECT ...]
```

### 功能明细

| 功能 | 说明 | 示例 |
|------|------|------|
| **列投影** | 指定列、`*`、别名、去重 | `SELECT col1, col2 AS alias` |
| **WHERE 过滤** | 比较/逻辑运算符、LIKE/IN/BETWEEN/IS NULL | `WHERE col = '值' AND col LIKE '%x%'` |
| **GROUP BY + 聚合** | SUM/AVG/COUNT/MAX/MIN + COUNT(DISTINCT) | `GROUP BY col HAVING AVG(col) > 80` |
| **ORDER BY** | ASC/DESC + NULLS FIRST/LAST | `ORDER BY col DESC NULLS LAST` |
| **JOIN** | INNER/LEFT/RIGHT JOIN，跨文件/跨格式 | `FROM t1 JOIN t2 ON t1.k = t2.k` |
| **UNION / UNION ALL** | 合并结果集 | `SELECT ... UNION ALL SELECT ...` |
| **子查询** | WHERE IN / NOT IN (SELECT ...) | `WHERE col IN (SELECT ...)` |
| **表达式** | 算术运算、括号分组 | `col1 / col2 * 100` |
| **CAST / ABS / ROUND** | 类型转换与数学函数 | `CAST(col AS FLOAT)`, `ROUND(col, 2)` |

### FROM 表名规则

- **CSV/TSV**: 表名 = 文件名（不含扩展名），非 DSN label
- **XLSX**: 表名 = Sheet 名，每 Sheet 为独立 SQL 表；单 Sheet 时文件名也可用
- **表别名**: `FROM table_name alias` 支持

### 跨文件与跨格式 JOIN

文件查询引擎支持以下 JOIN 场景：

| JOIN 场景 | 配置要求 | 示例 |
|-----------|---------|------|
| 同 DSN 内 JOIN | 无需额外配置（目录/Glob 多文件） | `FROM file1 JOIN file2 ON ...` |
| 跨 DSN JOIN | 需两个 DSN 条目 | `DB1=csv:///a.csv DB2=csv:///b.csv` |
| XLSX 跨 Sheet JOIN | 无需额外配置（自动加载所有 Sheet） | `FROM Sheet1 JOIN Sheet2 ON ...` |
| CSV ↔ XLSX 跨格式 JOIN | 需两个 DSN 分别指向两个文件 | `DB1=csv:///a.csv DB2=xlsx:///b.xlsx` |

### 执行示例

```bash
# CSV 全表扫描
dbexplain execute -dsn 'csv:///tmp/data.csv?label=test' 'SELECT * LIMIT 5'

# WHERE 条件过滤
dbexplain execute -dsn 'csv:///tmp/data.csv?label=test' "SELECT * FROM data WHERE col = '江苏分行'" --human

# GROUP BY + 聚合 + HAVING
dbexplain execute -dsn 'csv:///tmp/data.csv?label=test' \
  'SELECT col, AVG(rate) AS avg_rate FROM data GROUP BY col HAVING avg_rate > 80' --human

# ORDER BY + 表达式
dbexplain execute -dsn 'csv:///tmp/data.csv?label=test' \
  'SELECT col, interact_cnt / tol_cnt * 100 AS weighted FROM data ORDER BY weighted DESC' --human

# XLSX 多 Sheet 查询
dbexplain execute -dsn 'xlsx:///tmp/report.xlsx?label=report' 'SELECT * FROM Sheet1 LIMIT 10'
dbexplain execute -dsn 'xlsx:///tmp/report.xlsx?label=report' 'SELECT * FROM Sheet2 LIMIT 10'

# XLSX 跨 Sheet JOIN
dbexplain execute -dsn 'xlsx:///tmp/report.xlsx?label=report' \
  'SELECT s1.*, s2.name FROM Sheet1 s1 JOIN Sheet2 s2 ON s1.id = s2.id' --human

# 跨文件 LEFT JOIN（两个 DSN）
dbexplain execute -env --label touch_data \
  'SELECT t.*, o.name FROM data t LEFT JOIN org o ON t.org_id = o.id' --human
```

### 安全约束

- **不经过 sqlguard**: 文件查询绕过 SQL 沙箱校验（文件本身只读）
- **Policy 引擎**: 文件查询受 `DENY_TABLES`（文件名/目录名/Sheet 名匹配）和 `MASK_COLUMNS`（列值屏蔽）约束
- **LIMIT 上限**: 默认最大返回 1000 行，可通过 `--limit N` 调整

---

## 8. 内存限制

CSV/TSV/XLSX 连接器在采集和执行时**全量读取文件到内存**（`ReadAll()` / `GetRows()`），
不支持流式读取。即使查询带 `LIMIT 1`，整个文件也会先读入内存再截断。

建议：
- 单文件行数 < 100 万行（约 500MB CSV）
- 超大文件建议先拆分再查询
- XLSX 文件建议 < 50MB

---

## 9. 构建要求

### CSV/TSV

- **零外部依赖**: 仅使用 Go 标准库 (`encoding/csv`)
- **始终可用**: 无需编译标签，`go build ./...` 即包含
- **GBK 编码**: 依赖 `golang.org/x/text`（已存在于 go.mod）

### XLSX

- **内建于主模块**: `github.com/xuri/excelize/v2` 是 `src/go.mod` 的永久依赖
- **无需编译标签**: `go build ./...` 或 `bash build.sh` 即包含 xlsx 支持
- **无需额外构建步骤**: 标准构建产物即为全功能二进制

---

## 10. CLI 帮助

```bash
# CSV/TSV 参考手册
dbexplain csv

# XLSX 参考手册
dbexplain xlsx
```

两个子命令均输出中英双语帮助，涵盖 DSN 格式、采集机制、查询限制和示例。

---

## 11. 与核心管道的集成

```
.env DSN → dsn.ParseDSN() → connector.GetConnector("csv"|"xlsx")
                                ↓
                          Collect() → *schema.Instance
                                ↓
                          ExecQuery() → *query.QueryResult
```

- **Schema 采集**: 通过 `-env`/`-dsn` 加载，与数据库 DSN 完全对等
- **Execute 查询**: 通过 `dbexplain execute` 入口，专用分支 `handleFileExecute()` 跳过 sqlguard（SELECT * 只读），但受 Policy 引擎约束（`DENY_TABLES`、`MASK_COLUMNS`）
- **json / --human**: 输出格式与数据库查询完全一致；`--human` 可放在查询语句之前或之后

---

## 参考文档

- `src/connector/csv.go` — CSV/TSV 连接器实现
- `src/connector/xlsx.go` — XLSX 连接器实现（含 excelize 封装）
- `src/connector/infer.go` — 类型推断共享逻辑
- `src/execute.go` — `handleFileExecute()` 独立执行路径
- `docs/test/05-file-processing.md` — 文件处理测试用例
