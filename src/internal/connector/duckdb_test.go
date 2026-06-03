//go:build duckdb

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func TestDuckDBConnector(t *testing.T) {
	// Use the pre-built test database
	dbPath := "/tmp/dbexplain_test.duckdb"
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Skipf("DuckDB not available: %v", err)
	}

	// Verify tables exist
	var count int
	db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'pg_catalog', 'temp')").Scan(&count)
	if count == 0 {
		db.Close()
		t.Skip("test database has no tables, run duck_setup.go first")
	}
	db.Close()

	parsed, err := dsn.ParseDSN("duckdb://" + dbPath + "?label=test")
	if err != nil {
		t.Fatalf("ParseDSN error: %v", err)
	}

	c := duckdbConnector{}
	ctx := context.Background()

	// Test Collect
	inst, err := c.Collect(ctx, parsed)
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	if len(inst.Databases) != 1 {
		t.Fatalf("expected 1 database, got %d", len(inst.Databases))
	}
	db0 := inst.Databases[0]
	if len(db0.Tables) < 2 {
		t.Fatalf("expected at least 2 tables, got %d: %v", len(db0.Tables), tableNamesDuck(db0.Tables))
	}

	// Find users table
	var users *schema.Table
	for _, t := range db0.Tables {
		if t.Name == "users" {
			users = t
			break
		}
	}
	if users == nil {
		t.Fatal("users table not found in collected schema")
	}

	if users.RowCount != 5 {
		t.Errorf("users row_count: expected 5, got %d", users.RowCount)
	}
	if len(users.Columns) != 5 {
		t.Errorf("users columns: expected 5, got %d", len(users.Columns))
	} else {
		// Check id is primary
		for _, c := range users.Columns {
			if c.Name == "id" && !c.IsPrimary {
				t.Error("id should be primary key")
			}
		}
	}

	// Find orders table and check FK
	var orders *schema.Table
	for _, t := range db0.Tables {
		if t.Name == "orders" {
			orders = t
			break
		}
	}
	if orders == nil {
		t.Fatal("orders table not found in collected schema")
	}
	if orders.RowCount != 6 {
		t.Errorf("orders row_count: expected 6, got %d", orders.RowCount)
	}
	if len(orders.ForeignKeys) == 0 {
		t.Errorf("orders should have foreign keys, got %d", len(orders.ForeignKeys))
	} else {
		fk := orders.ForeignKeys[0]
		if fk.RefTable != "users" {
			t.Errorf("expected FK ref users, got %q", fk.RefTable)
		}
	}

	// Test Capabilities
	caps := c.Capabilities()
	capMap := make(map[string]bool)
	for _, cap := range caps {
		capMap[string(cap)] = true
	}
	if !capMap["sql"] {
		t.Error("expected CapSQL")
	}
	if !capMap["row_count"] {
		t.Error("expected CapRowCount")
	}
	if !capMap["sampling"] {
		t.Error("expected CapSampling")
	}

	fmt.Printf("DuckDB Collect OK: %d tables, %s\n", len(db0.Tables), inst.DSN)
	fmt.Printf("  users: %d rows, %d cols\n", users.RowCount, len(users.Columns))
	fmt.Printf("  orders: %d rows, %d cols, fks=%d\n", orders.RowCount, len(orders.Columns), len(orders.ForeignKeys))
}

func tableNamesDuck(tables []*schema.Table) []string {
	var names []string
	for _, t := range tables {
		names = append(names, t.Name)
	}
	return names
}
