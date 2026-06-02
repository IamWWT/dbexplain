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
	"strings"
	"sync"
	"time"

	"github.com/IamWWT/dbexplain/internal/analyze"
	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/cache"
	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/diff"
	"github.com/IamWWT/dbexplain/internal/encrypt"
	"github.com/IamWWT/dbexplain/internal/list"
	"github.com/IamWWT/dbexplain/internal/manual"
	"github.com/IamWWT/dbexplain/internal/output"
	"github.com/IamWWT/dbexplain/internal/version"
	"github.com/IamWWT/dbexplain/internal/render"
	"github.com/IamWWT/dbexplain/internal/schema"
)

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
	// Intercept subcommands BEFORE flag.Parse
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "encrypt":
			encrypt.Handle(os.Args[2:])
			return
		case "mysql", "postgres", "postgresql", "pg", "gaussdb",
			"clickhouse", "ch", "sqlite", "sqlite3",
			"redis", "mongodb", "elasticsearch", "es", "qdrant",
			"csv", "tsv", "xlsx":
			manual.PrintDBManual(os.Args[1], os.Args[2:])
			return
		case "all":
			manual.HandleAllManual(os.Args[2:])
			return
		case "execute":
			handleExecute(os.Args[2:])
			return
		case "list":
			list.Handle(os.Args[2:])
			return
		case "diff":
			handleDiff(os.Args[2:])
			return
		}
	}

	userLang := preScanLanguage()

	// Intercept -h/--help before flag.Parse for localized output
	if hasHelpFlag() {
		manual.PrintHelp()
		return
	}
	flag.Usage = func() { manual.PrintHelp() }

	var dsnFlags []string
	flag.Func("dsn", "...", func(s string) error { dsnFlags = append(dsnFlags, s); return nil })
	configFile := flag.String("config", "", "JSON config file with array of DSNs")
	useEnv := flag.Bool("env", false, "use .env file (prefix DB1=, DB2=...)")
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
	showManual := flag.Bool("manual", false, "print comprehensive manual and exit")
	language := flag.String("language", userLang, "manual language: zh (Chinese) or en (English)")
	filterFlag := flag.String("filter", "", "filter --manual output by keyword (case-insensitive)")
	flag.Parse()

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
	if *showManual {
		fmt.Fprintln(os.Stderr, "Note: --manual is deprecated, use: dbexplain all")
		manual.PrintManual(*language, *filterFlag)
		return
	}

	var entries []config.DSNEntry
	for _, raw := range dsnFlags {
		entries = append(entries, config.DSNEntry{Raw: raw})
	}
	if *useEnv {
		configPath := config.FindConfigFile()
		if configPath == "" {
			log.Fatal("no config file found. Create .env.dbexplain (or .env.dbexplain.enc) in " + config.ConfigDirDisplay() + " or current directory.")
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			log.Printf("warning: load config %s: %v", configPath, config.SanitizeErr(err))
		} else {
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
		log.Fatal("no DSNs provided (or all filtered out). Use -dsn, -env, or -config")
	}

	// Print DSN mapping summary when loaded from .env
	if *useEnv && !*jsonOut {
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

	// Collection summary log
	collectLogFile, err := os.OpenFile(filepath.Join(logDir, "collect.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("create collect log: %v", err)
		collectLogFile = os.Stderr
	} else {
		defer collectLogFile.Close()
	}
	collectLogger := log.New(collectLogFile, "", log.LstdFlags)

	// Semaphore to limit concurrent connections
	sem := make(chan struct{}, *maxConcurrent)

	for i, rawDSN := range dsns {
		i := i
		rawDSN := rawDSN
		wg.Add(1)
		sem <- struct{}{} // acquire (blocks if at capacity)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release
			parsed, err := dsn.ParseDSN(rawDSN)
			if err != nil {
				log.Printf("invalid DSN: %v", config.SanitizeErr(err))
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

			logger.Printf("[采集中] %s", label)
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
			logger.Printf("[完成] %s (%d 表) 耗时 %v", label, nTables, elapsed)
		}()
	}

	wg.Wait()
	if len(instances) == 0 {
		collectLogger.Printf("[!] 所有 DSN 采集均失败，报告为空。请检查日志: %s", logDir)
	} else {
		collectLogger.Printf("全部采集完成，总耗时 %v", time.Since(startAll))
	}

	// Build capability map by database kind
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

	// Delta scan with fingerprint cache
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

			// Output field-level detailed diff
			detail := store.DiffDetailed(universe)
			if len(detail.Tables) > 0 {
				detailData, _ := json.MarshalIndent(detail, "", "  ")
				diffFile := strings.TrimSuffix(*cacheFile, ".json") + "_diff.json"
				os.WriteFile(diffFile, detailData, 0644)
				fmt.Fprintf(os.Stderr, "[diff] %d tables with field-level changes → %s\n",
					len(detail.Tables), diffFile)
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
		output.WriteContext(*contextDir, result)
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
			out = output.CaptureText(result, *humanOut)
			data, err := encodeOutput(out)
			if err != nil {
				log.Fatalf("encode output: %v", err)
			}
			if err := os.WriteFile(*outputFile, data, 0644); err != nil {
				log.Fatal(err)
			}
		}
		fmt.Fprintln(os.Stderr, "Report written to", *outputFile)
	} else if *jsonOut {
		// Terminal JSON: direct output
		render.PrintJSON(result)
	} else {
		// Terminal text: direct render (with color highlights)
		render.Print(result, *humanOut)
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
			data, _ := json.MarshalIndent(detail, "", "  ")
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
			data, _ := json.MarshalIndent(detail, "", "  ")
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
			data, _ := json.MarshalIndent(detail, "", "  ")
			fmt.Println(string(data))
		}
		return
	}

	log.Fatal("usage: dbexplain diff --cache FILE --current FILE [--human]\n" +
		"       dbexplain diff --cache FILE --since VERSION [--human]\n" +
		"       dbexplain diff --cache FILE --list-versions\n" +
		"       dbexplain diff --before FILE --after FILE")
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
