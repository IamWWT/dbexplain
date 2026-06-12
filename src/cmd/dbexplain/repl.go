package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/connector/filequery"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/dsl"
	"github.com/IamWWT/dbexplain/internal/executor"
	"github.com/IamWWT/dbexplain/internal/sqlast"
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

	hasExplicitSource := *configFile != "" || *dsnFlag != ""
	shouldLoadEnv := *envMode || !hasExplicitSource

	if shouldLoadEnv {
		configPath := config.FindConfigFile()
		if configPath == "" {
			if *envMode {
				fmt.Fprintln(os.Stderr, "ERROR: no config file found")
				os.Exit(1)
			}
			config.PrintNoConfigFound()
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		if len(envEntries) == 0 && !*envMode {
			config.PrintEmptyConfigFound(configPath)
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
		// No DSN entries — start in disconnected state
		currentEntry := config.DSNEntry{Raw: ""}
		currentLabel := "(disconnected)"

		fmt.Println("dbexplain REPL (no connections)")
		fmt.Println("Type .help for commands, .connect <dsn> to connect, .exit to quit")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		replLoop(scanner, currentEntry, currentLabel, allEntries, *limit, *timeout)
		return
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

	replLoop(scanner, currentEntry, currentLabel, allEntries, *limit, *timeout)
}

// replLoop is the main REPL input loop. Exits on .exit, EOF, or scanner error.
func replLoop(scanner *bufio.Scanner, currentEntry config.DSNEntry, currentLabel string, allEntries []config.DSNEntry, limit int, timeoutSec int) {
	disconnected := currentEntry.Raw == ""

	for {
		fmt.Printf("dbexplain[%s]> ", currentLabel)
		if !scanner.Scan() {
			break // EOF (Ctrl+D)
		}
		line := strings.TrimSpace(scanner.Text())
		// Strip trailing semicolons — ClickHouse driver appends SETTINGS/FORMAT JSON
		// after the query, and a trailing ; breaks it as multi-statement.
		line = strings.TrimRight(line, ";")
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
			case line == ".list" || line == ".databases":
				if len(allEntries) == 0 {
					fmt.Println("No connections.")
					break
				}
				fmt.Println("Configured databases:")
				// Find max column widths for alignment
				maxLabel := 8
				maxDSN := 3
				type entry struct {
					label string
					kind  string
					dsn   string
				}
				entries := make([]entry, len(allEntries))
				for i, entry := range allEntries {
					d, err := dsn.ParseDSN(entry.Raw)
					if err != nil {
						entries[i].label = "(invalid)"
						entries[i].kind = "?"
						entries[i].dsn = config.SanitizeErr(err).Error()
						continue
					}
					lbl := d.Label
					if lbl == "" {
						lbl = d.Kind
					}
					entries[i].label = lbl
					entries[i].kind = d.Kind
					entries[i].dsn = d.Redacted()
					if len(lbl) > maxLabel {
						maxLabel = len(lbl)
					}
					if len(entries[i].dsn) > maxDSN {
						maxDSN = len(entries[i].dsn)
					}
				}
				// Header
				fmt.Printf("  %-3s  %-*s  %-*s  %-14s  Status\n", "#", maxLabel, "Label", maxDSN, "DSN", "Kind")
				fmt.Printf("  %-3s  %-*s  %-*s  %-14s  %s\n", "---", maxLabel, "------", maxDSN, "---", "----", "------")
				// Rows
				for i, e := range entries {
					current := ""
					if e.label == currentLabel {
						current = "← current"
					}
					fmt.Printf("  %-3d  %-*s  %-*s  %-14s  %s\n", i+1, maxLabel, e.label, maxDSN, e.dsn, e.kind, current)
				}
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
						disconnected = false
						found = true
						fmt.Printf("Switched to: %s\n", d.Label)
						break
					}
				}
				if !found {
					fmt.Printf("No DSN with label %q found\n", parts[1])
				}
			case strings.HasPrefix(line, ".connect"):
				parts := strings.Fields(line)
				if len(parts) < 2 {
					fmt.Println("Usage: .connect <dsn-url>")
					fmt.Println("  e.g. .connect mysql://user:pass@localhost:3306/mydb?label=mydb")
					continue
				}
				raw := parts[1]
				// Rejoin remaining parts (DSN may contain spaces in params)
				for i := 2; i < len(parts); i++ {
					raw += " " + parts[i]
				}
				d, err := dsn.ParseDSN(raw)
				if err != nil {
					fmt.Printf("Invalid DSN: %v\n", config.SanitizeErr(err))
					continue
				}
				if d.Label == "" {
					d.Label = d.Kind
				}
				entry := config.DSNEntry{Raw: raw}
				allEntries = append(allEntries, entry)
				currentEntry = entry
				currentLabel = d.Label
				disconnected = false
				fmt.Printf("Connected to: %s\n", currentLabel)
			default:
				fmt.Printf("Unknown command: %s (try .help)\n", line)
			}
			continue
		}

		// Check disconnected state before query execution
		if disconnected {
			fmt.Println("Not connected. Use .connect <dsn-url> to connect, or use -env flag to pre-load connections.")
			continue
		}

		// Execute query — DSL or single-source
		start := time.Now()
		var err error
		if strings.Contains(line, "@") {
			err = replExecDSL(line, allEntries, limit, timeoutSec)
		} else {
			err = execQuery(currentEntry.Raw, line, limit, timeoutSec, allEntries)
		}
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
		return fmt.Errorf("invalid DSN: %w", config.SanitizeErr(err))
	}

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		return fmt.Errorf("get connector: %w", err)
	}
	caps := capabilities.FromProvider(c)
	isSQL := caps.Has(capabilities.CapSQL)
	isFile := caps.Has(capabilities.CapFile)

	policies := policy.Load("")

	// ES JSON native queries → non-SQL pipeline (bypasses sqlguard, uses _search endpoint)
	if parsed.Kind == "elasticsearch" && isJSONLike(sql) {
		result, execErr := executor.ExecQuery(&executor.ExecOptions{
			Conn: c, Parsed: parsed, SQL: sql,
			Limit: limit, Explain: false, TimeoutSec: timeout,
			Policies: policies, Lock: query.NewQueryLock(), IsSQL: false,
		})
		if execErr != nil {
			return execErr
		}
		fmt.Print(render.FormatHuman(result))
		return nil
	}

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

// ── DSL support (REPL-safe versions, return error instead of os.Exit) ──

// replExecDSL parses and executes a DSL query with @label.table references.
// Handles single-source (SQL/file) and federated (cross-source) execution.
func replExecDSL(line string, allEntries []config.DSNEntry, limit int, timeoutSec int) error {
	dslQuery, err := dsl.Parse(line)
	if err != nil {
		return err
	}
	if !dslQuery.HasSourceRefs() {
		return fmt.Errorf("no @ references found in query")
	}

	bound, err := dsl.Bind(dslQuery, allEntries)
	if err != nil {
		return err
	}

	kinds := bound.SourceKinds()
	if len(kinds) == 0 {
		return fmt.Errorf("DSL error: no source references resolved")
	}

	// Federated: multiple source kinds → materialize all + filequery merge
	if len(kinds) > 1 {
		return replExecFederated(dslQuery, bound, allEntries, limit, timeoutSec)
	}

	// Single source — route by Vendor (same pattern as handleDSLExecute in execute.go)
	primary := bound.PrimarySource()
	if primary == nil {
		return fmt.Errorf("DSL error: no resolved source")
	}
	switch primary.Vendor {
	case dsl.VendorSQL:
		return replExecSQL(dslQuery, bound, limit, timeoutSec, allEntries)
	case dsl.VendorFile:
		return replExecFile(dslQuery, bound, limit, allEntries)
	case dsl.VendorPromQL:
		return replExecPromQL(dslQuery, bound, limit, timeoutSec, allEntries)
	default:
		return fmt.Errorf("DSL error: vendor %d data sources not supported in DSL mode", primary.Vendor)
	}
}

// replExecSQL executes a DSL query against a single SQL data source.
func replExecSQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, limit int, timeoutSec int, allEntries []config.DSNEntry) error {
	compiledSQL, err := dsl.CompileToSQL(dslQuery, bound)
	if err != nil {
		return err
	}

	primary := bound.PrimarySource()
	if primary == nil {
		return fmt.Errorf("DSL error: no resolved source")
	}
	parsed := primary.DSN

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		return err
	}

	policies := policy.Load(envKeyForLabel(primary.DSN.Label, allEntries))

	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn: c, Parsed: parsed, SQL: compiledSQL,
		Limit: limit, Explain: false, TimeoutSec: timeoutSec,
		Policies: policies, Lock: query.NewQueryLock(), IsSQL: true,
	})
	if execErr != nil {
		return execErr
	}

	fmt.Print(render.FormatHuman(result))
	return nil
}

// replExecFile executes a DSL query against a single file data source.
func replExecFile(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, limit int, allEntries []config.DSNEntry) error {
	primary := bound.PrimarySource()
	if primary == nil {
		return fmt.Errorf("DSL error: no resolved source")
	}
	parsed := primary.DSN
	policies := policy.Load(envKeyForLabel(parsed.Label, allEntries))
	queryutil.HandleFileExecute(parsed, dslQuery.SQL, true, limit, policies, allEntries)
	return nil
}

// replExecPromQL executes a DSL query against a Prometheus data source.
// Converts SQL AST → QueryIR → PromQL, then executes via non-SQL pipeline.
func replExecPromQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, limit int, timeoutSec int, allEntries []config.DSNEntry) error {
	primary := bound.PrimarySource()
	if primary == nil {
		return fmt.Errorf("DSL error: no resolved source")
	}
	parsed := primary.DSN
	metricName := primary.Ref.Table

	// Layer 1 security: validate metric name against DenyTables at DSL level
	policies := policy.Load(envKeyForLabel(parsed.Label, allEntries))
	if policies != nil {
		for _, denied := range policies.DenyTables {
			if strings.EqualFold(metricName, denied) {
				return fmt.Errorf("ACCESS_DENIED: metric %q is not allowed for query", metricName)
			}
		}
	}

	// Convert SQL AST → QueryIR
	stmt, ok := dslQuery.Stmt.(*sqlast.SelectStmt)
	if !ok {
		return fmt.Errorf("DSL error: Prometheus DSL requires a SELECT statement")
	}
	ir, irErr := dsl.SelectStmtToIR(stmt)
	if irErr != nil {
		return fmt.Errorf("DSL error: %v", irErr)
	}

	// Override From with actual metric name from bound source
	ir.From = metricName

	// Compile QueryIR → PromQL
	promQL, compErr := dsl.CompileToPromQL(ir)
	if compErr != nil {
		return compErr
	}

	// Get connector
	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		return fmt.Errorf("ERROR: %v", err)
	}

	// Execute through non-SQL pipeline (IsSQL=false → bypasses sqlguard)
	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        promQL,
		Limit:      limit,
		Explain:    false,
		TimeoutSec: timeoutSec,
		Policies:   policies,
		Lock:       query.NewQueryLock(),
		IsSQL:      false,
	})
	if execErr != nil {
		return execErr
	}

	fmt.Print(render.FormatHuman(result))
	return nil
}

// replExecFederated executes a DSL query referencing multiple source kinds
// by materializing all data in memory and merging via the file query engine.
func replExecFederated(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, allEntries []config.DSNEntry, limit int, timeoutSec int) error {
	type materialized struct {
		alias  string
		header []string
		rows   [][]string
	}
	var allData []materialized

	for _, bs := range bound.Sources {
		switch bs.Kind {
		case dsl.SourceSQL:
			selectSQL := fmt.Sprintf("SELECT * FROM %s", bs.Ref.Table)
			c, err := connector.GetConnector(bs.DSN.Kind)
			if err != nil {
				return fmt.Errorf("connector error for @%s.%s: %v", bs.Ref.Label, bs.Ref.Table, err)
			}
			policies := policy.Load(envKeyForLabel(bs.DSN.Label, allEntries))
			result, execErr := executor.ExecQuery(&executor.ExecOptions{
				Conn: c, Parsed: bs.DSN, SQL: selectSQL,
				Limit: limit, Explain: false, TimeoutSec: timeoutSec,
				Policies: policies, Lock: query.NewQueryLock(), IsSQL: true,
			})
			if execErr != nil {
				return fmt.Errorf("query error for @%s.%s: %v", bs.Ref.Label, bs.Ref.Table, execErr)
			}

			rows := make([][]string, len(result.Rows))
			for i, r := range result.Rows {
				srow := make([]string, len(r))
				for j, cell := range r {
					if cell != nil {
						srow[j] = *cell
					}
				}
				rows[i] = srow
			}
			header := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				header[i] = col.Name
			}
			rows = truncateFederatedRows(bs.Ref.Label, bs.Ref.Table, bs.DSN.Kind, rows, limit)
			allData = append(allData, materialized{
				alias:  bs.Ref.Table,
				header: header,
				rows:   rows,
			})

		case dsl.SourceFile:
			c, err := connector.GetConnector(bs.DSN.Kind)
			if err != nil {
				return fmt.Errorf("connector error for @%s.%s: %v", bs.Ref.Label, bs.Ref.Table, err)
			}
			q, ok := c.(query.Queryable)
			if !ok {
				return fmt.Errorf("QUERY_NOT_SUPPORTED: %s", bs.DSN.Kind)
			}
			ctx := context.Background()
			fileOpts := query.ExecuteOpts{
				DSN: bs.DSN, SQL: "SELECT *",
				MaxRows: limit, Timeout: timeoutSec,
			}
			result, err := q.ExecQuery(ctx, fileOpts)
			if err != nil {
				return fmt.Errorf("file query error for @%s.%s: %v", bs.Ref.Label, bs.Ref.Table, err)
			}

			rows := make([][]string, len(result.Rows))
			for i, r := range result.Rows {
				srow := make([]string, len(r))
				for j, cell := range r {
					if cell != nil {
						srow[j] = *cell
					}
				}
				rows[i] = srow
			}
			header := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				header[i] = col.Name
			}
			rows = truncateFederatedRows(bs.Ref.Label, bs.Ref.Table, bs.DSN.Kind, rows, limit)
			allData = append(allData, materialized{
				alias:  bs.Ref.Table,
				header: header,
				rows:   rows,
			})

		case dsl.SourceNative:
			if bs.Vendor == dsl.VendorPromQL {
				// Materialize Prometheus data for federated query
				promQL := bs.Ref.Table
				c, err := connector.GetConnector(bs.DSN.Kind)
				if err != nil {
					return fmt.Errorf("connector error for @%s.%s: %v", bs.Ref.Label, bs.Ref.Table, err)
				}
				policies := policy.Load(envKeyForLabel(bs.DSN.Label, allEntries))
				result, execErr := executor.ExecQuery(&executor.ExecOptions{
					Conn: c, Parsed: bs.DSN, SQL: promQL,
					Limit: limit, Explain: false, TimeoutSec: timeoutSec,
					Policies: policies, Lock: query.NewQueryLock(), IsSQL: false,
				})
				if execErr != nil {
					return fmt.Errorf("query error for @%s.%s: %v", bs.Ref.Label, bs.Ref.Table, execErr)
				}
				rows := make([][]string, len(result.Rows))
				for i, r := range result.Rows {
					srow := make([]string, len(r))
					for j, cell := range r {
						if cell != nil {
							srow[j] = *cell
						}
					}
					rows[i] = srow
				}
				header := make([]string, len(result.Columns))
				for i, col := range result.Columns {
					header[i] = col.Name
				}
				allData = append(allData, materialized{
					alias:  bs.Ref.Table,
					header: header,
					rows:   rows,
				})
			} else {
				return fmt.Errorf("DSL error: native sources not supported in federated queries: @%s.%s", bs.Ref.Label, bs.Ref.Table)
			}
		}
	}

	if len(allData) == 0 {
		return fmt.Errorf("DSL error: no materialized data")
	}

	// Build filequery-compatible SQL by replacing placeholders with table names
	fileSQL := dslQuery.SQL
	for _, bs := range bound.Sources {
		fileSQL = strings.ReplaceAll(fileSQL, bs.Ref.Placeholder, bs.Ref.Table)
	}

	// Determine primary table: the first placeholder in the original SQL
	firstPos := len(dslQuery.SQL)
	firstPlaceholder := ""
	for placeholder := range bound.Sources {
		pos := strings.Index(dslQuery.SQL, placeholder)
		if pos >= 0 && pos < firstPos {
			firstPos = pos
			firstPlaceholder = placeholder
		}
	}
	if firstPlaceholder != "" {
		primaryTable := bound.Sources[firstPlaceholder].Ref.Table
		for i, d := range allData {
			if d.alias == primaryTable && i > 0 {
				allData[0], allData[i] = allData[i], allData[0]
				break
			}
		}
	}

	// Primary source as main table, rest as extras
	primary := allData[0]
	var extras []filequery.NamedData
	for _, d := range allData[1:] {
		extras = append(extras, filequery.NamedData{
			Alias: d.alias, Header: d.header, Rows: d.rows,
		})
	}

	result, execErr := filequery.Execute(fileSQL, primary.header, primary.rows, extras, limit)
	if execErr != nil {
		return fmt.Errorf("QUERY_ERROR: %v", execErr)
	}

	fmt.Print(render.FormatHuman(result))
	return nil
}

func printREPLHelp() {
	fmt.Print(`
dbexplain REPL — Interactive query mode
========================================
Supported: All 13 data sources (SQL / NoSQL / TSDB / Files), DuckDB requires -tags duckdb build
DSL syntax: @label.table references supported (single-source & federated cross-source JOIN)
  Prometheus PromQL via DSL: SELECT * FROM @label.metric [WHERE label="val"]
Commands:
  .connect <dsn>  Connect to a new data source by DSN URL
  .conn <label>   Switch to another data source by label (load with -env)
  .dsn <label>    Alias for .conn
  .list           List all configured databases
  .databases      Alias for .list
  .help           Show this help
  .exit / .quit   Exit REPL
  Ctrl+D          Exit REPL

DSL Examples:
  @mydb.users u JOIN @analytics.orders o ON u.id = o.user_id
  SELECT * FROM @csv.report WHERE region = 'APAC'
  @pg.employees e LEFT JOIN @mysql.dept d ON e.dept_id = d.id
  SELECT * FROM @prom.up WHERE job="node"          -- PromQL via DSL

Examples:
  dbexplain repl --dsn "sqlite:////tmp/test.db"
  dbexplain repl -env
  dbexplain repl -env --limit 5000 --timeout 60
`)
}

// isJSONLike performs a quick check for JSON-like input (starts with '{' or '[').
func isJSONLike(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) > 0 && (s[0] == '{' || s[0] == '[')
}
