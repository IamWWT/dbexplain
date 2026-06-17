package connector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/IamWWT/dbexplain/internal/query"
)

// executeSQLQuery runs a read-only SQL query using database/sql and
// returns a QueryResult. This is the shared implementation used by
// MySQL, PostgreSQL, GaussDB, and SQLite connector ExecQuery methods.
func executeSQLQuery(ctx context.Context, db *sql.DB, sqlStr string, maxRows int) (*query.QueryResult, error) {
	start := time.Now()

	// Log SQL with truncation for safety
	logSQL := TruncateSQL(sqlStr)
	Logf(ctx, "[query] [execute] %s", logSQL)

	rows, err := db.QueryContext(ctx, sqlStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get column metadata
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("column types: %w", err)
	}
	colNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	result := &query.QueryResult{
		Columns: make([]query.ColumnInfo, len(colTypes)),
	}
	for i, ct := range colTypes {
		result.Columns[i] = query.ColumnInfo{
			Name: colNames[i],
			Type: ct.DatabaseTypeName(),
		}
	}

	// Scan rows
	rowCount := 0
	for rows.Next() {
		if rowCount >= maxRows {
			result.Truncated = true
			break
		}

		// Create scan targets
		scanArgs := make([]interface{}, len(colNames))
		for i := range scanArgs {
			var v interface{}
			scanArgs[i] = &v
		}

		if err := rows.Scan(scanArgs...); err != nil {
			log.Printf("[query] scan row: %v", err)
			continue
		}

		row := make([]*string, len(colNames))
		for i, arg := range scanArgs {
			val := arg.(*interface{})
			if *val == nil {
				row[i] = nil
			} else {
				s := fmt.Sprintf("%v", *val)
				// Handle []byte → string for MySQL driver
				if b, ok := (*val).([]byte); ok {
					s = string(b)
				}
				row[i] = &s
			}
		}
		result.Rows = append(result.Rows, row)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	result.RowCount = rowCount
	result.ExecutionTime = time.Since(start).String()
	return result, nil
}
