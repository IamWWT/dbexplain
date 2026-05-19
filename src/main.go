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
	flag.Parse()

	if *showVersion {
		fmt.Println("dbexplain", version)
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
		if err := os.WriteFile(*outputFile, []byte(out), 0644); err != nil {
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
