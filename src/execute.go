package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/capabilities"
	"github.com/IamWWT/dbexplain/connector"
	"github.com/IamWWT/dbexplain/dsn"
	"github.com/IamWWT/dbexplain/policy"
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

	// Go flag.FlagSet stops at first non-flag arg (the SQL query).
	// Allow flags like --human after the query for convenience.
	for _, a := range fs.Args() {
		if a == "--human" {
			*human = true
		}
	}

	sqlArg := fs.Arg(0)
	if sqlArg == "" {
		fmt.Fprintln(os.Stderr, "READ_ONLY_VIOLATION: empty query")
		os.Exit(1)
	}

	// Resolve DSN — gather ALL entries before label filter (needed for JOIN source resolution)
	var allEntries []dsnEntry
	if *envMode {
		configPath := findConfigFile()
		if configPath == "" {
			fmt.Fprintln(os.Stderr, "ERROR: no config file found")
			os.Exit(1)
		}
		envEntries, err := loadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, sanitizeErr(err))
			os.Exit(1)
		}
		allEntries = append(allEntries, envEntries...)
	}
	if *configFile != "" {
		for _, raw := range loadFromConfig(*configFile) {
			allEntries = append(allEntries, dsnEntry{raw: raw})
		}
	}
	if *dsnFlag != "" {
		allEntries = append(allEntries, dsnEntry{raw: *dsnFlag})
	}

	// Now filter by label/dbIndex to select the primary DSN for this query
	entries := filterEntries(allEntries, label, dbIndex)
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no matching DSN found")
		os.Exit(1)
	}
	if len(entries) > 1 {
		msg := fmt.Sprintf("ERROR: %d DSNs matched — use --label to select one:\n", len(entries))
		for _, e := range entries {
			d, err := dsn.ParseDSN(e.raw)
			if err == nil {
				msg += fmt.Sprintf("  --label %s (%s)\n", d.Label, d.FilePath())
			}
		}
		fmt.Fprint(os.Stderr, msg)
		os.Exit(1)
	}

	parsed, err := dsn.ParseDSN(entries[0].raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: invalid DSN: %v\n", sanitizeErr(err))
		os.Exit(1)
	}

	// Get connector early — used for capability-based routing.
	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	caps := capabilities.FromProvider(c)
	isSQL := caps.Has(capabilities.CapSQL)
	isFile := caps.Has(capabilities.CapFile)

	// File datasources (CSV, XLSX): bypass sqlguard (SELECT * only — inherently read-only),
	// but still enforce policy engine rules (DENY_TABLES, MASK_COLUMNS).
	if isFile {
		policies := policy.Load(entries[0].envKey)
		handleFileExecute(parsed, sqlArg, human, limit, policies, allEntries)
		return
	}

	// Only validate SQL for SQL-based connectors. Native connectors
	// (Redis, MongoDB, Qdrant) do their own validation in ExecQuery.
	if isSQL {
		if err := sqlguard.Validate(sqlArg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Fine-grained policy check (applies to ALL database kinds)
	policies := policy.Load(entries[0].envKey)
	if isSQL {
		if err := policies.CheckSQL(sqlArg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		if err := policies.CheckNative(sqlArg, parsed.Kind); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Apply auto-limit and optional EXPLAIN wrapping (SQL only)
	var sql string
	if isSQL {
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

	// Apply post-execution column masking (replaces sensitive values)
	policies.ApplyMask(result)

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

// handleFileExecute handles execute for csv/xlsx — skips sqlguard (SELECT * only),
// but enforces policy engine rules for compliance scenarios.
// allEntries provides all env DSN entries for JOIN source resolution.
func handleFileExecute(parsed *dsn.DSN, sqlArg string, human *bool, limit *int, policies *policy.Config, allEntries []dsnEntry) {
	// DENY_TABLES check: derive table name from DSN path (filename without extension)
	if policies != nil && len(policies.DenyTables) > 0 {
		if tableName := fileTableName(parsed); tableName != "" {
			for _, denied := range policies.DenyTables {
				if strings.EqualFold(tableName, denied) {
					fmt.Fprintf(os.Stderr, "ACCESS_DENIED: table %q is not allowed for query\n", denied)
					os.Exit(1)
				}
			}
		}
	}

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	q, ok := c.(query.Queryable)
	if !ok {
		fmt.Fprintf(os.Stderr, "QUERY_NOT_SUPPORTED: %s does not support query execution\n", parsed.Kind)
		os.Exit(1)
	}

	ctx := context.Background()
	opts := query.ExecuteOpts{
		DSN:     parsed,
		SQL:     sqlArg,
		MaxRows: *limit,
	}

	// Resolve JOIN sources: load data from additional DSNs referenced in SQL
	if detectJoinQuick(sqlArg) && len(allEntries) > 1 {
		if joinTables, err := resolveJoinSources(sqlArg, allEntries); err == nil && len(joinTables) > 0 {
			opts.ExtraTables = joinTables
		}
	}

	result, err := q.ExecQuery(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QUERY_ERROR: %v\n", err)
		os.Exit(1)
	}

	// Apply post-execution column masking (replaces sensitive values)
	if policies != nil {
		policies.ApplyMask(result)
	}

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

// fileTableName derives a table name from a file DSN's path (filename stem).
//   csv:///tmp/orders.csv  → "orders"
//   xlsx:///tmp/report.xlsx → "report"
//   csv:///tmp/data_dir/   → "data_dir"
func fileTableName(d *dsn.DSN) string {
	path := d.FilePath()
	if path == "" {
		return ""
	}
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}

// sanitizeCell strips ANSI escape codes and control characters from cell values
// to prevent terminal injection. Allows tab, newline, and printable characters.
func sanitizeCell(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		// Strip ANSI escape sequences: ESC + '[' + parameters + letter
		if s[i] == 27 {
			if i+1 < len(s) && s[i+1] == '[' {
				i += 2 // skip ESC and '['
				for ; i < len(s); i++ {
					if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
						break
					}
				}
				continue
			}
			// lone ESC without '[' — just skip it
			continue
		}
		// Allow tab, newline, carriage return
		if s[i] == '\t' || s[i] == '\n' || s[i] == '\r' {
			b.WriteByte(s[i])
			continue
		}
		// Strip other control characters (0-31, 127)
		if s[i] < 32 || s[i] == 127 {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
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
				strRow[j] = sanitizeCell(*cell)
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
	// Cap column widths to prevent OOM from huge cell values
	const maxColWidth = 256
	for i := range widths {
		if widths[i] > maxColWidth {
			widths[i] = maxColWidth
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
			if len(cell) > widths[i] {
				cell = cell[:widths[i]-1] + "…"
			}
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

// filterEntries filters DSN entries by label or dbIndex.
func filterEntries(entries []dsnEntry, label *string, dbIndex *int) []dsnEntry {
	var filtered []dsnEntry

	// Filter by label
	if *label != "" {
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
		// Re-slice from entries filtered by label
		for i, e := range entries {
			if i+1 == *dbIndex {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	return entries
}

// --- JOIN source resolution ---

// detectJoinQuick does a fast string check for JOIN in SQL.
func detectJoinQuick(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	words := strings.Fields(upper)
	for _, w := range words {
		if w == "JOIN" {
			return true
		}
	}
	return false
}

// resolveJoinSources scans SQL for table references after FROM and JOIN,
// matches them to DSN entries (by label or filename), and loads their data.
func resolveJoinSources(sql string, entries []dsnEntry) ([]query.ExtraTable, error) {
	// Extract table names from SQL
	tables := extractTableNames(sql)
	if len(tables) == 0 {
		return nil, nil
	}

	var extras []query.ExtraTable

	for _, tableName := range tables {
		matched := false
		for _, entry := range entries {
			d, err := dsn.ParseDSN(entry.raw)
			if err != nil {
				continue
			}

			// Match by label or filename
			dsnLabel := d.Label
			dsnFile := fileTableName(d)

			if strings.EqualFold(tableName, dsnLabel) || strings.EqualFold(tableName, dsnFile) {
				// Skip if this is the primary DSN (already loaded)
				if matched {
					continue
				}

				// Load data
				switch d.Kind {
				case "csv", "tsv":
					delimiter := connector.GetDelimiter(d)
					encoding := d.DSNParam("encoding")
					if encoding == "" {
						encoding = "utf-8"
					}
					rows, header, err := connector.ReadCSVFile(d.FilePath(), delimiter, encoding)
					if err != nil {
						continue
					}
					extras = append(extras, query.ExtraTable{
						Alias:  tableName,
						Header: header,
						Rows:   rows,
					})
					matched = true

				case "xlsx":
					// Load all sheets — each sheet is available as a table
					sheets, err := connector.ReadXLSXSheets(d.FilePath())
					if err != nil {
						continue
					}
					for _, s := range sheets {
						extras = append(extras, query.ExtraTable{
							Alias:  s.Alias,
							Header: s.Header,
							Rows:   s.Rows,
						})
					}
					matched = true
				}
			}
		}
	}

	return extras, nil
}

// extractTableNames extracts table names from FROM and JOIN clauses using simple string parsing.
// e.g., "SELECT * FROM pb_touch_ops t JOIN t_sec_org o ON t.org_refno = o.org_refno"
// returns: ["pb_touch_ops", "t_sec_org"]
func extractTableNames(sql string) []string {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	var tables []string
	seen := make(map[string]bool)

	// Find FROM clause
	fromIdx := strings.Index(upper, "FROM ")
	if fromIdx < 0 {
		return nil
	}

	// Extract table name after FROM
	afterFrom := strings.TrimSpace(sql[fromIdx+5:])
	firstWord := nextWord(afterFrom)
	if firstWord != "" && !seen[firstWord] {
		tables = append(tables, firstWord)
		seen[firstWord] = true
	}

	// Find all JOIN clauses
	searchFrom := fromIdx + 5
	for {
		joinIdx := strings.Index(upper[searchFrom:], " JOIN ")
		if joinIdx < 0 {
			break
		}
		joinIdx += searchFrom + 6 // skip " JOIN "
		afterJoin := strings.TrimSpace(sql[joinIdx:])
		joinWord := nextWord(afterJoin)
		if joinWord != "" && !seen[joinWord] {
			tables = append(tables, joinWord)
			seen[joinWord] = true
		}
		searchFrom = joinIdx + len(joinWord)
	}

	// Remove the primary table (first one) — it's already loaded via DSN
	if len(tables) > 1 {
		return tables[1:]
	}
	if len(tables) == 1 {
		// If only one table found (no JOIN), return empty
		// This means the SQL has a FROM but no JOIN — not our concern
		return nil
	}
	return tables
}

// nextWord extracts the first word from a SQL fragment.
func nextWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	end := strings.IndexAny(s, " \t\n\r")
	if end < 0 {
		return s
	}
	return s[:end]
}

