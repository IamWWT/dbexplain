//go:build !csv && !xlsx && !full

package queryutil

import (
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/query"
)

// ResolveJoinSources returns nil when csv and xlsx connectors are not compiled in.
func ResolveJoinSources(sql string, entries []config.DSNEntry) ([]query.ExtraTable, error) {
	return nil, nil
}
