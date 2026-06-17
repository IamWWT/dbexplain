// Package check handles the "dbexplain check" subcommand for validating
// configuration files and testing database connectivity.
package check

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/dsn"
)

// Handle processes the check subcommand.
func Handle(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	var dsnFlags []string
	fs.Func("dsn", "Direct DSN string (repeatable)", func(s string) error {
		dsnFlags = append(dsnFlags, s)
		return nil
	})
	configFile := fs.String("config", "", "JSON config file path")
	timeout := fs.Duration("timeout", 10*time.Second, "Per-DSN connection timeout")
	sample := fs.Bool("sample", false, "enable sample row fetching for comment inference (default: off)")
	labelFilter := fs.String("label", "", "filter by label")
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

	// ── Header ──
	fmt.Println()
	fmt.Printf("  Config file: %s\n", config.DescribeConfigSource(configPath))
	fmt.Printf("  DSN count:   %d\n\n", len(entries))

	// ── Open dbexplain.log for check progress logging ──
	logDir := config.ResolveLogDir(*logDirFlag)
	logFile, logErr := os.OpenFile(filepath.Join(logDir, "dbexplain.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if logErr == nil {
		defer logFile.Close()
	}
	baseCheckLogger := log.New(logFile, "[check] ", log.LstdFlags)

	// ── Check each DSN ──
	type result struct {
		envKey    string
		label     string
		kind      string
		hostPort  string
		syntaxOK  bool
		syntaxErr string
		connOK    bool
		connMsg   string
		latency   string
	}

	results := make([]result, 0, len(entries))
	var syntaxFails, connFails, connOK int

	for i, e := range entries {
		r := result{envKey: e.EnvKey}

		// 1. Parse DSN syntax
		parsed, err := dsn.ParseDSN(e.Raw)
		if err != nil {
			r.syntaxOK = false
			r.syntaxErr = config.SanitizeErr(err).Error()
			syntaxFails++
			results = append(results, r)
			baseCheckLogger.Printf("(#%d) SYNTAX ERROR: %s", i+1, r.syntaxErr)
			continue
		}
		r.syntaxOK = true
		r.kind = parsed.Kind
		r.label = parsed.Label
		if r.label == "" {
			r.label = "(no label)"
		}
		r.hostPort = parsed.Host
		if parsed.Port != "" {
			r.hostPort += ":" + parsed.Port
		}

		// ── Log check progress to dbexplain.log ──
		var checkLogger *log.Logger
		if logErr == nil {
			checkLogger = log.New(logFile, fmt.Sprintf("[label=%s] [kind=%s] [check] ", r.label, r.kind), log.LstdFlags)
			checkLogger.Printf("(#%d) connecting ...", i+1)
		}

		// 2. Test connectivity via connector.Collect with timeout
		// Suppress collection logs during connectivity check
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
		// lib/pq context cancellation is unreliable when the server is unresponsive,
		// so a select+channel pattern ensures we don't hang forever.
		type chkResult struct {
			err error
		}
		ch := make(chan chkResult, 1)
		go func() {
			_, subErr := connector.Collect(ctx, e.Raw)
			ch <- chkResult{subErr}
		}()
		// Use time.NewTimer instead of ctx.Done() for reliable timeout.
		// Go's context.WithTimeout timer may not fire when lib/pq is blocked
		// in a syscall (the Go runtime cannot schedule the timer goroutine).
		// time.NewTimer uses the runtime timer heap which fires independently.
		timeTimer := time.NewTimer(*timeout)
		select {
		case res := <-ch:
			err = res.err
			timeTimer.Stop()
		case <-timeTimer.C:
			err = context.DeadlineExceeded
		}
		cancel()

		log.SetOutput(oldLogOut)
		elapsed := time.Since(start)
		r.latency = fmt.Sprintf("%dms", elapsed.Milliseconds())

		if err != nil {
			r.connOK = false
			r.connMsg = config.SanitizeErr(err).Error()
			connFails++
			if checkLogger != nil {
				checkLogger.Printf("(#%d) FAIL after %s: %s", i+1, r.latency, r.connMsg)
			}
		} else {
			r.connOK = true
			connOK++
			if checkLogger != nil {
				checkLogger.Printf("(#%d) OK (%s)", i+1, r.latency)
			}
		}

		results = append(results, r)
	}

	// ── Table header ──
	fmt.Println("  No. EnvKey Label              Kind       Host:Port             Syntax  Connect")
	fmt.Println("  " + strings.Repeat("─", 95))

	for i, r := range results {
		idx := fmt.Sprintf("%-4d", i+1)
		key := r.envKey
		if key == "" {
			key = "—"
		}

		if !r.syntaxOK {
			// Syntax error row — show error as the label, blank out kind/host
			errMsg := truncateErr(r.syntaxErr, 55)
			fmt.Printf("  %s %-6s %-18s %-10s %-20s %-7s ❌ %s\n",
				idx, key, "",
				"", "",
				"❌ FAIL", errMsg)
			continue
		}

		// Syntax OK row — show connect status
		label := r.label
		if len(label) > 18 {
			label = label[:15] + "..."
		}

		syntaxCol := "✅ OK"

		var connCol string
		if r.connOK {
			connCol = fmt.Sprintf("✅ OK %s", r.latency)
		} else if strings.Contains(r.connMsg, "context deadline exceeded") || strings.Contains(r.connMsg, "deadline") {
			connCol = fmt.Sprintf("⏱ timeout (%s)", r.latency)
		} else {
			connCol = fmt.Sprintf("❌ FAIL %s", r.latency)
		}

		fmt.Printf("  %s %-6s %-18s %-10s %-20s %-7s %s\n",
			idx, key, label, r.kind, r.hostPort, syntaxCol, connCol)

		// Print connection error details on the next line
		if !r.connOK && !strings.Contains(r.connMsg, "deadline") {
			errLine := truncateErr(r.connMsg, 100)
			fmt.Printf("  %s%s\n", strings.Repeat(" ", 75), errLine)
		}
	}

	// ── Summary ──
	fmt.Println()
	fmt.Printf("  Summary: %d total — %d connected, %d connection failed, %d invalid syntax\n",
		len(entries), connOK, connFails, syntaxFails)
	fmt.Println()

	// Exit with non-zero if any failure
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
