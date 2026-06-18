package manual

import "fmt"

func printManualMySQL(p func(string, string) string) {
	fmt.Print(p(`

─── MySQL ─────────────────────────────────────────────────────

    DSN 格式:
      mysql://用户:密码@主机:端口/库名?label=别名

    端口: 默认 3306
    库名: 必填 (或用 SHOW DATABASES 全量采集非系统库)

    采集机制:
      • 表元数据 — INFORMATION_SCHEMA.TABLES: 表名、引擎(InnoDB/MyISAM)、
        行数估算(TABLE_ROWS)、数据大小、表注释
      • 列信息   — SHOW FULL COLUMNS: 名称、类型、可空、键类型(PRI/UNI/MUL)、
        默认值、注释；对无注释字段取首行数据通过规则引擎推断语义
      • 索引     — SHOW INDEX FROM: 主键(PRIMARY) + 二级索引，含列列表与唯一性
      • 外键     — INFORMATION_SCHEMA.KEY_COLUMN_USAGE: 约束名、本表列、
        引用库/表/列
      • 主键识别 — SHOW INDEX WHERE Key_name='PRIMARY'

    安全机制:
      • 跳过系统库: information_schema, performance_schema, mysql, sys
      • 所有查询使用参数化，标识符严格转义
      • 密码在日志和输出中脱敏 (替换为 ***)

    示例:
      dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
`,
		`

─── MySQL ─────────────────────────────────────────────────────

    DSN format:
      mysql://user:password@host:port/dbname?label=alias

    Port: default 3306
    DBName: required (or all non-system DBs are auto-collected)

    Collection mechanism:
      • Table metadata — INFORMATION_SCHEMA.TABLES: name, engine (InnoDB/MyISAM),
        row estimate (TABLE_ROWS), data size, table comment
      • Column info   — SHOW FULL COLUMNS: name, type, nullable, key (PRI/UNI/MUL),
        default, comment; uncommented columns get semantic inference from sample row
      • Indexes       — SHOW INDEX FROM: PRIMARY key + secondary indexes, with
        column lists and uniqueness
      • Foreign keys  — INFORMATION_SCHEMA.KEY_COLUMN_USAGE: constraint name,
        local columns, referenced DB/table/columns
      • PK detection  — SHOW INDEX WHERE Key_name='PRIMARY'

    Safety:
      • Skips system DBs: information_schema, performance_schema, mysql, sys
      • All queries parameterized, identifiers strictly escaped
      • Passwords redacted in logs and output (replaced with ***)

    Example:
      dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
`))
}

func printManualPostgres(p func(string, string) string) {
	fmt.Print(p(`

─── PostgreSQL ────────────────────────────────────────────────

    DSN 格式:
      postgres://用户:密码@主机:端口/库名?label=别名&sslmode=disable

    端口: 默认 5432
    别名: postgres, postgresql, pg

    特有参数:
      sslmode=<mode>  SSL 连接模式:
                        disable (默认) — 不加密
                        require       — 必须 SSL，不验证证书
                        verify-ca     — 验证服务器证书由可信 CA 签发
                        verify-full   — 验证证书 + 主机名匹配

    采集机制:
      • 多 Schema — 自动采集所有非系统 schema (pg_catalog/information_schema 除外)
      • 库列表   — pg_database (跳过模板库和不可连接库)
      • 表元数据 — pg_class + pg_namespace: 表名、行数(n_live_tup)、
        体积(pg_total_relation_size)、注释(obj_description)
      • 列信息   — pg_attribute + pg_class + pg_namespace:
        名称、类型(format_type)、可空、默认值、主键/唯一约束标记、注释(col_description)
      • 索引     — pg_indexes: 名称、唯一性、列列表
      • 外键     — pg_constraint (confreltype='f'): 约束名、列、引用表/列
      • 行数统计 — pg_stat_user_tables.n_live_tup (近似值)

    安全机制:
      • SSL 可配置，默认 disable (兼容内网环境)
      • 跳过系统 schema: pg_%, information_schema
      • 参数化查询，密码脱敏

    示例:
      dbexplain -dsn 'postgres://u:p@host:5432/warehouse?label=my-pg&sslmode=disable'
`,
		`

─── PostgreSQL ────────────────────────────────────────────────

    DSN format:
      postgres://user:password@host:port/dbname?label=alias&sslmode=disable

    Port: default 5432
    Aliases: postgres, postgresql, pg

    Specific parameters:
      sslmode=<mode>  SSL connection mode:
                        disable (default) — no encryption
                        require       — SSL required, no cert verification
                        verify-ca     — verify server cert signed by trusted CA
                        verify-full   — verify cert + hostname match

    Collection mechanism:
      • Multi-schema — Auto-collects all non-system schemas (excluding
        pg_catalog / information_schema)
      • DB list      — pg_database (skips templates and non-connectable DBs)
      • Table meta   — pg_class + pg_namespace: name, row count (n_live_tup),
        size (pg_total_relation_size), comment (obj_description)
      • Column info  — pg_attribute + pg_class + pg_namespace:
        name, type (format_type), nullable, default, PK/UNIQUE flag, comment (col_description)
      • Indexes      — pg_indexes: name, uniqueness, column list
      • Foreign keys — pg_constraint (confreltype='f'): constraint name, columns,
        referenced table/columns
      • Row stats    — pg_stat_user_tables.n_live_tup (approximate)

    Safety:
      • SSL configurable, defaults to disable (internal network friendly)
      • Skips system schemas: pg_%, information_schema
      • Parameterized queries, password redaction

    Example:
      dbexplain -dsn 'postgres://u:p@host:5432/warehouse?label=my-pg&sslmode=disable'
`))
}

func printManualGaussDB(p func(string, string) string) {
	fmt.Print(p(`

─── GaussDB (华为高斯) ────────────────────────────────────────

    DSN 格式:
      gaussdb://用户:密码@主机:端口/库名?label=别名&sslmode=disable[&oracleCompatible=true]

    端口: 默认 25308
    别名: gaussdb, opengauss
    可选参数:
      oracleCompatible=true — Oracle 兼容模式优化，跳过 datistemplate 查询

    Oracle 兼容模式 (DBCOMPATIBILITY='A' / 'ORA'):
      GaussDB 使用 PostgreSQL 协议通过 gaussdb-go 驱动连接（华为 fork 的 pgx，原生支持 SHA256/SM3 认证）。
      采集机制与 PostgreSQL 基于相同的 pg_catalog 系统表。
      已实机验证（v0.1.7）在 Oracle 兼容模式下可用。

    采集机制:
      • 多 Schema — 自动采集所有非系统 schema (public 非必需)
      • 库列表   — pg_database (oracleCompatible 时跳过 datistemplate 查询)
      • 表元数据 — pg_class + pg_namespace JOIN（不使用 ::regclass）
      • 列信息   — pg_attribute + pg_constraint: 名称、类型、可空、默认值、
        主键/唯一约束、注释
      • 索引     — pg_indexes
      • 外键     — pg_constraint (含 ON UPDATE / ON DELETE 动作)
      • 行数统计 — pg_stat_user_tables.n_live_tup (近似值)

    已知限制 (Oracle 兼容模式):
      • EXPLAIN 不支持 BUFFERS 选项（自动省略）
      • statement_timeout GUC 可能不兼容（应用层超时兜底）
      • ::regclass 类型转换不可用（已通过 JOIN 替代）
      • 分布键、分区信息尚未采集

    示例:
      dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
      dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?oracleCompatible=true&label=my-gauss-oracle'
`,
		`

─── GaussDB (Huawei) ──────────────────────────────────────────

    DSN format:
      gaussdb://user:password@host:port/dbname?label=alias&sslmode=disable[&oracleCompatible=true]

    Port: default 25308
    Aliases: gaussdb, opengauss
    Optional params:
      oracleCompatible=true — skip datistemplate query for Oracle-compatible mode

    Oracle-compatible mode (DBCOMPATIBILITY='A' / 'ORA'):
      GaussDB connects via PostgreSQL wire protocol using gaussdb-go driver (Huawei fork of pgx with native SHA256/SM3 auth).
      Collection mechanism is based on pg_catalog, same as PostgreSQL.
      Verified (v0.1.7) working in Oracle-compatible mode.

    Collection mechanism:
      • Multi-schema — auto-collects all non-system schemas (public not required)
      • DB list      — pg_database (skips datistemplate query when oracleCompatible)
      • Table meta   — pg_class + pg_namespace JOIN (no ::regclass)
      • Column info  — pg_attribute + pg_constraint: name, type, nullable,
        default, PK/UNIQUE, comment
      • Indexes      — pg_indexes
      • Foreign keys — pg_constraint (with ON UPDATE / ON DELETE actions)
      • Row stats    — pg_stat_user_tables.n_live_tup (approximate)

    Known limitations (Oracle-compatible mode):
      • EXPLAIN does not support BUFFERS option (automatically omitted)
      • statement_timeout GUC may be unavailable (app-level timeout fallback)
      • ::regclass cast not supported (replaced by JOIN pattern)
      • Distribution keys and partition info not yet collected

    Example:
      dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
      dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?oracleCompatible=true&label=my-gauss-oracle'
`))
}

func printManualClickHouse(p func(string, string) string) {
	fmt.Print(p(`

─── ClickHouse ────────────────────────────────────────────────

    DSN 格式:
      clickhouse://用户:密码@主机:端口/库名?label=别名

    端口: 默认 8123 (HTTP 接口)
    别名: clickhouse, ch

    采集机制:
      • HTTP 接口 — 直接通过 HTTP POST 发送查询 (非标准 database/sql 驱动)
      • 库列表   — SHOW DATABASES (跳过 system, information_schema)
      • 表元数据 — system.tables: 名称、引擎(MergeTree/ReplacingMergeTree/...)、
        行数、体积、注释、排序键(sorting_key)、分区键(partition_key)、主键
      • 列信息   — system.columns: 名称、类型、默认值、注释、
        is_in_primary_key, is_in_sorting_key, is_in_partition_key
      • 列标志   — 自动识别: PK (主键), SORT (排序键), PART (分区键)

    特有引擎:
      MergeTree           — 标准合并树 (有排序键、分区键)
      ReplacingMergeTree  — 去重合并树 (按排序键去重)
      Distributed         — 分布式表

    已知局限:
      • 无传统外键约束
      • 注释推断依赖首行采样数据
      • 不支持 View 引擎表的行数统计

    示例:
      dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
`,
		`

─── ClickHouse ────────────────────────────────────────────────

    DSN format:
      clickhouse://user:password@host:port/dbname?label=alias

    Port: default 8123 (HTTP interface)
    Aliases: clickhouse, ch

    Collection mechanism:
      • HTTP interface — Queries sent via HTTP POST (non-standard database/sql driver)
      • DB list       — SHOW DATABASES (skips system, information_schema)
      • Table meta    — system.tables: name, engine (MergeTree/ReplacingMergeTree/...),
        row count, size, comment, sorting_key, partition_key, primary key
      • Column info   — system.columns: name, type, default, comment,
        is_in_primary_key, is_in_sorting_key, is_in_partition_key
      • Column flags  — Auto-detected: PK (primary key), SORT (sorting key),
        PART (partition key)

    Notable engines:
      MergeTree           — Standard merge tree (with sort/partition keys)
      ReplacingMergeTree  — Deduplicating merge tree (dedup by sort key)
      Distributed         — Distributed table

    Known limitations:
      • No traditional foreign key constraints
      • Comment inference relies on first row sampling
      • View engine tables excluded from row counts

    Example:
      dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
`))
}

func printManualSQLite(p func(string, string) string) {
	fmt.Print(p(`

─── SQLite ────────────────────────────────────────────────────

    DSN 格式:
      sqlite:///绝对路径?label=别名

    别名: sqlite, sqlite3

    说明:
      纯 Go 驱动 (无 CGO)，零外部依赖。
      路径为绝对路径，用户/密码留空。

    采集机制:
      • 表列表   — sqlite_master: 表名 (跳过 sqlite_% 内部表)
      • 列信息   — PRAGMA table_info(): 名称、类型、可空、默认值、主键标志
      • 行数     — SELECT COUNT(*) (直接查询)
      • 索引     — PRAGMA index_list() + PRAGMA index_info(): 名称、唯一性、列列表
      • 外键     — PRAGMA foreign_key_list(): 列、引用表/列、ON UPDATE/DELETE 动作
      • 注释推断 — 首行数据 + 规则引擎 (SQLite 无原生注释)

    安全机制:
      • 只读操作，无写/改/删
      • 跳过 sqlite_% 内部表
      • 密码在日志中脱敏

    示例:
      dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-sqlite'
`,
		`

─── SQLite ────────────────────────────────────────────────────

    DSN format:
      sqlite:///absolute/path?label=alias

    Aliases: sqlite, sqlite3

    Notes:
      Pure Go driver (no CGO), zero external dependencies.
      Path must be absolute; user/password left empty.

    Collection mechanism:
      • Table list  — sqlite_master: table names (skips sqlite_% internal tables)
      • Column info — PRAGMA table_info(): name, type, nullable, default, PK flag
      • Row count   — SELECT COUNT(*) (direct query)
      • Indexes     — PRAGMA index_list() + PRAGMA index_info(): name, unique, columns
      • Foreign keys— PRAGMA foreign_key_list(): columns, ref table/columns,
        ON UPDATE/DELETE actions
      • Comment inf.— First row sampling + rule engine (SQLite has no native comments)

    Safety:
      • Read-only operations, no write/update/delete
      • Skips sqlite_% internal tables
      • Passwords redacted in logs

    Example:
      dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-sqlite'
`))
}

func printManualRedis(p func(string, string) string) {
	fmt.Print(p(`

─── Redis ─────────────────────────────────────────────────────

    DSN 格式 (单机):
      redis://:密码@主机:端口/数据库编号?label=别名
    DSN 格式 (集群):
      redis://:密码@任意节点:端口/0?cluster=true&label=别名

    端口: 默认 6379
    别名: redis, rediss (rediss 自动启用 TLS)

    特有参数:
      cluster=true    启用 Redis 集群模式，自动扫描所有分片
      tls=true        启用 TLS 加密连接

    采集机制:
      • 键空间扫描 — SCAN 非阻塞迭代 (集群: ForEachMaster 遍历分片)
      • 采样限制   — 最多扫描 2000 个 key，每批 100 个
      • 模式推断   — 正则规范化: \d{2,}→{id}, hex→{hex}, UUID→{uuid}
        将相似 key 聚合为模式 (如 session:{hex}), 生成"表"
      • Pipeline   — 批量 TYPE/TTL/MEMORY USAGE 查询，减少网络往返
      • 类型采样   — string: GETRANGE 截取前 512 字节推断类型
                      hash: HSCAN 采样 5 个字段
                      stream: XRANGE 采样 10 条消息
                      set/zset/list: 统计成员数

    风险诊断:
      • 无 TTL 安全敏感键 (session/token/auth/otp/captcha/login/credential)
      • 超大容器 (hash>1000 字段, set/list/zset>10000 成员)
      • 大 key (string>1MB)
      • 未消费 stream (无消费者组, 消息>1000)
      • 超长 TTL (>30 天)

    安全机制:
      • SCAN 非阻塞 (不会阻塞 Redis)
      • 严格采样上限 (2000 key, 5 hash 字段, 512 字节, 10 stream 消息)
      • 全量只读, 绝不写/改/删
      • 集群模式仅访问 db0

    示例:
      # 单机
      dbexplain -dsn 'redis://:pwd@127.0.0.1:6379/0?label=my-redis'
      # 集群
      dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
`,
		`

─── Redis ─────────────────────────────────────────────────────

    DSN format (standalone):
      redis://:password@host:port/db_index?label=alias
    DSN format (cluster):
      redis://:password@any-node:port/0?cluster=true&label=alias

    Port: default 6379
    Aliases: redis, rediss (rediss auto-enables TLS)

    Specific parameters:
      cluster=true    Enable Redis cluster mode, auto-scan all shards
      tls=true        Enable TLS encrypted connection

    Collection mechanism:
      • Keyspace scan — SCAN non-blocking iteration (cluster: ForEachMaster per shard)
      • Sampling limit— Max 2000 keys, 100 per batch
      • Pattern infer — Regex normalization: \d{2,}→{id}, hex→{hex}, UUID→{uuid}
        Groups similar keys into patterns (e.g. session:{hex}), generating "tables"
      • Pipeline      — Batch TYPE/TTL/MEMORY USAGE queries, fewer round-trips
      • Type sampling — string: GETRANGE first 512 bytes to infer value type
                        hash: HSCAN sample 5 fields
                        stream: XRANGE sample 10 messages
                        set/zset/list: count members

    Risk diagnostics:
      • No-TTL security-sensitive keys (session/token/auth/otp/captcha/login/credential)
      • Oversized containers (hash>1000 fields, set/list/zset>10000 members)
      • Large keys (string>1MB)
      • Unconsumed streams (no consumer group, messages>1000)
      • Very long TTL (>30 days)

    Safety:
      • SCAN is non-blocking (won't block Redis)
      • Strict sampling caps (2000 keys, 5 hash fields, 512 bytes, 10 stream msgs)
      • Fully read-only, never write/update/delete
      • Cluster mode only accesses db0

    Example:
      # Standalone
      dbexplain -dsn 'redis://:pwd@127.0.0.1:6379/0?label=my-redis'
      # Cluster
      dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
`))
}

func printManualElasticsearch(p func(string, string) string) {
	fmt.Print(p(`

─── Elasticsearch ─────────────────────────────────────────────

    DSN 格式 (HTTP):
      elasticsearch://用户:密码@主机:端口?label=别名
    DSN 格式 (HTTPS):
      elasticsearchs://用户:密码@主机:端口?label=别名
      或 elasticsearch://...?tls=true&label=别名

    端口: 默认 9200
    别名: elasticsearch, es, elasticsearchs

    特有参数:
      tls=true              启用 HTTPS
      tls-skip-verify=true  跳过证书验证 (仅诊断环境)

    采集机制:
      • 索引列表 — Cat Indices API: 获取所有索引名
      • 索引映射 — Indices.GetMapping: 遍历 properties 提取字段名和类型
      • 字段     — 名称 (如 title, severity), 类型 (text/keyword/date/nested/...)
      • 列标志   — 自动标记: NN (非空, 所有 ES 字段视为必填)

    过滤:
      • 跳过以 . 开头的系统索引 (如 .kibana, .security 等)

    已知局限:
      • 不获取文档数 (row_count)
      • 不获取索引设置和别名
      • 不获取嵌套字段的展开结构

    示例:
      # HTTP
      dbexplain -dsn 'elasticsearch://elastic:pwd@127.0.0.1:9200?label=my-es'
      # HTTPS
      dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
`,
		`

─── Elasticsearch ─────────────────────────────────────────────

    DSN format (HTTP):
      elasticsearch://user:password@host:port?label=alias
    DSN format (HTTPS):
      elasticsearchs://user:password@host:port?label=alias
      or elasticsearch://...?tls=true&label=alias

    Port: default 9200
    Aliases: elasticsearch, es, elasticsearchs

    Specific parameters:
      tls=true              Enable HTTPS
      tls-skip-verify=true  Skip certificate verification (diagnostic only)

    Collection mechanism:
      • Index list  — Cat Indices API: get all index names
      • Index maps  — Indices.GetMapping: iterate properties to extract field name & type
      • Fields      — name (e.g. title, severity), type (text/keyword/date/nested/...)
      • Column flags— Auto-marked: NN (not-null, all ES fields treated as required)

    Filtering:
      • Skips system indices starting with . (e.g. .kibana, .security)

    Known limitations:
      • Document counts not fetched (row_count)
      • Index settings and aliases not fetched
      • Nested field structures not expanded

    Example:
      # HTTP
      dbexplain -dsn 'elasticsearch://elastic:pwd@127.0.0.1:9200?label=my-es'
      # HTTPS
      dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
`))
}

func printManualMongoDB(p func(string, string) string) {
	fmt.Print(p(`

─── MongoDB ───────────────────────────────────────────────────

    DSN 格式:
      mongodb://用户:密码@主机:端口/库名?authSource=认证库&label=别名

    端口: 默认 27017

    必填参数:
      authSource=<db>  认证数据库名 (用户创建所在的库, 如 admin)
      库名              路径中必须指定数据库名

    采集机制:
      • 集合列表 — db.ListCollectionNames(): 获取所有集合
      • 文档数   — EstimatedDocumentCount(): 近似文档数 (零数据风险)
      • 列信息   — 固定 _id 列 (objectId 类型, "mongodb document primary key")
      • 引擎标记 — 硬编码为 WiredTiger

    安全机制:
      • 强制要求库名 (防止意外全实例扫描)
      • 超时 10s, 禁读重试, 禁写重试 (快速失败)
      • 仅 ListCollections + EstimatedDocumentCount (零数据风险)

    已知局限:
      • 不获取文档 Schema/字段结构 (MongoDB 无固定模式)
      • 不获取索引信息
      • EstimatedDocumentCount 为近似值 (非精确 COUNT)

    示例:
      dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
`,
		`

─── MongoDB ───────────────────────────────────────────────────

    DSN format:
      mongodb://user:password@host:port/dbname?authSource=authDB&label=alias

    Port: default 27017

    Required parameters:
      authSource=<db>  Authentication database name (where user was created, e.g. admin)
      dbname           Database name must be specified in the path

    Collection mechanism:
      • Collections — db.ListCollectionNames(): get all collection names
      • Doc count   — EstimatedDocumentCount(): approximate count (zero data risk)
      • Column info — Fixed _id column (objectId, "mongodb document primary key")
      • Engine tag  — Hardcoded as WiredTiger

    Safety:
      • DB name required (prevents accidental full-instance scan)
      • Timeout 10s, read retries disabled, write retries disabled (fast fail)
      • Only ListCollections + EstimatedDocumentCount (zero data risk)

    Known limitations:
      • Document schema/field structure not collected (MongoDB is schemaless)
      • Index information not collected
      • EstimatedDocumentCount is approximate (not exact COUNT)

    Example:
      dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
`))
}

func printManualQdrant(p func(string, string) string) {
	fmt.Print(p(`

─── Qdrant ────────────────────────────────────────────────────

    DSN 格式:
      qdrant://:api密钥@主机:端口?label=别名

    端口: 默认 6334 (gRPC)

    说明:
      Qdrant 是向量数据库，用户名为空 (仅用 API Key 认证)。

    采集机制:
      • 集合列表 — client.ListCollections(): 获取所有集合
      • 点数量   — info.GetPointsCount(): 向量数量
      • 列信息   — 固定 vector 列 (float[] 类型, "embedding vector")
      • 引擎标记 — 硬编码为 qdrant

    已知局限:
      • 不获取向量维度
      • 不获取 payload schema
      • 当前仅支持非 TLS 连接 (UseTLS=false)

    示例:
      dbexplain -dsn 'qdrant://:my-api-key@127.0.0.1:6334?label=my-qdrant'
`,
		`

─── Qdrant ────────────────────────────────────────────────────

    DSN format:
      qdrant://:api-key@host:port?label=alias

    Port: default 6334 (gRPC)

    Notes:
      Qdrant is a vector database. Username is empty (API Key auth only).

    Collection mechanism:
      • Collections — client.ListCollections(): get all collections
      • Point count — info.GetPointsCount(): vector count
      • Column info — Fixed vector column (float[], "embedding vector")
      • Engine tag  — Hardcoded as qdrant

    Known limitations:
      • Vector dimensions not collected
      • Payload schema not collected
      • Currently only non-TLS connections supported (UseTLS=false)

    Example:
      dbexplain -dsn 'qdrant://:my-api-key@127.0.0.1:6334?label=my-qdrant'
`))
}

func printManualFile(p func(string, string) string) {
	fmt.Print(p(`

─── CSV/TSV 文件 ───────────────────────────────────────────

    DSN 格式:
      csv:///文件路径?label=别名[&encoding=gbk][&delimiter=,]
      tsv:///文件路径?label=别名[&delimiter=%09]
      csv:///目录路径/?label=别名
      csv:///通配/表达/式/*.csv?label=别名

    说明:
      CSV/TSV 文件以文件系统路径作为 DSN，无需数据库连接。
      文件全量读取到内存（不适合超大文件）。

    采集机制:
      • 单文件 — 首行作列名，采样推断列类型
      • 目录 — 扫描所有 .csv / .tsv 文件
      • Glob — 通配符表达式匹配文件
      • 编码 — 默认 UTF-8，可通过 ?encoding=gbk 指定 GBK

    查询限制:
      • 仅支持 SELECT * [LIMIT N [OFFSET M]]
      • 不支持 WHERE/JOIN/ORDER BY

    示例:
      dbexplain -dsn 'csv:///tmp/data.csv?label=csv-test'
      dbexplain execute -dsn 'csv:///tmp/data.csv?label=csv-test' 'SELECT * LIMIT 5'
      dbexplain -dsn 'tsv:///tmp/data.tsv?label=tsv&delimiter=%09'

`,
		`

─── CSV/TSV Files ──────────────────────────────────────────

    DSN format:
      csv:///file/path?label=alias[&encoding=gbk][&delimiter=,]
      tsv:///file/path?label=alias[&delimiter=%09]
      csv:///directory/path/?label=alias
      csv:///glob/pattern/*.csv?label=alias

    Description:
      CSV/TSV files use filesystem paths as DSNs — no database needed.
      Files are read into memory entirely (not suitable for huge files).

    Collection:
      • Single file — first row as column names, type inference by sampling
      • Directory — scans all .csv / .tsv files
      • Glob — wildcard-pattern file matching
      • Encoding — default UTF-8, ?encoding=gbk for GBK

    Query limitations:
      • Only SELECT * [LIMIT N [OFFSET M]] supported
      • WHERE/JOIN/ORDER BY not supported

    Examples:
      dbexplain -dsn 'csv:///tmp/data.csv?label=csv-test'
      dbexplain execute -dsn 'csv:///tmp/data.csv?label=csv-test' 'SELECT * LIMIT 5'
      dbexplain -dsn 'tsv:///tmp/data.tsv?label=tsv&delimiter=%09'

`))
}

func printManualXLSX(p func(string, string) string) {
	fmt.Print(p(`

─── Excel (.xlsx) 文件 ─────────────────────────────────────

    DSN 格式:
      xlsx:///文件路径?label=别名

    说明:
      Excel 文件支持已内建于主模块 (github.com/xuri/excelize/v2)。
      每个 Sheet 作为一张"表"。
      标准构建 (bash build.sh) 即包含 xlsx 功能。

    采集机制:
      • 遍历所有 Sheet
      • 首行作列名，采样推断列类型
      • 行数统计

    查询限制:
      • 默认查询第一个 Sheet
      • 仅支持 SELECT * [LIMIT N [OFFSET M]]

    示例:
      dbexplain -dsn 'xlsx:///tmp/report.xlsx?label=report'

`,
		`

─── Excel (.xlsx) Files ────────────────────────────────────

    DSN format:
      xlsx:///file/path?label=alias

    Description:
      Excel support is built into the main module (github.com/xuri/excelize/v2).
      Each sheet is a "table".
      Standard build (bash build.sh) includes xlsx support.

    Collection:
      • Iterate all sheets
      • First row as column names, type inference by sampling
      • Row count

    Query limitations:
      • Queries the first sheet by default
      • Only SELECT * [LIMIT N [OFFSET M]] supported

    Example:
      dbexplain -dsn 'xlsx:///tmp/report.xlsx?label=report'

`))
}

func printManualDuckDB(p func(string, string) string) {
	fmt.Print(p(`

─── DuckDB ────────────────────────────────────────────────────

    DSN 格式:
      duckdb:///绝对路径?label=别名      文件数据库
      duckdb:///:memory:?label=别名     内存模式

    别名: duckdb

    说明:
      嵌入式 SQL 引擎，可选 CGO 构建 (需 -tags duckdb)。
      标准版 (-std) 不含 DuckDB 驱动。

    特有参数:
      allowed_path=<路径>   Parquet/JSON/CSV 文件读取路径白名单

    采集机制:
      • 表列表   — information_schema.tables: 表名、类型 (跳过系统表)
      • 列信息   — pragma_table_info(): 名称、类型、可空、默认值、主键
      • 行数     — SELECT COUNT(*) (直接查询)
      • 约束     — duckdb_constraints(): 主键/唯一/外键
      • 注释推断 — 首行数据 + 规则引擎 (DuckDB 无原生注释)
      • 文件分析 — 通过 read_parquet/read_csv_auto/read_json 函数查询
                   外部 Parquet/CSV/JSON 文件，需 allowed_path 参数授权

    安全机制:
      • 自动 access_mode=READ_ONLY 只读模式
      • read_*() 文件函数需 allowed_path DSN 参数授权
      • 路径越界检查：filepath.Clean + strings.HasPrefix 防御性验证

    构建要求:
      • DuckDB 驱动需要 CGO (gcc/clang) 和 C 工具链
      • 编译命令: CGO_ENABLED=1 go build -tags duckdb ./cmd/dbexplain
      • build.sh 通过 duckdb tag 自动启用 CGO:
          bash build.sh minimal duckdb,mysql,postgres,csv,xlsx
      • release.sh 自动产出 -duckdb 后缀的 DuckDB 版二进制
      • 当前平台原生编译，不支持交叉编译

    示例:
      dbexplain -dsn 'duckdb:///:memory:?label=analysis'
      dbexplain execute -dsn 'duckdb:///:memory:?label=analysis' 'SELECT 1'
      dbexplain -dsn 'duckdb:///data/warehouse.db?label=warehouse'

    文件分析示例 (需 allowed_path):
      dbexplain execute -dsn 'duckdb:///:memory:?allowed_path=/data/' \\
        "SELECT region, SUM(amount) FROM read_parquet('/data/sales.parquet') GROUP BY region"
`,
		`

─── DuckDB ────────────────────────────────────────────────────

    DSN format:
      duckdb:///absolute/path?label=alias     File database
      duckdb:///:memory:?label=alias          In-memory mode

    Alias: duckdb

    Notes:
      Embedded SQL engine, optional CGO build (requires -tags duckdb).
      Standard edition (-std) does not include DuckDB driver.

    DSN parameters:
      allowed_path=<path>   File read whitelist for Parquet/JSON/CSV

    Collection mechanism:
      • Table list  — information_schema.tables: name, type (skips system tables)
      • Column info — pragma_table_info(): name, type, nullable, default, PK
      • Row count   — SELECT COUNT(*) (direct query)
      • Constraints — duckdb_constraints(): PK/Unique/FK
      • Comment inf.— First row sampling + rule engine (DuckDB has no native comments)
      • File analysis — read_parquet/read_csv_auto/read_json functions for
                    external Parquet/CSV/JSON files; requires allowed_path

    Safety:
      • Auto-enforces access_mode=READ_ONLY
      • read_*() file functions require allowed_path DSN parameter
      • Path traversal defense: filepath.Clean + strings.HasPrefix

    Build requirements:
      • DuckDB driver requires CGO (gcc/clang) and C toolchain
      • Build command: CGO_ENABLED=1 go build -tags duckdb ./cmd/dbexplain
      • build.sh auto-enables CGO when duckdb tag is present:
          bash build.sh minimal duckdb,mysql,postgres,csv,xlsx
      • release.sh auto-produces -duckdb suffixed binary
      • Native build only (current platform), no cross-compilation

    Examples:
      dbexplain -dsn 'duckdb:///:memory:?label=analysis'
      dbexplain execute -dsn 'duckdb:///:memory:?label=analysis' 'SELECT 1'
      dbexplain -dsn 'duckdb:///data/warehouse.db?label=warehouse'

    File analysis example (requires allowed_path):
      dbexplain execute -dsn 'duckdb:///:memory:?allowed_path=/data/' \\
        "SELECT region, SUM(amount) FROM read_parquet('/data/sales.parquet') GROUP BY region"
`))
}

func printManualOracle(p func(string, string) string) {
	fmt.Print(p(`

─── Oracle ─────────────────────────────────────────────────────

    DSN 格式:
      oracle://用户:密码@主机:端口/服务名?label=别名
      oracles://用户:密码@主机:端口/服务名?label=别名  (TLS)

    端口: 默认 1521
    别名: oracle, oracles (oracles 自动启用 TLS)

    采集机制:
      • 用户(Schema) — all_tables 按 owner 分组，跳过系统 Schema
      • 表元数据   — all_tables: 表名、num_rows 估算、注释
      • 列信息     — all_tab_columns + all_col_comments: 名称、类型、可空、
        默认值、注释；对无注释字段取首行数据通过规则引擎推断语义
      • 约束       — all_constraints + all_cons_columns: 主键(P)、唯一(U)
      • 索引       — all_ind_columns + all_indexes: 名称、唯一性、列列表
      • 外键       — all_constraints (R) + all_cons_columns:
        4 表 JOIN 位置对齐，支持复合外键，解析引用表/列/删除规则
      • 采样       — FETCH FIRST 1 ROWS ONLY (12c+)，规则引擎推断注释

    能力标签: CapSQL, CapForeignKey, CapRowCount, CapSampling, CapIndex

    安全机制:
      • 跳过 20+ Oracle 系统 Schema (SYS/SYSTEM/XDB 等)
      • 参数化查询 (:1, :2 占位符)，标识符严格转义
      • 密码在日志和输出中脱敏

    已知局限:
      • num_rows 来自 all_tables 统计信息，需 ANALYZE TABLE 采集后方准确
      • 不采集 dba_segments 数据 (需要 DBA 权限)
      • 不采集存储过程/函数/触发器等 PL/SQL 对象
      • 不支持 TNS (DESCRIPTION=...) 连接串格式
      • EXPLAIN 使用两步法 (EXPLAIN PLAN FOR + DBMS_XPLAN.DISPLAY())，
        需要 PLAN_TABLE 存在 (Oracle 自动创建)
      • FETCH FIRST N ROWS ONLY 需要 Oracle 12c+
      • LIMIT → FETCH FIRST 自动适配 (正则替换)

    示例:
      dbexplain -dsn 'oracle://user:pwd@host:1521/XE?label=my-oracle'
      dbexplain -dsn 'oracles://user:pwd@host:1521/XEPDB1?label=my-tls-oracle'
`,
		`

─── Oracle ─────────────────────────────────────────────────────

    DSN format:
      oracle://user:password@host:port/service?label=alias
      oracles://user:password@host:port/service?label=alias  (TLS)

    Port: default 1521
    Aliases: oracle, oracles (oracles auto-enables TLS)

    Collection mechanism:
      • Schema (Owner) — grouped by owner from all_tables, skips system schemas
      • Table metadata  — all_tables: name, num_rows estimate, comment
      • Column info     — all_tab_columns + all_col_comments: name, type, nullable,
        default, comment; uncommented columns get semantic inference from sample row
      • Constraints     — all_constraints + all_cons_columns: primary key (P), unique (U)
      • Indexes         — all_ind_columns + all_indexes: name, uniqueness, column list
      • Foreign keys    — all_constraints (R) + all_cons_columns:
        4-table JOIN with position alignment, supports composite FKs,
        resolves ref table/columns/delete rule
      • Sampling        — FETCH FIRST 1 ROWS ONLY (12c+), rule engine comment inference

    Capabilities: CapSQL, CapForeignKey, CapRowCount, CapSampling, CapIndex

    Safety:
      • Skips 20+ Oracle system schemas (SYS/SYSTEM/XDB etc.)
      • Parameterized queries (:1, :2 placeholders), strict identifier escaping
      • Passwords redacted in logs and output

    Known limitations:
      • num_rows sourced from all_tables stats — accurate only after ANALYZE TABLE
      • dba_segments not collected (requires DBA privilege)
      • PL/SQL objects (procedures/functions/triggers) not collected
      • TNS (DESCRIPTION=...) connection format not supported
      • EXPLAIN uses two-step method (EXPLAIN PLAN FOR + DBMS_XPLAN.DISPLAY()),
        requires PLAN_TABLE (Oracle creates it automatically)
      • FETCH FIRST N ROWS ONLY requires Oracle 12c+
      • LIMIT → FETCH FIRST auto-adaptation (regex replacement)

    Examples:
      dbexplain -dsn 'oracle://user:pwd@host:1521/XE?label=my-oracle'
      dbexplain -dsn 'oracles://user:pwd@host:1521/XEPDB1?label=my-tls-oracle'
`))
}

func printManualHive(p func(string, string) string) {
	fmt.Print(p(`

─── Hive ───────────────────────────────────────────────────────

    DSN 格式:
      hive://用户:密码@主机:端口/库名?label=别名[&auth=NOSASL]
      hives://用户:密码@主机:端口/库名?label=别名  (TLS)

    端口: 默认 10000 (HiveServer2)
    别名: hive, hives (hives 自动启用 TLS)

    认证方式 (通过 ?auth= 参数指定):
      NOSASL   — 无认证 (默认，无需用户密码)
      NONE     — SASL PLAIN 用户名密码认证 (有用户时默认)
      LDAP     — LDAP 认证
      KERBEROS — Kerberos 认证 (纯 Go，无需 CGO/libkrb5)

    采集机制:
      • 库列表   — SHOW DATABASES (跳过 information_schema, sys, default)
      • 表列表   — SHOW TABLES IN dbname
      • 列信息   — DESCRIBE FORMATTED db.table: 名称、类型、注释
        自动跳过 # 分隔行，停止于 # Detailed Table Information
      • 行数     — 固定返回 -1 (unknown)，避免触发 MapReduce/Tez 作业
      • 采样     — SELECT * FROM db.table LIMIT 1，规则引擎推断注释

    能力标签: CapSQL, CapRowCount, CapSampling

    特有参数:
      transport=<mode>  传输模式: binary (默认) 或 http
      http_path=<path>  HTTP 传输模式的路径
      service=<name>    Kerberos 服务名 (默认 hive)
      sslcert=<file>    SSL 客户端证书路径
      sslkey=<file>     SSL 客户端密钥路径
      sslca=<file>      SSL CA 证书路径

    安全机制:
      • DSN 参数完全控制 (无外部配置文件)
      • 密码在日志和输出中脱敏
      • 纯 Go Kerberos (beltran/gosasl)，无 CGO 依赖

    已知局限:
      • 使用 HiveServer2 SQL (端口 10000)，非 Metastore Thrift API (端口 9083)
      • 行数固定返回 -1 (SELECT COUNT(*) 会触发 MR/Tez 作业，不予执行)
      • 不支持索引和主键/外键约束采集 (Hive 无传统约束)
      • DESCRIBE FORMATTED 依赖于 HiveServer2 实现
      • TLS 连接需提供 sslcert + sslkey 文件路径

    示例:
      # NOSASL 无认证
      dbexplain -dsn 'hive://host:10000/default?label=my-hive'
      # LDAP 认证
      dbexplain -dsn 'hive://user:pwd@host:10000/default?auth=LDAP&label=my-hive'
      # Kerberos
      dbexplain -dsn 'hive://host:10000/default?auth=KERBEROS&label=my-hive'
`,
		`

─── Hive ───────────────────────────────────────────────────────

    DSN format:
      hive://user:password@host:port/dbname?label=alias[&auth=NOSASL]
      hives://user:password@host:port/dbname?label=alias  (TLS)

    Port: default 10000 (HiveServer2)
    Aliases: hive, hives (hives auto-enables TLS)

    Authentication (?auth= parameter):
      NOSASL   — No authentication (default, no user/password needed)
      NONE     — SASL PLAIN username/password auth (default when user provided)
      LDAP     — LDAP authentication
      KERBEROS — Kerberos authentication (pure Go, no CGO/libkrb5 needed)

    Collection mechanism:
      • DB list    — SHOW DATABASES (skips information_schema, sys, default)
      • Table list — SHOW TABLES IN dbname
      • Column info— DESCRIBE FORMATTED db.table: name, type, comment
        auto-skips # separator lines, stops at # Detailed Table Information
      • Row count  — Fixed -1 (unknown), avoids triggering MapReduce/Tez jobs
      • Sampling   — SELECT * FROM db.table LIMIT 1, rule engine comment inference

    Capabilities: CapSQL, CapRowCount, CapSampling

    DSN parameters:
      transport=<mode>  Transport mode: binary (default) or http
      http_path=<path>  HTTP transport path
      service=<name>    Kerberos service name (default hive)
      sslcert=<file>    SSL client certificate file path
      sslkey=<file>     SSL client key file path
      sslca=<file>      SSL CA certificate file path

    Safety:
      • DSN parameters for full configuration (no external config files)
      • Passwords redacted in logs and output
      • Pure Go Kerberos (beltran/gosasl), no CGO dependency

    Known limitations:
      • Uses HiveServer2 SQL (port 10000), not Metastore Thrift API (port 9083)
      • Row count fixed at -1 (SELECT COUNT(*) triggers MR/Tez, not executed)
      • Indexes and PK/FK constraints not collected (Hive has no traditional constraints)
      • DESCRIBE FORMATTED depends on HiveServer2 implementation
      • TLS requires sslcert + sslkey file paths

    Examples:
      # NOSASL no auth
      dbexplain -dsn 'hive://host:10000/default?label=my-hive'
      # LDAP auth
      dbexplain -dsn 'hive://user:pwd@host:10000/default?auth=LDAP&label=my-hive'
      # Kerberos
      dbexplain -dsn 'hive://host:10000/default?auth=KERBEROS&label=my-hive'
`))
}

func printManualPrometheus(p func(string, string) string) {
	fmt.Print(p(`

─── Prometheus ─────────────────────────────────────────────────

    DSN 格式:
      prometheus://主机:端口?label=别名

    端口: 默认 9090

    能力:
      • Schema 采集 — 抓取 targets（按 job 分组为表）、标签列表、指标元数据
      • 只读查询 — 通过 DSL 模式执行 PromQL（SQL 语法编译为 PromQL）
      • 三层安全检查 — DSL 层指标名校验 + CheckNative 策略引擎 + 列掩码
      • 能力标签: CapPromQL（不进 sqlguard，走独立安全路径）

    DSL 查询（仅限 SELECT * + WHERE 标签过滤，Phase 1）:
      # 查询即时向量
      SELECT * FROM @prom.up
      SELECT * FROM @prom.up WHERE job="prometheus"
      SELECT * FROM @prom.up WHERE job!="prometheus"
      SELECT * FROM @prom.node_cpu_seconds_total WHERE job="node"

    Phase 1 限制（当前版本不支持）:
      • COUNT / SUM / AVG 等聚合函数（PromQL 语义不兼容）
      • GROUP BY / ORDER BY / LIMIT
      • JOIN（Prometheus 单指标模型）
      • 数值条件（WHERE value > 0）
      • 范围向量（[5m]）

    安全机制:
      • Layer 1 — DSL 层: 编译前检查 metric name vs DENY_TABLES
      • Layer 2 — CheckNative: 执行前再次校验 metric + label vs 策略
      • Layer 3 — ApplyMask: 执行后列掩码
      • 不走 SQL 安全网关（sqlguard）— PromQL 语义独立

    示例:
      dbexplain execute -dsn 'prometheus://192.168.0.1:9090?label=prom' \\
        --dsl --human 'SELECT * FROM @prom.up'
      dbexplain execute --label prom --dsl --human \\
        'SELECT * FROM @prom.node_cpu_seconds_total WHERE job="node"'
`, `

─── PROMETHEUS ─────────────────────────────────────────────────

    DSN format:
      prometheus://host:port?label=name

    Port: default 9090

    Capabilities:
      • Schema collection — scrape targets (grouped by job as tables), label list, metric metadata
      • Read-only query — PromQL via DSL mode (SQL syntax compiled to PromQL)
      • Three-layer security — DSL-level metric check + CheckNative policy engine + column masking
      • Capability: CapPromQL (bypasses sqlguard, follows independent security path)

    DSL queries (Phase 1: SELECT * + WHERE label filter only):
      # Instant vector queries
      SELECT * FROM @prom.up
      SELECT * FROM @prom.up WHERE job="prometheus"
      SELECT * FROM @prom.up WHERE job!="prometheus"
      SELECT * FROM @prom.node_cpu_seconds_total WHERE job="node"

    Phase 1 limitations (not supported in current version):
      • Aggregation functions COUNT/SUM/AVG (incompatible PromQL semantics)
      • GROUP BY / ORDER BY / LIMIT
      • JOIN (Prometheus single-metric model)
      • Numeric conditions (WHERE value > 0)
      • Range vectors ([5m])

    Security:
      • Layer 1 — DSL level: metric name check against DENY_TABLES before compilation
      • Layer 2 — CheckNative: re-validates metric + labels against policies before execution
      • Layer 3 — ApplyMask: post-execution column masking
      • Bypasses sqlguard — PromQL has independent semantics

    Examples:
      dbexplain execute -dsn 'prometheus://192.168.0.1:9090?label=prom' \\
        --dsl --human 'SELECT * FROM @prom.up'
      dbexplain execute --label prom --dsl --human \\
        'SELECT * FROM @prom.node_cpu_seconds_total WHERE job="node"'
`))
}
