# SQL 语法参考

> 覆盖 SQL 数据源（MySQL/PG/GaussDB/ClickHouse/SQLite/DuckDB/Oracle/Hive）和文件查询引擎（CSV/TSV/XLSX）。
> LLM 已知的标准 SQL 语法不再赘述，只标注各数据源差异和文件引擎特有规则。

---

## 1. 各数据源差异

| 数据源 | EXPLAIN | 日期函数 | 特殊注意 |
|--------|---------|---------|----------|
| MySQL | `EXPLAIN SELECT ...` | `DATE()`, `YEAR()` | `SHOW DATABASES/TABLES` |
| PostgreSQL | `EXPLAIN (FORMAT JSON) SELECT ...` | `DATE_TRUNC('month', col)` | Schema 感知 |
| GaussDB | 同 PG | 同 PG | Oracle 兼容模式需 DSN 加 `oracleCompatible=true` |
| ClickHouse | `EXPLAIN SELECT ...` | `toDate()`, `toMonth()` | **execute 命令行不要加分号** |
| SQLite | `EXPLAIN SELECT ...` | `date()`, `strftime()` | `PRAGMA table_info(t)` |
| DuckDB | `EXPLAIN SELECT ...` | 标准 SQL | 文件分析：`read_parquet()`, `read_csv_auto()`, `read_json()` |
| Oracle | `EXPLAIN PLAN FOR SELECT ...` | `TRUNC(col, 'MM')` | 分页用 12c+ `OFFSET m ROWS FETCH NEXT n ROWS ONLY` |
| Hive | `EXPLAIN SELECT ...` | 标准 SQL | `SHOW PARTITIONS t`；窗口函数原生支持 |

---

## 2. 文件引擎特有规则（CSV/TSV/XLSX）

### 表名来源

| 类型 | 表名 |
|------|------|
| CSV/TSV | 文件名（不含扩展名） |
| XLSX | Sheet 名（单 Sheet 也可用文件名） |

```sql
-- CSV sales_data.csv → 表名 sales_data
SELECT * FROM sales_data WHERE department = 'Sales'
-- XLSX → Sheet1
SELECT * FROM Sheet1
```

### 跨文件 JOIN

```bash
DB1=csv:///path/a.csv?label=a
DB2=csv:///path/b.csv?label=b
```
```sql
SELECT a.*, b.val FROM a_data t JOIN b_data b ON a.id = b.id
```

### XLSX 同文件跨 Sheet JOIN

```sql
SELECT s1.dept, s1.rate, s2.branch FROM Sheet1 s1 JOIN Sheet2 s2 ON s1.dept_id = s2.dept_id
```

### CAST / 表达式

`CAST(col AS FLOAT|INTEGER|TEXT)`, `ABS(col)`, `ROUND(col, n)`, 列间四则运算

### UNION / 子查询

`UNION ALL`, `UNION`；WHERE 中子查询 `IN (SELECT ...)` 支持。

### 已知限制

- 不支持 CTE（WITH 语句）、FROM 子查询
- 窗口函数 ✅ v0.1.1 已支持
