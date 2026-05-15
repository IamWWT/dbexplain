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

func main() {
	_ = godotenv.Load()

	var dsnFlags []string
	flag.Func("dsn", "...", func(s string) error { dsnFlags = append(dsnFlags, s); return nil })
	configFile := flag.String("config", "", "JSON config file with array of DSNs")
	useEnv := flag.Bool("env", false, "use .env file (prefix DB1=, DB2=...)")
	jsonOut := flag.Bool("json", false, "output JSON")
	outputFile := flag.String("o", "", "write output to file")
	perDSNTimeout := flag.Duration("timeout", 20*time.Second, "per-DSN collect timeout")
	flag.Parse()

	dsns := dsnFlags
	if *useEnv {
		dsns = append(dsns, loadFromEnv()...)
	}
	if *configFile != "" {
		dsns = append(dsns, loadFromConfig(*configFile)...)
	}
	if len(dsns) == 0 {
		log.Fatal("no DSNs provided. Use -dsn, -env, or -config")
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
				log.Printf("invalid DSN %s: %v", rawDSN, err)
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
				logger.Printf("skip %s: %v", rawDSN, err)
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
	fmt.Fprintf(os.Stderr, "全部采集完成，总耗时 %v\n", time.Since(startAll))

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

func loadFromEnv() []string {
	var dsns []string
	type entry struct {
		idx int
		val string
	}
	var entries []entry

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
		entries = append(entries, entry{idx, val})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].idx < entries[j].idx
	})

	for _, e := range entries {
		dsns = append(dsns, e.val)
	}
	return dsns
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