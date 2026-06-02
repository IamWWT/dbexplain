package diff

import (
	"encoding/json"
	"testing"

	"github.com/IamWWT/dbexplain/internal/schema"
)

func TestDiffColumns_Identical(t *testing.T) {
	old := []*schema.Column{
		{Name: "id", Type: "INTEGER", Nullable: false, IsPrimary: true},
		{Name: "name", Type: "TEXT", Nullable: true, Comment: "user name"},
	}
	new := []*schema.Column{
		{Name: "id", Type: "INTEGER", Nullable: false, IsPrimary: true},
		{Name: "name", Type: "TEXT", Nullable: true, Comment: "user name"},
	}

	changes := diffColumns(old, new)
	if changes != nil {
		t.Errorf("expected nil for identical columns, got %d changes", len(changes))
	}
}

func TestDiffColumns_Added(t *testing.T) {
	old := []*schema.Column{
		{Name: "id", Type: "INTEGER"},
	}
	new := []*schema.Column{
		{Name: "id", Type: "INTEGER"},
		{Name: "email", Type: "TEXT", Nullable: true},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "email" || changes[0].Field != "type" {
		t.Errorf("expected email/type change, got %s/%s", changes[0].Name, changes[0].Field)
	}
	if changes[0].NewVal == "" {
		t.Errorf("expected NewVal to be set for added column")
	}
}

func TestDiffColumns_Removed(t *testing.T) {
	old := []*schema.Column{
		{Name: "id", Type: "INTEGER"},
		{Name: "age", Type: "INTEGER"},
	}
	new := []*schema.Column{
		{Name: "id", Type: "INTEGER"},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Name != "age" || changes[0].Field != "type" {
		t.Errorf("expected age/type change, got %s/%s", changes[0].Name, changes[0].Field)
	}
	if changes[0].OldVal == "" {
		t.Errorf("expected OldVal to be set for removed column")
	}
}

func TestDiffColumns_TypeChange(t *testing.T) {
	old := []*schema.Column{
		{Name: "rate", Type: "INTEGER"},
	}
	new := []*schema.Column{
		{Name: "rate", Type: "FLOAT"},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "type" || changes[0].OldVal != "INTEGER" || changes[0].NewVal != "FLOAT" {
		t.Errorf("unexpected change: %+v", changes[0])
	}
}

func TestDiffColumns_NullableChange(t *testing.T) {
	old := []*schema.Column{
		{Name: "phone", Type: "TEXT", Nullable: true},
	}
	new := []*schema.Column{
		{Name: "phone", Type: "TEXT", Nullable: false},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "nullable" {
		t.Errorf("expected nullable change, got %s", changes[0].Field)
	}
}

func TestDiffColumns_CommentChange(t *testing.T) {
	old := []*schema.Column{
		{Name: "status", Type: "TEXT", Comment: "old comment"},
	}
	new := []*schema.Column{
		{Name: "status", Type: "TEXT", Comment: "new comment"},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "comment" {
		t.Errorf("expected comment change, got %s", changes[0].Field)
	}
}

func TestDiffColumns_DefaultChange(t *testing.T) {
	old := []*schema.Column{
		{Name: "flag", Type: "INTEGER", Default: "0"},
	}
	new := []*schema.Column{
		{Name: "flag", Type: "INTEGER", Default: "1"},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "default" {
		t.Errorf("expected default change, got %s", changes[0].Field)
	}
}

func TestDiffColumns_PrimaryKeyChange(t *testing.T) {
	old := []*schema.Column{
		{Name: "id", Type: "INTEGER", IsPrimary: false},
	}
	new := []*schema.Column{
		{Name: "id", Type: "INTEGER", IsPrimary: true},
	}

	changes := diffColumns(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "is_primary" {
		t.Errorf("expected is_primary change, got %s", changes[0].Field)
	}
}

func TestDiffColumns_MultipleChanges(t *testing.T) {
	old := []*schema.Column{
		{Name: "user", Type: "VARCHAR(100)", Nullable: true, Default: "", Comment: "old"},
		{Name: "id", Type: "INTEGER", IsPrimary: true},
	}
	new := []*schema.Column{
		{Name: "user", Type: "VARCHAR(200)", Nullable: false, Default: "admin", Comment: "new"},
		{Name: "id", Type: "INTEGER", IsPrimary: true},
		{Name: "extra", Type: "TEXT"},
	}

	changes := diffColumns(old, new)
	if changes == nil {
		t.Fatal("expected changes, got nil")
	}

	// user should have 4 field changes
	typeCount := 0
	for _, c := range changes {
		if c.Name == "user" {
			typeCount++
		}
	}
	if typeCount != 4 {
		t.Errorf("expected 4 changes for 'user' column, got %d", typeCount)
	}

	// extra should be added
	foundExtra := false
	for _, c := range changes {
		if c.Name == "extra" {
			foundExtra = true
			break
		}
	}
	if !foundExtra {
		t.Error("expected 'extra' column to be marked as added")
	}
}

func TestDiffIndexes_Identical(t *testing.T) {
	old := []*schema.Index{
		{Name: "idx_name", Columns: []string{"name"}, Unique: false},
	}
	new := []*schema.Index{
		{Name: "idx_name", Columns: []string{"name"}, Unique: false},
	}

	changes := diffIndexes(old, new)
	if changes != nil {
		t.Errorf("expected nil for identical indexes, got %d changes", len(changes))
	}
}

func TestDiffIndexes_Added(t *testing.T) {
	old := []*schema.Index{}
	new := []*schema.Index{
		{Name: "idx_email", Columns: []string{"email"}, Unique: true, Type: "BTREE"},
	}

	changes := diffIndexes(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != "added" {
		t.Errorf("expected added status, got %s", changes[0].Status)
	}
}

func TestDiffIndexes_Removed(t *testing.T) {
	old := []*schema.Index{
		{Name: "idx_old", Columns: []string{"legacy"}},
	}
	new := []*schema.Index{}

	changes := diffIndexes(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != "removed" {
		t.Errorf("expected removed status, got %s", changes[0].Status)
	}
}

func TestDiffIndexes_ColumnChange(t *testing.T) {
	old := []*schema.Index{
		{Name: "idx_cols", Columns: []string{"a", "b"}},
	}
	new := []*schema.Index{
		{Name: "idx_cols", Columns: []string{"a", "b", "c"}},
	}

	changes := diffIndexes(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != "changed" {
		t.Errorf("expected changed status, got %s", changes[0].Status)
	}
}

func TestDiffFKs_Identical(t *testing.T) {
	old := []*schema.ForeignKey{
		{Name: "fk_uid", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
	}
	new := []*schema.ForeignKey{
		{Name: "fk_uid", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
	}

	changes := diffFKs(old, new)
	if changes != nil {
		t.Errorf("expected nil for identical FKs, got %d changes", len(changes))
	}
}

func TestDiffFKs_Added(t *testing.T) {
	old := []*schema.ForeignKey{}
	new := []*schema.ForeignKey{
		{Name: "fk_order_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}},
	}

	changes := diffFKs(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != "added" {
		t.Errorf("expected added status, got %s", changes[0].Status)
	}
}

func TestDiffFKs_RefTableChange(t *testing.T) {
	old := []*schema.ForeignKey{
		{Name: "fk_ref", Columns: []string{"oid"}, RefTable: "old_table", RefColumns: []string{"id"}},
	}
	new := []*schema.ForeignKey{
		{Name: "fk_ref", Columns: []string{"oid"}, RefTable: "new_table", RefColumns: []string{"id"}},
	}

	changes := diffFKs(old, new)
	if changes == nil || len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != "changed" {
		t.Errorf("expected changed status, got %s", changes[0].Status)
	}
	if changes[0].OldRefTable != "old_table" || changes[0].NewRefTable != "new_table" {
		t.Errorf("unexpected ref table values: old=%s new=%s", changes[0].OldRefTable, changes[0].NewRefTable)
	}
}

func TestDiffTables_NoChange(t *testing.T) {
	old := &schema.Table{
		Name: "test",
		Columns: []*schema.Column{
			{Name: "id", Type: "INTEGER"},
		},
	}
	new := &schema.Table{
		Name: "test",
		Columns: []*schema.Column{
			{Name: "id", Type: "INTEGER"},
		},
	}

	result := DiffTables(old, new)
	if result != nil {
		t.Errorf("expected nil for identical tables, got %+v", result)
	}
}

func TestDiffTables_WithChanges(t *testing.T) {
	old := &schema.Table{
		Name: "test",
		Columns: []*schema.Column{
			{Name: "id", Type: "INTEGER", IsPrimary: true},
			{Name: "name", Type: "TEXT"},
		},
		Indexes: []*schema.Index{
			{Name: "idx_name", Columns: []string{"name"}},
		},
	}
	new := &schema.Table{
		Name: "test",
		Columns: []*schema.Column{
			{Name: "id", Type: "INTEGER", IsPrimary: true},
			{Name: "name", Type: "VARCHAR(255)"},
			{Name: "email", Type: "TEXT"},
		},
		Indexes: []*schema.Index{
			{Name: "idx_name", Columns: []string{"name"}},
			{Name: "idx_email", Columns: []string{"email"}, Unique: true},
		},
	}

	result := DiffTables(old, new)
	if result == nil {
		t.Fatal("expected changes, got nil")
	}
	if result.Status != "changed" {
		t.Errorf("expected status=changed, got %s", result.Status)
	}
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 column changes (type change + added), got %d", len(result.Columns))
	}
	if len(result.Indexes) != 1 {
		t.Errorf("expected 1 index change, got %d", len(result.Indexes))
	}
}

func TestDiffTables_NilInput(t *testing.T) {
	// Should not panic on nil
	old := &schema.Table{
		Name:    "test",
		Columns: nil,
	}
	new := &schema.Table{
		Name:    "test",
		Columns: nil,
	}

	result := DiffTables(old, new)
	if result != nil {
		t.Errorf("expected nil for nil-column tables, got %+v", result)
	}
}

func TestDiffTables_TypeOrderInsensitive(t *testing.T) {
	// Column comparison should be case-insensitive for names
	old := []*schema.Column{
		{Name: "ID", Type: "INTEGER"},
	}
	new := []*schema.Column{
		{Name: "id", Type: "INTEGER"},
	}

	changes := diffColumns(old, new)
	if changes != nil {
		t.Errorf("expected no changes for case-insensitive name match, got %d", len(changes))
	}
}

func TestDiffUniverse_AllScenarios(t *testing.T) {
	oldFPs := map[string]bool{
		"mysql/mydb/users":  true,
		"mysql/mydb/orders": true,
		"mysql/mydb/legacy": true,
	}
	newFPs := map[string]bool{
		"mysql/mydb/users":  true,
		"mysql/mydb/orders": true,
		"mysql/mydb/audit":  true,
	}

	oldSnapshots := map[string]*schema.Table{
		"mysql/mydb/users": {
			Name: "users",
			Columns: []*schema.Column{
				{Name: "id", Type: "INTEGER", IsPrimary: true},
				{Name: "name", Type: "VARCHAR(100)"},
			},
		},
		"mysql/mydb/orders": {
			Name: "orders",
			Columns: []*schema.Column{
				{Name: "id", Type: "INTEGER"},
				{Name: "status", Type: "TEXT", Comment: "old comment"},
			},
		},
		"mysql/mydb/legacy": {
			Name: "legacy",
			Columns: []*schema.Column{
				{Name: "data", Type: "TEXT"},
			},
		},
	}
	newSnapshots := map[string]*schema.Table{
		"mysql/mydb/users": {
			Name: "users",
			Columns: []*schema.Column{
				{Name: "id", Type: "INTEGER", IsPrimary: true},
				{Name: "name", Type: "VARCHAR(100)"},
			},
		},
		"mysql/mydb/orders": {
			Name: "orders",
			Columns: []*schema.Column{
				{Name: "id", Type: "INTEGER"},
				{Name: "status", Type: "TEXT", Comment: "updated comment"},
			},
		},
		"mysql/mydb/audit": {
			Name: "audit",
			Columns: []*schema.Column{
				{Name: "entry_id", Type: "INTEGER"},
			},
		},
	}

	instanceFn := func(key string) string { return "mysql" }
	dbFn := func(key string) string { return "mydb" }

	result := DiffUniverse(oldFPs, newFPs, oldSnapshots, newSnapshots, instanceFn, dbFn)

	// unchanged tables (users) are skipped — only added/removed/changed appear
	if len(result.Tables) != 3 {
		t.Fatalf("expected 3 table diffs (added/removed/changed), got %d", len(result.Tables))
	}

	// Check "removed" table (legacy)
	foundRemoved := false
	// Check "added" table (audit)
	foundAdded := false
	// Check "changed" table (orders)
	foundChanged := false
	// "unchanged" table (users) should be skipped

	for _, td := range result.Tables {
		switch td.Table {
		case "legacy":
			if td.Status != "removed" {
				t.Errorf("legacy expected removed, got %s", td.Status)
			}
			foundRemoved = true
		case "audit":
			if td.Status != "added" {
				t.Errorf("audit expected added, got %s", td.Status)
			}
			foundAdded = true
		case "orders":
			if td.Status != "changed" {
				t.Errorf("orders expected changed, got %s", td.Status)
			}
			if len(td.Columns) != 1 || td.Columns[0].Field != "comment" {
				t.Errorf("orders expected comment change, got %+v", td.Columns)
			}
			foundChanged = true
		case "users":
			t.Error("users should be skipped (no changes)")
		}
	}

	if !foundRemoved {
		t.Error("legacy table not found in results")
	}
	if !foundAdded {
		t.Error("audit table not found in results")
	}
	if !foundChanged {
		t.Error("orders table not found in results")
	}
}

func TestDiffUniverse_EmptySnapshots(t *testing.T) {
	result := DiffUniverse(
		map[string]bool{},
		map[string]bool{},
		map[string]*schema.Table{},
		map[string]*schema.Table{},
		func(string) string { return "" },
		func(string) string { return "" },
	)

	if len(result.Tables) != 0 {
		t.Errorf("expected 0 tables for empty input, got %d", len(result.Tables))
	}
}

func TestJSONSerialization(t *testing.T) {
	td := TableDiff{
		Instance: "mysql",
		DB:       "mydb",
		Table:    "users",
		Status:   "changed",
		Columns: []ColumnChange{
			{Name: "email", Field: "type", OldVal: "TEXT", NewVal: "VARCHAR(255)"},
		},
		Indexes: []IndexChange{
			{Name: "idx_email", Status: "added", NewColumns: []string{"email"}, NewUnique: true},
		},
	}

	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var decoded TableDiff
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if decoded.Table != "users" || decoded.Status != "changed" {
		t.Errorf("unexpected decoded values: %+v", decoded)
	}
	if len(decoded.Columns) != 1 || decoded.Columns[0].Name != "email" {
		t.Errorf("unexpected columns: %+v", decoded.Columns)
	}
}
