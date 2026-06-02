package queryutil

import (
	"path/filepath"
	"strings"

	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/query"
)

// FileTableName derives a table name from a file DSN's path (filename stem).
//
//	csv:///tmp/orders.csv  → "orders"
//	xlsx:///tmp/report.xlsx → "report"
//	csv:///tmp/data_dir/   → "data_dir"
func FileTableName(d *dsn.DSN) string {
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

// DetectJoinQuick does a fast string check for JOIN in SQL.
func DetectJoinQuick(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	words := strings.Fields(upper)
	for _, w := range words {
		if w == "JOIN" {
			return true
		}
	}
	return false
}

// ResolveJoinSources scans SQL for table references after FROM and JOIN,
// matches them to DSN entries (by label or filename), and loads their data.
func ResolveJoinSources(sql string, entries []config.DSNEntry) ([]query.ExtraTable, error) {
	// Extract table names from SQL
	tables := extractTableNames(sql)
	if len(tables) == 0 {
		return nil, nil
	}

	var extras []query.ExtraTable

	for _, tableName := range tables {
		matched := false
		for _, entry := range entries {
			d, err := dsn.ParseDSN(entry.Raw)
			if err != nil {
				continue
			}

			// Match by label or filename
			dsnLabel := d.Label
			dsnFile := FileTableName(d)

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
