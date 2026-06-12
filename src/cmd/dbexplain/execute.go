// Package main handles the "execute" subcommand for read-only query execution.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/connector/filequery"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/dsl"
	"github.com/IamWWT/dbexplain/internal/dsnfilter"
	"github.com/IamWWT/dbexplain/internal/executor"
	"github.com/IamWWT/dbexplain/internal/policy"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/queryutil"
	"github.com/IamWWT/dbexplain/internal/render"
	"github.com/IamWWT/dbexplain/internal/sqlast"
)

var queryLock = query.NewQueryLock()

func handleExecute(args []string) {
	fs := flag.NewFlagSet("execute", flag.ExitOnError)
	envMode := fs.Bool("env", false, "Load config from .env file and match by --label/--db")
	label := fs.String("label", "", "Match DSN by label")
	dbIndex := fs.Int("db", 0, "Match DSN by DB<N> index (1-based)")
	dsnFlag := fs.String("dsn", "", "Direct DSN connection string")
	configFile := fs.String("config", "", "JSON config file with array of DSNs")
	limit := fs.Int("limit", 1000, "Max rows to return")
	timeout := fs.Int("timeout", 30, "Query timeout in seconds")
	explain := fs.Bool("explain", false, "Wrap query with EXPLAIN")
	dslMode := fs.Bool("dsl", false, "Enable DSL mode: parse @label.table references")
	human := fs.Bool("human", false, "Human-readable table output (default: JSON)")
	fs.Parse(args)

	// Allow flags after the SQL query for convenience.
	// Go's flag.FlagSet stops parsing at the first non-flag argument,
	// so we re-scan remaining args for any flags placed after the query.
	// Only boolean and named-value flags are supported here (not positional).
	extraArgs := fs.Args()
	for i := 0; i < len(extraArgs); i++ {
		a := extraArgs[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		name, val, hasEq := strings.Cut(a, "=")
		switch name {
		case "--human":
			*human = true
		case "--explain":
			*explain = true
		case "--env":
			*envMode = true
		case "--dsl":
			*dslMode = true
		case "--label":
			if hasEq && val != "" {
				*label = val
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				*label = extraArgs[i]
			}
		case "--db":
			if hasEq && val != "" {
				fmt.Sscanf(val, "%d", dbIndex)
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				fmt.Sscanf(extraArgs[i], "%d", dbIndex)
			}
		case "--limit":
			if hasEq && val != "" {
				fmt.Sscanf(val, "%d", limit)
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				fmt.Sscanf(extraArgs[i], "%d", limit)
			}
		case "--timeout":
			if hasEq && val != "" {
				fmt.Sscanf(val, "%d", timeout)
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				fmt.Sscanf(extraArgs[i], "%d", timeout)
			}
		case "--dsn":
			if hasEq && val != "" {
				*dsnFlag = val
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				*dsnFlag = extraArgs[i]
			}
		case "--config":
			if hasEq && val != "" {
				*configFile = val
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				*configFile = extraArgs[i]
			}
		}
	}

	sqlArg := fs.Arg(0)
	if sqlArg == "" {
		fmt.Fprintln(os.Stderr, "ERROR: missing SQL query — usage: dbexplain execute [flags] <SQL>")
		fmt.Fprintln(os.Stderr, "  Flags (any position): --label, --db, --explain, --human, --env, --dsl, --limit, --timeout, --dsn, --config")
		os.Exit(1)
	}

	// Resolve DSNs — gather ALL entries before label filter (needed for JOIN source resolution)
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
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, config.SanitizeErr(err))
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

	// DSL mode: parse @label.table references and route to the correct backend.
	if *dslMode {
		dslQuery, dslErr := dsl.Parse(sqlArg)
		if dslErr != nil {
			fmt.Fprintln(os.Stderr, dslErr)
			os.Exit(1)
		}
		if dslQuery.HasSourceRefs() {
			handleDSLExecute(dslQuery, allEntries, *human, *limit, *explain, *timeout)
			return
		}
	}

	// Filter by label/dbIndex to select the primary DSN
	entries := dsnfilter.FilterEntries(allEntries, label, dbIndex)
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no matching DSN found")
		os.Exit(1)
	}
	if len(entries) > 1 {
		msg := fmt.Sprintf("ERROR: %d DSNs matched — use --label to select one:\n", len(entries))
		for _, e := range entries {
			d, err := dsn.ParseDSN(e.Raw)
			if err == nil {
				msg += fmt.Sprintf("  --label %s (%s)\n", d.Label, d.Redacted())
			}
		}
		fmt.Fprint(os.Stderr, msg)
		os.Exit(1)
	}

	parsed, err := dsn.ParseDSN(entries[0].Raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: invalid DSN: %v\n", config.SanitizeErr(err))
		os.Exit(1)
	}

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	caps := capabilities.FromProvider(c)
	isSQL := caps.Has(capabilities.CapSQL)
	isFile := caps.Has(capabilities.CapFile)

	policies := policy.Load(entries[0].EnvKey)

	// File datasources: bypass sqlguard, enforce policy engine
	if isFile {
		queryutil.HandleFileExecute(parsed, sqlArg, *human, *limit, policies, allEntries)
		return
	}

	// SQL and native execution via shared pipeline
	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        sqlArg,
		Limit:      *limit,
		Explain:    *explain,
		TimeoutSec: *timeout,
		Policies:   policies,
		Lock:       queryLock,
		IsSQL:      isSQL,
	})
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}

	// Output
	if *human {
		if *explain && parsed.Kind == "mysql" {
			fmt.Print(render.FormatExplainJSON(result))
		} else {
			fmt.Print(render.FormatHuman(result))
		}
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// handleDSLExecute handles the DSL execution path.
func handleDSLExecute(dslQuery *dsl.DSLQuery, allEntries []config.DSNEntry, human bool, limit int, explain bool, timeoutSec int) {
	bound, err := dsl.Bind(dslQuery, allEntries)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	kinds := bound.SourceKinds()
	if len(kinds) == 0 {
		fmt.Fprintln(os.Stderr, "DSL error: no source references resolved")
		os.Exit(1)
	}
	if len(kinds) > 1 {
		dslExecFederated(dslQuery, bound, allEntries, human, limit, explain, timeoutSec)
		return
	}

	// Route by vendor
	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}

	switch primary.Vendor {
	case dsl.VendorSQL:
		dslExecSQL(dslQuery, bound, human, limit, explain, timeoutSec, allEntries)
	case dsl.VendorFile:
		dslExecFile(dslQuery, bound, human, limit, allEntries)
	case dsl.VendorPromQL:
		dslExecPromQL(dslQuery, bound, human, limit, timeoutSec, allEntries)
	default:
		fmt.Fprintf(os.Stderr, "DSL error: %s data sources not supported in DSL mode\n", primary.Vendor)
		os.Exit(1)
	}
}

func kindName(k dsl.SourceKind) string {
	switch k {
	case dsl.SourceSQL:
		return "SQL"
	case dsl.SourceFile:
		return "file"
	case dsl.SourceNative:
		return "native"
	default:
		return "unknown"
	}
}

// dslExecSQL compiles and executes a DSL query against an SQL database.
func dslExecSQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int, explain bool, timeoutSec int, allEntries []config.DSNEntry) {
	compiledSQL, err := dsl.CompileToSQL(dslQuery, bound)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}
	parsed := primary.DSN

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	policies := policy.Load(envKeyForLabel(primary.DSN.Label, allEntries))

	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        compiledSQL,
		Limit:      limit,
		Explain:    explain,
		TimeoutSec: timeoutSec,
		Policies:   policies,
		Lock:       queryLock,
		IsSQL:      true,
	})
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}

	if human {
		if explain && primary.DSN.Kind == "mysql" {
			fmt.Print(render.FormatExplainJSON(result))
		} else {
			fmt.Print(render.FormatHuman(result))
		}
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// dslExecFile executes a DSL query against a file data source (CSV/XLSX).
func dslExecFile(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int, allEntries []config.DSNEntry) {
	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}
	parsed := primary.DSN

	policies := policy.Load(envKeyForLabel(parsed.Label, allEntries))
	queryutil.HandleFileExecute(parsed, dslQuery.SQL, human, limit, policies, allEntries)
}

// dslExecPromQL executes a DSL query against a Prometheus data source.
// It converts the SQL AST to QueryIR, compiles to PromQL, and executes
// through the non-SQL pipeline (IsSQL=false, bypassing sqlguard).
func dslExecPromQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int, timeoutSec int, allEntries []config.DSNEntry) {
	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}
	parsed := primary.DSN
	metricName := primary.Ref.Table

	// Layer 1 security: validate metric name against DenyTables at DSL level
	policies := policy.Load(envKeyForLabel(parsed.Label, allEntries))
	if policies != nil {
		for _, denied := range policies.DenyTables {
			if strings.EqualFold(metricName, denied) {
				fmt.Fprintf(os.Stderr, "ACCESS_DENIED: metric %q is not allowed for query\n", metricName)
				os.Exit(1)
			}
		}
	}

	// Convert SQL AST → QueryIR
	stmt, ok := dslQuery.Stmt.(*sqlast.SelectStmt)
	if !ok {
		fmt.Fprintln(os.Stderr, "DSL error: Prometheus DSL requires a SELECT statement")
		os.Exit(1)
	}
	ir, irErr := dsl.SelectStmtToIR(stmt)
	if irErr != nil {
		fmt.Fprintf(os.Stderr, "DSL error: %v\n", irErr)
		os.Exit(1)
	}

	// Override From with actual metric name from bound source
	// (the AST uses placeholder __dsl_N from DSL preprocessing)
	ir.From = metricName

	// Compile QueryIR → PromQL
	promQL, compErr := dsl.CompileToPromQL(ir)
	if compErr != nil {
		fmt.Fprintln(os.Stderr, compErr)
		os.Exit(1)
	}

	// Get connector
	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
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
		Lock:       queryLock,
		IsSQL:      false,
	})
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}

	// Output
	if human {
		fmt.Print(render.FormatHuman(result))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// dslExecFederated executes a DSL query that references multiple source kinds
// (e.g., SQL + file, or multiple SQL DBs). It materializes all data in memory
// then uses the file query engine as a federated merge layer.
func dslExecFederated(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, allEntries []config.DSNEntry, human bool, limit int, explain bool, timeoutSec int) {
	// Materialize all sources
	type materialized struct {
		alias  string
		header []string
		rows   [][]string
	}
	var allData []materialized

	for _, bs := range bound.Sources {
		switch bs.Kind {
		case dsl.SourceSQL:
			// Execute SELECT * from the table against the SQL DB
			selectSQL := fmt.Sprintf("SELECT * FROM %s", bs.Ref.Table)
			c, err := connector.GetConnector(bs.DSN.Kind)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			policies := policy.Load(envKeyForLabel(bs.DSN.Label, allEntries))
			result, execErr := executor.ExecQuery(&executor.ExecOptions{
				Conn:       c,
				Parsed:     bs.DSN,
				SQL:        selectSQL,
				Limit:      limit,
				Explain:    false,
				TimeoutSec: timeoutSec,
				Policies:   policies,
				Lock:       queryLock,
				IsSQL:      true,
			})
			if execErr != nil {
				fmt.Fprintln(os.Stderr, execErr)
				os.Exit(1)
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
				alias:  bs.Ref.Placeholder,
				header: header,
				rows:   rows,
			})
		case dsl.SourceFile:
			// Load file data via connector
			c, err := connector.GetConnector(bs.DSN.Kind)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			q, ok := c.(query.Queryable)
			if !ok {
				fmt.Fprintf(os.Stderr, "QUERY_NOT_SUPPORTED: %s\n", bs.DSN.Kind)
				os.Exit(1)
			}
			ctx := context.Background()
			fileOpts := query.ExecuteOpts{
				DSN:     bs.DSN,
				SQL:     "SELECT *",
				MaxRows: limit,
				Timeout: timeoutSec,
			}
			result, err := q.ExecQuery(ctx, fileOpts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "QUERY_ERROR: %v\n", err)
				os.Exit(1)
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
				alias:  bs.Ref.Placeholder,
				header: header,
				rows:   rows,
			})

		case dsl.SourceNative:
			if bs.Vendor == dsl.VendorPromQL {
				// Materialize Prometheus data for federated query
				promQL := bs.Ref.Table
				c, err := connector.GetConnector(bs.DSN.Kind)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
					os.Exit(1)
				}
				policies := policy.Load(envKeyForLabel(bs.DSN.Label, allEntries))
				result, execErr := executor.ExecQuery(&executor.ExecOptions{
					Conn: c, Parsed: bs.DSN, SQL: promQL,
					Limit: limit, Explain: false, TimeoutSec: timeoutSec,
					Policies: policies, Lock: queryLock, IsSQL: false,
				})
				if execErr != nil {
					fmt.Fprintln(os.Stderr, execErr)
					os.Exit(1)
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
					alias:  bs.Ref.Placeholder,
					header: header,
					rows:   rows,
				})
			} else {
				fmt.Fprintf(os.Stderr, "DSL error: native sources not supported in federated queries: @%s.%s\n", bs.Ref.Label, bs.Ref.Table)
				os.Exit(1)
			}
		}
	}

	if len(allData) == 0 {
		fmt.Fprintln(os.Stderr, "DSL error: no materialized data")
		os.Exit(1)
	}

	// Build filequery-compatible SQL by replacing placeholders with table names
	fileSQL := dslQuery.SQL
	for _, bs := range bound.Sources {
		fileSQL = strings.ReplaceAll(fileSQL, bs.Ref.Placeholder, bs.Ref.Table)
	}

	// Primary source as main table, rest as extras
	primary := allData[0]
	var extras []filequery.NamedData
	for _, d := range allData[1:] {
		extras = append(extras, filequery.NamedData{
			Alias:  d.alias,
			Header: d.header,
			Rows:   d.rows,
		})
	}

	result, execErr := filequery.Execute(fileSQL, primary.header, primary.rows, extras, limit)
	if execErr != nil {
		fmt.Fprintf(os.Stderr, "QUERY_ERROR: %v\n", execErr)
		os.Exit(1)
	}

	// Output
	if human {
		fmt.Print(render.FormatHuman(result))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// envKeyForLabel maps a DSN label back to its EnvKey (e.g. "DB1") from allEntries.
// Returns "" if not found (only global policies apply).
func envKeyForLabel(label string, allEntries []config.DSNEntry) string {
	for _, e := range allEntries {
		if parsed, err := dsn.ParseDSN(e.Raw); err == nil && parsed.Label == label {
			return e.EnvKey
		}
	}
	return ""
}
