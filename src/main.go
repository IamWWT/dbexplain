package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"dbexplain/analyze"
	"dbexplain/connector"
	"dbexplain/dsn"
	"dbexplain/render"
	"dbexplain/schema"
)

var version = "dev"

type dsnEntry struct {
	raw    string // DSN string
	envKey string // e.g. "DB1" if from .env, "" otherwise
}

func main() {
	_ = godotenv.Load()

	var dsnFlags []string
	flag.Func("dsn", "...", func(s string) error { dsnFlags = append(dsnFlags, s); return nil })
	configFile := flag.String("config", "", "JSON config file with array of DSNs")
	useEnv := flag.Bool("env", false, "use .env file (prefix DB1=, DB2=...)")
	includeFilter := flag.String("include", "", "comma-separated kinds/labels/env-keys to include (e.g. mysql,redis or DB1,DB3)")
	excludeFilter := flag.String("exclude", "", "comma-separated kinds/labels/env-keys to exclude (e.g. mongodb,qdrant or DB5)")
	jsonOut := flag.Bool("json", false, "output JSON")
	outputFile := flag.String("o", "", "write output to file")
	perDSNTimeout := flag.Duration("timeout", 20*time.Second, "per-DSN collect timeout")
	showVersion := flag.Bool("version", false, "print version and exit")
	showManual := flag.Bool("manual", false, "print comprehensive manual and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("dbexplain", version)
		return
	}
	if *showManual {
		printManual()
		return
	}

	var entries []dsnEntry
	for _, raw := range dsnFlags {
		entries = append(entries, dsnEntry{raw: raw})
	}
	if *useEnv {
		entries = append(entries, loadFromEnv()...)
	}
	if *configFile != "" {
		for _, raw := range loadFromConfig(*configFile) {
			entries = append(entries, dsnEntry{raw: raw})
		}
	}

	// 过滤
	entries = filterDSNs(entries, *includeFilter, *excludeFilter)
	if len(entries) == 0 {
		log.Fatal("no DSNs provided (or all filtered out). Use -dsn, -env, or -config")
	}

	var dsns []string
	for _, e := range entries {
		dsns = append(dsns, e.raw)
	}

	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("create log dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var instances []*schema.Instance
	var mu sync.Mutex
	var wg sync.WaitGroup

	startAll := time.Now() // 记录总开始时间

	for i, rawDSN := range dsns {
		i := i
		rawDSN := rawDSN
		wg.Add(1)
		go func() {
			defer wg.Done()
			parsed, err := dsn.ParseDSN(rawDSN)
			if err != nil {
				log.Printf("invalid DSN: %v (DSN redacted)", err)
				return
			}
			label := parsed.Label
			if label == "" {
				label = fmt.Sprintf("db_%d", i)
			}
			logFileName := filepath.Join(logDir, label+".log")
			logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Printf("create log file %s: %v", logFileName, err)
				logFile = os.Stderr
			} else {
				defer logFile.Close()
			}
			logger := log.New(logFile, "", log.LstdFlags)
			collectCtx := connector.WithLogger(ctx, logger)
			collectCtx, cancel := context.WithTimeout(collectCtx, *perDSNTimeout)
			defer cancel()

			fmt.Fprintf(os.Stderr, "[采集中] %s\n", label)
			start := time.Now()
			inst, err := connector.Collect(collectCtx, rawDSN)
			elapsed := time.Since(start)

			if err != nil {
				logger.Printf("skip %s: %v", parsed.Redacted(), err)
				return
			}

			mu.Lock()
			instances = append(instances, inst)
			mu.Unlock()

			nTables := totalTables(inst)
			fmt.Fprintf(os.Stderr, "[完成] %s (%d 表) 耗时 %v\n", label, nTables, elapsed)
		}()
	}

	wg.Wait()
	if len(instances) == 0 {
		fmt.Fprintf(os.Stderr, "⚠ 所有 DSN 采集均失败，报告为空。请检查日志: %s\n", logDir)
	} else {
		fmt.Fprintf(os.Stderr, "全部采集完成，总耗时 %v\n", time.Since(startAll))
	}

	universe := &schema.Universe{Instances: instances}
	result := analyze.Analyze(universe)

	var out string
	if *jsonOut {
		out = captureJSON(result)
	} else {
		out = captureText(result)
	}

	if *outputFile != "" {
		// Prepend UTF-8 BOM so Windows Notepad/CMD recognizes the encoding
		data := append([]byte("\xEF\xBB\xBF"), []byte(out)...)
		if err := os.WriteFile(*outputFile, data, 0644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Report written to", *outputFile)
	} else {
		fmt.Print(out)
	}
}

// totalTables 统计一个实例中所有数据库的总表数
func totalTables(inst *schema.Instance) int {
	total := 0
	for _, db := range inst.Databases {
		total += len(db.Tables)
	}
	return total
}

// ── DSN 过滤 ──

func filterDSNs(entries []dsnEntry, include, exclude string) []dsnEntry {
	if include == "" && exclude == "" {
		return entries
	}

	includeSet := parseFilterSet(include)
	excludeSet := parseFilterSet(exclude)

	var filtered []dsnEntry
	for _, e := range entries {
		// 解析失败的 DSN 保留（到采集阶段报错）
		parsed, err := dsn.ParseDSN(e.raw)
		if err != nil {
			filtered = append(filtered, e)
			continue
		}

		// include 优先：匹配 include 的 DSN 不会被 exclude 移除
		if len(includeSet) > 0 && matchesDSNFilter(parsed, e.envKey, includeSet) {
			filtered = append(filtered, e)
			continue
		}

		if len(includeSet) > 0 {
			// 有 include 但不匹配，跳过
			log.Printf("skipping %s (did not match include filter)", e.raw)
			continue
		}

		if len(excludeSet) > 0 && matchesDSNFilter(parsed, e.envKey, excludeSet) {
			log.Printf("excluding %s (matched exclude filter)", e.raw)
			continue
		}

		filtered = append(filtered, e)
	}
	return filtered
}

func parseFilterSet(csv string) map[string]bool {
	set := make(map[string]bool)
	if csv == "" {
		return set
	}
	for _, item := range strings.Split(csv, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			set[strings.ToLower(item)] = true
		}
	}
	return set
}

// matchesDSNFilter 检查 DSN 是否匹配过滤集合
// 匹配维度：数据库类型(kind)、标签(label)、环境变量键(envKey, 如 DB1)
func matchesDSNFilter(d *dsn.DSN, envKey string, filterSet map[string]bool) bool {
	if filterSet[strings.ToLower(d.Kind)] {
		return true
	}
	if filterSet[strings.ToLower(d.Label)] {
		return true
	}
	if envKey != "" && filterSet[strings.ToLower(envKey)] {
		return true
	}
	return false
}

// ── 环境变量加载 ──

type envEntry struct {
	idx int
	key string
	val string
}

func loadFromEnv() []dsnEntry {
	var entries []envEntry

	for _, env := range os.Environ() {
		eqIdx := strings.Index(env, "=")
		if eqIdx < 0 {
			continue
		}
		key := env[:eqIdx]
		val := env[eqIdx+1:]

		if !strings.HasPrefix(key, "DB") {
			continue
		}
		numStr := key[2:]
		idx, err := strconv.Atoi(numStr)
		if err != nil || idx <= 0 {
			continue
		}
		entries = append(entries, envEntry{idx, key, val})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].idx < entries[j].idx
	})

	var result []dsnEntry
	for _, e := range entries {
		result = append(result, dsnEntry{raw: e.val, envKey: e.key})
	}
	return result
}

func loadFromConfig(filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var dsnList []string
	if err := json.Unmarshal(data, &dsnList); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	return dsnList
}

// ── 输出捕获 ──

func captureText(result *analyze.Result) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	render.Print(result)
	w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}

func captureJSON(result *analyze.Result) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	render.PrintJSON(result)
	w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}

// ── 完整帮助手册 ──

func printManual() {
	fmt.Print(`
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
    图与问题诊断。

    支持 MySQL、PostgreSQL、GaussDB、SQLite、ClickHouse、Redis、
    Qdrant、Elasticsearch、MongoDB 共 9 种数据库。

    所有操作为只读，安全无副作用。

─── DSN 格式详解 ──────────────────────────────────────────────

    通用格式:
      scheme://[用户:密码@]主机[:端口][/库名][?param1=val1&param2=val2]

    scheme 一览:
      mysql://           MySQL (默认端口 3306)
      postgres://        PostgreSQL (默认端口 5432)
      gaussdb://         GaussDB (兼容 PostgreSQL 协议)
      clickhouse://      ClickHouse HTTP (默认端口 8123)
      sqlite://          本地 SQLite (路径为绝对路径)
      redis://           Redis (默认端口 6379)
      elasticsearch://   Elasticsearch HTTP (默认端口 9200)
      elasticsearchs://  Elasticsearch HTTPS (同 elasticsearch://?tls=true)
      mongodb://         MongoDB (默认端口 27017)
      qdrant://          Qdrant 向量数据库 (默认端口 6334)

    DSN 参数速查:
      label=<别名>        实例别名，决定日志文件名 logs/<label>.log
      cluster=true        Redis 集群模式，自动扫描所有分片
      tls=true            Elasticsearch / Redis 启用 TLS 加密
      sslmode=<mode>      PostgreSQL SSL 模式:
                            disable (默认) / require / verify-ca / verify-full
      authSource=<db>     MongoDB 认证数据库名

    示例:
      'mysql://root:pass@127.0.0.1:3306/shop?label=shop-db'
      'postgres://u:p@host:5432/warehouse?sslmode=disable&label=my-pg'
      'redis://:pass@10.0.0.1:7000/0?cluster=true&label=redis-cluster'
      'sqlite:///home/user/data/app.db?label=local-sqlite'
      'mongodb://admin:p@host:27017/mydb?authSource=admin&label=mongo'
      'qdrant://:api-key@127.0.0.1:6334?label=qdrant'

─── 参数 (OPTIONS) ───────────────────────────────────────────

    -dsn <DSN-string>
        指定一个数据库连接串。可重复使用以连接多个数据库。

    -config <file>
        从 JSON 文件读取 DSN 列表。文件格式:
          ["dsn1", "dsn2", ...]

    -env
        从当前目录或上层目录的 .env 文件加载 DSN。
        .env 格式: DB<n>=<DSN>  (如 DB1=mysql://... DB2=redis://...)
        编号无需连续，程序按数字升序加载。
        使用 -env 前请确保 .env 文件存在且 DSN 已正确配置。

    -include <filter>
        仅包含匹配的 DSN，过滤维度:
          - 数据库类型:    mysql, postgres, redis, mongodb ...
          - 标签 (label):  shop-db, my-pg ...
          - .env 编号:     DB1, DB3, DB5 ...
        逗号分隔，大小写不敏感。示例:
          -include 'mysql,postgres'        仅采集 MySQL 和 PostgreSQL
          -include 'DB1,DB3'                仅采集指定编号的 .env DSN

    -exclude <filter>
        排除匹配的 DSN，过滤维度同 -include。示例:
          -exclude 'mongodb,qdrant'         跳过 MongoDB 和 Qdrant
          -exclude 'DB5'                    排除指定 .env 编号

        注意: 当 -include 和 -exclude 同时使用时，-include 优先。

    -json
        输出 JSON 格式而非终端美化输出。适合程序消费或管道处理。
        详见下方「JSON 输出格式」章节。

    -o <file>
        将报告写入指定文件（而非 stdout）。
        输出内容自动添加 UTF-8 BOM，兼容 Windows Notepad/CMD。
        与 -json 配合使用时写入 JSON 格式。

    -timeout <duration>
        每个 DSN 的采集超时时间 (默认 20s)。
        格式: 30s, 1m, 500ms 等。
        单库采集超时不影响其他 DSN，该库被跳过并记录日志。

    --version
        输出版本号并退出。

    --manual
        打印此完整帮助手册并退出。

    -h, --help
        打印简版参数列表并退出。

─── 使用示例 ──────────────────────────────────────────────────

    # 1) 分析单个 MySQL 库
    ./dbexplain -dsn 'mysql://root:pwd@localhost:3306/mydb?label=my'

    # 2) 同时分析多个不同数据库
    ./dbexplain \
      -dsn 'mysql://root:pwd@172.0.0.1:3306/orders' \
      -dsn 'postgres://u:p@10.0.0.2:5432/users?label=pg' \
      -dsn 'redis://:pwd@10.0.0.3:6379/0?label=cache'

    # 3) 使用 .env 文件 (推荐)
    cat > .env << 'EOF'
    DB1=mysql://root:pwd@127.0.0.1:3306/shop?label=my-mysql
    DB3=redis://:pwd@127.0.0.1:6379/0?label=my-redis
    DB5=postgres://u:p@localhost:5432/warehouse?label=my-pg
    EOF
    ./dbexplain -env

    # 4) 用 -include 过滤 .env 中的指定 DSN
    ./dbexplain -env -include 'mysql,DB3'

    # 5) 输出 JSON 到文件 (程序消费)
    ./dbexplain -env -json -o report.json

    # 6) 输出美化报告到文件 (人工阅读)
    ./dbexplain -env -o report.md

    # 7) 自定义超时
    ./dbexplain -env -timeout 60s

─── JSON 输出格式 ────────────────────────────────────────────

    顶层结构:

    {
      "instances": [ ... ],   // 实例数组
      "refs":      [ ... ],   // 表关系数组 (外键 + 推断)
      "groups":    [ ... ],   // 表聚类数组 (可选)
      "issues":    [ ... ]    // 问题诊断数组
    }

    ── instances[] ──────────────────────────────────────────

      {
        "label":     "my-mysql",       // 实例别名 (来自 DSN ?label=)
        "kind":      "mysql",          // 数据库类型
        "databases": [                 // 数据库数组
          {
            "name":        "mydb",     // 数据库名
            "table_count": 5,          // 该库下表的数量
            "tables": [                // 表数组
              {
                "name":          "orders",      // 表名
                "comment":       "订单表",        // 表注释 (可选)
                "engine":        "InnoDB",       // 存储引擎 (可选)
                "row_count":     42000,          // 近似行数 (可选)
                "size_bytes":    1572864,        // 表大小 (字节, 可选)
                "partition_key": "...",          // 分区键 (可选, ClickHouse)
                "order_by_key":  "...",          // 排序键 (可选, ClickHouse)
                "key_pattern":   "...",          // Redis 键模式 (可选)
                "data_type":     "...",          // 数据类型 (可选, Redis)
                "columns": [                     // 列数组
                  {
                    "name":             "id",
                    "type":             "int(11)",
                    "nullable":         false,
                    "default":          "",         // 默认值 (可选)
                    "comment":          "标识符",    // 列注释 (可选)
                    "is_primary":       true,       // 主键 (可选)
                    "is_unique":        false,      // 唯一约束 (可选)
                    "is_index":         false,      // 有索引 (可选)
                    "is_sort_key":      false,      // 排序键 (可选)
                    "is_partition_key": false       // 分区键 (可选)
                  }
                ],
                "indexes": [                      // 索引数组 (可选)
                  {
                    "name":    "idx_user_id",
                    "columns": ["user_id"],
                    "unique":  false,
                    "type":    ""                 // 索引类型 (可选)
                  }
                ],
                "foreign_keys": [                 // 外键数组 (可选)
                  {
                    "name":         "fk_orders_users",
                    "columns":      ["user_id"],
                    "ref_instance": "my-mysql",
                    "ref_db":       "mydb",
                    "ref_table":    "users",
                    "ref_columns":  ["id"]
                  }
                ]
              }
            ]
          }
        ]
      }

    ── refs[] ────────────────────────────────────────────────

      {
        "from":       "my-mysql/mydb.orders(user_id)",
        "to":         "my-mysql/mydb.users(id)",
        "inferred":   false,          // false=显式外键, true=命名推断
        "confidence": 85              // 推断置信度 (仅 inferred=true 时有效)
      }

    ── groups[] ──────────────────────────────────────────────

      {
        "name":   "orders* cluster",   // 聚类名称
        "tables": [
          { "instance": "my-mysql", "db": "mydb", "table": "orders" },
          { "instance": "my-pg",    "db": "warehouse", "table": "order_items" }
        ]
      }

    ── issues[] ──────────────────────────────────────────────

      {
        "severity": "warn",
        "table":    "my-mysql/mydb/orders",
        "message":  "FK column \"user_id\" has no index"
      }

─── EXIT CODES ───────────────────────────────────────────────

    0   成功
    1   参数错误、配置读取失败、所有 DSN 采集失败
`)
}
