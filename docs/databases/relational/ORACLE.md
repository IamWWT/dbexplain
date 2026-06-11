# Oracle 数据库连接器

## DSN 格式

```bash
# 普通连接
oracle://user:password@host:1521/XE?label=my-oracle

# TLS 连接
oracles://user:password@host:1521/XEPDB1?label=my-tls-oracle

# 指定服务名和端口
oracle://scott:tiger@192.168.1.100:1521/ORCL?label=prod-db
```

| 项目 | 说明 |
|------|------|
| Scheme | `oracle://`（明文）或 `oracles://`（TLS） |
| 默认端口 | 1521 |
| 默认服务 | XE |
| 驱动 | github.com/sijms/go-ora/v2（纯 Go，无 CGO） |

> **注意**：`oracles://` 会自动设置 `?ssl=true` 参数（go-ora 使用 URL 参数而非 scheme 前缀启用 TLS）。

## 功能

| 能力 | 支持 | 说明 |
|------|------|------|
| Schema 采集 | ✅ | 表、列、约束、索引、外键 |
| SQL 查询 | ✅ | 通过 database/sql + go-ora |
| EXPLAIN | ✅ | 两步法：EXPLAIN PLAN FOR + DBMS_XPLAN.DISPLAY() |
| 行数统计 | ✅ | all_tables.num_rows（统计值，需 ANALYZE） |
| 索引采集 | ✅ | all_ind_columns + all_indexes |
| 外键采集 | ✅ | 4 表 JOIN 位置对齐，支持复合外键 |
| 采样 | ✅ | FETCH FIRST 1 ROWS ONLY（需 12c+） |
| 注释推断 | ✅ | 首行采样 + 规则引擎 |

## 系统 Schema 过滤

自动跳过以下 Oracle 内部 Schema：

```
SYS, SYSTEM, DBSNMP, XDB, DVSYS, AUDSYS, GSMADMIN_INTERNAL,
OJVMSYS, LBACSYS, OUTLN, APPQOSSYS, CTXSYS, MDSYS, ORDSYS,
ORDDATA, ORDPLUGINS, SI_INFORMTN_SCHEMA, DMSYS, OLAPSYS,
EXFSYS, WMSYS, PERFSTAT, STDBYPERF
```

## Schema 采集详情

### 表信息
通过 `all_tables` 视图采集：表名、行数估算（`num_rows`）、表注释。行数来自优化器统计信息，需要定期 `ANALYZE TABLE` 采集方保持准确。

### 列信息
通过 `all_tab_columns` + `all_col_comments` 视图采集：列名、数据类型、可空性、默认值、注释。对无注释字段自动取首行数据进行语义推断。

### 主键/唯一约束
通过 `all_constraints` + `all_cons_columns` 视图采集：标记 `constraint_type = 'P'`（主键）和 `'U'`（唯一约束）。

### 索引
通过 `all_ind_columns` + `all_indexes` 视图采集：索引名、唯一性标志、列列表（按位置排序）。

### 外键
通过 `all_constraints`（`constraint_type = 'R'`）+ `all_cons_columns` 视图，4 表 JOIN 对齐位置实现：

1. 查询本表外键列（`all_cons_columns` + `all_constraints`）
2. 查询引用表名 + 删除规则（`all_constraints` WHERE `constraint_type = 'P'`）
3. 查询引用列名（`all_cons_columns` 按 position 排序）

支持复合外键（多列），`ON DELETE` 规则从 `delete_rule` 字段获取（NO ACTION / CASCADE / SET NULL）。

## EXPLAIN 支持

Oracle 的 EXPLAIN 使用两步法：

```sql
-- Step 1: 写入计划表（PLAN_TABLE）
EXPLAIN PLAN FOR SELECT * FROM employees WHERE department_id = 10

-- Step 2: 读取格式化计划
SELECT * FROM TABLE(DBMS_XPLAN.DISPLAY())
```

**重要**：两步必须在同一个数据库 Session 中执行（PLAN_TABLE 是 Session 级别的）。实现使用 `db.Conn(ctx)` 固定的连接，避免 `sql.DB` 连接池将两步路由到不同连接导致空结果。

## AutoLimit 适配

工具自动将非 Oracle 风格的 `LIMIT N` 语法适配为 Oracle 12c+ 的 `FETCH FIRST N ROWS ONLY`：

```
SELECT * FROM users LIMIT 10
→ SELECT * FROM users FETCH FIRST 10 ROWS ONLY
```

使用 `(?i)\s+LIMIT\s+(\d+)\s*$` 正则匹配。**要求 Oracle 12c+**。

## 已知限制

1. **num_rows 不准确**：来自 `all_tables` 统计信息，需 `ANALYZE TABLE` 采集方准确
2. **dba_segments 未采集**：需要 DBA 权限，不表大小信息
3. **无 PL/SQL 对象**：不采集存储过程、函数、触发器
4. **无 TNS 支持**：不支持 `(DESCRIPTION=...)` 连接串格式，仅支持 `host:port/service` 格式
5. **FETCH FIRST 需要 12c+**：AutoLimit 和采样依赖 Oracle 12c+ 功能
6. **PLAN_TABLE 要求**：EXPLAIN 需要 PLAN_TABLE 存在（通常 Oracle 自动创建）
7. **ON UPDATE 不支持**：Oracle 不支持 `ON UPDATE CASCADE`（只有 `ON DELETE`）
8. **大小写敏感性**：Oracle 默认大写存储，连接串中的 DBName 自动转为大写

## 构建要求

Oracle 连接器通过 go-ora 纯 Go 驱动实现，无 CGO 依赖。包含方式：

```bash
# 包含 Oracle
bash build.sh minimal oracle

# 联合构建
bash build.sh minimal oracle,mysql,postgres

# 全量构建（包含 Oracle）
bash build.sh
# 或
bash build.sh prod
```
