package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/connector"
	"github.com/IamWWT/dbexplain/dsn"
	"github.com/IamWWT/dbexplain/query"
	"github.com/IamWWT/dbexplain/sqlguard"
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
	human := fs.Bool("human", false, "Human-readable table output (default: JSON)")
	fs.Parse(args)

	sqlArg := fs.Arg(0)
	if sqlArg == "" {
		fmt.Fprintln(os.Stderr, "READ_ONLY_VIOLATION: empty query")
		os.Exit(1)
	}

	// Resolve DSN first (needed to determine SQL vs native)
	entries := resolveDSNEntries(envMode, dsnFlag, configFile, label, dbIndex)
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no matching DSN found")
		os.Exit(1)
	}
	if len(entries) > 1 {
		fmt.Fprintln(os.Stderr, "ERROR: multiple DSNs matched — use --label or --db to select one")
		os.Exit(1)
	}

	parsed, err := dsn.ParseDSN(entries[0].raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: invalid DSN: %v\n", err)
		os.Exit(1)
	}

	// Only validate SQL for SQL-based connectors. Native connectors
	// (Redis, MongoDB, Qdrant) do their own validation in ExecQuery.
	if isSQLKind(parsed.Kind) {
		if err := sqlguard.Validate(sqlArg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Apply auto-limit and optional EXPLAIN wrapping (SQL only)
	var sql string
	if isSQLKind(parsed.Kind) {
		if *explain {
			sql = "EXPLAIN " + sqlArg
		} else {
			sql = sqlguard.AutoLimit(sqlArg, *limit)
		}
	} else {
		sql = sqlArg
	}

	// Concurrent control
	lockLabel := parsed.Label
	if !queryLock.Lock(lockLabel) {
		fmt.Fprintf(os.Stderr, "CONCURRENT_LIMIT: a query is already running for label %q\n", lockLabel)
		os.Exit(1)
	}
	defer queryLock.Unlock(lockLabel)

	// Get connector
	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Check if connector supports Queryable
	q, ok := c.(query.Queryable)
	if !ok {
		fmt.Fprintf(os.Stderr, "QUERY_NOT_SUPPORTED: %s does not support SQL query execution\n", parsed.Kind)
		os.Exit(1)
	}

	// Execute
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout+5)*time.Second)
	defer cancel()

	opts := query.ExecuteOpts{
		DSN:     parsed,
		SQL:     sql,
		MaxRows: *limit,
		Timeout: *timeout,
		Explain: *explain,
	}

	result, err := q.ExecQuery(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QUERY_ERROR: %v\n", err)
		os.Exit(1)
	}

	// Output
	if *human {
		fmt.Print(formatHuman(result))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// formatHuman renders a QueryResult as an ASCII table for human consumption.
func formatHuman(r *query.QueryResult) string {
	if len(r.Columns) == 0 {
		return "(empty result)\n"
	}

	// Collect column headers
	headers := make([]string, len(r.Columns))
	for i, c := range r.Columns {
		headers[i] = c.Name
	}

	// Collect row values as strings
	strRows := make([][]string, len(r.Rows))
	for i, row := range r.Rows {
		strRow := make([]string, len(row))
		for j, cell := range row {
			if cell == nil {
				strRow[j] = "NULL"
			} else {
				strRow[j] = *cell
			}
		}
		strRows[i] = strRow
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range strRows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Helper: build a separator line
	buildSep := func() string {
		var b strings.Builder
		b.WriteByte('+')
		for _, w := range widths {
			b.WriteString(strings.Repeat("-", w+2))
			b.WriteByte('+')
		}
		b.WriteByte('\n')
		return b.String()
	}

	// Helper: build a data row
	buildRow := func(cells []string) string {
		var b strings.Builder
		b.WriteByte('|')
		for i, cell := range cells {
			fmt.Fprintf(&b, " %-*s |", widths[i], cell)
		}
		b.WriteByte('\n')
		return b.String()
	}

	var out strings.Builder

	// Table
	sep := buildSep()
	out.WriteString(sep)
	out.WriteString(buildRow(headers))
	out.WriteString(sep)
	for _, row := range strRows {
		out.WriteString(buildRow(row))
	}
	out.WriteString(sep)

	// Footer: row count + execution time
	out.WriteString(fmt.Sprintf("%d row(s) in set (%s)\n", r.RowCount, r.ExecutionTime))
	if r.Truncated {
		out.WriteString("(result set was truncated)\n")
	}

	return out.String()
}

// resolveDSNEntries gathers DSN entries from env/config/dsn flags.
func resolveDSNEntries(envMode *bool, dsnFlag, configFile, label *string, dbIndex *int) []dsnEntry {
	var entries []dsnEntry

	if *envMode {
		configPath := findConfigFile()
		if configPath == "" {
			fmt.Fprintln(os.Stderr, "ERROR: no config file found")
			os.Exit(1)
		}
		if err := loadEnvFile(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, sanitizeErr(err))
			os.Exit(1)
		}
		entries = append(entries, loadFromEnv()...)
	}

	if *configFile != "" {
		for _, raw := range loadFromConfig(*configFile) {
			entries = append(entries, dsnEntry{raw: raw})
		}
	}

	if *dsnFlag != "" {
		entries = append(entries, dsnEntry{raw: *dsnFlag})
	}

	// Filter by label
	if *label != "" {
		var filtered []dsnEntry
		for _, e := range entries {
			d, err := dsn.ParseDSN(e.raw)
			if err == nil && d.Label == *label {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Filter by db index
	if *dbIndex > 0 {
		var filtered []dsnEntry
		for i, e := range entries {
			if i+1 == *dbIndex {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	return entries
}

// isSQLKind returns true for database kinds that accept SQL syntax.
func isSQLKind(kind string) bool {
	switch kind {
	case "mysql", "postgres", "gaussdb", "sqlite", "clickhouse", "elasticsearch":
		return true
	default:
		return false
	}
}
