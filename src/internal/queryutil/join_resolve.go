//go:build csv || xlsx || full

package queryutil

import (
	"strings"

	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/query"
)

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
func extractTableNames(sql string) []string {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	var tables []string
	seen := make(map[string]bool)

	fromIdx := strings.Index(upper, "FROM ")
	if fromIdx < 0 {
		return nil
	}

	afterFrom := strings.TrimSpace(sql[fromIdx+5:])
	firstWord := nextWord(afterFrom)
	if firstWord != "" && !seen[firstWord] {
		tables = append(tables, firstWord)
		seen[firstWord] = true
	}

	searchFrom := fromIdx + 5
	for {
		joinIdx := strings.Index(upper[searchFrom:], " JOIN ")
		if joinIdx < 0 {
			break
		}
		joinIdx += searchFrom + 6
		afterJoin := strings.TrimSpace(sql[joinIdx:])
		joinWord := nextWord(afterJoin)
		if joinWord != "" && !seen[joinWord] {
			tables = append(tables, joinWord)
			seen[joinWord] = true
		}
		searchFrom = joinIdx + len(joinWord)
	}

	if len(tables) > 1 {
		return tables[1:]
	}
	if len(tables) == 1 {
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
