package diff

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/schema"
)

// DiffTables compares two schema.Table objects and returns field-level changes.
// Returns nil if the tables are identical.
func DiffTables(old, new *schema.Table) *TableDiff {
	// Compare identity fields
	changes := diffColumns(old.Columns, new.Columns)
	indexChanges := diffIndexes(old.Indexes, new.Indexes)
	fkChanges := diffFKs(old.ForeignKeys, new.ForeignKeys)

	if len(changes) == 0 && len(indexChanges) == 0 && len(fkChanges) == 0 {
		return nil
	}

	return &TableDiff{
		Status:  "changed",
		Columns: changes,
		Indexes: indexChanges,
		FKs:     fkChanges,
	}
}

// DiffUniverse compares two universe snapshots and returns the full diff result.
// oldFPs / newFPs are fingerprints keyed by "instance/db/table".
// oldSnapshots / newSnapshots are full table metadata for field-level comparison.
func DiffUniverse(
	oldFPs, newFPs map[string]bool,
	oldSnapshots, newSnapshots map[string]*schema.Table,
	instanceLabel func(key string) string,
	dbName func(key string) string,
) DiffResult {
	result := DiffResult{
		ScannedAt: time.Now(),
	}

	// Build key set
	allKeys := make(map[string]bool)
	for k := range oldFPs {
		allKeys[k] = true
	}
	for k := range newFPs {
		allKeys[k] = true
	}

	// Sort for deterministic output
	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		_, inOld := oldFPs[key]
		_, inNew := newFPs[key]

		td := TableDiff{
			Instance: instanceLabel(key),
			DB:       dbName(key),
			Table:    extractTable(key),
		}

		if inOld && !inNew {
			td.Status = "removed"
			// Include old columns/indexes/FKs for reference
			if oldT, ok := oldSnapshots[key]; ok && oldT != nil {
				for _, c := range oldT.Columns {
					td.Columns = append(td.Columns, ColumnChange{
						Name:  c.Name,
						Field: "type",
						OldVal: fmt.Sprintf("%s nullable=%v default=%q comment=%q primary=%v",
							c.Type, c.Nullable, c.Default, c.Comment, c.IsPrimary),
					})
				}
			}
		} else if !inOld && inNew {
			td.Status = "added"
			if newT, ok := newSnapshots[key]; ok && newT != nil {
				for _, c := range newT.Columns {
					td.Columns = append(td.Columns, ColumnChange{
						Name:  c.Name,
						Field: "type",
						NewVal: fmt.Sprintf("%s nullable=%v default=%q comment=%q primary=%v",
							c.Type, c.Nullable, c.Default, c.Comment, c.IsPrimary),
					})
				}
			}
		} else if inOld && inNew {
			oldT, oldOK := oldSnapshots[key]
			newT, newOK := newSnapshots[key]
			if oldOK && newOK {
				td2 := DiffTables(oldT, newT)
				if td2 != nil {
					td.Status = "changed"
					td.Columns = td2.Columns
					td.Indexes = td2.Indexes
					td.FKs = td2.FKs
				} else {
					continue // no changes
				}
			}
		}

		result.Tables = append(result.Tables, td)
	}

	return result
}

// diffColumns compares two column lists and returns changes.
func diffColumns(old, new []*schema.Column) []ColumnChange {
	oldMap := make(map[string]*schema.Column)
	for _, c := range old {
		oldMap[strings.ToLower(c.Name)] = c
	}

	newMap := make(map[string]*schema.Column)
	for _, c := range new {
		newMap[strings.ToLower(c.Name)] = c
	}

	var changes []ColumnChange

	// Check for removed and changed columns
	for name, oldCol := range oldMap {
		newCol, exists := newMap[name]
		if !exists {
			changes = append(changes, ColumnChange{
				Name:   oldCol.Name,
				Field:  "type",
				OldVal: fmt.Sprintf("%s nullable=%v", oldCol.Type, oldCol.Nullable),
			})
			continue
		}
		// Compare each field
		changes = append(changes, compareColumnFields(oldCol, newCol)...)
	}

	// Check for added columns
	for name, newCol := range newMap {
		if _, exists := oldMap[name]; !exists {
			changes = append(changes, ColumnChange{
				Name:   newCol.Name,
				Field:  "type",
				NewVal: fmt.Sprintf("%s nullable=%v", newCol.Type, newCol.Nullable),
			})
		}
	}

	if len(changes) == 0 {
		return nil
	}

	// Sort by column name for deterministic output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Name < changes[j].Name
	})

	return changes
}

// compareColumnFields compares individual fields of two columns with the same name.
func compareColumnFields(old, new *schema.Column) []ColumnChange {
	var changes []ColumnChange

	if normalizeType(old.Type) != normalizeType(new.Type) {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "type",
			OldVal: old.Type,
			NewVal: new.Type,
		})
	}

	if old.Nullable != new.Nullable {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "nullable",
			OldVal: fmt.Sprintf("%v", old.Nullable),
			NewVal: fmt.Sprintf("%v", new.Nullable),
		})
	}

	if old.Default != new.Default {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "default",
			OldVal: old.Default,
			NewVal: new.Default,
		})
	}

	if old.Comment != new.Comment {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "comment",
			OldVal: old.Comment,
			NewVal: new.Comment,
		})
	}

	if old.IsPrimary != new.IsPrimary {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "is_primary",
			OldVal: fmt.Sprintf("%v", old.IsPrimary),
			NewVal: fmt.Sprintf("%v", new.IsPrimary),
		})
	}

	if old.IsUnique != new.IsUnique {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "is_unique",
			OldVal: fmt.Sprintf("%v", old.IsUnique),
			NewVal: fmt.Sprintf("%v", new.IsUnique),
		})
	}

	if old.IsIndex != new.IsIndex {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "is_index",
			OldVal: fmt.Sprintf("%v", old.IsIndex),
			NewVal: fmt.Sprintf("%v", new.IsIndex),
		})
	}

	if old.IsSortKey != new.IsSortKey {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "is_sort_key",
			OldVal: fmt.Sprintf("%v", old.IsSortKey),
			NewVal: fmt.Sprintf("%v", new.IsSortKey),
		})
	}

	if old.IsPartitionKey != new.IsPartitionKey {
		changes = append(changes, ColumnChange{
			Name:   old.Name,
			Field:  "is_partition_key",
			OldVal: fmt.Sprintf("%v", old.IsPartitionKey),
			NewVal: fmt.Sprintf("%v", new.IsPartitionKey),
		})
	}

	return changes
}

// diffIndexes compares old and new index lists.
func diffIndexes(old, new []*schema.Index) []IndexChange {
	oldMap := make(map[string]*schema.Index)
	for _, idx := range old {
		oldMap[strings.ToLower(idx.Name)] = idx
	}

	newMap := make(map[string]*schema.Index)
	for _, idx := range new {
		newMap[strings.ToLower(idx.Name)] = idx
	}

	var changes []IndexChange

	// Find removed indexes
	for name, oldIdx := range oldMap {
		if _, exists := newMap[name]; !exists {
			changes = append(changes, IndexChange{
				Name:       oldIdx.Name,
				Status:     "removed",
				OldColumns: oldIdx.Columns,
				OldUnique:  oldIdx.Unique,
				OldType:    oldIdx.Type,
			})
		}
	}

	// Find added and changed indexes
	for name, newIdx := range newMap {
		oldIdx, exists := oldMap[name]
		if !exists {
			changes = append(changes, IndexChange{
				Name:       newIdx.Name,
				Status:     "added",
				NewColumns: newIdx.Columns,
				NewUnique:  newIdx.Unique,
				NewType:    newIdx.Type,
			})
			continue
		}
		// Check for changes
		if !stringSliceEqual(oldIdx.Columns, newIdx.Columns) ||
			oldIdx.Unique != newIdx.Unique ||
			oldIdx.Type != newIdx.Type {
			changes = append(changes, IndexChange{
				Name:        newIdx.Name,
				Status:      "changed",
				OldColumns:  oldIdx.Columns,
				NewColumns:  newIdx.Columns,
				OldUnique:   oldIdx.Unique,
				NewUnique:   newIdx.Unique,
				OldType:     oldIdx.Type,
				NewType:     newIdx.Type,
			})
		}
	}

	if len(changes) == 0 {
		return nil
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Name < changes[j].Name
	})
	return changes
}

// diffFKs compares old and new foreign key lists.
func diffFKs(old, new []*schema.ForeignKey) []FKChange {
	oldMap := make(map[string]*schema.ForeignKey)
	for _, fk := range old {
		oldMap[strings.ToLower(fk.Name)] = fk
	}

	newMap := make(map[string]*schema.ForeignKey)
	for _, fk := range new {
		newMap[strings.ToLower(fk.Name)] = fk
	}

	var changes []FKChange

	// Find removed FKs
	for name, oldFK := range oldMap {
		if _, exists := newMap[name]; !exists {
			changes = append(changes, FKChange{
				Name:          oldFK.Name,
				Status:        "removed",
				OldColumns:    oldFK.Columns,
				OldRefTable:   oldFK.RefTable,
				OldRefColumns: oldFK.RefColumns,
			})
		}
	}

	// Find added and changed FKs
	for name, newFK := range newMap {
		oldFK, exists := oldMap[name]
		if !exists {
			changes = append(changes, FKChange{
				Name:          newFK.Name,
				Status:        "added",
				NewColumns:    newFK.Columns,
				NewRefTable:   newFK.RefTable,
				NewRefColumns: newFK.RefColumns,
			})
			continue
		}
		// Check for changes
		if !stringSliceEqual(oldFK.Columns, newFK.Columns) ||
			oldFK.RefTable != newFK.RefTable ||
			!stringSliceEqual(oldFK.RefColumns, newFK.RefColumns) {
			changes = append(changes, FKChange{
				Name:          newFK.Name,
				Status:        "changed",
				OldColumns:    oldFK.Columns,
				NewColumns:    newFK.Columns,
				OldRefTable:   oldFK.RefTable,
				NewRefTable:   newFK.RefTable,
				OldRefColumns: oldFK.RefColumns,
				NewRefColumns: newFK.RefColumns,
			})
		}
	}

	if len(changes) == 0 {
		return nil
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Name < changes[j].Name
	})
	return changes
}

// normalizeType normalizes type strings for comparison.
func normalizeType(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

// stringSliceEqual compares two string slices element-by-element.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// extractTable extracts the table name from a "instance/db/table" key.
func extractTable(key string) string {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return key
}

// Helper to extract instance from key.
func keyInstance(key string) string {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// Helper to extract db from key.
func keyDB(key string) string {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// NewInstanceLabelFunc creates a key→instance label function.
func NewInstanceLabelFunc() func(string) string {
	return keyInstance
}

// NewDBNameFunc creates a key→db name function.
func NewDBNameFunc() func(string) string {
	return keyDB
}
