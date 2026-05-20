// Package capabilities defines the Capability Architecture for dbexplain.
//
// Rather than branching on database type (if mysql / if postgres / if redis),
// connectors declare their capabilities and extractors work by capability.
// This means adding a new database type never requires changing the pipeline.
//
// Design principles:
//   - Connector declares → Extractor consumes by capability
//   - New databases only need to implement the Connector interface and declare
//     which existing capabilities they support
//   - The pipeline never contains "if kind == mysql" branches
package capabilities

// Capability is a named feature that a connector may support.
type Capability string

const (
	// CapForeignKey — the database has explicit foreign key constraints (DDL-declared).
	CapForeignKey Capability = "foreign_key"

	// CapSampling — the database supports sampling row data for comment inference.
	CapSampling Capability = "sampling"

	// CapTTL — the database has key-level TTL semantics (Redis).
	CapTTL Capability = "ttl"

	// CapPartition — the database has table partitioning (ClickHouse MergeTree).
	CapPartition Capability = "partition"

	// CapVector — the database stores vector embeddings (Qdrant).
	CapVector Capability = "vector"

	// CapRowCount — the database provides row/document/point count estimates.
	CapRowCount Capability = "row_count"

	// CapIndex — the database supports traditional secondary indexes.
	CapIndex Capability = "index"
)

// Provider is something that declares which capabilities it supports.
// Every connector implements this interface.
type Provider interface {
	Capabilities() []Capability
}

// Set is a capability set used by extractors to check what is available.
type Set struct {
	items map[Capability]bool
}

// NewSet creates a Set from a list of capabilities.
func NewSet(caps ...Capability) *Set {
	s := &Set{items: make(map[Capability]bool, len(caps))}
	for _, c := range caps {
		s.items[c] = true
	}
	return s
}

// FromProvider builds a Set from a Provider.
func FromProvider(p Provider) *Set {
	return NewSet(p.Capabilities()...)
}

// Has returns true if the capability is present.
func (s *Set) Has(c Capability) bool {
	return s.items[c]
}

// HasAny returns true if any of the given capabilities are present.
func (s *Set) HasAny(caps ...Capability) bool {
	for _, c := range caps {
		if s.items[c] {
			return true
		}
	}
	return false
}

// HasAll returns true if all given capabilities are present.
func (s *Set) HasAll(caps ...Capability) bool {
	for _, c := range caps {
		if !s.items[c] {
			return false
		}
	}
	return true
}
