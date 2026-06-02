# SQL 语法参考

> 本文档按数据源类型分章节组织。**当前仅实现「文件数据源」章节**，后续可按需新增「MySQL」「PostgreSQL」「Redis」「SQLite」等数据库专有语法章节。

---

# 文件数据源（CSV / TSV / XLSX）

当前 `dbexplain` 文件查询引擎为 CSV/XLSX 数据提供内存 SQL 查询能力。以下为当前已支持的语法，持续扩展中。

> **SQL 字符串可用单引号或双引号**：`WHERE col = 'value'` 和 `WHERE col = "value"` 均支持。
>
> **bash 中写 SQL 字符串**：用双引号包裹整个 SQL，SQL 内部用单引号：
> ```bash
> dbexplain execute -env --label my_data "SELECT * FROM t WHERE col = 'value'" --human
> ```

---

## SELECT 语句

```sql
SELECT [DISTINCT [ON (col1, col2, ...)]] col1, col2, ...
FROM table_name [alias]
[JOIN / LEFT JOIN / RIGHT JOIN other_table [alias] ON condition]
[WHERE condition]
[GROUP BY col1, col2, ...]
[HAVING condition]
[ORDER BY col1 [ASC|DESC] [NULLS FIRST|LAST], ...]
[LIMIT n] [OFFSET m]
```

### 列投影

| 语法 | 说明 |
|------|------|
| `SELECT *` | 所有列 |
| `SELECT col1, col2` | 指定列 |
| `SELECT col AS alias` | 列别名（AS 可选） |
| `SELECT DISTINCT col` | 去重 |
| `SELECT DISTINCT ON (col1) col1, col2 ORDER BY ...` | 按列去重（保留每组第一条，需配合 ORDER BY） |

### FROM 子句

FROM 子句使用的表名取决于数据源类型：

#### CSV/TSV 数据源

表名 = **文件名（不含扩展名）**，而非 DSN label。

```
Schema 输出：DB1 → my_data → csv:///...sales_data.csv
                ↑label      ↑文件名 = sales_data

正确：SELECT * FROM sales_data
错误：SELECT * FROM my_data
```

#### XLSX 数据源

表名 = **Sheet 名**。XLSX 文件的每一 Sheet 都是一个独立的 SQL 表。

```
Schema 输出：DB1 → report → xlsx:///...data.xlsx
  Tables:
    ├── Sheet1   (20列, 5000行)
    └── Sheet2   (5列, 200行)

查询某一 Sheet：
SELECT * FROM Sheet1
SELECT * FROM Sheet2
```

如果 XLSX 文件只有一个 Sheet，也可用文件名查询（向后兼容）：
```sql
SELECT * FROM filename   -- 文件名或 Sheet 名均可
```

#### 表别名

```sql
SELECT t.col1, o.col2 FROM sales_data t JOIN org_info o ON t.dept_id = o.dept_id
```

---

## WHERE 条件

### 比较运算符

| 运算符 | 说明 |
|--------|------|
| `=`, `!=`, `<>` | 等于/不等于 |
| `<`, `>`, `<=`, `>=` | 大小比较 |
| `IS NULL`, `IS NOT NULL` | 空值判断（CSV 空字符串视为 NULL） |
| `LIKE`, `NOT LIKE` | 模式匹配（`%` 任意序列，`_` 单个字符） |
| `IN (...)`, `NOT IN (...)` | 值列表匹配 |
| `BETWEEN a AND b` | 范围匹配 |

### 逻辑运算符

| 运算符 | 说明 |
|--------|------|
| `AND` | 与 |
| `OR` | 或 |
| `NOT` | 非 |

### 示例

```sql
WHERE department = 'Sales'
WHERE branch_name IS NULL
WHERE department LIKE '%East'
WHERE dept_id IN ('D001', 'D002', 'D003')
WHERE completion_rate BETWEEN 80 AND 100
WHERE department = 'Sales' AND completion_rate > 80
```

---

## JOIN

支持跨文件哈希 JOIN。CSV 和 XLSX 的 JOIN 能力完全一致。

| JOIN 类型 | 说明 |
|-----------|------|
| `JOIN` | INNER JOIN（默认），仅返回匹配行 |
| `LEFT JOIN` | 左连接，左表全保留，右表无匹配则填空 |
| `RIGHT JOIN` | 右连接（实现为交换左右表后做 LEFT JOIN） |
| `LEFT OUTER JOIN` | 同 LEFT JOIN |
| `RIGHT OUTER JOIN` | 同 RIGHT JOIN |

### 跨文件 JOIN

两个独立文件（通过不同 DSN 配置）之间的 JOIN：

```bash
# .env.dbexplain 配置两个数据源
echo 'DB1=csv:///path/to/sales_data.csv?label=sales' > .env.dbexplain
echo 'DB2=csv:///path/to/org_info.csv?label=org' >> .env.dbexplain
```

```sql
-- INNER JOIN
SELECT t.*, o.branch_name FROM sales_data t
JOIN org_info o ON t.dept_id = o.dept_id

-- LEFT JOIN（保留左表所有行）
SELECT t.*, o.branch_name FROM sales_data t
LEFT JOIN org_info o ON t.dept_id = o.dept_id
```

### XLSX 同文件跨 Sheet JOIN

同一 XLSX 文件内不同 Sheet 之间可以直接 JOIN，无需额外配置：

```bash
# 只需一个 xlsx DSN，多 Sheet 自动可用
echo 'DB1=xlsx:///path/to/report.xlsx?label=report' > .env.dbexplain
```

```sql
-- 同一 xlsx 内跨 Sheet JOIN
SELECT s1.dept_name, s1.completion_rate, s2.branch_name
FROM Sheet1 s1
JOIN Sheet2 s2 ON s1.dept_id = s2.dept_id
```

### 跨格式 JOIN（CSV ↔ XLSX）

```bash
# .env.dbexplain 混合配置
echo 'DB1=csv:///path/to/sales_data.csv?label=sales' > .env.dbexplain
echo 'DB2=xlsx:///path/to/org_info.xlsx?label=org' >> .env.dbexplain
```

```sql
-- CSV 主表 JOIN XLSX 表
SELECT t.*, o.branch_name
FROM sales_data t
JOIN org_info o ON t.dept_id = o.dept_id
```

> **注意**：跨格式 JOIN 需要一个 DSN 配置一个文件。

---

## GROUP BY 与聚合

### 聚合函数

| 函数 | 说明 |
|------|------|
| `SUM(col)` | 求和 |
| `AVG(col)` | 平均值 |
| `COUNT(col)`, `COUNT(*)` | 计数 |
| `COUNT(DISTINCT col)` | 去重计数 |
| `MAX(col)` | 最大值 |
| `MIN(col)` | 最小值 |

### HAVING 过滤

GROUP BY 后可使用 HAVING 对聚合结果过滤：

```sql
SELECT department, AVG(completion_rate) AS avg_rate
FROM sales_data
GROUP BY department
HAVING avg_rate > 80
ORDER BY avg_rate DESC
```

HAVING 中可引用 SELECT 列表中的别名。

---

## ORDER BY

| 语法 | 说明 |
|------|------|
| `ORDER BY col` | 升序（默认） |
| `ORDER BY col DESC` | 降序 |
| `ORDER BY col ASC NULLS FIRST` | 升序，空值排前 |
| `ORDER BY col DESC NULLS LAST` | 降序，空值排后 |

---

## LIMIT / OFFSET

```sql
LIMIT 10            -- 最多返回 10 行
LIMIT 10 OFFSET 5   -- 跳过 5 行，返回最多 10 行
```

---

## UNION

```sql
-- UNION ALL：保留重复行
SELECT col1, col2 FROM t1
UNION ALL
SELECT col1, col2 FROM t2

-- UNION：去重
SELECT col1, col2 FROM t1
UNION
SELECT col1, col2 FROM t2
```

---

## 子查询

```sql
-- 仅支持 IN / NOT IN 中的 SELECT 子查询
SELECT * FROM sales_data
WHERE dept_id IN (SELECT dept_id FROM org_info WHERE branch_name = 'East Region')
```

---

## 表达式

### 算术运算

| 运算符 | 说明 |
|--------|------|
| `+`, `-`, `*`, `/` | 四则运算（支持列间运算） |
| `(expr)` | 括号分组 |

```sql
SELECT department, total_cnt / active_cnt * 100 AS active_ratio FROM sales_data
```

### 类型转换

```sql
CAST(col AS FLOAT)     -- 转浮点数
CAST(col AS INTEGER)   -- 转整数
CAST(col AS TEXT)      -- 转文本（默认）
```

支持的类型别名：`FLOAT`/`DOUBLE`/`REAL`/`DECIMAL`、`INT`/`INTEGER`/`BIGINT`/`SMALLINT`。

### 数学函数

| 函数 | 说明 |
|------|------|
| `ABS(col)` | 绝对值 |
| `ROUND(col)` | 四舍五入到整数 |
| `ROUND(col, n)` | 四舍五入到 n 位小数 |

### NULL 处理

- CSV 空字符串（`""`）视为 NULL
- `IS NULL` / `IS NOT NULL` 判断空值
- SUM/AVG 在组内所有值非数值时返回空字符串（SQL NULL 语义）

---

## 兼容性说明

### CSV 与 XLSX 能力一致

| 功能 | CSV | XLSX |
|------|-----|------|
| SELECT / WHERE / GROUP BY / ORDER BY | ✅ | ✅ |
| JOIN / LEFT JOIN / RIGHT JOIN | ✅ | ✅ |
| HAVING / IS NULL / UNION / 子查询 | ✅ | ✅ |
| CAST / ABS / ROUND | ✅ | ✅ |
| 多 Sheet 查询 | N/A（单表） | ✅（Sheet 名作表名） |
| 同文件跨 Sheet JOIN | N/A（单表） | ✅（自动加载所有 Sheet） |
| 跨格式 JOIN（CSV ↔ XLSX） | ✅ | ✅ |

### 已知限制

- 不支持 `HAVING` 中引用未在 SELECT 列表中出现的别名
- 不支持子查询在 WHERE 以外的位置（如 FROM 子查询）
- ~~不支持窗口函数（ROW_NUMBER、RANK 等）~~ ✅ v0.1.1 已支持
- 不支持 CTE（WITH 语句）
- 不支持 UPDATE/INSERT/DELETE（只读查询引擎）
- 不支持函数索引和表达式索引
