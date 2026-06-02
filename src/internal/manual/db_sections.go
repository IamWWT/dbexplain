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
      • 列信息   — information_schema.columns + pg_constraint:
        名称、类型、可空、默认值、主键/唯一约束标记、注释(col_description)
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
      • Column info  — information_schema.columns + pg_constraint:
        name, type, nullable, default, PK/UNIQUE flag, comment (col_description)
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

─── GaussDB ───────────────────────────────────────────────────

    DSN 格式:
      gaussdb://用户:密码@主机:端口/库名?label=别名

    端口: 默认 25308
    别名: gaussdb, opengauss

    说明:
      GaussDB 兼容 PostgreSQL 协议，使用同一个 Connector。
      采集机制、安全策略与 PostgreSQL 完全相同 (pg_catalog)。
      支持行数统计、多 Schema 自动采集。

    示例:
      dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
`,
		`

─── GaussDB ───────────────────────────────────────────────────

    DSN format:
      gaussdb://user:password@host:port/dbname?label=alias

    Port: default 25308
    Aliases: gaussdb, opengauss

    Notes:
      GaussDB is PostgreSQL-protocol compatible and uses the same Connector.
      Collection mechanism and safety policies are identical to PostgreSQL
      (pg_catalog). Supports row stats and multi-schema auto-collection.

    Example:
      dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
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
