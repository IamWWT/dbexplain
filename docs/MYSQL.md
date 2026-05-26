# MySQL 结构采集与排障手册

本文档详细说明 `dbexplain` 工具中 MySQL 连接器（`connector/mysql.go`）的实现机制，帮助理解其如何通过标准数据库驱动安全地获取数据库列表、表结构、列信息、索引、外键，并对无注释字段进行语义推断，同时提供常见问题的排障方法。

---

## 一、代码中的重要机制

### 1.1 连接建立与安全 Ping

```
connStr := buildMySQLDSN(d)
db, err := sql.Open("mysql", connStr)
defer db.Close()

pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
if err := db.PingContext(pingCtx); err != nil { ... }
```

- **驱动选择**：使用 `github.com/go-sql-driver/mysql`，纯 Go 实现，无需 CGO。
- **DSN 构建**：自动生成 `用户名:密码@tcp(主机:端口)/库名?charset=utf8mb4&parseTime=true&timeout=5s` 格式的连接字符串，支持连接超时和字符集设置。
- **超时控制**：Ping 操作使用独立的 5 秒超时，避免长时间阻塞。
- **错误包装**：所有 error 均通过 `schema.NewDBError` 返回，记录脱敏 DSN 和操作上下文（`open`、`ping`、`list databases` 等）。

### 1.2 数据库列表获取

```
if d.DBName != "" {
    dbNames = []string{d.DBName}
} else {
    rows, err := db.QueryContext(ctx, "SHOW DATABASES")
    ...
    if !isMySQLSystemDB(n) {
        dbNames = append(dbNames, n)
    }
}
```

- **指定库优先**：若 DSN 中提供了库名，则只采集该库。
- **系统库过滤**：排除 `information_schema`、`performance_schema`、`mysql`、`sys` 四个系统库，仅保留业务库。
- **只读操作**：`SHOW DATABASES` 不涉及任何数据修改。

### 1.3 表信息采集

```
rows, err := db.QueryContext(ctx, `
    SELECT TABLE_NAME, COALESCE(TABLE_ROWS,0), COALESCE(DATA_LENGTH+INDEX_LENGTH,0),
           COALESCE(TABLE_COMMENT,''), COALESCE(ENGINE,'')
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'
    ORDER BY TABLE_NAME`, dbName)
```

- **仅采集基表**：`TABLE_TYPE='BASE TABLE'` 过滤掉视图和系统视图。
- **表属性**：获取近似行数（`TABLE_ROWS`，InnoDB 为估算值）、数据+索引大小（`DATA_LENGTH+INDEX_LENGTH`）、注释（`TABLE_COMMENT`）、引擎类型（`ENGINE`）。
- **参数化查询**：使用 `?` 占位符防止 SQL 注入。
- **进度日志**：每采集一个表输出 `[dbName] 采集表 X/总数: 表名`。

### 1.4 列信息与语义推断

**列信息**：
```
colRows, err := db.QueryContext(ctx, `
    SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY,
           COALESCE(COLUMN_DEFAULT,''), EXTRA, COALESCE(COLUMN_COMMENT,'')
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA=? AND TABLE_NAME=?
    ORDER BY ORDINAL_POSITION`, dbName, t.Name)
```

- **完整属性**：包括列名、类型（含长度）、是否可空、键类型（`PRI`/`UNI`/`MUL`）、默认值、额外信息（如 `auto_increment`）、注释。
- **标志推断**：
  - `IsPrimary`：`COLUMN_KEY = 'PRI'`
  - `IsUnique`：`COLUMN_KEY = 'UNI'`
  - `IsIndex`：`COLUMN_KEY = 'MUL'`（非唯一索引）
- **无注释推断**：对于 `COLUMN_COMMENT` 为空的列，尝试获取一行样本数据（`LIMIT 1`），调用 `schema.InferComment` 根据列名、类型和样本值生成语义注释（如“标识符”、“金额/数量”等）。
- **安全采样**：使用 `SELECT * FROM 库名.表名 LIMIT 1`，仅读取第一行，无全表扫描风险。若表为空或查询失败，仅记录日志，不影响整体流程。

### 1.5 索引与外键采集

**索引（非主键）**：
```
SHOW INDEX FROM `表名` WHERE Key_name != 'PRIMARY'
```
- 解析 `SHOW INDEX` 结果，按索引名聚合列列表，记录 `Unique` 标志（`Non_unique = 0`）。
- 主键索引单独查询（`Key_name = 'PRIMARY'`），作为特殊索引添加到列表中。

**外键**：
```
SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME,
       k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE k
WHERE k.TABLE_SCHEMA=? AND k.TABLE_NAME=? AND k.REFERENCED_TABLE_NAME IS NOT NULL
ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION
```

- **仅显式外键**：通过 `REFERENCED_TABLE_NAME IS NOT NULL` 过滤，返回约束名、本表列、引用库、引用表、引用列。
- **跨库引用**：保留 `RefDB` 信息，若引用同一实例的其他库，会在报告中正确显示。
- **聚合处理**：同一约束下的多列外键会被合并为单一 `ForeignKey` 对象。

### 1.6 标识符转义与 SQL 注入防护

- 使用 `quoteMySQL` 函数将表名和库名中的反引号转义（`` ` `` → `` `` ``），并包裹反引号，安全拼接 `SHOW INDEX FROM` 和采样查询。
- 系统表查询均采用参数化占位符，杜绝注入风险。

### 1.7 错误处理与日志

- 所有数据库操作均使用 `schema.NewDBError` 包装，提供脱敏 DSN、数据库名、表名、操作类型（如 `query columns`、`query tables`）。
- 进度信息通过 `logf` 写入 `logs/<label>.log`，包括数据库采集开始、表采集进度、采样失败警告。
- 即使单个表或列查询失败，工具会继续处理下一个表，不影响其他数据库的采集。

---

## execute 只读查询

`dbexplain` 提供 `execute` 子命令，支持对 MySQL 实例执行只读 SQL 查询，安全地将结果以表格形式输出到终端。

### 查询格式

标准 SQL 语句，支持 `SELECT`、`EXPLAIN`、`WITH`（CTE）、`SHOW`、`DESCRIBE`/`DESC`、`PRAGMA` 等只读操作。

### 校验机制

- **SQLGuard 动词白名单**：所有查询在到达数据库之前，先经过 `sqlguard` 模块的语句动词白名单校验，仅允许 `SELECT`、`EXPLAIN`、`WITH`、`SHOW`、`DESCRIBE`、`DESC`、`PRAGMA` 七类只读动词通过。任何包含 `INSERT`、`UPDATE`、`DELETE`、`DROP`、`ALTER` 等写操作的语句将被拒绝。
- **多语句检测**：禁止分号分隔的多条 SQL 语句，防止通过 `SELECT 1; DROP TABLE ...` 等方式绕过白名单。

### 自动 LIMIT 追加

- 若 `SELECT`、`WITH`、`EXPLAIN` 语句中未显式包含 `LIMIT` 子句，工具会自动追加 `LIMIT 1000`，防止全量数据刷屏或大结果集耗竭内存。
- `SHOW`、`DESCRIBE` 等命令不追加 LIMIT，因其返回行数天然可控。

### 超时控制

- **数据库层**：执行前通过 `SET SESSION max_execution_time=N000`（N 为毫秒数）设置 MySQL 会话级最大执行时间，查询超时将由 MySQL Server 主动终止。
- **应用层**：通过 Go `context.WithTimeout` 设置应用级超时，双重保障避免长时间阻塞。

### 执行方式

使用 Go 标准库 `database/sql` 统一接口，底层驱动为 `go-sql-driver/mysql`，通过参数化查询执行。

### 最大行数控制

由 `--limit` 命令行标志控制，默认值为 1000。超出该行数的结果将被截断。

---

## 二、常见问题与排障

### 2.1 连接被拒绝或认证失败

**错误示例**：
```
skip mysql://...: mysql ping: dial tcp 127.0.0.1:3306: connect: connection refused
```

**可能原因与解决方案**：
- **服务未运行**：检查 MySQL 服务状态，确认监听端口（默认 3306）。
- **防火墙或云安全组**：确保客户端 IP 被允许访问 MySQL 端口。
- **用户权限**：用户需至少具有 `SELECT` 权限于 `information_schema` 和业务库，建议授予 `SELECT ON *.*` 或更细粒度的只读权限。
- **密码特殊字符**：密码中包含 `#`、`!`、`@` 等字符时，在命令行中需用单引号包裹整个 DSN，或在 `.env` 文件中直接书写。

### 2.2 表数量多导致采集时间过长

- 采集时间与表数量成线性关系（每个表约 3-5 次查询）。
- 可通过 `-timeout 30s` 增加全局超时。
- 日志文件会显示每个表的采集进度，可通过 `tail -f logs/<label>.log` 观察。

### 2.3 外键关系未显示或不全

- 工具仅采集通过 `FOREIGN KEY` 约束定义的外键，不自动推断命名规则外键（如 `user_id` -> `users.id`）。推断关系由分析模块（`analyze/analyze.go`）完成。
- 若跨实例引用，外键信息可能不完整，因为 MySQL 中无法直接记录远程实例。

### 2.4 注释推断不理想

- 推断基于首行数据，若首行数据为空或为默认值，生成的注释可能不准确。
- 建议在数据库中手动添加 `COMMENT ON COLUMN`，工具会优先使用原生注释。
- 可修改 `schema.InferComment` 函数适应特定业务术语。

### 2.5 行数显示不准确

- InnoDB 的 `TABLE_ROWS` 是估算值，可能与实际行数有 10%~30% 的偏差，仅供参考。

---

## 三、核心命令速查（mysql 客户端验证）

| 目的 | 命令 |
|------|------|
| 测试连接 | `mysql -h host -P port -u user -p -e "SELECT 1"` |
| 列出所有数据库 | `SHOW DATABASES;` |
| 查看库内所有表 | `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='dbname';` |
| 查看表结构 | `DESCRIBE dbname.tablename;` 或 `SHOW CREATE TABLE dbname.tablename;` |
| 查看表注释 | `SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA='db' AND TABLE_NAME='tbl';` |
| 查看列注释 | `SELECT COLUMN_NAME, COLUMN_COMMENT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='db' AND TABLE_NAME='tbl';` |
| 查看索引 | `SHOW INDEX FROM dbname.tablename;` |
| 查看外键 | `SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA='db' AND TABLE_NAME='tbl' AND REFERENCED_TABLE_NAME IS NOT NULL;` |

---

## 四、经验总结

1. **权限最小化**：工具仅需 `SELECT` 权限于 `information_schema` 及业务库，无需写入或 DDL 权限。
2. **参数化安全**：所有系统表查询均使用占位符，杜绝 SQL 注入；`SHOW INDEX` 等动态语句通过转义反引号保护。
3. **行数近似**：InnoDB 行数是估算值，不要依赖其精确性，主要用于感知表大小。
4. **采样安全性**：`LIMIT 1` 查询快速且轻量，不会对生产环境造成负载。
5. **多库分析**：若一个实例包含大量数据库（>50），建议使用 `-timeout` 调大超时，或分批指定库名采集。
6. **日志详查**：所有错误和进度记录在 `logs/<label>.log`，排查问题时优先查看。
7. **外键与关系**：显式外键能准确显示，但业务中大量隐含的引用关系需依赖分析模块的命名推断补充，工具会一并呈现。
8. **版本兼容**：测试于 MySQL 5.7、8.0 及 MariaDB 10.x，`information_schema` 结构一致，均可正常工作。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 MySQL 实例进行结构探查与语义丰富。 