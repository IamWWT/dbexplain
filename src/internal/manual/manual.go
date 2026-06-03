// Package manual provides the comprehensive help manual and per-database references.
package manual

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/IamWWT/dbexplain/internal/version"
)

// duckdbHelp shows duckdb in Database types list, adjusted by build tags.
var duckdbHelp = "  duckdb\n"

// langText holds bilingual text content.
type langText struct {
	ZH string
	EN string
}

func (lt langText) Get(lang string) string {
	if lang == "en" {
		return lt.EN
	}
	return lt.ZH
}

func preScanLanguage() string {
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--language" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return "zh"
}

// PrintHelp prints the concise flag summary.
func PrintHelp() {
	lang := preScanLanguage()
	p := func(zh, en string) string {
		if lang == "en" {
			return en
		}
		return zh
	}
	out := os.Stderr

	fmt.Fprint(out, p(
		"\ndbexplain — Database Context Compiler  "+version.Version+"\n\n"+
			"Commands:\n"+
			"  Schema:\n"+
			"    dbexplain [flags]              Collect & analyze database schemas\n"+
			"    dbexplain collect [flags]      Explicit schema collection subcommand\n"+
			"    dbexplain diff [flags]         Schema diff / delta detection\n"+
			"  Query:\n"+
			"    dbexplain execute <query>      Run read-only query (SQL / JSON / native)\n"+
			"    dbexplain repl                 Interactive REPL mode\n"+
			"  Utility:\n"+
			"    dbexplain list                 List all configured databases\n"+
			"    dbexplain encrypt <file>       Encrypt .env config with machine fingerprint\n"+
			"  Help:\n"+
			"    dbexplain <dbtype>             Database-specific reference (e.g. mysql, redis)\n"+
			"    dbexplain all                  Full reference manual\n\n",
		"\ndbexplain — Database Context Compiler  "+version.Version+"\n\n"+
			"Commands:\n"+
			"  Schema:\n"+
			"    dbexplain [flags]              Collect & analyze database schemas\n"+
			"    dbexplain collect [flags]      Explicit schema collection subcommand\n"+
			"    dbexplain diff [flags]         Schema diff / delta detection\n"+
			"  Query:\n"+
			"    dbexplain execute <query>      Run read-only query (SQL / JSON / native)\n"+
			"    dbexplain repl                 Interactive REPL mode\n"+
			"  Utility:\n"+
			"    dbexplain list                 List all configured databases\n"+
			"    dbexplain encrypt <file>       Encrypt .env config with machine fingerprint\n"+
			"  Help:\n"+
			"    dbexplain <dbtype>             Database-specific reference (e.g. mysql, redis)\n"+
			"    dbexplain all                  Full reference manual\n\n",
	))

	fmt.Fprint(out, p(
		"Supported databases:\n"+
			"  SQL:   mysql, postgres/pg, gaussdb, clickhouse/ch, sqlite/sqlite3,\n"+
			"       "+duckdbHelp+
			"  NoSQL: redis, mongodb, elasticsearch/es, qdrant\n"+
			"  File:  csv, tsv, xlsx\n\n",
		"Supported databases:\n"+
			"  SQL:   mysql, postgres/pg, gaussdb, clickhouse/ch, sqlite/sqlite3,\n"+
			"       "+duckdbHelp+
			"  NoSQL: redis, mongodb, elasticsearch/es, qdrant\n"+
			"  File:  csv, tsv, xlsx\n\n",
	))

	fmt.Fprint(out, p(
		"Flags (dbexplain [flags]):\n"+
			"  -dsn, -env, -config                 Input sources\n"+
			"  -include, -exclude, -label          Filter by type/label/key\n"+
			"  -json, --human, -o <file>     Output format\n"+
			"  --context <dir>, --cache <f>  AI context / delta scan\n"+
			"  --log-dir <dir>, -timeout d   Logs / timeout (default /var/log/dbexplain, 20s)\n"+
			"  --conn N                     Max concurrent connections (default 10)\n"+
			"  --language zh|en, --version   Language / version\n\n",
		"Flags (dbexplain [flags]):\n"+
			"  -dsn, -env, -config                 Input sources\n"+
			"  -include, -exclude, -label          Filter by type/label/key\n"+
			"  -json, --human, -o <file>     Output format\n"+
			"  --context <dir>, --cache <f>  AI context / delta scan\n"+
			"  --log-dir <dir>, -timeout d   Logs / timeout (default /var/log/dbexplain, 20s)\n"+
			"  --conn N                     Max concurrent connections (default 10)\n"+
			"  --language zh|en, --version   Language / version\n\n",
	))

	fmt.Fprint(out, p(
		"Examples:\n"+
			"  dbexplain -env                    Scan all databases from config\n"+
			"  dbexplain list                    List configured databases (DB index → label)\n"+
			"  dbexplain execute -env --db 1     Run SQL query on first database\n"+
			"    'SELECT COUNT(*) FROM users'\n"+
			"  dbexplain encrypt config.env      Encrypt config file\n"+
			"  dbexplain mysql                   MySQL reference manual\n"+
			"  dbexplain all                     Full reference manual\n"+
			"  dbexplain all --filter redis      Search manual by keyword\n\n",
		"Examples:\n"+
			"  dbexplain -env                    Scan all databases from config\n"+
			"  dbexplain list                    List configured databases (DB index → label)\n"+
			"  dbexplain execute -env --db 1     Run SQL query on first database\n"+
			"    'SELECT COUNT(*) FROM users'\n"+
			"  dbexplain encrypt config.env      Encrypt config file\n"+
			"  dbexplain mysql                   MySQL reference manual\n"+
			"  dbexplain all                     Full reference manual\n"+
			"  dbexplain all --filter redis      Search manual by keyword\n\n",
	))

	fmt.Fprint(out, p(
		"See:\n"+
			"  dbexplain list -h       list subcommand help\n"+
			"  dbexplain execute -h    execute subcommand help\n"+
			"  dbexplain encrypt -h    encrypt subcommand help\n"+
			"  dbexplain all -h        full manual help\n"+
			"  GitHub: https://github.com/IamWWT/dbexplain\n"+
			"    (build with -tags duckdb for DuckDB support, requires CGO)\n",
		"See:\n"+
			"  dbexplain list -h       list subcommand help\n"+
			"  dbexplain execute -h    execute subcommand help\n"+
			"  dbexplain encrypt -h    encrypt subcommand help\n"+
			"  dbexplain all -h        full manual help\n"+
			"  GitHub: https://github.com/IamWWT/dbexplain\n"+
			"    (build with -tags duckdb for DuckDB support, requires CGO)\n",
	))
}

// PrintManual prints the full manual with optional keyword filter.
func PrintManual(lang, filter string) {
	if filter == "" {
		printManualContent(lang)
		return
	}
	captured := captureManualOutput(lang)
	kw := strings.ToLower(filter)

	sections := splitSections(captured)
	var matched []string
	for _, sec := range sections {
		if strings.Contains(strings.ToLower(sec), kw) {
			matched = append(matched, sec)
		}
	}
	if len(matched) == 0 {
		fmt.Printf("No matches for filter: %q\n", filter)
		return
	}
	fmt.Printf("=== Filtered by: %q (%d section(s)) ===\n\n", filter, len(matched))
	for _, sec := range matched {
		fmt.Print(sec)
		if !strings.HasSuffix(sec, "\n") {
			fmt.Println()
		}
	}
}

func splitSections(text string) []string {
	idx := strings.Index(text, "\n───")
	if idx < 0 {
		return []string{text}
	}
	preamble := text[:idx]
	body := text[idx+1:]

	var sections []string
	if strings.TrimSpace(preamble) != "" {
		sections = append(sections, preamble)
	}

	parts := strings.Split(body, "\n───")
	for _, p := range parts {
		if p == "" {
			continue
		}
		sections = append(sections, "───"+p)
	}
	return sections
}

func captureManualOutput(lang string) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] captureManualOutput: %v", r)
			}
		}()
		io.Copy(&buf, r)
		close(done)
	}()

	printManualContent(lang)
	w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}

// HandleAllManual handles the "all" subcommand: prints full manual, supports --filter.
func HandleAllManual(args []string) {
	lang := preScanLanguage()
	filter := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--filter":
			if i+1 < len(args) {
				filter = args[i+1]
				i++
			}
		case "--language":
			if i+1 < len(args) {
				lang = args[i+1]
				i++
			}
		}
	}
	PrintManual(lang, filter)
}

// DBSubcommands maps subcommand names to their manual print functions.
var DBSubcommands = map[string]func(func(string, string) string){
	"mysql":         printManualMySQL,
	"postgres":      printManualPostgres,
	"postgresql":    printManualPostgres,
	"pg":            printManualPostgres,
	"gaussdb":       printManualGaussDB,
	"clickhouse":    printManualClickHouse,
	"ch":            printManualClickHouse,
	"sqlite":        printManualSQLite,
	"sqlite3":       printManualSQLite,
	"redis":         printManualRedis,
	"mongodb":       printManualMongoDB,
	"elasticsearch": printManualElasticsearch,
	"es":            printManualElasticsearch,
	"qdrant":        printManualQdrant,
	"csv":           printManualFile,
	"tsv":           printManualFile,
	"xlsx":          printManualXLSX,
	"duckdb":        printManualDuckDB,
}

// PrintDBManual prints the database-specific manual section for the given subcommand.
func PrintDBManual(sub string, _ []string) {
	lang := preScanLanguage()
	p := func(zh, en string) string {
		if lang == "en" {
			return en
		}
		return zh
	}

	fn := DBSubcommands[sub]
	if fn == nil {
		fmt.Fprintf(os.Stderr, "Unknown database type: %s\n", sub)
		os.Exit(1)
	}

	displayName := sub
	if displayName == "pg" {
		displayName = "postgres"
	} else if displayName == "ch" {
		displayName = "clickhouse"
	} else if displayName == "es" {
		displayName = "elasticsearch"
	} else if displayName == "sqlite3" {
		displayName = "sqlite"
	} else if displayName == "postgresql" {
		displayName = "postgres"
	}

	fmt.Fprint(os.Stdout, p(
		"\ndbexplain "+displayName+" — Database Context Compiler  "+version.Version+"\n\n",
		"\ndbexplain "+displayName+" — Database Context Compiler  "+version.Version+"\n\n",
	))
	fn(p)
	fmt.Fprint(os.Stdout, p(
		"\nSee: dbexplain all  for full manual | dbexplain <type> for another database\n",
		"\nSee: dbexplain all  for full manual | dbexplain <type> for another database\n",
	))
}

func printManualContent(lang string) {
	p := func(zh, en string) string {
		if lang == "en" {
			return en
		}
		return zh
	}

	fmt.Print(p(`
NAME
    dbexplain — 零依赖多数据库结构探查与关系分析工具

SYNOPSIS
    dbexplain -dsn '<scheme>://[user:pass@]host[:port][/db][?params]'
    dbexplain -env
    dbexplain -config dbs.json
    dbexplain -dsn '...' -json -o report.json

DESCRIPTION
    dbexplain 是一个静态编译、无外部运行时依赖的命令行工具。
    只需提供数据库连接串，即可自动导出表结构、列信息、索引、外键，
    分析跨库/跨实例的表关系（显式外键 + 命名推断），生成聚类关系
    图与问题诊断。所有操作为只读，安全无副作用。
`,
		`
NAME
    dbexplain — zero-dependency multi-database schema explorer and relationship analyzer

SYNOPSIS
    dbexplain -dsn '<scheme>://[user:pass@]host[:port][/db][?params]'
    dbexplain -env
    dbexplain -config dbs.json
    dbexplain -dsn '...' -json -o report.json

DESCRIPTION
    dbexplain is a statically compiled, zero-runtime-dependency CLI tool.
    Given database connection strings, it auto-exports table structures,
    column info, indexes, foreign keys, analyzes cross-db/cross-instance
    table relationships (explicit FK + naming inference), generates
    cluster graphs and issue diagnostics. All read-only, safe by default.
`))

	fmt.Print(p(`

─── DSN 格式 ──────────────────────────────────────────────────

    通用格式:
      scheme://[用户:密码@]主机[:端口][/库名][?param1=val1&param2=val2]

    DSN 参数速查:
      label=<别名>        实例别名，决定日志文件名 <label>.log
      cluster=true        Redis 集群模式 (自动扫描所有分片)
      tls=true            ES / Redis 启用 TLS 加密
      sslmode=<mode>      PostgreSQL SSL: disable/require/verify-ca/verify-full
      authSource=<db>     MongoDB 认证数据库名

    配置文件搜索优先级 (-env 模式):
      1. DBPROBE_ENV_FILE 环境变量指定路径（可选覆盖）
      2. 当前目录 .env.dbexplain（明文）
      3. 当前目录 .env.dbexplain.enc（加密，自动解密）
      4. ~/.config/dbexplain/.env.dbexplain（明文）
      5. ~/.config/dbexplain/.env.dbexplain.enc（加密，自动解密）
      6. 当前目录 .env（向下兼容旧版）
      搜索规则与二进制路径无关 —— findConfigFile() 编译在二进制内，
      只依赖当前工作目录(CWD)和用户家目录，不关心二进制放在哪里。
      加密后务必删除明文配置文件，否则优先匹配明文。
      密码模式密码从 ~/.config/dbexplain/.encryption_key 文件自动读取。
`,
		`

─── DSN FORMAT ────────────────────────────────────────────────

    General format:
      scheme://[user:password@]host[:port][/dbname][?param1=val1&param2=val2]

    DSN parameters:
      label=<name>        Instance alias, determines log file name <label>.log
      cluster=true        Redis cluster mode (auto-scan all shards)
      tls=true            ES / Redis enable TLS encryption
      sslmode=<mode>      PostgreSQL SSL: disable/require/verify-ca/verify-full
      authSource=<db>     MongoDB authentication database name

    Config file search order (-env mode):
      1. DBPROBE_ENV_FILE environment variable (optional override)
      2. .env.dbexplain in current directory (plaintext)
      3. .env.dbexplain.enc in current directory (encrypted, auto-decrypt)
      4. ~/.config/dbexplain/.env.dbexplain (plaintext)
      5. ~/.config/dbexplain/.env.dbexplain.enc (encrypted, auto-decrypt)
      6. .env in current directory (legacy backward compat)
      Search order is independent of binary location — findConfigFile() is compiled
      into the binary, depends only on CWD and user home directory, not on binary path.
      Delete plaintext config after encryption, or it will take priority.
      Password-mode key is auto-read from ~/.config/dbexplain/.encryption_key.
`))

	fmt.Print(p(`

─── 全局参数 ──────────────────────────────────────────────────

    -dsn <string>         数据库连接串，可重复多次指定多个库
    -config <file>        从 JSON 文件读取 DSN 列表 (数组格式)
    -env                  从配置文件加载 DSN (格式: DB<n>=<DSN>, 搜索优先级见 DSN 格式章节)
    -include <filter>     仅包含匹配的 DSN (按类型/label/env编号, 逗号分隔)
    -exclude <filter>     排除匹配的 DSN (格式同 -include)
    -label <name>         按 label 过滤 (等效于 -include)
    -json                 输出 JSON 格式 (适合程序消费)
    -human                人类友好输出：带上下文标记 [table=] [pattern=] 和视觉分隔
    -o <file>             将报告写入文件 (自动添加 UTF-8 BOM)
    --log-dir <dir>       日志输出目录 (默认 /var/log/dbexplain, 包含 filter.log 和各实例日志)
    -context <dir>        写入 AI 上下文文件到目录 (summary.json/topology.json/diagnostics.json/chunks/)
    -cache <file>         Schema 指纹缓存文件，用于增量变更检测 (.json)
    -timeout <duration>   每 DSN 采集超时 (默认 20s, 如 30s/1m)
    --version             输出版本号并退出
    --manual              打印此完整手册并退出
    --language <zh|en>    手册语言 (默认 zh)
    -h, --help            打印简版参数列表并退出
`,
		`

─── GLOBAL OPTIONS ────────────────────────────────────────────

    -dsn <string>         Database connection string, repeatable for multiple databases
    -config <file>        Read DSN list from JSON file (array format)
    -env                  Load DSNs from config file (format: DB<n>=<DSN>, search order see DSN FORMAT)
    -include <filter>     Only include matching DSNs (by kind/label/env-key, comma-sep)
    -exclude <filter>     Exclude matching DSNs (same format as -include)
    -label <name>         Filter by label (alias for -include)
    -json                 Output JSON format (for programmatic consumption)
    -human                Human-friendly output: context markers [table=] [pattern=] etc.
    -o <file>             Write report to file (auto-prepends UTF-8 BOM)
    --log-dir <dir>       Log output directory (default /var/log/dbexplain, filter.log + per-instance logs)
    -context <dir>        Write AI context files to directory (summary.json/topology.json/diagnostics.json/chunks/)
    -cache <file>         Schema fingerprint cache file for incremental delta detection (.json)
    -timeout <duration>   Per-DSN collect timeout (default 20s, e.g. 30s/1m)
    --version             Print version and exit
    --manual              Print this comprehensive manual and exit
    --language <zh|en>    Manual language (default zh)
    -h, --help            Print brief flag list and exit
`))

	fmt.Print(p(`

─── 支持数据库总览 ────────────────────────────────────────────

    类型             默认端口   采集方式               元数据亮点
    ───────────────  ────────  ────────────────────  ──────────────────
    mysql            3306      information_schema     FK、索引、注释推断
    postgres         5432      pg_catalog             多Schema、行数统计
    gaussdb          25308     pg_catalog (兼容PG)    行数、表大小
    clickhouse       8123      HTTP/system.tables     排序键/分区键/主键
    sqlite           -         PRAGMA                 纯Go驱动(无CGO)
    redis            6379      SCAN/Pipeline           键模式推断/风险诊断
    elasticsearch    9200      Cat Indices            索引映射
    mongodb          27017     ListCollections        近似文档数
    qdrant           6334      gRPC                   集合向量维度
    csv              -         文件首行/采样            列名+类型推断
    tsv              -         文件首行/采样            列名+类型推断
    xlsx             -         excelize 库             多Sheet、列名+类型推断
    duckdb           -         duckdb_* 系统函数        嵌入式引擎，Parquet/JSON/CSV 文件分析
`,
		`

─── SUPPORTED DATABASES ──────────────────────────────────────

    Kind             Port     Collection Method      Metadata Highlights
    ───────────────  ───────  ────────────────────   ──────────────────
    mysql            3306     information_schema     FK, indexes, comment inference
    postgres         5432     pg_catalog             Multi-schema, row estimates
    gaussdb          25308    pg_catalog (PG compat) Row counts, table sizes
    clickhouse       8123     HTTP/system.tables     Sort/partition/primary keys
    sqlite           -        PRAGMA                 Pure Go driver (no CGO)
    redis            6379     SCAN/Pipeline          Key pattern inference, risk diag
    elasticsearch    9200     Cat Indices            Index mappings
    mongodb          27017    ListCollections        Estimated doc counts
    qdrant           6334     gRPC                   Collection vector dimensions
    csv              -        File header/sampling    Column names + type inference
    tsv              -        File header/sampling    Column names + type inference
    xlsx             -        excelize library        Multi-sheet, column names + type inference
    duckdb           -        duckdb_* system funcs   Embedded engine, Parquet/JSON/CSV file analysis
`))

	// Per-database sections
	printManualMySQL(p)
	printManualPostgres(p)
	printManualGaussDB(p)
	printManualClickHouse(p)
	printManualSQLite(p)
	printManualRedis(p)
	printManualElasticsearch(p)
	printManualMongoDB(p)
	printManualQdrant(p)
	printManualDuckDB(p)

	fmt.Print(p(`

─── JSON 输出格式 ────────────────────────────────────────────

    顶层结构:
      {
        "instances": [ ... ],   实例数组
        "refs":      [ ... ],   表关系数组 (外键 + 推断)
        "groups":    [ ... ],   表聚类数组 (可选)
        "issues":    [ ... ]    问题诊断数组
      }

    instances[].databases[].tables[] 包含:
      name, comment, engine, row_count, size_bytes,
      partition_key, order_by_key, key_pattern, data_type,
      columns[] (name, type, nullable, is_primary, is_unique,
                 is_index, is_sort_key, is_partition_key, comment),
      indexes[] (name, columns, unique, type),
      foreign_keys[] (name, columns, ref_instance, ref_db, ref_table, ref_columns)

    refs[] 字段:
      from, to, inferred (bool), confidence (int, 推断置信度)

    issues[] 字段:
      severity (warn|info), table, message
`,
		`

─── JSON OUTPUT FORMAT ───────────────────────────────────────

    Top-level:
      {
        "instances": [ ... ],   Instance array
        "refs":      [ ... ],   Table relationships (FK + inferred)
        "groups":    [ ... ],   Table clusters (optional)
        "issues":    [ ... ]    Issue diagnostics
      }

    instances[].databases[].tables[] contains:
      name, comment, engine, row_count, size_bytes,
      partition_key, order_by_key, key_pattern, data_type,
      columns[] (name, type, nullable, is_primary, is_unique,
                 is_index, is_sort_key, is_partition_key, comment),
      indexes[] (name, columns, unique, type),
      foreign_keys[] (name, columns, ref_instance, ref_db, ref_table, ref_columns)

    refs[] fields:
      from, to, inferred (bool), confidence (int, inference confidence)

    issues[] fields:
      severity (warn|info), table, message
`))

	fmt.Print(p(`

─── 列出可用数据库 ───────────────────────────────────────────

    子命令:

      dbexplain list

    列出当前环境中所有已配置的数据库连接（从 .env / .env.dbexplain /
    .env.dbexplain.enc 加载）。支持加密配置文件自动解密。

    输出字段:
      INDEX    DB 索引（用于 --db N）
      LABEL    DSN 标签（用于 --label）
      KIND     数据库类型
      HOST:PORT  主机与端口
      DATABASE   数据库名

    密码安全:
      此命令仅显示元数据（标签/类型/主机/库名），不输出 DSN 连接串、
      密码或任何凭证信息。加密配置文件的内容不会被解密显示。

    示例:
      dbexplain list                    # 列出 .env 中所有数据库
      dbexplain list --config db.json   # 从 JSON 配置文件列出

─── LIST CONFIGURED DATABASES ─────────────────────────────────

    Subcommand:

      dbexplain list

    Lists all configured database connections from .env / .env.dbexplain /
    .env.dbexplain.enc. Supports automatic decryption of encrypted configs.

    Output fields:
      INDEX    DB index (for --db N)
      LABEL    DSN label (for --label)
      KIND     Database type
      HOST:PORT  Host and port
      DATABASE   Database name

    Password safety:
      Only metadata (label/type/host/dbname) is displayed. No DSN connection
      strings, passwords, or credentials are ever exposed. Encrypted config
      content is never decrypted for display.

    Examples:
      dbexplain list                    # List all databases from .env
      dbexplain list --config db.json   # List from JSON config file

─── 显式 Schema 采集 ──────────────────────────────────────────

    子命令:
      dbexplain collect [flags]

    显式执行 Schema 采集，与顶层 dbexplain -env 等效但通过子命令调用。

    参数:
      -env                      从配置文件加载 DSN
      -dsn <string>             直接指定连接串
      -config <file>            JSON 配置文件
      -include/-exclude         按 DB 类型/标签过滤
      -label <name>             按 label 过滤
      -json                     输出 JSON 格式
      -human                    人类友好输出
      -context <dir>            AI 上下文目录
      -cache <file>             Schema 指纹缓存
      -o <file>                 输出到文件

    示例:
      dbexplain collect -env                        # 从 .env 采集全部
      dbexplain collect -env --include mysql,redis   # 仅采集 MySQL 和 Redis
      dbexplain collect -env --context ./ctx         # 输出 AI 上下文目录
      dbexplain collect -env --cache ./cache.json    # 启用增量变更检测

─── EXPLICIT SCHEMA COLLECTION ────────────────────────────────

    Subcommand:
      dbexplain collect [flags]

    Explicit schema collection, equivalent to dbexplain -env but called
    as a subcommand.

    Flags:
      -env                      Load DSNs from config file
      -dsn <string>             Direct DSN string
      -config <file>            JSON config file
      -include/-exclude         Filter by kind/label
      -label <name>             Filter by label
      -json                     Output JSON
      -human                    Human-friendly output
      -context <dir>            AI context directory
      -cache <file>             Fingerprint cache file
      -o <file>                 Output to file

    Examples:
      dbexplain collect -env                        # Collect all from .env
      dbexplain collect -env --include mysql,redis   # Collect MySQL and Redis only
      dbexplain collect -env --context ./ctx         # AI context output
      dbexplain collect -env --cache ./cache.json    # Enable delta detection

─── Schema 差异对比 ──────────────────────────────────────────

    子命令:
      dbexplain diff [flags]

    比较两个 Schema 快照或缓存版本的差异，支持字段级变更追踪。

    模式:
      1. --cache FILE --current FILE    缓存 vs 当前采集结果
      2. --cache FILE --since VERSION   缓存 vs 历史版本标签
      3. --before FILE --after FILE     两个历史 JSON 文件对比
      4. --cache FILE --list-versions   列出所有已存储版本

    参数:
      --cache <file>            Schema 指纹缓存文件
      --current <file>          当前扫描 JSON 文件
      --before <file>           之前的扫描 JSON 文件
      --after <file>            之后的扫描 JSON 文件
      --since <version>         与指定版本标签对比
      --list-versions           列出所有存储的版本标签
      -human                    人类友好输出（默认 JSON）

    示例:
      dbexplain diff --cache cache.json --current report.json --human
      dbexplain diff --cache cache.json --since v20260601 --human
      dbexplain diff --before old.json --after new.json
      dbexplain diff --cache cache.json --list-versions

─── SCHEMA DIFF ───────────────────────────────────────────────

    Subcommand:
      dbexplain diff [flags]

    Compare two schema snapshots or cache versions with field-level tracking.

    Modes:
      1. --cache FILE --current FILE    Cache vs current scan
      2. --cache FILE --since VERSION   Cache vs historical version label
      3. --before FILE --after FILE     Two JSON files comparison
      4. --cache FILE --list-versions   List all stored version labels

    Flags:
      --cache <file>            Fingerprint cache file
      --current <file>          Current scan JSON file
      --before <file>           Previous scan JSON file
      --after <file>            Later scan JSON file
      --since <version>         Compare against this version label
      --list-versions           List stored version labels
      -human                    Human-friendly output (default JSON)

    Examples:
      dbexplain diff --cache cache.json --current report.json --human
      dbexplain diff --cache cache.json --since v20260601 --human
      dbexplain diff --before old.json --after new.json
      dbexplain diff --cache cache.json --list-versions

─── REPL 交互模式 ─────────────────────────────────────────────

    子命令:
      dbexplain repl [flags]

    交互式只读查询终端，支持 SQL / 原生 / DSL 查询。

    参数:
      -env                      从配置文件加载 DSN
      -dsn <string>             直接指定连接串
      -config <file>            JSON 配置文件
      --label <name>            按 label 匹配 DSN
      --db <N>                  按 DB 编号匹配
      --dsl                     启用 DSL 模式

    REPL 内命令:
      .help                     显示帮助
      .exit, .quit              退出 REPL
      Ctrl+D                    退出 REPL

    示例:
      dbexplain repl -env --label mysql
      dbexplain repl -env --dsl --label mysql
      dbexplain repl -dsn 'mysql://user:pass@host:3306/mydb'

─── REPL INTERACTIVE MODE ─────────────────────────────────────

    Subcommand:
      dbexplain repl [flags]

    Interactive read-only query terminal supporting SQL / native / DSL queries.

    Flags:
      -env                      Load DSNs from config file
      -dsn <string>             Direct DSN string
      -config <file>            JSON config file
      --label <name>            Match DSN by label
      --db <N>                  Match DSN by index
      --dsl                     Enable DSL mode

    REPL commands:
      .help                     Show help
      .exit, .quit              Exit REPL
      Ctrl+D                    Exit REPL

    Examples:
      dbexplain repl -env --label mysql
      dbexplain repl -env --dsl --label mysql
      dbexplain repl -dsn 'mysql://user:pass@host:3306/mydb'

─── 只读查询执行 ──────────────────────────────────────────────

    子命令:
      dbexplain execute [flags] <query>

    在沙箱保护下执行只读 SQL/原生查询，返回结构化数据表。
    输出格式与 schema 采集 (instances/refs) 完全分离 (columns/rows)。

    参数:
      -env                      从配置文件加载 DSN
      -dsn <string>             直接指定连接串
      -config <file>            JSON 配置文件
      --label <name>            按 label 匹配 DSN
      --db <N>                  按 DB 编号匹配 (DB1=1)
      --limit <N>               最大返回行数 (默认 1000)
      --timeout <N>             查询超时秒数 (默认 30)
      --explain                 包裹 EXPLAIN 返回查询计划
      --dsl                     使用 DSL 模式（支持 @label.table 语法）

    SQL 查询 (MySQL/PG/GaussDB/SQLite/ClickHouse/ES):
      dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'
      dbexplain execute -env --db 1 --explain 'SELECT * FROM users WHERE id=1'
      dbexplain execute -env --label es 'SHOW TABLES'

    DSL 查询（统一语法，支持所有数据源）:
      dbexplain execute -env --label mydb --dsl 'SELECT * FROM @mydb.users WHERE id > 10'
      dbexplain execute -env --label csv-data --dsl 'SELECT col1, col2 FROM @csv-data.data'

    非 SQL 原生查询:
      dbexplain execute -env --label mongo '{"find":"users","filter":{},"limit":10}'
      dbexplain execute -env --label redis 'GET user:1001'
      dbexplain execute -env --label qdrant '{"count":"documents"}'

    安全保护:
      • SQL 三层校验 — 动词白名单 + 多语句检测 + 自动 LIMIT
      • DSL 通道 AST 级校验 — 统一 AST 解析，覆盖所有数据源
      • 非 SQL 内部白名单 — Redis 30+ 命令，MongoDB find/aggregate
      • 并发互斥 — 同一 label 同时仅一个查询
      • 双超时 — 应用层 context + 数据库层语句超时
      • 密码脱敏 — 查询结果不含任何连接信息或密码
`,
		`

─── EXPLICIT SCHEMA COLLECTION ────────────────────────────────

    Subcommand:
      dbexplain collect [flags]

    Explicit schema collection, equivalent to dbexplain -env but called
    as a subcommand.

    Flags:
      -env                      Load DSNs from config file
      -dsn <string>             Direct DSN string
      -config <file>            JSON config file
      -include/-exclude         Filter by kind/label
      -label <name>             Filter by label
      -json                     Output JSON
      -human                    Human-friendly output
      -context <dir>            AI context directory
      -cache <file>             Fingerprint cache file
      -o <file>                 Output to file

    Examples:
      dbexplain collect -env                        # Collect all from .env
      dbexplain collect -env --include mysql,redis   # Collect MySQL and Redis only
      dbexplain collect -env --context ./ctx         # AI context output
      dbexplain collect -env --cache ./cache.json    # Enable delta detection

─── SCHEMA DIFF ───────────────────────────────────────────────

    Subcommand:
      dbexplain diff [flags]

    Compare two schema snapshots or cache versions with field-level tracking.

    Modes:
      1. --cache FILE --current FILE    Cache vs current scan
      2. --cache FILE --since VERSION   Cache vs historical version label
      3. --before FILE --after FILE     Two JSON files comparison
      4. --cache FILE --list-versions   List all stored version labels

    Flags:
      --cache <file>            Fingerprint cache file
      --current <file>          Current scan JSON file
      --before <file>           Previous scan JSON file
      --after <file>            Later scan JSON file
      --since <version>         Compare against this version label
      --list-versions           List stored version labels
      -human                    Human-friendly output (default JSON)

    Examples:
      dbexplain diff --cache cache.json --current report.json --human
      dbexplain diff --cache cache.json --since v20260601 --human
      dbexplain diff --before old.json --after new.json
      dbexplain diff --cache cache.json --list-versions

─── REPL INTERACTIVE MODE ─────────────────────────────────────

    Subcommand:
      dbexplain repl [flags]

    Interactive read-only query terminal supporting SQL / native / DSL queries.

    Flags:
      -env                      Load DSNs from config file
      -dsn <string>             Direct DSN string
      -config <file>            JSON config file
      --label <name>            Match DSN by label
      --db <N>                  Match DSN by index
      --dsl                     Enable DSL mode

    REPL commands:
      .help                     Show help
      .exit, .quit              Exit REPL
      Ctrl+D                    Exit REPL

    Examples:
      dbexplain repl -env --label mysql
      dbexplain repl -env --dsl --label mysql
      dbexplain repl -dsn 'mysql://user:pass@host:3306/mydb'

─── READ-ONLY QUERY EXECUTION ─────────────────────────────────

    Subcommand:
      dbexplain execute [flags] <query>

    Run sandboxed read-only SQL/native queries with structured data output.
    Output format is fully separated from schema collection (columns/rows vs instances/refs).

    Flags:
      -env                      Load DSNs from config file
      -dsn <string>             Direct DSN connection string
      -config <file>            JSON config file
      --label <name>            Match DSN by label
      --db <N>                  Match DSN by DB index (DB1=1)
      --limit <N>               Max rows returned (default 1000)
      --timeout <N>             Query timeout in seconds (default 30)
      --explain                 Wrap with EXPLAIN for query plan
      --dsl                     Enable DSL mode (supports @label.table syntax)

    SQL queries (MySQL/PG/GaussDB/SQLite/ClickHouse/ES):
      dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'
      dbexplain execute -env --db 1 --explain 'SELECT * FROM users WHERE id=1'
      dbexplain execute -env --label es 'SHOW TABLES'

    DSL queries (unified syntax, all datasources):
      dbexplain execute -env --label mydb --dsl 'SELECT * FROM @mydb.users WHERE id > 10'
      dbexplain execute -env --label csv-data --dsl 'SELECT col1, col2 FROM @csv-data.data'

    Non-SQL native queries:
      dbexplain execute -env --label mongo '{"find":"users","filter":{},"limit":10}'
      dbexplain execute -env --label redis 'GET user:1001'
      dbexplain execute -env --label qdrant '{"count":"documents"}'

    Security:
      • SQL triple-layer check — verb whitelist + multi-statement detect + auto LIMIT
      • DSL AST-level validation — unified AST parsing, covers all datasources
      • Non-SQL internal whitelist — Redis 30+ commands, MongoDB find/aggregate
      • Concurrent mutex — only one query per label at a time
      • Dual timeout — application context + database statement timeout
      • Password redaction — query results contain no connection info or passwords
`))

	fmt.Print(p(`

─── 配置文件加密 ──────────────────────────────────────────────

    子命令:
      dbexplain encrypt [<file>] [flags]

    加密 .env 配置文件，使用机器指纹作为密钥。
    加密后的文件仅能在同一台机器上解密。

    参数:
      -password, --password    交互式输入密码（PBKDF2 + 机器指纹双重保护）
      -o, --output <file>      输出文件路径（默认：<输入文件>.enc）
      -h, --help               显示 encrypt 帮助

    示例:
      # 仅机器指纹加密（无需密码）
      dbexplain encrypt

      # 指定输入文件
      dbexplain encrypt .env.dbexplain

      # 密码 + 机器指纹双重加密
      dbexplain encrypt --password

      # 指定输出路径
      dbexplain encrypt .env.dbexplain -o config.enc

    使用加密文件（无需环境变量，自动发现 .enc 文件）：
      # 直接运行，工具自动搜索并解密
      dbexplain -env

      # 如果使用了 --password 加密，将密码写入密钥文件：
      echo "your-password" > ~/.config/dbexplain/.encryption_key
      chmod 600 ~/.config/dbexplain/.encryption_key

      # 也可通过环境变量显式指定（可选）：
      # export DBPROBE_ENV_FILE=.env.dbexplain.enc
      # export APP_ENCRYPTION_KEY="your-password"

    技术细节:
      加密算法: XChaCha20-Poly1305 (AEAD)
      密钥派生: SHA-256(硬件指纹) → 机器模式
                PBKDF2-HMAC-SHA256(密码, 指纹, 100k) → 密码模式
      文件格式: [1B mode][16B salt?][24B nonce][ciphertext+tag]

    平台指纹来源:
      Linux:   /etc/machine-id, /sys/class/dmi/id/product_uuid,
               /proc/cpuinfo, hostname
      macOS:   hw.uuid (sysctl), hw.model, hw.machine, hostname
      Windows: HKLM\\SOFTWARE\\Microsoft\\Cryptography\\MachineGuid

    注意事项:
      • 更换硬件后需重新加密配置文件
      • 加密文件权限为 0600（仅所有者可读写）
      • 原始明文文件应在加密后安全删除
`,
		`

─── CONFIG ENCRYPTION ─────────────────────────────────────────

    Subcommand:
      dbexplain encrypt [<file>] [flags]

    Encrypt a .env configuration file using machine fingerprint.
    The encrypted file can only be decrypted on the same machine.

    Flags:
      -password, --password    Interactive password prompt (PBKDF2 + machine fingerprint)
      -o, --output <file>      Output file path (default: <input>.enc)
      -h, --help               Show encrypt help

    Examples:
      # Machine fingerprint only (no password)
      dbexplain encrypt

      # Specify input file
      dbexplain encrypt .env.dbexplain

      # Password + machine fingerprint double protection
      dbexplain encrypt --password

      # Specify output path
      dbexplain encrypt .env.dbexplain -o config.enc

    Using encrypted files (no env vars needed — auto-discovery):
      # Just run, the tool auto-searches and decrypts
      dbexplain -env

      # If encrypted with --password, save password to key file:
      echo "your-password" > ~/.config/dbexplain/.encryption_key
      chmod 600 ~/.config/dbexplain/.encryption_key

      # Or override via environment variables (optional):
      # export DBPROBE_ENV_FILE=.env.dbexplain.enc
      # export APP_ENCRYPTION_KEY="your-password"

    Technical details:
      Algorithm:  XChaCha20-Poly1305 (AEAD)
      Key deriv:  SHA-256(hardware fingerprint) → machine mode
                  PBKDF2-HMAC-SHA256(password, fingerprint, 100k) → password mode
      Format:     [1B mode][16B salt?][24B nonce][ciphertext+tag]

    Platform fingerprint sources:
      Linux:   /etc/machine-id, /sys/class/dmi/id/product_uuid,
               /proc/cpuinfo, hostname
      macOS:   hw.uuid (sysctl), hw.model, hw.machine, hostname
      Windows: HKLM\\SOFTWARE\\Microsoft\\Cryptography\\MachineGuid

    Notes:
      • Re-encrypt config after hardware changes
      • Encrypted file permissions are 0600 (owner read/write only)
      • Delete the original plaintext file after encryption
`))

	fmt.Print(p(`

─── 退出码 ────────────────────────────────────────────────────

    0   成功
    1   参数错误、配置读取失败、所有 DSN 采集失败
`,
		`

─── EXIT CODES ───────────────────────────────────────────────

    0   Success
    1   Argument error, config read failure, all DSN collects failed
`))
}
