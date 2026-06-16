# GaussDB 结构采集与排障手册

本文档详细说明 `dbexplain` 工具中 GaussDB 连接器（`connector/gaussdb.go` + `connector/postgres.go`）的实现机制。GaussDB 使用独立的 `gaussdbConnector`（复用 `postgres` 包级函数如 `collectPGDB()`、`buildPGDSN()`），通过 `lib/pq` 驱动连接，帮助理解其如何安全地获取数据库列表、表结构、列信息、索引和外键，并对无注释字段进行语义推断，同时提供常见问题的排障方法。

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

- **驱动选择**：使用 `github.com/lib/pq`，与 PostgreSQL 共用同一驱动。GaussDB 兼容 PostgreSQL 有线协议，因此无需独立驱动。
- **独立连接器**：GaussDB 使用 `gaussdb.go` 中独立的 `gaussdbConnector`，与 `postgresConnector` 分离。包级函数（`collectPGDB()`、`buildPGDSN()`、`executeSQLQuery()`）复用避免代码重复。
- **Kind 标识**：Kind 字段从 DSN 的 scheme 中提取（`gaussdb://`），由 `gaussdbConnector.Collect()` 设置为 `"gaussdb"`，确保输出报告中正确标识。
- **DSN 格式**：`gaussdb://user:password@host:25308/dbname?label=xxx&sslmode=disable`。GaussDB 默认端口为 25308（区别于 PostgreSQL 的 5432）。label 参数用于在 `.env` 文件中匹配配置项（`DBEXPLAIN_DSN_xxx`）。
- **SSL 控制**：通过 DSN 查询参数 `sslmode` 控制，支持 `disable`、`require`、`verify-ca`、`verify-full` 等标准 lib/pq 模式。默认使用 `sslmode=disable` 适用于内网环境；若服务器要求 SSL，需在 DSN 中指定 `sslmode=require` 并提供 CA 证书路径。
- **超时控制**：Ping 操作使用独立的 5 秒超时，避免长时间阻塞。
- **错误包装**：所有 error 均通过 `schema.NewDBError` 返回，记录脱敏 DSN 和操作上下文（`open`、`ping`、`list databases` 等）。

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

- **指定数据库优先**：若 DSN 中提供了库名（`/dbname` 路径段），则仅采集该库。
- **系统库过滤**：通过 `NOT datistemplate AND datallowconn` 排除模板库（`template0`、`template1`）和不允许连接的库，保留业务库及 `postgres` 系统库。
- **Oracle 兼容模式优化**：DSN 中设 `oracleCompatible=true` 时跳过 `datistemplate` 查询，直接使用 `SELECT datname FROM pg_database WHERE datallowconn`。Oracle 兼容模式（`DBCOMPATIBILITY='A'` / `'ORA'`）的 `pg_database` 表无 `datistemplate` 列，不设此参数会导致首次查询失败并触发自动回退（查询耗时 + 日志噪音）。
- **只读查询**：仅读取 `pg_database` 系统表，无任何数据修改风险。

### 1.3 表信息采集（多 Schema 支持）

```
rows, err := db.QueryContext(ctx, `
    SELECT table_schema, tablename,
           pg_size_pretty(pg_total_relation_size(quote_ident(table_schema)||'.'||quote_ident(tablename))),
           COALESCE(obj_description(quote_ident(table_schema)||'.'||quote_ident(tablename)::regclass,'pg_class'),'')
    FROM pg_tables
    WHERE table_schema NOT IN ('pg_catalog','information_schema')
    ORDER BY table_schema, tablename`)
```

- **多 Schema 覆盖**：与 PostgreSQL 连接器仅采集 `public` schema 不同，GaussDB 采集所有非系统 schema（排除 `pg_catalog` 和 `information_schema`）。对于 `auth`、`logs`、`billing` 等业务 schema，均会被完整采集。
- **表大小**：通过 `pg_total_relation_size` 获取表 + 索引总大小，并使用 `pg_size_pretty` 格式化为易读字符串（如 `256 MB`）。
- **表注释**：通过 `obj_description` 获取表级别 COMMENT，若为 NULL 则填充空字符串。
- **行数估算**：从 `pg_stat_user_tables.n_live_tup` 获取近似行数。该值为统计信息估算值（由 GaussDB 自动收集统计信息或手动 `ANALYZE` 触发），偏差通常在 10%~30%。

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
    ORDER BY a.attnum`, schemaName+"."+t.Name)
```

- **系统表查询**：使用 `pg_attribute`、`pg_attrdef`、`pg_constraint` 获取列名、类型、是否可空、默认值、注释和主键/唯一约束信息。
- **类型格式化**：通过 `format_type` 返回带修饰符的类型（如 `character varying(255)`、`numeric(10,2)`）。
- **约束检测**：将 `contype` 字符串聚合，若含 `p` 则标记 `IsPrimary`，`u` 则标记 `IsUnique`。
- **字段注释推断**：对于无注释的列，尝试获取一条样本行（`SELECT * FROM schema.tablename LIMIT 1`），通过 `schema.InferComment` 规则引擎生成语义注释（如"标识符"、"名称"、"金额"等）。
- **安全采样**：`LIMIT 1` 查询仅读取第一行，无大表全扫风险。若表为空或查询失败，仅记录日志，不中断采集。

### 1.5 索引与外键采集

**索引**：
```
idxRows, err := db.QueryContext(ctx, `
    SELECT indexname, indexdef FROM pg_indexes
    WHERE schemaname=$1 AND tablename=$2`, schemaName, t.Name)
```
- 从 `pg_indexes` 视图读取索引定义，解析出唯一性和列名列表。GaussDB 的全局分区索引、本地分区索引等特殊索引类型同样会被采集。

**外键**：
```
fkRows, err := db.QueryContext(ctx, `
    SELECT c.conname, a.attname, c2.relname, a2.attname,
           c.confupdtype, c.confdeltype
    FROM pg_constraint c
    JOIN pg_class c1 ON c1.oid=c.conrelid
    JOIN pg_class c2 ON c2.oid=c.confrelid
    JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
    JOIN pg_attribute a2 ON a2.attrelid=c.confrelid AND a2.attnum=ANY(c.confkey)
    WHERE c.contype='f' AND c1.relname=$1`, t.Name)
```
- **外键动作映射**：`confupdtype` 和 `confdeltype` 列记录 `ON UPDATE` 和 `ON DELETE` 的外键动作（`a` = NO ACTION, `r` = RESTRICT, `c` = CASCADE, `n` = SET NULL, `d` = SET DEFAULT），通过 `pgFKAction()` 函数映射为可读字符串。
- 仅查询外键约束（`contype='f'`），跨 schema 的引用也会被正确采集。

### 1.6 操作统计信息采集

```
SELECT schemaname, relname, seq_scan, idx_scan,
       n_tup_ins, n_tup_upd, n_tup_del
FROM pg_stat_user_tables
WHERE schemaname=$1 AND relname=$2
```

- **扫描统计**：`seq_scan`（全表顺序扫描次数）和 `idx_scan`（索引扫描次数）帮助评估索引使用效率。
- **DML 统计**：`n_tup_ins`（插入行数）、`n_tup_upd`（更新行数）、`n_tup_del`（删除行数）提供表的读写负载特征，辅助 AI Agent 识别热点表。

### 1.7 错误处理与进度日志

- 所有数据库操作均通过 `schema.NewDBError` 包装，记录操作类型（`query columns`、`query tables`、`ping` 等）。
- 使用 `logf` 输出进度到 `logs/<label>.log`，包括数据库名、schema 名、表采集序号（`采集表 X/总数`）以及采样失败警告。
- 即使单个表或列查询失败，工具也会继续处理剩余对象，不会中断整个实例的采集。

---

## 二、常见问题与排障

### 2.1 连接被拒绝或认证失败

**错误示例**：
```
skip gaussdb://...: gaussdb ping: dial tcp x.x.x.x:25308: connect: connection refused
```

**可能原因与解决方案**：
- **端口错误**：GaussDB 默认监听 25308 端口，确认 DSN 中端口号正确。若为集群部署，协调节点（CN）和数据节点（DN）端口可能不同，使用 CN 节点端口连接。
- **pg_hba.conf 未授权**：检查 GaussDB 的 `pg_hba.conf` 配置文件，确保客户端 IP 被允许。修改后执行 `gs_ctl reload` 重新加载配置。
- **用户或密码错误**：确认 DSN 中的用户名和密码。GaussDB 中默认管理员用户通常为 `gaussdb` 或安装时指定的用户。
- **SSL 要求冲突**：如果服务器配置了 `ssl=on`，需要在 DSN 中添加 `sslmode=require` 参数。

### 2.2 采集不到任何表或数据库

- **Schema 权限不足**：用户需具有各业务 schema 的 `USAGE` 权限。检查 `GRANT USAGE ON SCHEMA xxx TO user` 是否已授权。
- **统计信息未收集**：若 `pg_stat_user_tables.n_live_tup` 返回 0，可能是因为未执行 `ANALYZE`。运行 `ANALYZE;` 更新统计信息后重新采集。
- **数据库为空**：如果数据库尚未创建任何业务表，报表会显示 `(no tables)`，属正常情况。

### 2.3 字段注释推断不理想或错误

- 推断基于首行数据，若首行数据为空或为默认值，生成的注释可能不准确。可视为补充信息，不应完全依赖。
- 若需要更精确的注释，建议通过 `COMMENT ON COLUMN schema.tablename.columnname IS '描述';` 手动添加，工具会优先使用原生注释。
- 可调整 `schema.InferComment` 函数以满足特定业务领域术语需求。

### 2.4 外键关系缺失或不完整

- 工具仅采集通过 `FOREIGN KEY` 约束定义的外键（`contype='f'`），不自动推断命名规则外键。推断关系由分析模块（`analyze/analyze.go`）完成。
- 若外键引用另一个数据库或实例，GaussDB 无法记录，需后续分析逻辑补充。

### 2.5 性能与超时

- GaussDB 集群环境下的网络延迟可能高于单机 PostgreSQL，建议适当增加超时：`-timeout 60s`。
- 若 schema 数量多且每个 schema 下表数量大（>200），采集时间会显著增加，建议分批采集。
- 操作统计信息查询（`pg_stat_user_tables`）在大规模集群中可能稍慢，属正常行为。

---

## 三、核心命令速查（gsql 验证）

| 目的 | gsql 命令 |
|------|-----------|
| 测试连接 | `gsql -h host -p 25308 -U user -d dbname -c "SELECT 1"` |
| 列出所有数据库 | `\l` 或 `SELECT datname FROM pg_database WHERE NOT datistemplate;` |
| 列出所有 schema | `\dn` 或 `SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('pg_catalog','information_schema');` |
| 查看某 schema 下表 | `\dt schema_name.*` |
| 查看表结构 | `\d+ schema_name.tablename` |
| 查看表大小 | `SELECT pg_size_pretty(pg_total_relation_size('schema_name.tablename'));` |
| 查看行数估算 | `SELECT schemaname, relname, n_live_tup FROM pg_stat_user_tables WHERE relname='tablename';` |
| 查看外键 | `SELECT conname, conrelid::regclass, confrelid::regclass, confupdtype, confdeltype FROM pg_constraint WHERE contype='f' AND conrelid='schema.tablename'::regclass;` |

---

## 四、经验总结

1. **与 PostgreSQL 的异同**：GaussDB 共用 PostgreSQL 连接器代码（`postgres.go`），但通过 DSN scheme（`gaussdb://`）区分 Kind 标识，确保报告中正确标注数据库类型。多 Schema 覆盖能力使其更适合大型企业级部署。
2. **权限最小化**：工具仅需 `CONNECT` 和对 `pg_catalog` 的 `SELECT` 权限。建议创建专用只读角色：`CREATE ROLE dbexplain_reader LOGIN PASSWORD 'xxx'; GRANT CONNECT ON DATABASE dbname TO dbexplain_reader; GRANT USAGE ON SCHEMA schema_name TO dbexplain_reader; GRANT SELECT ON ALL TABLES IN SCHEMA schema_name TO dbexplain_reader;`。
3. **SSL 配置**：生产环境建议启用 SSL，在 DSN 中使用 `sslmode=require`。若使用自签名证书，需配合 `sslcert`、`sslkey`、`sslrootcert` 参数指定证书路径。
4. **行数统计依赖**：`n_live_tup` 的准确性取决于统计信息的新鲜度，在频繁写入的场景下建议定期执行 `ANALYZE`。不应用于精确计数场景。
5. **多 Schema 注意事项**：GaussDB 默认采集所有非系统 schema，若实例中包含数百个 schema，建议通过 DSN 中指定库名来缩小范围，或增加 `-timeout` 值。
6. **分区表**：GaussDB 的分区表（包括范围分区、列表分区、哈希分区）通过 `pg_tables` 可正常采集，子分区表作为独立表出现在表列表中。
7. **日志记录**：所有查询错误被记录到 `logs/<label>.log`，包括权限不足、超时、连接中断等。排查问题时优先查看此日志。
8. **版本兼容**：测试于 GaussDB 200/300 主版本，兼容 GaussDB for openGauss 以及华为云 GaussDB 服务。`pg_catalog` 系统表结构与标准 PostgreSQL 高度一致，采集逻辑通用。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 GaussDB 实例进行结构探查与语义丰富。
