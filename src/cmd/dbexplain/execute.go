// Package main handles the "execute" subcommand for read-only query execution.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	label := fs.String("label", "", "Match DSN by label")
	dbIndex := fs.Int("db", 0, "Match DSN by DB<N> index (1-based)")
	dsnFlag := fs.String("dsn", "", "Direct DSN connection string")
	configFile := fs.String("config", "", "JSON config file with array of DSNs")
	limit := fs.Int("limit", 1000, "Max rows to return")
	timeout := fs.Int("timeout", 30, "Query timeout in seconds")
	explain := fs.Bool("explain", false, "Wrap query with EXPLAIN")
	dslMode := fs.Bool("dsl", false, "Enable DSL mode: parse @label.table references")
	human := fs.Bool("human", false, "Human-readable table output (default: JSON)")
	logDirFlag := fs.String("log-dir", "/var/log/dbexplain", "directory for log files")
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
		case "--log-dir":
			if hasEq && val != "" {
				*logDirFlag = val
			} else if i+1 < len(extraArgs) && !strings.HasPrefix(extraArgs[i+1], "--") {
				i++
				*logDirFlag = extraArgs[i]
			}
		}
	}

	sqlArg := fs.Arg(0)
	if sqlArg == "" {
		fmt.Fprintln(os.Stderr, "ERROR: missing SQL query — usage: dbexplain execute [flags] <SQL>")
		fmt.Fprintln(os.Stderr, "  Flags (any position): --label, --db, --explain, --human, --dsl, --limit, --timeout, --dsn, --config")
		os.Exit(1)
	}

	// Resolve DSNs — gather ALL entries before label filter (needed for JOIN source resolution)
	var allEntries []config.DSNEntry
	hasExplicitSource := *configFile != "" || *dsnFlag != ""
	if !hasExplicitSource {
		configPath := config.FindConfigFile()
		if configPath == "" {
			config.PrintNoConfigFound()
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, config.SanitizeErr(err))
			os.Exit(1)
		}
		if len(envEntries) == 0 {
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
			logDir := config.ResolveLogDir(*logDirFlag)
			handleDSLExecute(dslQuery, allEntries, *human, *limit, *explain, *timeout, logDir)
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
	execCtx := context.Background()
	logDir := config.ResolveLogDir(*logDirFlag)
	if parsed.Label != "" {
		logFileName := filepath.Join(logDir, parsed.Label+".log")
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			defer logFile.Close()
			execCtx = connector.WithLogger(execCtx, log.New(logFile, "", log.LstdFlags))
		}
	}
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
		Context:    execCtx,
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
func handleDSLExecute(dslQuery *dsl.DSLQuery, allEntries []config.DSNEntry, human bool, limit int, explain bool, timeoutSec int, logDir string) {
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
		dslExecFederated(dslQuery, bound, allEntries, human, limit, explain, timeoutSec, logDir)
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
		dslExecSQL(dslQuery, bound, human, limit, explain, timeoutSec, allEntries, logDir)
	case dsl.VendorFile:
		dslExecFile(dslQuery, bound, human, limit, allEntries)
	case dsl.VendorPromQL:
		dslExecPromQL(dslQuery, bound, human, limit, timeoutSec, allEntries, logDir)
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
func dslExecSQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int, explain bool, timeoutSec int, allEntries []config.DSNEntry, logDir string) {
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

	execCtx := context.Background()
	if parsed.Label != "" {
		if lf, err := os.OpenFile(filepath.Join(logDir, parsed.Label+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			defer lf.Close()
			execCtx = connector.WithLogger(execCtx, log.New(lf, "", log.LstdFlags))
		}
	}

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
		Context:    execCtx,
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
func dslExecPromQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int, timeoutSec int, allEntries []config.DSNEntry, logDir string) {
	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}
	parsed := primary.DSN
	metricName := primary.Ref.Table
	isRawPromQL := primary.Ref.IsRawPromQL

	policies := policy.Load(envKeyForLabel(parsed.Label, allEntries))

	// Layer 1 security: validate metric name against DenyTables at DSL level.
	// Skip for raw PromQL expressions (no single metric name to validate).
	if !isRawPromQL && policies != nil {
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
	ir.IsRawPromQL = isRawPromQL

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

	// Determine effective limit for execution.
	// When ORDER BY is present, bypass connector truncation by passing a
	// large limit — sorting must see all rows before user LIMIT is applied.
	execLimit := limit
	if len(ir.OrderBy) > 0 {
		execLimit = math.MaxInt32
	}

	// Execute through non-SQL pipeline (IsSQL=false → bypasses sqlguard)
	execCtx := context.Background()
	if parsed.Label != "" {
		if lf, err := os.OpenFile(filepath.Join(logDir, parsed.Label+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			defer lf.Close()
			execCtx = connector.WithLogger(execCtx, log.New(lf, "", log.LstdFlags))
		}
	}
	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        promQL,
		Limit:      execLimit,
		Explain:    false,
		TimeoutSec: timeoutSec,
		Policies:   policies,
		Lock:       queryLock,
		IsSQL:      false,
		Context:    execCtx,
	})
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}

	// ── ORDER BY / LIMIT / OFFSET post-processing (single-source Prometheus) ──
	hasOrderBy := len(ir.OrderBy) > 0
	hasLimit := ir.Limit > 0
	hasOffset := ir.Offset > 0

	if hasOrderBy || hasLimit || hasOffset {
		colIndex := make(map[string]int, len(result.Columns))
		for i, c := range result.Columns {
			colIndex[c.Name] = i
		}

		if hasOrderBy {
			sort.SliceStable(result.Rows, func(i, j int) bool {
				for _, ob := range ir.OrderBy {
					idx, ok := colIndex[ob.Column]
					if !ok {
						continue
					}
					a, b := result.Rows[i][idx], result.Rows[j][idx]
					cmp := compareStringValues(a, b, result.Columns[idx].Type)
					if cmp != 0 {
						if ob.Desc {
							return cmp > 0
						}
						return cmp < 0
					}
				}
				return false
			})
		}

		// Apply LIMIT: prefer SQL-level LIMIT, fall back to CLI --limit
		applyLimit := ir.Limit
		if !hasLimit && hasOrderBy && limit > 0 && len(result.Rows) > limit {
			applyLimit = limit
		}
		if applyLimit > 0 && len(result.Rows) > applyLimit {
			result.Rows = result.Rows[:applyLimit]
			result.Truncated = true
		}

		// Apply OFFSET
		if ir.Offset > 0 && ir.Offset < len(result.Rows) {
			result.Rows = result.Rows[ir.Offset:]
		} else if ir.Offset > 0 {
			result.Rows = result.Rows[:0]
		}

		result.RowCount = len(result.Rows)
	}

	// ── SELECT column projection (non-AllColumns) ──
	if !ir.AllColumns && len(ir.Columns) > 0 {
		selectCols := make([]struct {
			name  string
			index int
		}, 0, len(ir.Columns))
		for _, col := range ir.Columns {
			// Determine display name and search name
			displayName := col.Name
			searchName := col.Name
			if col.Func != "" {
				// Aggregation function: e.g. COUNT(value) → display as "count(value)"
				aggDisplay := col.Func
				if col.Alias != "" {
					aggDisplay = col.Alias
				} else if len(col.Args) > 0 {
					aggDisplay = fmt.Sprintf("%s(%s)", strings.ToLower(col.Func), col.Args[0].Name)
				}
				displayName = aggDisplay
				// Match by the first argument's name (e.g. "value")
				if len(col.Args) > 0 {
					searchName = col.Args[0].Name
				}
			} else if col.Alias != "" {
				displayName = col.Alias
			}
			idx := -1
			for i, rc := range result.Columns {
				if strings.EqualFold(rc.Name, searchName) {
					idx = i
					break
				}
			}
			if idx >= 0 {
				selectCols = append(selectCols, struct {
					name  string
					index int
				}{displayName, idx})
			}
		}
		if len(selectCols) > 0 && len(selectCols) < len(result.Columns) {
			newCols := make([]query.ColumnInfo, len(selectCols))
			for i, sc := range selectCols {
				newCols[i] = result.Columns[sc.index]
				newCols[i].Name = sc.name
			}
			newRows := make([][]*string, len(result.Rows))
			for i, row := range result.Rows {
				nr := make([]*string, len(selectCols))
				for j, sc := range selectCols {
					if sc.index < len(row) {
						nr[j] = row[sc.index]
					}
				}
				newRows[i] = nr
			}
			result.Columns = newCols
			result.Rows = newRows
			result.RowCount = len(newRows)
		}
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
func dslExecFederated(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, allEntries []config.DSNEntry, human bool, limit int, explain bool, timeoutSec int, logDir string) {
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
			fedCtx := context.Background()
			if bs.DSN.Label != "" {
				if lf, err := os.OpenFile(filepath.Join(logDir, bs.DSN.Label+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
					defer lf.Close()
					fedCtx = connector.WithLogger(fedCtx, log.New(lf, "", log.LstdFlags))
				}
			}
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
				Context:    fedCtx,
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
			rows = truncateFederatedRows(bs.Ref.Label, bs.Ref.Table, bs.DSN.Kind, rows, limit)
			allData = append(allData, materialized{
				alias:  bs.Ref.Table,
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
					fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
					os.Exit(1)
				}
				policies := policy.Load(envKeyForLabel(bs.DSN.Label, allEntries))
				fedCtx := context.Background()
				if bs.DSN.Label != "" {
					if lf, err := os.OpenFile(filepath.Join(logDir, bs.DSN.Label+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
						defer lf.Close()
						fedCtx = connector.WithLogger(fedCtx, log.New(lf, "", log.LstdFlags))
					}
				}
				result, execErr := executor.ExecQuery(&executor.ExecOptions{
					Conn: c, Parsed: bs.DSN, SQL: promQL,
					Limit: limit, Explain: false, TimeoutSec: timeoutSec,
					Policies: policies, Lock: queryLock, IsSQL: false,
					Context: fedCtx,
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
				// For raw PromQL expressions, use the placeholder as alias
				// (the expression is not a valid SQL identifier for the file engine)
				alias := bs.Ref.Table
				if bs.Ref.IsRawPromQL {
					alias = bs.Ref.Placeholder
				}
				allData = append(allData, materialized{
					alias:  alias,
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

	// Reorder allData so the FROM table's data is primary (first).
	// Without this, map iteration order determines allData order, which may
	// mismatch the SQL FROM clause after placeholder→table replacement.
	fileSQL := dslQuery.SQL
	for _, bs := range bound.Sources {
		if bs.Ref.IsRawPromQL {
			continue // placeholder stays as table name (alias) in file SQL
		}
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
		// For raw PromQL, the materialized alias is the placeholder, not the expression
		if bound.Sources[firstPlaceholder].Ref.IsRawPromQL {
			primaryTable = firstPlaceholder
		}
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

// truncateFederatedRows limits materialized rows to `limit` with a warning.
// This is the federated-query counterpart of sqlguard.AutoLimit, which cannot
// be used for native sources (PromQL) that lack SQL LIMIT semantics. It also
// serves as defense-in-depth for SQL/file sources in the federated path.
func truncateFederatedRows(label, table, kind string, rows [][]string, limit int) [][]string {
	if limit > 0 && len(rows) > limit {
		fmt.Fprintf(os.Stderr, "WARNING: @%s.%s (%s) returned %d rows, truncated to %d (use --limit to adjust)\n",
			label, table, kind, len(rows), limit)
		return rows[:limit]
	}
	return rows
}

// compareStringValues compares two *string values for sorting.
// It handles nil pointers (NULLS LAST: nil > non-nil) and parses float
// columns numerically so that "100" > "9" for value/timestamp columns.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareStringValues(a, b *string, colType string) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1 // NULLS LAST
	}
	if b == nil {
		return -1 // NULLS LAST
	}

	// Numeric comparison for float columns (value, timestamp)
	if colType == "float" {
		fa, erra := strconv.ParseFloat(*a, 64)
		fb, errb := strconv.ParseFloat(*b, 64)
		if erra == nil && errb == nil {
			if fa < fb {
				return -1
			}
			if fa > fb {
				return 1
			}
			return 0
		}
		// Fall through to string comparison if parse fails
	}

	// Default: lexicographic string comparison
	if *a < *b {
		return -1
	}
	if *a > *b {
		return 1
	}
	return 0
}