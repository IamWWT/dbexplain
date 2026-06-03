// Package render provides table rendering utilities for query results.
package render

import (
	"fmt"
	"strings"

	"github.com/IamWWT/dbexplain/internal/query"
)

// visualWidth returns the display width of a string, where CJK/Hangul/wide
// characters count as 2 and ASCII as 1. This is needed because fmt.Printf("%-*s")
// pads by byte count, not visual width.
func visualWidth(s string) int {
	w := 0
	for _, r := range s {
		// CJK Unified Ideographs, Hangul, fullwidth forms
		if r >= 0x4E00 && r <= 0x9FFF ||
			r >= 0xAC00 && r <= 0xD7AF ||
			r >= 0x3000 && r <= 0x303F ||
			r >= 0xFF01 && r <= 0xFF60 ||
			r >= 0xFFE0 && r <= 0xFFE6 ||
			r >= 0x20000 && r <= 0x2FFFF ||
			r >= 0x30000 && r <= 0x3FFFF {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

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

	// Calculate column widths (using visual width for CJK support)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = visualWidth(h)
	}
	for _, row := range strRows {
		for i, cell := range row {
			if w := visualWidth(cell); w > widths[i] {
				widths[i] = w
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
			// Truncate if visual width exceeds column width
			if visualWidth(cell) > widths[i] {
				runes := []rune(cell)
				// Estimate: truncate to runes that fit visually
				n, w := 0, 0
				for _, r := range runes {
					rw := 1
					if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0xAC00 && r <= 0xD7AF) ||
						(r >= 0x3000 && r <= 0x303F) || (r >= 0xFF01 && r <= 0xFF60) ||
						(r >= 0xFFE0 && r <= 0xFFE6) || r >= 0x20000 {
						rw = 2
					}
					if w+rw > widths[i]-1 {
						break
					}
					w += rw
					n++
				}
				cell = string(runes[:n]) + "…"
			}
			// Adjust padding: fmt.Printf pads by bytes, compensate for CJK
			padAdjust := len(cell) - visualWidth(cell)
			if padAdjust < 0 {
				padAdjust = 0
			}
			fmt.Fprintf(&b, " %-*s |", widths[i]+padAdjust, cell)
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
