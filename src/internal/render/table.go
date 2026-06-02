// Package render provides table rendering utilities for query results.
package render

import (
	"fmt"
	"strings"

	"github.com/IamWWT/dbexplain/internal/query"
)

const maxColWidth = 256

// FormatHuman renders a QueryResult as an ASCII table for human consumption.
func FormatHuman(r *query.QueryResult) string {
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
				strRow[j] = SanitizeCell(*cell)
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

// SanitizeCell strips ANSI escape codes and control characters from cell values
// to prevent terminal injection. Allows tab, newline, and printable characters.
func SanitizeCell(s string) string {
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
