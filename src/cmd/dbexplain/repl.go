package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/executor"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/queryutil"
	"github.com/IamWWT/dbexplain/internal/render"
	"github.com/IamWWT/dbexplain/internal/policy"
)

func handleREPL(args []string) {
	fs := flag.NewFlagSet("repl", flag.ExitOnError)
	envMode := fs.Bool("env", false, "Load config from .env file")
	configFile := fs.String("config", "", "JSON config file")
	dsnFlag := fs.String("dsn", "", "Direct DSN connection string")
	limit := fs.Int("limit", 1000, "Max rows to return")
	timeout := fs.Int("timeout", 30, "Query timeout in seconds")
	fs.Parse(args)

	// Pre-load DSN entries
	var allEntries []config.DSNEntry
	if *envMode {
		configPath := config.FindConfigFile()
		if configPath == "" {
			fmt.Fprintln(os.Stderr, "ERROR: no config file found")
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		allEntries = append(allEntries, envEntries...)
	}
	if *configFile != "" {
		for _, raw := range config.LoadFromConfig(*configFile) {
			allEntries = append(allEntries, config.DSNEntry{Raw: raw})
		}
	}
	if *dsnFlag != "" {
		allEntries = append(allEntries, config.DSNEntry{Raw: *dsnFlag})
	}

	if len(allEntries) == 0 {
		fmt.Fprintln(os.Stderr, "No DSN entries. Use --env, --config, or --dsn")
		os.Exit(1)
	}

	// Use first entry as initial connection
	currentEntry := allEntries[0]
	currentLabel := ""
	if d, err := dsn.ParseDSN(currentEntry.Raw); err == nil {
		currentLabel = d.Label
		if currentLabel == "" {
			currentLabel = d.Kind
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	// Increase max token size for long queries
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	fmt.Printf("dbexplain REPL (connected: %s)\n", currentLabel)
	fmt.Println("Type .help for commands, .exit to quit")

	for {
		fmt.Printf("dbexplain[%s]> ", currentLabel)
		if !scanner.Scan() {
			break // EOF (Ctrl+D)
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle dot commands
		if strings.HasPrefix(line, ".") {
			switch {
			case line == ".exit" || line == ".quit":
				fmt.Println("Goodbye.")
				return
			case line == ".help":
				printREPLHelp()
			case strings.HasPrefix(line, ".conn") || strings.HasPrefix(line, ".dsn"):
				parts := strings.Fields(line)
				if len(parts) < 2 {
					fmt.Println("Usage: .conn <label>")
					continue
				}
				found := false
				for _, entry := range allEntries {
					if d, err := dsn.ParseDSN(entry.Raw); err == nil && d.Label == parts[1] {
						currentEntry = entry
						currentLabel = d.Label
						if currentLabel == "" {
							currentLabel = d.Kind
						}
						found = true
						fmt.Printf("Switched to: %s\n", d.Label)
						break
					}
				}
				if !found {
					fmt.Printf("No DSN with label %q found\n", parts[1])
				}
			default:
				fmt.Printf("Unknown command: %s (try .help)\n", line)
			}
			continue
		}

		// Execute query
		start := time.Now()
		err := execQuery(currentEntry.Raw, line, *limit, *timeout, allEntries)
		elapsed := time.Since(start)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		} else {
			fmt.Printf("(query completed in %v)\n", elapsed.Round(time.Millisecond))
		}
	}
}

// execQuery runs a single query against a DSN and prints the result.
// Returns an error on failure (no os.Exit).
func execQuery(dsnRaw string, sql string, limit int, timeout int, allEntries []config.DSNEntry) error {
	parsed, err := dsn.ParseDSN(dsnRaw)
	if err != nil {
		return fmt.Errorf("invalid DSN: %v", config.SanitizeErr(err))
	}

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	caps := capabilities.FromProvider(c)
	isSQL := caps.Has(capabilities.CapSQL)
	isFile := caps.Has(capabilities.CapFile)

	policies := policy.Load("")

	if isFile {
		// File datasource: reuse HandleFileExecute but without os.Exit
		queryutil.HandleFileExecute(parsed, sql, true, limit, policies, allEntries)
		return nil
	}

	// SQL and native execution
	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        sql,
		Limit:      limit,
		Explain:    false,
		TimeoutSec: timeout,
		Policies:   policies,
		Lock:       query.NewQueryLock(),
		IsSQL:      isSQL,
	})
	if execErr != nil {
		return execErr
	}

	fmt.Print(render.FormatHuman(result))
	return nil
}

func printREPLHelp() {
	fmt.Print(`
dbexplain REPL — Interactive query mode
========================================
Supported: All 11 data sources (SQL / NoSQL / Files)
Not supported: DSL mode (@label.table), federated cross-source queries

Commands:
  .conn <label>   Switch to another data source by label (load with -env)
  .dsn <label>    Alias for .conn
  .help           Show this help
  .exit / .quit   Exit REPL
  Ctrl+D          Exit REPL

Examples:
  dbexplain repl --dsn "sqlite:////tmp/test.db"
  dbexplain repl -env
  dbexplain repl -env --limit 5000 --timeout 60
`)
}
