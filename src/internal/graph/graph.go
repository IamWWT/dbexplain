// Package graph provides the unified graph model for dbexplain.
//
// All database objects are internally modeled as graph nodes and edges.
// Markdown, JSON, and HTML output are merely renderers on top of this graph.
// The graph model is database-type-agnostic — MySQL, PostgreSQL, Redis,
// and MongoDB all use the same primitives.
//
// Design principles:
//   - Graph First: everything is a node or an edge
//   - Read-only construction: once built, the graph is immutable
//   - No rendering logic: rendering belongs in the render package
//   - Queryable: provides subgraph, neighbor, and topology operations
package graph

import (
	"sort"
	"strings"

	"github.com/IamWWT/dbexplain/internal/ir"
)

// ID is a helper to construct canonical node IDs.
// Format: "instance/db/table" or "instance/db/table.column"
type ID string

// Join builds a node ID from components.
func Join(parts ...string) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(p)
	}
	return b.String()
}

// ColumnID builds a column ID: "instance/db/table.column"
func ColumnID(instance, db, table, column string) string {
	return Join(instance, db, table) + "." + column
}

// Graph is a mutable builder for the universal graph.
// After construction, Build() returns the immutable ir.Graph.
type Graph struct {
	nodes   map[string]*ir.Node
	columns []ir.Column
	edges   []ir.Edge
}

// New creates an empty Graph builder.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]*ir.Node),
	}
}

// AddNode adds a node to the graph.
// Duplicate IDs are silently ignored.
func (g *Graph) AddNode(id string, kind ir.NodeKind, label, engine string) *Graph {
	if _, ok := g.nodes[id]; ok {
		return g
	}
	g.nodes[id] = &ir.Node{
		ID:     id,
		Kind:   kind,
		Label:  label,
		Engine: engine,
	}
	return g
}

// SetMeta sets a metadata key on a node.
func (g *Graph) SetMeta(nodeID, key string, value any) *Graph {
	if n, ok := g.nodes[nodeID]; ok {
		if n.Metadata == nil {
			n.Metadata = make(map[string]any)
		}
		n.Metadata[key] = value
	}
	return g
}

// AddColumn adds a column node and returns its ID.
func (g *Graph) AddColumn(instance, db, table string, col *ir.Column) string {
	id := ColumnID(instance, db, table, col.Name)
	col.ID = id
	g.columns = append(g.columns, *col)
	return id
}

// AddEdge adds a directed edge to the graph.
func (g *Graph) AddEdge(source, target string, edgeType ir.EdgeType, confidence int) *Graph {
	g.edges = append(g.edges, ir.Edge{
		Source:     source,
		Target:     target,
		Type:       edgeType,
		Confidence: confidence,
	})
	return g
}

// AddEdgeMeta adds a metadata-bearing edge.
func (g *Graph) AddEdgeMeta(source, target string, edgeType ir.EdgeType, confidence int, meta map[string]any) *Graph {
	g.edges = append(g.edges, ir.Edge{
		Source:     source,
		Target:     target,
		Type:       edgeType,
		Confidence: confidence,
		Metadata:   meta,
	})
	return g
}

// Build returns the immutable ir.Graph.
func (g *Graph) Build() *ir.Graph {
	nodes := make([]ir.Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, *n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	return &ir.Graph{
		Nodes:   nodes,
		Columns: g.columns,
		Edges:   g.edges,
	}
}

// NodeCount returns the total number of nodes.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// EdgeCount returns the total number of edges.
func (g *Graph) EdgeCount() int { return len(g.edges) }

// ColumnCount returns the total number of columns.
func (g *Graph) ColumnCount() int { return len(g.columns) }

// ── Query operations ──

// Query provides read-only access to a built graph for analysis.
type Query struct {
	g *ir.Graph
}

// NewQuery creates a Query from a built ir.Graph.
func NewQuery(g *ir.Graph) *Query {
	return &Query{g: g}
}

// Neighbors returns all node IDs connected to the given node by any edge.
func (q *Query) Neighbors(nodeID string) []string {
	seen := make(map[string]bool)
	for _, e := range q.g.Edges {
		if e.Source == nodeID {
			seen[e.Target] = true
		}
		if e.Target == nodeID {
			seen[e.Source] = true
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Degree returns the total degree (in + out) of a node.
func (q *Query) Degree(nodeID string) int {
	count := 0
	for _, e := range q.g.Edges {
		if e.Source == nodeID || e.Target == nodeID {
			count++
		}
	}
	return count
}

// Isolated returns all table nodes with no edges.
func (q *Query) Isolated() []ir.Node {
	connected := make(map[string]bool)
	for _, e := range q.g.Edges {
		connected[e.Source] = true
		connected[e.Target] = true
	}
	var out []ir.Node
	for _, n := range q.g.Nodes {
		if n.Kind == ir.KindTable && !connected[n.ID] {
			out = append(out, n)
		}
	}
	return out
}

// Subgraphs returns connected components using union-find.
// Each subgraph is a list of table node IDs.
func (q *Query) Subgraphs() [][]string {
	// Union-find on table nodes
	parent := make(map[string]string)
	for _, n := range q.g.Nodes {
		if n.Kind == ir.KindTable {
			parent[n.ID] = n.ID
		}
	}

	var find func(x string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// Union nodes connected by edges
	for _, e := range q.g.Edges {
		if _, ok := parent[e.Source]; ok {
			if _, ok2 := parent[e.Target]; ok2 {
				union(e.Source, e.Target)
			}
		}
	}

	// Group by root
	groups := make(map[string][]string)
	for id := range parent {
		root := find(id)
		groups[root] = append(groups[root], id)
	}

	// Sort by size descending
	type group struct {
		ids []string
	}
	var sorted []group
	for _, ids := range groups {
		sort.Strings(ids)
		sorted = append(sorted, group{ids})
	}
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i].ids) > len(sorted[j].ids) })

	out := make([][]string, len(sorted))
	for i, g := range sorted {
		out[i] = g.ids
	}
	return out
}

// HasEdge returns true if there is an edge of the given type between source and target.
func (q *Query) HasEdge(source, target string, edgeType ir.EdgeType) bool {
	for _, e := range q.g.Edges {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}
