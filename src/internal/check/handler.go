// Package check handles the "dbexplain check" subcommand for validating
// configuration files and testing database connectivity.
package check

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/dsn"
)

// checkResult is the JSON-serializable structure for the check subcommand.
type checkResult struct {
	EnvKey    string `json:"envKey,omitempty"`
	Label     string `json:"label,omitempty"`
	Kind      string `json:"kind,omitempty"`
	HostPort  string `json:"hostPort,omitempty"`
	SyntaxOK  bool   `json:"syntaxOK"`
	SyntaxErr string `json:"syntaxErr,omitempty"`
	ConnOK    bool   `json:"connOK"`
	ConnMsg   string `json:"connMsg,omitempty"`
	Latency   string `json:"latency,omitempty"`
}

// checkSummary is the top-level JSON output structure.
type checkSummary struct {
	Total     int            `json:"total"`
	Connected int            `json:"connected"`
	Failed    int            `json:"failed"`
	Invalid   int            `json:"invalid"`
	Results   []checkResult  `json:"results"`
}

// Handle processes the check subcommand.
func Handle(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	var dsnFlags []string
	fs.Func("dsn", "Direct DSN string (repeatable)", func(s string) error {
		dsnFlags = append(dsnFlags, s)
		return nil
	})
	configFile := fs.String("config", "", "JSON config file path")
	timeout := fs.Duration("timeout", 20*time.Second, "Per-DSN connection timeout")
	sample := fs.Bool("sample", false, "enable sample row fetching for comment inference (default: off)")
	labelFilter := fs.String("label", "", "filter by label")
	jsonOut := fs.Bool("json", false, "output JSON")
	logDirFlag := fs.String("log-dir", "/var/log/dbexplain", "directory for log files")
	fs.Parse(args)

	var entries []config.DSNEntry
	configPath := ""

	// Auto-load from .env if no explicit source given
	hasExplicitSource := len(dsnFlags) > 0 || *configFile != ""
	if !hasExplicitSource {
		configPath = config.FindConfigFile()
		if configPath == "" {
			fmt.Fprintf(os.Stderr, "ERROR: no config file found.\n")
			fmt.Fprintf(os.Stderr, "  Create .env.dbexplain in %s or current directory.\n", config.ConfigDirDisplay())
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, config.SanitizeErr(err))
			os.Exit(1)
		}
		entries = append(entries, envEntries...)
	}

	// Load from --config
	if *configFile != "" {
		for _, raw := range config.LoadFromConfig(*configFile) {
			entries = append(entries, config.DSNEntry{Raw: raw})
		}
	}

	// Load from --dsn
	for _, raw := range dsnFlags {
		entries = append(entries, config.DSNEntry{Raw: raw})
	}

	// Filter entries by label if --label is set
	if *labelFilter != "" {
		filtered := make([]config.DSNEntry, 0, len(entries))
		for _, e := range entries {
			parsed, err := dsn.ParseDSN(e.Raw)
			if err != nil {
				continue // skip invalid DSNs — will be reported later
			}
			if parsed.Label == *labelFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no DSNs found. Use --dsn or --config.")
		os.Exit(1)
	}

	// ── Open dbexplain.log for check progress logging ──
	logDir := config.ResolveLogDir(*logDirFlag)
	logFile, logErr := os.OpenFile(filepath.Join(logDir, "dbexplain.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if logErr == nil {
		defer logFile.Close()
	}

	// ── Check each DSN ──
	results := make([]checkResult, 0, len(entries))
	var syntaxFails, connFails, connOK int

	// Pre-parse all DSNs synchronously; separate invalid entries.
	type connTask struct {
		entry  config.DSNEntry
		parsed *dsn.DSN
		index  int // 0-based
	}
	var tasks []connTask
	for i, e := range entries {
		parsed, err := dsn.ParseDSN(e.Raw)
		if err != nil {
			r := checkResult{EnvKey: e.EnvKey, SyntaxOK: false, SyntaxErr: config.SanitizeErr(err).Error()}
			syntaxFails++
			results = append(results, r)
			continue
		}
		r := checkResult{
			EnvKey:  e.EnvKey,
			SyntaxOK: true,
			Kind:    parsed.Kind,
			Label:   parsed.Label,
			HostPort: parsed.Host,
		}
		if r.Label == "" {
			r.Label = "(no label)"
		}
		if parsed.Port != "" {
			r.HostPort += ":" + parsed.Port
		}
		results = append(results, r)
		tasks = append(tasks, connTask{entry: e, parsed: parsed, index: i})
	}

	// ── Table header (non-JSON mode) ──
	if !*jsonOut {
		fmt.Println()
		fmt.Printf("  Config file: %s\n", config.DescribeConfigSource(configPath))
		fmt.Printf("  DSN count:   %d\n\n", len(entries))
		fmt.Println("  No. EnvKey Label              Kind       Host:Port             Syntax  Connect")
		fmt.Println("  " + strings.Repeat("─", 95))

		// Print syntax-invalid rows immediately
		for i, r := range results {
			if r.SyntaxOK {
				continue
			}
			idx := fmt.Sprintf("%-4d", i+1)
			key := r.EnvKey
			if key == "" {
				key = "—"
			}
			errMsg := truncateErr(r.SyntaxErr, 55)
			fmt.Printf("  %s %-6s %-18s %-10s %-20s %-7s ❌ %s\n",
				idx, key, "", "", "", "❌ FAIL", errMsg)
		}
	}

	// ── Concurrent connection checks ──
	resultCh := make(chan struct {
		index int
		r     checkResult
	}, len(tasks))
	var wg sync.WaitGroup

	for _, t := range tasks {
		wg.Add(1)
		go func(t connTask) {
			defer wg.Done()
			r := results[t.index] // partial result from pre-parse (missing conn fields)

			// Log check progress to dbexplain.log
			var checkLogger *log.Logger
			if logErr == nil {
				checkLogger = log.New(logFile, fmt.Sprintf("[label=%s] [kind=%s] [check] ", r.Label, r.Kind), log.LstdFlags)
				checkLogger.Printf("(#%d) connecting ...", t.index+1)
			}

			// Test connectivity via connector.Collect with timeout
			collectCtx := connector.WithLogger(context.Background(),
				log.New(io.Discard, "", 0))
			if *sample {
				collectCtx = connector.WithSample(collectCtx)
			}
			ctx, cancel := context.WithTimeout(collectCtx, *timeout)
			oldLogOut := log.Writer()
			log.SetOutput(io.Discard)
			start := time.Now()

			// Run collection in sub-goroutine with timeout guard.
			type chkResult struct {
				err error
			}
			ch := make(chan chkResult, 1)
			go func() {
				_, subErr := connector.Collect(ctx, t.entry.Raw)
				ch <- chkResult{subErr}
			}()
			var connErr error // actual error from collector (nil=connected)
			timeTimer := time.NewTimer(*timeout)
			select {
			case res := <-ch:
				r.ConnOK = res.err == nil
				connErr = res.err
				timeTimer.Stop()
			case <-timeTimer.C:
				r.ConnOK = false
				connErr = context.DeadlineExceeded
			}
			cancel()
			log.SetOutput(oldLogOut)
			elapsed := time.Since(start)
			r.Latency = fmt.Sprintf("%dms", elapsed.Milliseconds())

			if r.ConnOK {
				if checkLogger != nil {
					checkLogger.Printf("(#%d) OK (%s)", t.index+1, r.Latency)
				}
			} else {
				err := connErr
				if err == nil {
					err = context.DeadlineExceeded
				}
				r.ConnMsg = config.SanitizeErr(err).Error()
				if checkLogger != nil {
					checkLogger.Printf("(#%d) FAIL after %s: %s", t.index+1, r.Latency, r.ConnMsg)
				}
			}

			resultCh <- struct {
				index int
				r     checkResult
			}{t.index, r}
		}(t)
	}

	// Close resultCh when all goroutines finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// ── Collect and stream results ──
	if *jsonOut {
		// JSON mode: collect all then output at once
		for res := range resultCh {
			results[res.index] = res.r
			if res.r.ConnOK {
				connOK++
			} else {
				connFails++
			}
		}
		summary := checkSummary{
			Total:     len(entries),
			Connected: connOK,
			Failed:    connFails,
			Invalid:   syntaxFails,
			Results:   results,
		}
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		if syntaxFails > 0 || connFails > 0 {
			os.Exit(1)
		}
		return
	}

	// Table mode: stream each result as it arrives
	for res := range resultCh {
		results[res.index] = res.r
		i := res.index
		r := res.r

		if r.ConnOK {
			connOK++
		} else {
			connFails++
		}

		idx := fmt.Sprintf("%-4d", i+1)
		key := r.EnvKey
		if key == "" {
			key = "—"
		}
		label := r.Label
		if len(label) > 18 {
			label = label[:15] + "..."
		}
		syntaxCol := "✅ OK"

		var connCol string
		if r.ConnOK {
			connCol = fmt.Sprintf("✅ OK %s", r.Latency)
		} else if strings.Contains(r.ConnMsg, "context deadline exceeded") || strings.Contains(r.ConnMsg, "deadline") {
			connCol = fmt.Sprintf("⏱ timeout (%s)", r.Latency)
		} else {
			connCol = fmt.Sprintf("❌ FAIL %s", r.Latency)
		}

		fmt.Printf("  %s %-6s %-18s %-10s %-20s %-7s %s\n",
			idx, key, label, r.Kind, r.HostPort, syntaxCol, connCol)

		if !r.ConnOK && !strings.Contains(r.ConnMsg, "deadline") {
			errLine := truncateErr(r.ConnMsg, 100)
			fmt.Printf("  %s%s\n", strings.Repeat(" ", 75), errLine)
		}
	}

	// ── Summary ──
	fmt.Println()
	fmt.Printf("  Summary: %d total — %d connected, %d connection failed, %d invalid syntax\n",
		len(entries), connOK, connFails, syntaxFails)
	fmt.Println()

	if syntaxFails > 0 || connFails > 0 {
		os.Exit(1)
	}
}

// truncateErr shortens long error messages for table display.
func truncateErr(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}
