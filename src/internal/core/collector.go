// Package core provides the public Go API for dbexplain.
// VeinMap and other Go programs can import this package and call
// Collect/CollectToGraph/CollectToJSON directly instead of launching
// the dbexplain binary as a subprocess.
package core

import (
	"context"
	"encoding/json"

	"github.com/IamWWT/dbexplain/internal/connector"
	ir "github.com/IamWWT/dbexplain/internal/ir"
	"github.com/IamWWT/dbexplain/internal/schema"
)

// Collect connects to the database described by rawDSN and returns
// a fully populated schema.Instance (in-memory typed struct).
func Collect(ctx context.Context, rawDSN string) (*schema.Instance, error) {
	return connector.Collect(ctx, rawDSN)
}

// CollectToGraph connects to the database and returns the result as an
// IR Graph (nodes + columns + edges). This is the preferred data format
// for VeinMap: type-safe, deterministic, database-type agnostic.
func CollectToGraph(ctx context.Context, rawDSN string) (*ir.Graph, error) {
	inst, err := connector.Collect(ctx, rawDSN)
	if err != nil {
		return nil, err
	}
	return BuildGraph(inst), nil
}

// CollectToJSON connects to the database and returns the JSON output
// that would be written to stdout/--o by the CLI binary.
func CollectToJSON(ctx context.Context, rawDSN string) ([]byte, error) {
	inst, err := connector.Collect(ctx, rawDSN)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent([]*schema.Instance{inst}, "", "  ")
}
