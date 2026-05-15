# PostgreSQL 结构采集与排障手册

本文档详细说明 `dbexplain` 工具中 PostgreSQL 连接器（`connector/postgres.go`）的实现机制，帮助理解其如何安全地获取数据库列表、表结构、列信息、索引和外键，并对无注释字段进行语义推断，同时提供常见问题的排障方法。

---

## 一、代码中的重要机制

### 1.1 连接建立与安全 Ping

```
connStr := buildPGDSN(d)
db, err := sql.Open("postgres", connStr)
defer db.Close()

pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
if err := db.PingContext(pingCtx); err != nil { ... }
```

- **驱动选择**：使用 `github.com/lib/pq` 纯 Go 实现 PostgreSQL 协议，无需 CGO。  
- **DSN 构建**：将 DSN 参数转换为 `host=... port=... user=... password=... dbname=... sslmode=disable connect_timeout=5` 格式，硬编码关闭 SSL（`sslmode=disable`），适用于内网环境或未配置 TLS 的数据库。若需 SSL 需修改代码。  
- **超时控制**：Ping 操作使用独立 5 秒超时，避免长时间阻塞。  
- **错误包装**：所有 error 均通过 `schema.NewDBError` 返回，包含脱敏 DSN 和上下文（`open`、`ping`、`list databases` 等），便于定位。

### 1.2 数据库列表获取

```
if d.DBName != "" {
    dbNames = []string{d.DBName}
} else {
    rows, err := db.QueryContext(ctx, 
        `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
    ...
}
```

- **指定数据库**：若 DSN 中提供了库名，则只采集该库，跳过其他。  
- **系统库过滤**：通过 `NOT datistemplate AND datallowconn` 排除模板库和不允许连接的库（如 `template0`、`template1`），保留业务库和 `postgres` 等。  
- **只读查询**：仅读取系统表 `pg_database`，无任何修改风险。

### 1.3 表信息采集

```
rows, err := db.QueryContext(ctx, `
    SELECT tablename,
           pg_size_pretty(pg_total_relation_size(quote_ident(tablename))),
           COALESCE(obj_description(quote_ident(tablename)::regclass,'pg_class'),'')
    FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
```

- **仅公共模式**：默认只采集 `public` schema 下的表。若需采集其他 schema，需修改代码或添加 `search_path` 参数。  
- **表大小**：通过 `pg_total_relation_size` 获取表+索引总大小，并使用 `pg_size_pretty` 格式化为易读字符串（如 `120 kB`）。  
- **表注释**：通过 `obj_description` 获取表级别注释，若为 NULL 则填充空字符串。  
- **行数估算**：当前代码未单独获取行数（仅取表大小），可通过 `pg_stat_user_tables.n_live_tup` 扩展，但当前版本未实现。

### 1.4 列信息采集与语义推断

```
colRows, err := db.QueryContext(ctx, `
    SELECT a.attname,
           pg_catalog.format_type(a.atttypid, a.atttypmod),
           NOT a.attnotnull,
           COALESCE(pg_get_expr(d.adbin, d.adrelid),''),
           COALESCE(col_description(a.attrelid, a.attnum),''),
           COALESCE((SELECT string_agg(contype::text,'')
                     FROM pg_constraint c
                     WHERE a.attnum = ANY(c.conkey) AND c.conrelid = a.attrelid),'')
    FROM pg_attribute a
    LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
    WHERE a.attrelid=$1::regclass AND a.attnum>0 AND NOT a.attisdropped
    ORDER BY a.attnum`, "public."+t.Name)
```

- **系统表查询**：使用 `pg_attribute`、`pg_attrdef`、`pg_constraint` 获取列名、类型、是否可空、默认值、注释、主键/唯一约束信息。  
- **类型格式化**：通过 `format_type` 返回带修饰符的类型（如 `character varying(100)`），直接呈现在报告中。  
- **约束检测**：将 `contype` 字符串聚合，若含 `p` 则为主键，`u` 为唯一，同时写入 `IsPrimary`、`IsUnique` 字段。  
- **字段注释推断**：对于无注释的列（`col_description` 返回空），尝试从表中获取一条样本行（`LIMIT 1`），并通过 `schema.InferComment` 规则引擎基于列名、类型和样本值生成注释（如“标识符”、“名称”、“时间”等）。  
- **安全采样**：采样查询使用 `SELECT * FROM public."表名" LIMIT 1`，仅读取第一行，无大表全扫风险。若表为空或查询失败，不会中断采集，仅记录日志并跳过推断。

### 1.5 索引与外键采集

**索引**：
```
idxRows, err := db.QueryContext(ctx, `
    SELECT indexname, indexdef FROM pg_indexes
    WHERE schemaname='public' AND tablename=$1`, t.Name)
```
- 从 `pg_indexes` 视图读取索引定义，解析出唯一性和列名列表。  
- 不对索引进行分类，仅输出名称和列（如 `UNI(rtsp_url)`）。

**外键**：
```
fkRows, err := db.QueryContext(ctx, `
    SELECT c.conname, a.attname, c2.relname, a2.attname
    FROM pg_constraint c
    JOIN pg_class c1 ON c1.oid=c.conrelid
    JOIN pg_class c2 ON c2.oid=c.confrelid
    JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
    JOIN pg_attribute a2 ON a2.attrelid=c.confrelid AND a2.attnum=ANY(c.confkey)
    WHERE c.contype='f' AND c1.relname=$1`, t.Name)
```
- 仅查询外键约束（`contype='f'`），通过连接 `pg_attribute` 获取本表列和引用表列。  
- 外键的 `RefInstance` 默认为空，表示同一实例内引用，若跨库/跨实例引用需通过命名推断补充（由分析模块完成）。

### 1.6 错误处理与进度日志

- 所有数据库操作均通过 `schema.NewDBError` 包装，记录操作类型（`query columns`、`query tables`、`ping` 等）。  
- 使用 `logf` 输出进度到 `logs/<label>.log`，包括数据库名、表采集序号（`采集表 X/总数`）以及采样失败警告。  
- 即使单个表或列查询失败，工具也会继续处理剩余对象，不会中断整个实例的采集。

---

## 二、常见问题与排障

### 2.1 连接被拒绝或认证失败

**错误示例**：
```
skip postgres://...: postgres ping: dial tcp x.x.x.x:5432: connect: connection refused
```
**可能原因与解决方案**：
- **服务未启动或监听地址错误**：检查 PostgreSQL 是否运行，`pg_isready -h host -p port`。  
- **pg_hba.conf 未授权客户端 IP**：修改配置文件允许该 IP 的连接，并执行 `pg_ctl reload`。  
- **用户或密码错误**：确认 DSN 中的用户名和密码正确，注意密码中的特殊字符在命令行中需用单引号包裹或在 `.env` 文件中直接书写。  
- **SSL 要求**：如果服务器要求 SSL（`sslmode=require`），当前工具硬编码 `sslmode=disable`，会导致连接被拒。需修改 `buildPGDSN` 函数增加 SSL 选项或导出为参数。

### 2.2 采集不到任何表或数据库

- 检查 `search_path` 设置：工具只采集 `public` schema。若表在其他 schema（如 `custom_schema`），需临时将表移动到 public 或修改代码增加 schema 参数。  
- 数据库权限不足：用户需具有 `CONNECT` 和 `SELECT` 系统表权限（通常对 `pg_catalog` 有默认只读权限）。  
- 数据库为空或无业务表：报表会显示 `(no columns)` 或表数为 0，属正常。

### 2.3 字段注释推断不理想或错误

- 推断基于首行数据，若首行数据为空或为默认值，生成的注释可能不准确。可视为补充信息，不应完全依赖。  
- 若需要更精确的注释，建议在数据库中通过 `COMMENT ON COLUMN` 手动添加，工具会优先使用原生注释。  
- 可调整 `schema.InferComment` 函数以满足特定领域需求。

### 2.4 外键关系缺失或错误

- 工具仅采集基于约束的外键，不自动推断命名规则的外键（如 `user_id` -> `users.id`）。推断关系由分析模块（`analyze/analyze.go`）完成，与连接器无关。  
- 如果外键约束在另一个数据库或实例中，连接器可能无法自动补全实例名，需后续分析逻辑补充。

### 2.5 性能与超时

- 采集时间随表数量线性增加，因为每个表需要查询列、索引、外键（3-5 次查询）。  
- 可通过 `-timeout 30s` 增加全局超时。  
- 若表数量极大（>500），建议分批采集或增加 `perDSNTimeout` 值。

---

## 三、核心命令速查（psql 验证）

| 目的 | psql 命令 |
|------|-----------|
| 测试连接 | `psql -h host -p port -U user -d dbname -c "SELECT 1"` |
| 列出所有数据库 | `\l` 或 `SELECT datname FROM pg_database WHERE NOT datistemplate;` |
| 查看当前 schema 的所有表 | `\dt public.*` |
| 查看表结构 | `\d+ tablename` |
| 查看表大小 | `SELECT pg_size_pretty(pg_total_relation_size('public.tablename'));` |
| 查看表注释 | `SELECT obj_description('public.tablename'::regclass, 'pg_class');` |
| 查看列注释 | `SELECT col_description('public.tablename'::regclass, attnum) FROM pg_attribute WHERE attname='colname';` |

---

## 四、经验总结

1. **权限最小化**：工具仅需 `CONNECT` 和 `pg_catalog` 的 `SELECT` 权限，无需对业务表拥有任何权限（因 `pg_class` 等是共享系统表），但仍建议创建专用只读角色。  
2. **SSL 处理**：生产环境若必须 SSL，应在连接字符串中添加 `sslmode=require` 并修改代码支持，或通过 SSL 隧道（如 stunnel）转换为非 SSL 连接。  
3. **Schema 范围**：当前限定 public 是常见默认行为，但多 schema 场景（如 `auth`、`logs`）可能遗漏数据，需根据需求扩展代码或使用 `search_path` 参数。  
4. **行数获取**：`pg_stat_user_tables.n_live_tup` 提供近似行数，但当前版本未使用，若要展示行数可扩展 `fillPGTable` 函数。  
5. **采样安全性**：`LIMIT 1` 查询是安全的，但在超大表中若存在长时间运行的事务可能会等待，一般不超过几毫秒。  
6. **日志记录**：所有查询错误会被记录到 `logs/<label>.log`，方便排查权限或网络问题。  
7. **版本兼容**：测试于 PostgreSQL 12~16 均正常，较低版本（如 9.x）部分系统视图差异可能影响结果，但大多数查询兼容。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 PostgreSQL 实例进行结构探查与语义丰富。 