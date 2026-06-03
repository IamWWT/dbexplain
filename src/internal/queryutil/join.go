package queryutil

import (
	"path/filepath"
	"strings"

	"github.com/IamWWT/dbexplain/internal/dsn"
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
