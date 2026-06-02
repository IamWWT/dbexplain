// Package diff provides field-level schema comparison between database scans.
//
// It produces structured change descriptions (ColumnChange, IndexChange, FKChange)
// that can be rendered as JSON or Markdown tables. Unlike cache.Delta which only
// reports table-level change status, diff provides per-field granularity.
package diff

import "time"

// ColumnChange describes a single field-level change in a column.
type ColumnChange struct {
	Name   string `json:"name"`            // column name
	Field  string `json:"field"`           // changed property: "type" | "nullable" | "default" | "comment" | "is_primary" | "is_unique" | "is_index"
	OldVal string `json:"old,omitempty"`   // previous value
	NewVal string `json:"new,omitempty"`   // current value
}

// IndexChange describes a change to an index.
type IndexChange struct {
	Name   string `json:"name"`            // index name
	Status string `json:"status"`          // "added" | "removed" | "changed"
	// For changed indexes
	OldColumns []string `json:"old_columns,omitempty"`
	NewColumns []string `json:"new_columns,omitempty"`
	OldUnique  bool     `json:"old_unique,omitempty"`
	NewUnique  bool     `json:"new_unique,omitempty"`
	OldType    string   `json:"old_type,omitempty"`
	NewType    string   `json:"new_type,omitempty"`
}

// FKChange describes a change to a foreign key.
type FKChange struct {
	Name   string `json:"name"`            // FK constraint name
	Status string `json:"status"`          // "added" | "removed" | "changed"
	// For changed FKs
	OldColumns    []string `json:"old_columns,omitempty"`
	NewColumns    []string `json:"new_columns,omitempty"`
	OldRefTable   string   `json:"old_ref_table,omitempty"`
	NewRefTable   string   `json:"new_ref_table,omitempty"`
	OldRefColumns []string `json:"old_ref_columns,omitempty"`
	NewRefColumns []string `json:"new_ref_columns,omitempty"`
}

// TableDiff captures all changes for a single table across two snapshots.
type TableDiff struct {
	Instance string         `json:"instance"`
	DB       string         `json:"db"`
	Table    string         `json:"table"`
	Status   string         `json:"status"` // "added" | "removed" | "changed"
	Columns  []ColumnChange `json:"columns,omitempty"`
	Indexes  []IndexChange  `json:"indexes,omitempty"`
	FKs      []FKChange     `json:"fks,omitempty"`
}

// DiffResult is the top-level diff output for a complete schema comparison.
type DiffResult struct {
	Tables    []TableDiff `json:"tables"`
	ScannedAt time.Time   `json:"scanned_at"`
}
