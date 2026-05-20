// Package ir defines the dbexplain IR v1 schema.
//
// IR (Internal Representation) is the project's most important asset.
// It uses universal graph primitives that are completely independent
// of database type — MySQL, PostgreSQL, Redis, and MongoDB all share
// the same Node, Column, and Edge types.
//
// Design principles:
//   - Deterministic facts only — no AI inference, no semantic guessing
//   - Long-term compatible — the v1 schema must remain stable
//   - Database-type-agnostic — one IR for all 9 supported databases
//   - JSON-serializable — all types have json struct tags
package ir

// NodeKind classifies a node in the universal graph.
type NodeKind string

const (
	KindInstance NodeKind = "instance" // a database instance (server)
	KindDatabase NodeKind = "database" // a database / schema
	KindTable    NodeKind = "table"    // a table, collection, or key pattern
	KindColumn   NodeKind = "column"   // a column or field
	KindIndex    NodeKind = "index"    // an index
)

// EdgeType classifies a relationship between two nodes.
type EdgeType string

const (
	EdgeDeclaredFK  EdgeType = "declared_fk"  // DDL-declared foreign key (confidence=100)
	EdgeInferredRef EdgeType = "inferred_ref" // naming-pattern inferred reference
	EdgeIndexEdge   EdgeType = "index_edge"   // index on column(s)
	EdgeClusterEdge EdgeType = "cluster_edge" // shared-reference cluster membership
)

// Node is a universal graph node representing any database object.
type Node struct {
	ID       string         `json:"id"`
	Kind     NodeKind       `json:"kind"`
	Label    string         `json:"label"`
	Engine   string         `json:"engine,omitempty"`   // mysql, postgres, redis, etc.
	Metadata map[string]any `json:"metadata,omitempty"` // row_count, size_bytes, key_pattern, etc.
}

// Column is a leaf node representing a column or field.
// Columns are always children of a KindTable node.
type Column struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DataType       string `json:"data_type"`
	Nullable       bool   `json:"nullable"`
	IsPrimary      bool   `json:"is_primary,omitempty"`
	IsUnique       bool   `json:"is_unique,omitempty"`
	IsIndex        bool   `json:"is_index,omitempty"`
	IsSortKey      bool   `json:"is_sort_key,omitempty"`
	IsPartitionKey bool   `json:"is_partition_key,omitempty"`
	DefaultValue   string `json:"default_value,omitempty"`
	Comment        string `json:"comment,omitempty"`
}

// Edge is a directed relationship between two nodes.
type Edge struct {
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Type       EdgeType       `json:"edge_type"`
	Confidence int            `json:"confidence"`          // 0-100, 100 = declared fact
	Metadata   map[string]any `json:"metadata,omitempty"`  // constraint_name, on_delete, etc.
}

// Graph is the universal graph container.
type Graph struct {
	Nodes   []Node   `json:"nodes"`
	Columns []Column `json:"columns"`
	Edges   []Edge   `json:"edges"`
}
