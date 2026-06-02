// Package queryutil provides query execution utilities for file datasources.
package queryutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	internalrender "github.com/IamWWT/dbexplain/internal/render"
	"github.com/IamWWT/dbexplain/internal/policy"
	"github.com/IamWWT/dbexplain/internal/query"
)

// HandleFileExecute handles execute for csv/xlsx — skips sqlguard (SELECT * only),
// but enforces policy engine rules for compliance scenarios.
// allEntries provides all env DSN entries for JOIN source resolution.
func HandleFileExecute(parsed *dsn.DSN, sqlArg string, human bool, limit int, policies *policy.Config, allEntries []config.DSNEntry) {
	// DENY_TABLES check: derive table name from DSN path (filename without extension)
	if policies != nil && len(policies.DenyTables) > 0 {
		if tableName := FileTableName(parsed); tableName != "" {
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
		MaxRows: limit,
	}

	// Resolve JOIN sources: load data from additional DSNs referenced in SQL
	if DetectJoinQuick(sqlArg) && len(allEntries) > 1 {
		if joinTables, err := ResolveJoinSources(sqlArg, allEntries); err == nil && len(joinTables) > 0 {
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

	if human {
		fmt.Print(internalrender.FormatHuman(result))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}
