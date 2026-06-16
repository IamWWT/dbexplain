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
	useEnv := fs.Bool("env", true, "Load from .env config file (default: auto-detect)")
	timeout := fs.Duration("timeout", 10*time.Second, "Per-DSN connection timeout")
	labelFilter := fs.String("label", "", "filter by label")
	fs.Parse(args)

	var entries []config.DSNEntry
	configPath := ""

	// Load from .env if no explicit source given, or --env is set
	hasExplicitSource := len(dsnFlags) > 0 || *configFile != ""
	shouldLoadEnv := *useEnv || !hasExplicitSource

	if shouldLoadEnv {
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
		fmt.Fprintln(os.Stderr, "ERROR: no DSNs found. Use --env, --dsn, or --config.")
		os.Exit(1)
	}

	// ── Header ──
	fmt.Println()
	fmt.Printf("  Config file: %s\n", config.DescribeConfigSource(configPath))
	fmt.Printf("  DSN count:   %d\n\n", len(entries))

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

	for _, e := range entries {
		r := result{envKey: e.EnvKey}

		// 1. Parse DSN syntax
		parsed, err := dsn.ParseDSN(e.Raw)
		if err != nil {
			r.syntaxOK = false
			r.syntaxErr = config.SanitizeErr(err).Error()
			syntaxFails++
			results = append(results, r)
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

		// 2. Test connectivity via connector.Collect with timeout
		// Suppress collection logs during connectivity check
		discardCtx := connector.WithLogger(context.Background(),
			log.New(io.Discard, "", 0))
		ctx, cancel := context.WithTimeout(discardCtx, *timeout)
		oldLogOut := log.Writer()
		log.SetOutput(io.Discard)
		start := time.Now()
		_, err = connector.Collect(ctx, e.Raw)
		log.SetOutput(oldLogOut)
		cancel()
		elapsed := time.Since(start)
		r.latency = fmt.Sprintf("%dms", elapsed.Milliseconds())

		if err != nil {
			r.connOK = false
			r.connMsg = config.SanitizeErr(err).Error()
			connFails++
		} else {
			r.connOK = true
			connOK++
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
