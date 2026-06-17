// Package main is the thin entry point for dbexplain.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IamWWT/dbexplain/internal/analyze"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/cache"
	"github.com/IamWWT/dbexplain/internal/check"
	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/diff"
	"github.com/IamWWT/dbexplain/internal/encrypt"
	"github.com/IamWWT/dbexplain/internal/list"
	"github.com/IamWWT/dbexplain/internal/manual"
	"github.com/IamWWT/dbexplain/internal/metrics"
	"github.com/IamWWT/dbexplain/internal/output"
	"github.com/IamWWT/dbexplain/internal/version"
	"github.com/IamWWT/dbexplain/internal/render"
	"github.com/IamWWT/dbexplain/internal/schema"
)


// preScanSQLLogMaxLen scans os.Args for --sql-log-max-len before any FlagSet.Parse.
// This allows it to work as a global flag across all subcommands (collect, execute, repl, check, etc.).
func preScanSQLLogMaxLen() {
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--sql-log-max-len" && i+1 < len(os.Args) {
			v, err := strconv.Atoi(os.Args[i+1])
			if err == nil && v > 0 {
				connector.MaxSQLLogLen = v
			}
			return
		}
	}
}

// preScanVerbose scans os.Args for --verbose before any FlagSet.Parse.
// Sets connector.Verbose = true so [DEBUG] logs are written to dbexplain.log.
func preScanVerbose() {
	for _, a := range os.Args {
		if a == "--verbose" {
			connector.Verbose = true
			return
		}
	}
}

func hasHelpFlag() bool {
	for _, a := range os.Args[1:] {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// metricSnapshot carries collection metrics from goroutines to the collector via channel.
type metricSnapshot struct {
	label     string
	kind      string
	success   bool
	duration  time.Duration
	numDBs    int
	numTables int
	errMsg    string
}

// collectParams holds shared parameters for collectInstance.
type collectParams struct {
	metricCh      chan metricSnapshot
	mu            *sync.Mutex
	instances     *[]*schema.Instance
	sample        bool
	skipOpstats   bool
	tableFilter   string
	perDSNTimeout time.Duration
	ctx           context.Context
	wg            *sync.WaitGroup
	sem           chan struct{}
}

// collectInstance runs collection for a single DSN inside a goroutine.
// Handles DSN parsing, label resolution, context setup, timeout guard, metric reporting,
// and mutex-protected result append — shared by the default collect and "dbexplain collect".
func collectInstance(rawDSN string, idx int, p collectParams) {
	defer p.wg.Done()
	defer func() { <-p.sem }()

	parsed, err := dsn.ParseDSN(rawDSN)
	if err != nil {
		log.Printf("invalid DSN: %v", config.SanitizeErr(err))
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: collect %s: %v", parsed.Redacted(), r)
		}
	}()
	label := parsed.Label
	if label == "" {
		label = fmt.Sprintf("db_%d", idx)
	}
	logger := log.New(log.Writer(), fmt.Sprintf("[label=%s] [kind=%s] ", label, parsed.Kind), log.LstdFlags)
	collectCtx := connector.WithLogger(p.ctx, logger)
	if p.sample {
		collectCtx = connector.WithSample(collectCtx)
	}
	if p.skipOpstats {
		collectCtx = connector.WithSkipOpstats(collectCtx)
	}
	if p.tableFilter != "" {
		collectCtx = connector.WithTableFilter(collectCtx, []string{p.tableFilter})
	}
	collectCtx, cancel := context.WithTimeout(collectCtx, p.perDSNTimeout)
	defer cancel()

	logger.Printf("[采集中] %s", label)
	start := time.Now()

	// Run collection in sub-goroutine with timeout guard.
	// lib/pq context cancellation is unreliable when the server is unresponsive
	// (GaussDB Oracle compat mode is known to hang). A select+channel pattern
	// ensures we don't hang forever — the sub-goroutine may leak but the
	// process continues.
	type collectOutcome struct {
		inst *schema.Instance
		err  error
	}
	outcome := make(chan collectOutcome, 1)
	go func() {
		subInst, subErr := connector.Collect(collectCtx, rawDSN)
		outcome <- collectOutcome{subInst, subErr}
	}()

	var (
		inst       *schema.Instance
		collectErr error
		elapsed    time.Duration
	)
	timeTimer := time.NewTimer(p.perDSNTimeout)
	select {
	case res := <-outcome:
		inst = res.inst
		collectErr = res.err
		elapsed = time.Since(start)
		timeTimer.Stop()
	case <-timeTimer.C:
		elapsed = time.Since(start)
		logger.Printf("[采集超时] %s (超过 %v) — 连接/认证/查询任一阶段卡住，已跳过，不影响其他 label", label, p.perDSNTimeout)
		p.metricCh <- metricSnapshot{label, parsed.Kind, false, elapsed, 0, 0, "timeout: collect hung"}
		return
	}

	if collectErr != nil {
		p.metricCh <- metricSnapshot{label, parsed.Kind, false, elapsed, 0, 0, config.SanitizeErr(collectErr).Error()}
		logger.Printf("skip %s: %v", parsed.Redacted(), config.SanitizeErr(collectErr))
		return
	}

	nTables := totalTables(inst)
	nDBs := len(inst.Databases)
	p.metricCh <- metricSnapshot{label, parsed.Kind, true, elapsed, nDBs, nTables, ""}

	p.mu.Lock()
	*p.instances = append(*p.instances, inst)
	p.mu.Unlock()

	logger.Printf("[完成] %s (%d 表) 耗时 %v", label, nTables, elapsed)
}

// buildKindCaps builds a kind→capabilities.Set map from collected instances.
func buildKindCaps(instances []*schema.Instance) map[string]*capabilities.Set {
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
	return kindCaps
}

func main() {
	// Pre-scan global flags before any FlagSet.Parse
	preScanSQLLogMaxLen()
	preScanVerbose()

	// Intercept subcommands BEFORE flag.Parse
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "encrypt":
			encrypt.Handle(os.Args[2:])
			return
		case "mysql", "postgres", "postgresql", "pg", "gaussdb",
			"clickhouse", "ch", "sqlite", "sqlite3",
			"redis", "mongodb", "elasticsearch", "es", "qdrant",
			"csv", "tsv", "xlsx",
			"prometheus", "prom",
			"duckdb",
			"oracle", "hive":
			manual.PrintDBManual(os.Args[1], os.Args[2:])
			return
		case "all":
			manual.HandleAllManual(os.Args[2:])
			return
		case "execute":
			handleExecute(os.Args[2:])
			return
		case "check":
			check.Handle(os.Args[2:])
			return
		case "list":
			list.Handle(os.Args[2:])
			return
		case "diff":
			handleDiff(os.Args[2:])
			return
		case "collect":
			handleCollect(os.Args[2:])
			return
		case "repl":
			handleREPL(os.Args[2:])
			return
		}
	}

	// No subcommand and no flags: show help instead of silently entering collection
	if len(os.Args) == 1 {
		manual.PrintHelp()
		return
	}

	if hasHelpFlag() {
		manual.PrintHelp()
		return
	}
	flag.Usage = func() { manual.PrintHelp() }

	var dsnFlags []string
	flag.Func("dsn", "...", func(s string) error { dsnFlags = append(dsnFlags, s); return nil })
	configFile := flag.String("config", "", "JSON config file with array of DSNs")
	includeFilter := flag.String("include", "", "comma-separated kinds/labels/env-keys to include (e.g. mysql,redis or DB1,DB3)")
	labelFilter := flag.String("label", "", "filter by label (alias for -include)")
	excludeFilter := flag.String("exclude", "", "comma-separated kinds/labels/env-keys to exclude (e.g. mongodb,qdrant or DB5)")
	jsonOut := flag.Bool("json", false, "output JSON")
	humanOut := flag.Bool("human", false, "human-friendly output with context markers and visual separators")
	contextDir := flag.String("context", "", "write AI context files to directory (summary.json, topology.json, diagnostics.json, chunks/)")
	cacheFile := flag.String("cache", "", "fingerprint cache file for delta scan (.json)")
	versionLabel := flag.String("version-label", "", "label for this cache version (auto: v{timestamp})")
	outputFile := flag.String("o", "", "write output to file")
	logDirFlag := flag.String("log-dir", "/var/log/dbexplain", "directory for log files (filter.log, <label>.log)")
	perDSNTimeout := flag.Duration("timeout", 20*time.Second, "per-DSN collect timeout")
	maxConcurrent := flag.Int("conn", 10, "max concurrent connections for schema collection")
	showVersion := flag.Bool("version", false, "print version and exit")
	metricsFlag := flag.Bool("metrics", false, "output collection metrics in Prometheus text format (to stderr)")
	sample := flag.Bool("sample", false, "enable sample row fetching for comment inference (default: off)")
	skipOpstats := flag.Bool("skip-opstats", false, "skip MySQL performance_schema op stats")
	tableName := flag.String("table", "", "only collect the specified table schema (SQL data sources only)")
	tablesOnly := flag.Bool("tables", false, "compact table list mode (name, engine, row count)")
	flag.Parse()

	if *tableName != "" && *tablesOnly {
		log.Fatal("--table and --tables are mutually exclusive")
	}

	// --label is an alias for -include (schema collection also supports label filtering)
	if *labelFilter != "" {
		if *includeFilter != "" {
			*includeFilter += "," + *labelFilter
		} else {
			*includeFilter = *labelFilter
		}
	}

	if *showVersion {
		fmt.Println("dbexplain", version.Version)
		return
	}

	var entries []config.DSNEntry
	for _, raw := range dsnFlags {
		entries = append(entries, config.DSNEntry{Raw: raw})
	}

	hasExplicitSource := len(dsnFlags) > 0 || *configFile != ""
	if !hasExplicitSource {
		configPath := config.FindConfigFile()
		if configPath == "" {
			config.PrintNoConfigFound()
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			log.Printf("warning: load config %s: %v", configPath, config.SanitizeErr(err))
		} else {
			if len(envEntries) == 0 {
				config.PrintEmptyConfigFound(configPath)
				os.Exit(1)
			}
			entries = append(entries, envEntries...)
		}
	}
	if *configFile != "" {
		for _, raw := range config.LoadFromConfig(*configFile) {
			entries = append(entries, config.DSNEntry{Raw: raw})
		}
	}

	logDir := config.ResolveLogDir(*logDirFlag)

	// Redirect standard library log output to log file
	logFile, err := os.OpenFile(filepath.Join(logDir, "dbexplain.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// Filter DSNs
	entries = config.FilterDSNs(entries, *includeFilter, *excludeFilter, logDir)
	if len(entries) == 0 {
		log.Fatal("no DSNs provided (or all filtered out). Use -dsn or -config")
	}

	// Print DSN mapping summary
	if len(entries) > 0 && !*jsonOut {
		config.PrintDSNMapping(entries)
	}

	var dsns []string
	for _, e := range entries {
		dsns = append(dsns, e.Raw)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var instances []*schema.Instance
	var mu sync.Mutex
	var wg sync.WaitGroup

	startAll := time.Now()

	// All goroutines write to the shared dbexplain.log with [label=X] [kind=Y] prefix
	// log.Writer() returns the io.Writer that standard log output is configured to use,
	// which is dbexplain.log (set above via log.SetOutput(logFile)).
	// No per-label files or collect.log are created — consolidation goal achieved.

	metricsCollector := metrics.NewCollector()

	// Metrics channel — goroutines send metrics instead of calling metricsCollector.Record
	// (which blocks with sync.Mutex in goroutine scheduling contexts when lib/pq is hung).
	metricCh := make(chan metricSnapshot, len(dsns))

	// Semaphore to limit concurrent connections
	sem := make(chan struct{}, *maxConcurrent)

	for i, rawDSN := range dsns {
		p := collectParams{
			metricCh:      metricCh,
			mu:            &mu,
			instances:     &instances,
			sample:        *sample,
			skipOpstats:   *skipOpstats,
			tableFilter:   *tableName,
			perDSNTimeout: *perDSNTimeout,
			ctx:           ctx,
			wg:            &wg,
			sem:           sem,
		}
		wg.Add(1)
		sem <- struct{}{}
		go collectInstance(rawDSN, i, p)
	}

	wg.Wait()
	// Drain metrics channel into collector (goroutines send via channel to avoid mutex blocking).
	close(metricCh)
	for snap := range metricCh {
		metricsCollector.Record(snap.label, snap.kind, snap.success, snap.duration, snap.numDBs, snap.numTables, snap.errMsg)
	}
	if len(instances) == 0 {
		log.Printf("[collect-summary] 所有 DSN 采集均失败，报告为空。请检查日志: %s", logDir)
	} else {
		log.Printf("[collect-summary] 全部采集完成，总耗时 %v", time.Since(startAll))
	}

	kindCaps := buildKindCaps(instances)

	universe := &schema.Universe{Instances: instances}
	result := analyze.Analyze(universe, kindCaps)
	result.Metrics = metricsCollector.Snapshots()

	// Delta scan with fingerprint cache
	if *cacheFile != "" {
		store, err := cache.LoadStore(*cacheFile)
		if err != nil {
			log.Printf("load cache: %v (starting fresh)", err)
		}
		delta := store.Diff(universe)
		if len(delta.Added)+len(delta.Removed)+len(delta.Changed) > 0 {
			data, err := json.MarshalIndent(delta, "", "  ")
			if err != nil {
				log.Printf("[delta] marshal: %v", err)
			} else {
				fmt.Fprintf(os.Stderr, "[delta] %d added, %d removed, %d changed\n",
					len(delta.Added), len(delta.Removed), len(delta.Changed))
				deltaFile := strings.TrimSuffix(*cacheFile, ".json") + "_delta.json"
				if err := os.WriteFile(deltaFile, data, 0644); err != nil {
					log.Printf("[delta] write %s: %v", deltaFile, err)
				}
			}

			// Output field-level detailed diff
			detail := store.DiffDetailed(universe)
			if len(detail.Tables) > 0 {
				detailData, err := json.MarshalIndent(detail, "", "  ")
				if err != nil {
					log.Printf("[diff] marshal: %v", err)
				} else {
					diffFile := strings.TrimSuffix(*cacheFile, ".json") + "_diff.json"
					if err := os.WriteFile(diffFile, detailData, 0644); err != nil {
						log.Printf("[diff] write %s: %v", diffFile, err)
					}
					fmt.Fprintf(os.Stderr, "[diff] %d tables with field-level changes → %s\n",
						len(detail.Tables), diffFile)
				}
			}
		}
		if err := store.Update(universe); err != nil {
			log.Printf("save cache: %v", err)
		}
		// Save version snapshot if --version-label provided or auto-label
		vl := *versionLabel
		if vl == "" {
			vl = "v" + time.Now().Format("20060102_150405")
		}
		if err := store.SaveVersion(vl); err != nil {
			log.Printf("save version %s: %v", vl, err)
		}
	}

	// Write AI context files
	if *contextDir != "" {
		output.WriteContext(*contextDir, result, *tableName)
	}

	// Output
	if *outputFile != "" {
		// File output: capture and write (no ANSI escape codes)
		var out string
		if *jsonOut {
			out = output.CaptureJSON(result)
			// JSON without BOM, standard compliant
			if err := os.WriteFile(*outputFile, []byte(out), 0644); err != nil {
				log.Fatal(err)
			}
		} else {
			out = output.CaptureText(result, *humanOut, *tablesOnly)
			data, err := encodeOutput(out)
			if err != nil {
				log.Fatalf("encode output: %v", err)
			}
			if err := os.WriteFile(*outputFile, data, 0644); err != nil {
				log.Fatal(err)
			}
		}
		fmt.Fprintln(os.Stderr, "Report written to", *outputFile)
		if *metricsFlag {
			fmt.Fprint(os.Stderr, metricsCollector.PrometheusText())
		}
	} else if *jsonOut {
		render.PrintJSON(result)
		if *metricsFlag {
			fmt.Fprint(os.Stderr, metricsCollector.PrometheusText())
		}
	} else {
		render.Print(result, *humanOut, *tablesOnly)
		if *metricsFlag {
			fmt.Fprint(os.Stderr, metricsCollector.PrometheusText())
		}
	}
}

// totalTables counts total tables across all databases in an instance.
func totalTables(inst *schema.Instance) int {
	total := 0
	for _, db := range inst.Databases {
		total += len(db.Tables)
	}
	return total
}

// handleDiff implements the "dbexplain diff" CLI subcommand.
// Usage:
//
//	dbexplain diff --cache FILE --current FILE [--human]
//	dbexplain diff --cache FILE --since VERSION [--human]
//	dbexplain diff --cache FILE --list-versions
//	dbexplain diff --before FILE --after FILE
func handleDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	cacheFile := fs.String("cache", "", "fingerprint cache file (.json)")
	currentFile := fs.String("current", "", "current scan JSON output file")
	beforeFile := fs.String("before", "", "previous scan JSON file")
	afterFile := fs.String("after", "", "later scan JSON file")
	sinceVersion := fs.String("since", "", "compare current cache state against this version label")
	listVersions := fs.Bool("list-versions", false, "list all stored version labels")
	humanOut := fs.Bool("human", false, "human-friendly output")
	fs.Parse(args)

	// Mode 0: list versions
	if *listVersions {
		if *cacheFile == "" {
			log.Fatal("--list-versions requires --cache")
		}
		store, err := cache.LoadStore(*cacheFile)
		if err != nil {
			log.Fatalf("load cache: %v", err)
		}
		versions := store.ListVersions()
		if len(versions) == 0 {
			fmt.Println("No versions stored.")
			return
		}
		fmt.Println("Stored versions:")
		for _, v := range versions {
			fmt.Printf("  %s\n", v)
		}
		return
	}

	// Mode 1: cache + current
	if *cacheFile != "" && *currentFile != "" {
		store, err := cache.LoadStore(*cacheFile)
		if err != nil {
			log.Fatalf("load cache: %v", err)
		}

		currentData, err := os.ReadFile(*currentFile)
		if err != nil {
			log.Fatalf("read current file: %v", err)
		}

		universe, err := universeFromFile(currentData)
		if err != nil {
			log.Fatalf("reconstruct universe: %v", err)
		}

		detail := store.DiffDetailed(universe)
		if *humanOut {
			renderDiffHuman(detail)
		} else {
			data, err := json.MarshalIndent(detail, "", "  ")
			if err != nil {
				log.Fatalf("marshal diff detail: %v", err)
			}
			fmt.Println(string(data))
		}
		return
	}

	// Mode 2: cache + since (compare current against stored version)
	if *cacheFile != "" && *sinceVersion != "" {
		store, err := cache.LoadStore(*cacheFile)
		if err != nil {
			log.Fatalf("load cache: %v", err)
		}
		// Reconstruct universe from current store state
		detail, err := store.DiffSince(*sinceVersion,
			diff.NewInstanceLabelFunc(), diff.NewDBNameFunc())
		if err != nil {
			log.Fatalf("diff since %s: %v", *sinceVersion, err)
		}
		if *humanOut {
			renderDiffHuman(detail)
		} else {
			data, err := json.MarshalIndent(detail, "", "  ")
			if err != nil {
				log.Fatalf("marshal diff since: %v", err)
			}
			fmt.Println(string(data))
		}
		return
	}

	// Mode 3: before + after
	if *beforeFile != "" && *afterFile != "" {
		beforeUniv, err := universeFromFile(readFileBytes(*beforeFile))
		if err != nil {
			log.Fatalf("parse before: %v", err)
		}
		afterUniv, err := universeFromFile(readFileBytes(*afterFile))
		if err != nil {
			log.Fatalf("parse after: %v", err)
		}

		oldSnaps := buildSnapshotMap(beforeUniv)
		newSnaps := buildSnapshotMap(afterUniv)
		oldKeys := make(map[string]bool)
		newKeys := make(map[string]bool)
		for k := range oldSnaps {
			oldKeys[k] = true
		}
		for k := range newSnaps {
			newKeys[k] = true
		}

		detail := diff.DiffUniverse(
			oldKeys, newKeys, oldSnaps, newSnaps,
			diff.NewInstanceLabelFunc(),
			diff.NewDBNameFunc(),
		)
		if *humanOut {
			renderDiffHuman(detail)
		} else {
			data, err := json.MarshalIndent(detail, "", "  ")
			if err != nil {
				log.Fatalf("marshal diff before/after: %v", err)
			}
			fmt.Println(string(data))
		}
		return
	}

	log.Fatal("usage: dbexplain diff --cache FILE --current FILE [--human]\n" +
		"       dbexplain diff --cache FILE --since VERSION [--human]\n" +
		"       dbexplain diff --cache FILE --list-versions\n" +
		"       dbexplain diff --before FILE --after FILE")
}

// handleCollect implements "dbexplain collect" — explicit schema collection subcommand.
func handleCollect(args []string) {
	fs := flag.NewFlagSet("collect", flag.ExitOnError)
	var dsnFlags []string
	fs.Func("dsn", "...", func(s string) error { dsnFlags = append(dsnFlags, s); return nil })
	configFile := fs.String("config", "", "JSON config file with array of DSNs")
	includeFilter := fs.String("include", "", "comma-separated kinds/labels/env-keys to include")
	labelFilter := fs.String("label", "", "filter by label (alias for -include)")
	excludeFilter := fs.String("exclude", "", "comma-separated kinds/labels/env-keys to exclude")
	jsonOut := fs.Bool("json", false, "output JSON")
	humanOut := fs.Bool("human", false, "human-friendly output")
	contextDir := fs.String("context", "", "write AI context files to directory")
	cacheFile := fs.String("cache", "", "fingerprint cache file for delta scan (.json)")
	versionLabel := fs.String("version-label", "", "label for this cache version")
	outputFile := fs.String("o", "", "write output to file")
	logDirFlag := fs.String("log-dir", "/var/log/dbexplain", "directory for log files")
	perDSNTimeout := fs.Duration("timeout", 20*time.Second, "per-DSN collect timeout")
	maxConcurrent := fs.Int("conn", 10, "max concurrent connections for schema collection")
	metricsFlag := fs.Bool("metrics", false, "output collection metrics in Prometheus text format (to stderr)")
	sample := fs.Bool("sample", false, "enable sample row fetching for comment inference (default: off)")
	skipOpstats := fs.Bool("skip-opstats", false, "skip MySQL performance_schema op stats")
	hcTableName := fs.String("table", "", "only collect the specified table schema (SQL data sources only)")
	hcTablesOnly := fs.Bool("tables", false, "compact table list mode (name, engine, row count)")
	fs.Parse(args)

	if *hcTableName != "" && *hcTablesOnly {
		log.Fatal("--table and --tables are mutually exclusive")
	}

	// --label is an alias for -include
	if *labelFilter != "" {
		if *includeFilter != "" {
			*includeFilter += "," + *labelFilter
		} else {
			*includeFilter = *labelFilter
		}
	}

	var entries []config.DSNEntry
	for _, raw := range dsnFlags {
		entries = append(entries, config.DSNEntry{Raw: raw})
	}

	hasExplicitSource := len(dsnFlags) > 0 || *configFile != ""
	if !hasExplicitSource {
		configPath := config.FindConfigFile()
		if configPath == "" {
			config.PrintNoConfigFound()
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			log.Printf("warning: load config %s: %v", configPath, config.SanitizeErr(err))
		} else {
			if len(envEntries) == 0 {
				config.PrintEmptyConfigFound(configPath)
				os.Exit(1)
			}
			entries = append(entries, envEntries...)
		}
	}
	if *configFile != "" {
		for _, raw := range config.LoadFromConfig(*configFile) {
			entries = append(entries, config.DSNEntry{Raw: raw})
		}
	}

	logDir := config.ResolveLogDir(*logDirFlag)

	logFile, err := os.OpenFile(filepath.Join(logDir, "dbexplain.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	entries = config.FilterDSNs(entries, *includeFilter, *excludeFilter, logDir)
	if len(entries) == 0 {
		log.Fatal("no DSNs provided (or all filtered out). Use -dsn or -config")
	}

	if len(entries) > 0 && !*jsonOut {
		config.PrintDSNMapping(entries)
	}

	var dsns []string
	for _, e := range entries {
		dsns = append(dsns, e.Raw)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var instances []*schema.Instance
	var mu sync.Mutex
	var wg sync.WaitGroup

	startAll := time.Now()

	// All goroutine logs go to the shared dbexplain.log with [label=X] [kind=Y] prefix.
	// No per-label files or collect.log are created.

	metricsCollector := metrics.NewCollector()

	// Metrics channel — goroutines send metrics instead of calling metricsCollector.Record
	// (which blocks with sync.Mutex on Go 1.26 in certain goroutine scheduling contexts).
	metricCh := make(chan metricSnapshot, len(dsns))

	sem := make(chan struct{}, *maxConcurrent)

	for i, rawDSN := range dsns {
		p := collectParams{
			metricCh:      metricCh,
			mu:            &mu,
			instances:     &instances,
			sample:        *sample,
			skipOpstats:   *skipOpstats,
			tableFilter:   *hcTableName,
			perDSNTimeout: *perDSNTimeout,
			ctx:           ctx,
			wg:            &wg,
			sem:           sem,
		}
		wg.Add(1)
		sem <- struct{}{}
		go collectInstance(rawDSN, i, p)
	}

	wg.Wait()
	// Drain metrics channel into collector (goroutines send via channel to avoid mutex blocking).
	close(metricCh)
	for snap := range metricCh {
		metricsCollector.Record(snap.label, snap.kind, snap.success, snap.duration, snap.numDBs, snap.numTables, snap.errMsg)
	}
	if len(instances) == 0 {
		log.Printf("[collect-summary] 所有 DSN 采集均失败，报告为空。请检查日志: %s", logDir)
	} else {
		log.Printf("[collect-summary] 全部采集完成，总耗时 %v", time.Since(startAll))
	}

	kindCaps := buildKindCaps(instances)

	universe := &schema.Universe{Instances: instances}
	result := analyze.Analyze(universe, kindCaps)
	result.Metrics = metricsCollector.Snapshots()

	if *cacheFile != "" {
		store, err := cache.LoadStore(*cacheFile)
		if err != nil {
			log.Printf("load cache: %v (starting fresh)", err)
		}
		delta := store.Diff(universe)
		if len(delta.Added)+len(delta.Removed)+len(delta.Changed) > 0 {
			data, err := json.MarshalIndent(delta, "", "  ")
			if err != nil {
				log.Printf("[delta] marshal: %v", err)
			}
			fmt.Fprintf(os.Stderr, "[delta] %d added, %d removed, %d changed\n",
				len(delta.Added), len(delta.Removed), len(delta.Changed))
			deltaFile := strings.TrimSuffix(*cacheFile, ".json") + "_delta.json"
			if err := os.WriteFile(deltaFile, data, 0644); err != nil {
				log.Printf("[delta] write %s: %v", deltaFile, err)
			}

			detail := store.DiffDetailed(universe)
			if len(detail.Tables) > 0 {
				detailData, err := json.MarshalIndent(detail, "", "  ")
				if err != nil {
					log.Printf("[diff] marshal: %v", err)
				}
				diffFile := strings.TrimSuffix(*cacheFile, ".json") + "_diff.json"
				if err := os.WriteFile(diffFile, detailData, 0644); err != nil {
					log.Printf("[diff] write %s: %v", diffFile, err)
				}
				fmt.Fprintf(os.Stderr, "[diff] %d tables with field-level changes → %s\n",
					len(detail.Tables), diffFile)
			}
		}
		if err := store.Update(universe); err != nil {
			log.Printf("save cache: %v", err)
		}
		vl := *versionLabel
		if vl == "" {
			vl = "v" + time.Now().Format("20060102_150405")
		}
		if err := store.SaveVersion(vl); err != nil {
			log.Printf("save version %s: %v", vl, err)
		}
	}

	if *contextDir != "" {
		output.WriteContext(*contextDir, result, *hcTableName)
	}

	if *outputFile != "" {
		var out string
		if *jsonOut {
			out = output.CaptureJSON(result)
			if err := os.WriteFile(*outputFile, []byte(out), 0644); err != nil {
				log.Fatal(err)
			}
		} else {
			out = output.CaptureText(result, *humanOut, *hcTablesOnly)
			data, err := encodeOutput(out)
			if err != nil {
				log.Fatalf("encode output: %v", err)
			}
			if err := os.WriteFile(*outputFile, data, 0644); err != nil {
				log.Fatal(err)
			}
		}
		fmt.Fprintln(os.Stderr, "Report written to", *outputFile)
		if *metricsFlag {
			fmt.Fprint(os.Stderr, metricsCollector.PrometheusText())
		}
	} else if *jsonOut {
		render.PrintJSON(result)
		if *metricsFlag {
			fmt.Fprint(os.Stderr, metricsCollector.PrometheusText())
		}
	} else {
		render.Print(result, *humanOut, *hcTablesOnly)
		if *metricsFlag {
			fmt.Fprint(os.Stderr, metricsCollector.PrometheusText())
		}
	}
}

// universeFromFile reconstructs a Universe from a JSON file that may be
// either an analyze.AnalysisResult (which contains instances) or a raw schema.Universe.
func universeFromFile(data []byte) (*schema.Universe, error) {
	// Try AnalysisResult first (standard --json -o output)
	var result struct {
		Instances []schema.Instance `json:"instances"`
	}
	if err := json.Unmarshal(data, &result); err == nil && len(result.Instances) > 0 {
		instances := make([]*schema.Instance, len(result.Instances))
		for i := range result.Instances {
			instances[i] = &result.Instances[i]
		}
		return &schema.Universe{Instances: instances}, nil
	}

	// Try raw Universe
	var u schema.Universe
	if err := json.Unmarshal(data, &u); err == nil && len(u.Instances) > 0 {
		return &u, nil
	}

	return nil, fmt.Errorf("no instances found in file")
}

// buildSnapshotMap builds a keyed snapshot map from universe.
func buildSnapshotMap(u *schema.Universe) map[string]*schema.Table {
	snaps := make(map[string]*schema.Table)
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				key := inst.Label + "/" + db.Name + "/" + t.Name
				snaps[key] = t
			}
		}
	}
	return snaps
}

// readFileBytes reads a file and returns its bytes, exiting on error.
func readFileBytes(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read file %s: %v", path, err)
	}
	return data
}

// renderDiffHuman prints a human-readable table of schema changes.
func renderDiffHuman(result diff.DiffResult) {
	if len(result.Tables) == 0 {
		fmt.Println("No schema changes detected.")
		return
	}

	fmt.Printf("Schema Diff Report — %d table(s) changed\n", len(result.Tables))
	fmt.Println(strings.Repeat("=", 60))

	for _, td := range result.Tables {
		fmt.Printf("\n[%s] %s.%s (%s)\n", td.Status, td.DB, td.Table, td.Instance)
		if len(td.Columns) > 0 {
			fmt.Printf("  Columns (%d):\n", len(td.Columns))
			for _, c := range td.Columns {
				oldPart := ""
				newPart := ""
				if c.OldVal != "" {
					oldPart = " → " + c.OldVal
				}
				if c.NewVal != "" {
					newPart = " → " + c.NewVal
				}
				fmt.Printf("    - %s [%s]%s%s\n", c.Name, c.Field, oldPart, newPart)
			}
		}
		if len(td.Indexes) > 0 {
			fmt.Printf("  Indexes (%d):\n", len(td.Indexes))
			for _, idx := range td.Indexes {
				fmt.Printf("    - %s (%s)\n", idx.Name, idx.Status)
			}
		}
		if len(td.FKs) > 0 {
			fmt.Printf("  Foreign Keys (%d):\n", len(td.FKs))
			for _, fk := range td.FKs {
				fmt.Printf("    - %s (%s)\n", fk.Name, fk.Status)
			}
		}
	}
}
