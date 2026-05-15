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
    "sync"
    "time"

    "github.com/joho/godotenv"
    "golang.org/x/sync/errgroup"

    "dbexplain/analyze"
    "dbexplain/connector"
    "dbexplain/dsn"          // 新增：用于解析 DSN
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

    // 创建日志根目录
    logDir := "./logs"
    if err := os.MkdirAll(logDir, 0755); err != nil {
        log.Fatalf("create log dir: %v", err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var instances []*schema.Instance
    mu := sync.Mutex{}
    g, _ := errgroup.WithContext(ctx)

    for i, ds := range dsns {
        i := i
        ds := ds
        g.Go(func() error {
            // 为每个 DSN 创建独立的日志文件
            label := fmt.Sprintf("db_%d", i) // 默认文件名
            // 尝试解析 DSN 获取更友好的 label
            if parsed, err := dsn.ParseDSN(ds); err == nil && parsed.Label != "" {
                label = parsed.Label
            } else if parsed != nil {
                label = fmt.Sprintf("%s-%s", parsed.Kind, parsed.Label)
            }
            logFileName := filepath.Join(logDir, label+".log")
            logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
            if err != nil {
                log.Printf("failed to create log file %s: %v, using default stderr", logFileName, err)
                logFile = os.Stderr // 回退到 stderr
            } else {
                defer logFile.Close()
            }

            logger := log.New(logFile, "", log.LstdFlags) // 不带前缀，保留时间戳
            // 将 logger 注入 context
            collectCtx := connector.WithLogger(context.WithValue(ctx, struct{}{}, nil), logger)
            // 为本次采集设置超时
            collectCtx, cancel := context.WithTimeout(collectCtx, *perDSNTimeout)
            defer cancel()

            inst, err := connector.Collect(collectCtx, ds)
            if err != nil {
                logger.Printf("skip %s: %v", ds, err)
                return nil
            }
            mu.Lock()
            instances = append(instances, inst)
            mu.Unlock()
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        log.Printf("collect error: %v", err) // 修正：使用 log.Printf
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

func loadFromEnv() []string {
	var dsns []string
	for i := 1; ; i++ {
		key := fmt.Sprintf("DB%d", i)
		val := os.Getenv(key)
		if val == "" {
			break
		}
		dsns = append(dsns, val)
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

// captureText / captureJSON redirect render output to string
func captureText(result *analyze.Result) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	render.Print(result)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureJSON(result *analyze.Result) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	render.PrintJSON(result)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}