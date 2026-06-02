// Package dsnfilter provides DSN filtering utilities for query execution.
package dsnfilter

import (
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
)

// FilterEntries filters DSN entries by label or dbIndex.
func FilterEntries(entries []config.DSNEntry, label *string, dbIndex *int) []config.DSNEntry {
	var filtered []config.DSNEntry

	// Filter by label
	if *label != "" {
		for _, e := range entries {
			d, err := dsn.ParseDSN(e.Raw)
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
