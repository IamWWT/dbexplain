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
	"dbexplain/capabilities"
	"dbexplain/connector"
	"dbexplain/cache"
	ctxcompress "dbexplain/context"
	"dbexplain/dsn"
	"dbexplain/render"
	"dbexplain/schema"
)

var version = "v0.0.4"

type dsnEntry struct {
	raw    string // DSN string
	envKey string // e.g. "DB1" if from .env, "" otherwise
}

func preScanLanguage() string {
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--language" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return "zh"
}

func hasHelpFlag() bool {
	for _, a := range os.Args[1:] {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func main() {
	_ = godotenv.Load()

	userLang := preScanLanguage()

	// Intercept -h/--help before flag.Parse for localized output
	if hasHelpFlag() {
		printHelp(userLang)
		return
	}
	flag.Usage = func() { printHelp(userLang) }

	var dsnFlags []string
	flag.Func("dsn", "...", func(s string) error { dsnFlags = append(dsnFlags, s); return nil })
	configFile := flag.String("config", "", "JSON config file with array of DSNs")
	useEnv := flag.Bool("env", false, "use .env file (prefix DB1=, DB2=...)")
	includeFilter := flag.String("include", "", "comma-separated kinds/labels/env-keys to include (e.g. mysql,redis or DB1,DB3)")
	excludeFilter := flag.String("exclude", "", "comma-separated kinds/labels/env-keys to exclude (e.g. mongodb,qdrant or DB5)")
	jsonOut := flag.Bool("json", false, "output JSON")
	humanOut := flag.Bool("human", false, "human-friendly output with context markers and visual separators")
	contextDir := flag.String("context", "", "write AI context files to directory (summary.json, topology.json, diagnostics.json, chunks/)")
	cacheFile := flag.String("cache", "", "fingerprint cache file for delta scan (.json)")
	outputFile := flag.String("o", "", "write output to file")
	perDSNTimeout := flag.Duration("timeout", 20*time.Second, "per-DSN collect timeout")
	showVersion := flag.Bool("version", false, "print version and exit")
	showManual := flag.Bool("manual", false, "print comprehensive manual and exit")
	language := flag.String("language", userLang, "manual language: zh (Chinese) or en (English)")
	filterFlag := flag.String("filter", "", "filter --manual output by keyword (case-insensitive)")
	flag.Parse()

	if *showVersion {
		fmt.Println("dbexplain", version)
		return
	}
	if *showManual {
		printManual(*language, *filterFlag)
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

	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("create log dir: %v", err)
	}

	// 过滤
	entries = filterDSNs(entries, *includeFilter, *excludeFilter, logDir)
	if len(entries) == 0 {
		log.Fatal("no DSNs provided (or all filtered out). Use -dsn, -env, or -config")
	}

	var dsns []string
	for _, e := range entries {
		dsns = append(dsns, e.raw)
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
		fmt.Fprintf(os.Stderr, "[!] 所有 DSN 采集均失败，报告为空。请检查日志: %s\n", logDir)
	} else {
		fmt.Fprintf(os.Stderr, "全部采集完成，总耗时 %v\n", time.Since(startAll))
	}

	// 按数据库类型构建能力映射 (每种类型只查询一次)
	kindCaps := make(map[string]*capabilities.Set)
	for _, inst := range instances {
		if _, ok := kindCaps[inst.Kind]; ok {
			continue
		}
		c, err := connector.GetConnector(inst.Kind)
		if err != nil {
			kindCaps[inst.Kind] = capabilities.NewSet()
		} else {
			kindCaps[inst.Kind] = capabilities.FromProvider(c)
		}
	}

	universe := &schema.Universe{Instances: instances}
	result := analyze.Analyze(universe, kindCaps)

	// 增量扫描: 加载指纹缓存并比较
	if *cacheFile != "" {
		store, err := cache.LoadStore(*cacheFile)
		if err != nil {
			log.Printf("load cache: %v (starting fresh)", err)
		}
		delta := store.Diff(universe)
		if len(delta.Added)+len(delta.Removed)+len(delta.Changed) > 0 {
			data, _ := json.MarshalIndent(delta, "", "  ")
			fmt.Fprintf(os.Stderr, "[delta] %d added, %d removed, %d changed\n",
				len(delta.Added), len(delta.Removed), len(delta.Changed))
			deltaFile := strings.TrimSuffix(*cacheFile, ".json") + "_delta.json"
			os.WriteFile(deltaFile, data, 0644)
		}
		if err := store.Update(universe); err != nil {
			log.Printf("save cache: %v", err)
		}
	}

	// 生成 AI Agent 上下文文件
	if *contextDir != "" {
		writeContext(*contextDir, result)
	}

	if *outputFile != "" {
		// 文件输出：捕获后写入（无 ANSI 转义码）
		var out string
		if *jsonOut {
			out = captureJSON(result)
		} else {
			out = captureText(result, *humanOut)
		}
		data, err := encodeOutput(out)
		if err != nil {
			log.Fatalf("encode output: %v", err)
		}
		if err := os.WriteFile(*outputFile, data, 0644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Report written to", *outputFile)
	} else if *jsonOut {
		// 终端 JSON：直接输出
		render.PrintJSON(result)
	} else {
		// 终端文本：直接渲染（保留颜色高亮）
		render.Print(result, *humanOut)
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

func filterDSNs(entries []dsnEntry, include, exclude string, logDir string) []dsnEntry {
	if include == "" && exclude == "" {
		return entries
	}

	includeSet := parseFilterSet(include)
	excludeSet := parseFilterSet(exclude)

	// 打开过滤日志文件，记录所有被跳过的 DSN
	filterLog, err := os.OpenFile(filepath.Join(logDir, "filter.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		filterLog = nil
	} else {
		defer filterLog.Close()
	}
	filterLogger := log.New(filterLog, "", log.LstdFlags)
	if filterLog == nil {
		filterLogger = log.Default() // fallback: 文件打不开就用 stderr
	}

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
			filterLogger.Printf("skipping %s (did not match include filter)", parsed.Redacted())
			continue
		}

		if len(excludeSet) > 0 && matchesDSNFilter(parsed, e.envKey, excludeSet) {
			filterLogger.Printf("excluding %s (matched exclude filter)", parsed.Redacted())
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

// ── 上下文输出 ──

func writeContext(dir string, result *analyze.Result) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("create context dir: %v", err)
	}

	// summary.json
	summary := ctxcompress.GenerateSummary(result, 10)
	writeJSON(filepath.Join(dir, "summary.json"), summary)

	// topology.json
	topo := ctxcompress.GenerateTopology(result)
	writeJSON(filepath.Join(dir, "topology.json"), topo)

	// diagnostics.json
	diag := ctxcompress.GenerateDiagnostics(result.Issues)
	writeJSON(filepath.Join(dir, "diagnostics.json"), diag)

	// retrieval chunks
	chunksDir := filepath.Join(dir, "chunks")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		log.Fatalf("create chunks dir: %v", err)
	}
	chunks := ctxcompress.GenerateChunks(result, 15)
	for _, chunk := range chunks {
		md := ctxcompress.RenderChunkMarkdown(&chunk)
		name := strings.ReplaceAll(strings.ReplaceAll(chunk.Table, "/", "_"), "\\", "_") + ".md"
		if err := os.WriteFile(filepath.Join(chunksDir, name), []byte(md), 0644); err != nil {
			log.Printf("write chunk %s: %v", name, err)
		}
	}

	fmt.Fprintf(os.Stderr, "Context written to %s (%d files)\n", dir, 3+len(chunks))
}

func writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("marshal %s: %v", path, err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("write %s: %v", path, err)
	}
}

// ── 输出捕获 ──

func captureText(result *analyze.Result, human bool) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	render.Print(result, human)
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

// printHelp prints a concise flag summary (like -h/--help) in the given language.
func printHelp(lang string) {
	p := func(zh, en string) string {
		if lang == "en" {
			return en
		}
		return zh
	}
	out := os.Stderr

	fmt.Fprint(out, p(
		"dbexplain — 数据库上下文编译器\n\n"+
			"用法: dbexplain [参数]\n\n",
		"dbexplain — Database Context Compiler\n\n"+
			"Usage: dbexplain [options]\n\n",
	))

	// Group 1: Input Sources
	fmt.Fprint(out, p(
		"数据源 (Input Sources):\n"+
			"  -dsn string        数据库连接串，可重复使用\n"+
			"  -env               从 .env 文件加载 DSN (DB1=, DB2=, ...)\n"+
			"  -config file       从 JSON 文件读取 DSN 数组\n\n",
		"Input Sources:\n"+
			"  -dsn string        Database connection string, repeatable\n"+
			"  -env               Load DSNs from .env file (DB1=, DB2=, ...)\n"+
			"  -config file       Read DSN array from JSON file\n\n",
	))

	// Group 2: Filtering
	fmt.Fprint(out, p(
		"过滤 (Filtering):\n"+
			"  -include filter    仅包含匹配的 DSN (按类型/标签/编号，逗号分隔)\n"+
			"  -exclude filter    排除匹配的 DSN\n\n",
		"Filtering:\n"+
			"  -include filter    Only include matching DSNs (by kind/label/key, comma-sep)\n"+
			"  -exclude filter    Exclude matching DSNs\n\n",
	))

	// Group 3: Output Control
	fmt.Fprint(out, p(
		"输出控制 (Output Control):\n"+
			"  -o file            将输出写入文件 (自动添加 UTF-8 BOM)\n\n",
		"Output Control:\n"+
			"  -o file            Write output to file (auto UTF-8 BOM)\n\n",
	))

	// Group 4: Display Format
	fmt.Fprint(out, p(
		"显示格式 (Display Format):\n"+
			"  -json              输出 JSON 格式\n"+
			"  --human            人类友好输出 (带上下文标记和视觉分隔)\n\n",
		"Display Format:\n"+
			"  -json              Output JSON format\n"+
			"  --human            Human-friendly output with context markers\n\n",
	))

	// Group 5: AI Context
	fmt.Fprint(out, p(
		"AI 上下文 (AI Context):\n"+
			"  --context dir      写入 AI 上下文文件 (summary.json/topology.json/chunks/)\n"+
			"  --cache file       Schema 指纹缓存，用于增量变更检测\n\n",
		"AI Context:\n"+
			"  --context dir      Write AI context files (summary.json/topology.json/chunks/)\n"+
			"  --cache file       Schema fingerprint cache for delta detection\n\n",
	))

	// Group 6: Performance
	fmt.Fprint(out, p(
		"性能 (Performance):\n"+
			"  -timeout duration  每 DSN 采集超时 (默认 20s, 例: 30s / 1m)\n\n",
		"Performance:\n"+
			"  -timeout duration  Per-DSN collect timeout (default 20s, e.g. 30s/1m)\n\n",
	))

	// Group 7: Help
	fmt.Fprint(out, p(
		"帮助 (Help):\n"+
			"  -h, --help         打印此参数列表并退出\n"+
			"  --manual           打印完整手册并退出\n"+
			"  --language zh|en   手册语言 (默认 zh)\n"+
			"  --filter keyword   过滤手册输出 (配合 --manual 使用)\n"+
			"  --version          输出版本号并退出\n\n",
		"Help:\n"+
			"  -h, --help         Print this option list and exit\n"+
			"  --manual           Print comprehensive manual and exit\n"+
			"  --language zh|en   Manual language (default zh)\n"+
			"  --filter keyword   Filter manual output (use with --manual)\n"+
			"  --version          Print version and exit\n\n",
	))

	// Footer
	fmt.Fprint(out, p(
		"完整手册: dbexplain --manual [--language zh|en]\n",
		"Full manual: dbexplain --manual [--language zh|en]\n",
	))
}

func printManual(lang, filter string) {
	if filter == "" {
		printManualContent(lang)
		return
	}
	captured := captureManualOutput(lang)
	kw := strings.ToLower(filter)

	// Split by ─── section boundaries; each section is a complete block
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

// splitSections splits the manual text into sections.
// Sections are delimited by lines that start with "───" (box-drawing header lines).
// The preamble (everything before the first ───) is returned as the first section.
func splitSections(text string) []string {
	// Find the first occurrence of "\n───" in the text
	idx := strings.Index(text, "\n───")
	if idx < 0 {
		// No section boundaries found; return entire text as one section
		return []string{text}
	}
	// Split preamble off
	preamble := text[:idx]
	body := text[idx+1:] // skip the leading \n

	var sections []string
	if strings.TrimSpace(preamble) != "" {
		sections = append(sections, preamble)
	}

	// Split remaining body on "\n───" to get each section
	parts := strings.Split(body, "\n───")
	for _, p := range parts {
		if p == "" {
			continue
		}
		// Re-prepend the ─── that was consumed by the split
		sections = append(sections, "───"+p)
	}
	return sections
}

func captureManualOutput(lang string) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	printManualContent(lang)
	w.Close()
	os.Stdout = old
	<-done
	return buf.String()
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
      label=<别名>        实例别名，决定日志文件名 logs/<label>.log
      cluster=true        Redis 集群模式 (自动扫描所有分片)
      tls=true            ES / Redis 启用 TLS 加密
      sslmode=<mode>      PostgreSQL SSL: disable/require/verify-ca/verify-full
      authSource=<db>     MongoDB 认证数据库名
`,
		`

─── DSN FORMAT ────────────────────────────────────────────────

    General format:
      scheme://[user:password@]host[:port][/dbname][?param1=val1&param2=val2]

    DSN parameters:
      label=<name>        Instance alias, determines log file name logs/<label>.log
      cluster=true        Redis cluster mode (auto-scan all shards)
      tls=true            ES / Redis enable TLS encryption
      sslmode=<mode>      PostgreSQL SSL: disable/require/verify-ca/verify-full
      authSource=<db>     MongoDB authentication database name
`))

	fmt.Print(p(`

─── 全局参数 ──────────────────────────────────────────────────

    -dsn <string>         数据库连接串，可重复多次指定多个库
    -config <file>        从 JSON 文件读取 DSN 列表 (数组格式)
    -env                  从 .env 加载 DSN (格式: DB<n>=<DSN>)
    -include <filter>     仅包含匹配的 DSN (按类型/label/env编号, 逗号分隔)
    -exclude <filter>     排除匹配的 DSN (格式同 -include)
    -json                 输出 JSON 格式 (适合程序消费)
    -human                人类友好输出：带上下文标记 [table=] [pattern=] 和视觉分隔
    -o <file>             将报告写入文件 (自动添加 UTF-8 BOM)
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
    -env                  Load DSNs from .env file (format: DB<n>=<DSN>)
    -include <filter>     Only include matching DSNs (by kind/label/env-key, comma-sep)
    -exclude <filter>     Exclude matching DSNs (same format as -include)
    -json                 Output JSON format (for programmatic consumption)
    -human                Human-friendly output: context markers [table=] [pattern=] etc.
    -o <file>             Write report to file (auto-prepends UTF-8 BOM)
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
`))

	// ─── Per-database sections ───

	printManualMySQL(p)
	printManualPostgres(p)
	printManualGaussDB(p)
	printManualClickHouse(p)
	printManualSQLite(p)
	printManualRedis(p)
	printManualElasticsearch(p)
	printManualMongoDB(p)
	printManualQdrant(p)

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

// ── 各数据库详细章节 ──

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
      ./dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
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
      ./dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
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
      ./dbexplain -dsn 'postgres://u:p@host:5432/warehouse?label=my-pg&sslmode=disable'
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
      ./dbexplain -dsn 'postgres://u:p@host:5432/warehouse?label=my-pg&sslmode=disable'
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
      ./dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
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
      ./dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
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
      ./dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
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
      • Comment inference relies on sample row data
      • View engine tables excluded from row counts

    Example:
      ./dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
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
      ./dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-sqlite'
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
      ./dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-sqlite'
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
      ./dbexplain -dsn 'redis://:pwd@127.0.0.1:6379/0?label=my-redis'
      # 集群
      ./dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
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
      ./dbexplain -dsn 'redis://:pwd@127.0.0.1:6379/0?label=my-redis'
      # Cluster
      ./dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
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
      tls=true        启用 HTTPS (InsecureSkipVerify, 诊断工具可接受)

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
      ./dbexplain -dsn 'elasticsearch://elastic:pwd@127.0.0.1:9200?label=my-es'
      # HTTPS
      ./dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
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
      tls=true        Enable HTTPS (InsecureSkipVerify, acceptable for diagnostics)

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
      ./dbexplain -dsn 'elasticsearch://elastic:pwd@127.0.0.1:9200?label=my-es'
      # HTTPS
      ./dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
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
      ./dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
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
      ./dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
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
      ./dbexplain -dsn 'qdrant://:my-api-key@127.0.0.1:6334?label=my-qdrant'
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
      ./dbexplain -dsn 'qdrant://:my-api-key@127.0.0.1:6334?label=my-qdrant'
`))
}
