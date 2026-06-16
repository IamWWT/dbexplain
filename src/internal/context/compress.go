// Package context provides the Context Compression layer for dbexplain.
//
// AI Agents have limited context windows. Instead of dumping the full schema,
// we produce a layered output that agents can consume progressively:
//
//  1. summary.json    — quick overview: core tables, largest, most-connected
//  2. topology.json   — subgraphs, isolated tables, cycles
//  3. diagnostics.json — categorized issues (missing PK, unindexed FK, etc.)
//  4. retrieval_chunks/ — per-table markdown for important tables
//
// All data is deterministic — no AI inference in this layer.
package context

import (
	"fmt"
	"sort"
	"strings"

	"github.com/IamWWT/dbexplain/internal/analyze"
	"github.com/IamWWT/dbexplain/internal/diagnostics"
	"github.com/IamWWT/dbexplain/internal/schema"
)

// ── Summary ──

// Summary provides a quick overview for AI agents to understand scope.
type Summary struct {
	TotalTables          int         `json:"total_tables"`
	TotalInstances       int         `json:"total_instances"`
	CoreTables           []string    `json:"core_tables"`
	LargestTables        []SizeRank  `json:"largest_tables"`
	HighlyConnectedTables []ConnRank `json:"highly_connected_tables"`
}

// SizeRank describes a table by row count.
type SizeRank struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

// ConnRank describes a table by connection degree.
type ConnRank struct {
	Table  string `json:"table"`
	Degree int    `json:"degree"`
}

// GenerateSummary creates a summary from analysis results.
// coreN controls how many top-ranked tables to include.
func GenerateSummary(result *analyze.Result, coreN int) *Summary {
	u := result.Universe
	s := &Summary{
		TotalInstances: len(u.Instances),
	}

	// Count total tables
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			s.TotalTables += len(db.Tables)
		}
	}

	// Core tables from importance ranking
	for i, r := range result.Ranks {
		if i >= coreN {
			break
		}
		qualified := fmt.Sprintf("%s/%s/%s", r.Instance, r.DB, r.Table)
		s.CoreTables = append(s.CoreTables, qualified)
	}

	// Largest tables by row count
	type rowEntry struct {
		name string
		rows int64
	}
	var rows []rowEntry
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				if t.RowCount > 0 {
					rows = append(rows, rowEntry{
						name: fmt.Sprintf("%s/%s/%s", inst.Label, db.Name, t.Name),
						rows: t.RowCount,
					})
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].rows > rows[j].rows })
	topN := coreN
	if len(rows) < topN {
		topN = len(rows)
	}
	for _, e := range rows[:topN] {
		s.LargestTables = append(s.LargestTables, SizeRank{Table: e.name, Rows: e.rows})
	}

	// Highly connected tables from ranking factors
	for _, r := range result.Ranks {
		deg := r.Factors["graph_degree"]
		if deg > 0 {
			qualified := fmt.Sprintf("%s/%s/%s", r.Instance, r.DB, r.Table)
			s.HighlyConnectedTables = append(s.HighlyConnectedTables, ConnRank{
				Table:  qualified,
				Degree: int(deg * 100), // approximate
			})
		}
	}
	// Limit
	if len(s.HighlyConnectedTables) > coreN {
		s.HighlyConnectedTables = s.HighlyConnectedTables[:coreN]
	}

	return s
}

// ── Topology ──

// Topology describes the graph structure of the database landscape.
type Topology struct {
	Subgraphs      []Subgraph `json:"subgraphs"`
	IsolatedTables []string   `json:"isolated_tables"`
	Cycles         [][]string `json:"cycles,omitempty"`
}

// Subgraph is a connected component of tables.
type Subgraph struct {
	Name   string   `json:"name"`
	Tables []string `json:"tables"`
}

// GenerateTopology creates topology from analysis results.
func GenerateTopology(result *analyze.Result) *Topology {
	topo := &Topology{}

	// Subgraphs from clustering
	for _, g := range result.Groups {
		if len(g.Tables) < 2 {
			continue
		}
		sg := Subgraph{Name: g.Name}
		for _, qt := range g.Tables {
			sg.Tables = append(sg.Tables, fmt.Sprintf("%s/%s/%s", qt.Instance, qt.DB, qt.Table))
		}
		topo.Subgraphs = append(topo.Subgraphs, sg)
	}

	// Isolated tables (those in single-member groups)
	for _, g := range result.Groups {
		if len(g.Tables) == 1 {
			qt := g.Tables[0]
			topo.IsolatedTables = append(topo.IsolatedTables,
				fmt.Sprintf("%s/%s/%s", qt.Instance, qt.DB, qt.Table))
		}
	}

	return topo
}

// ── Diagnostics Report ──

// DiagnosticsReport categorizes issues for AI consumption.
type DiagnosticsReport struct {
	MissingPK    []diagnostics.Issue `json:"missing_pk"`
	UnindexedFK  []diagnostics.Issue `json:"unindexed_fk"`
	WideTables   []diagnostics.Issue `json:"wide_tables"`
	NoTimestamp  []diagnostics.Issue `json:"no_timestamp"`
}

// GenerateDiagnostics categorizes issues by type.
func GenerateDiagnostics(issues []diagnostics.Issue) *DiagnosticsReport {
	dr := &DiagnosticsReport{}
	for _, iss := range issues {
		switch {
		case strings.Contains(iss.Message, "no primary key"):
			dr.MissingPK = append(dr.MissingPK, iss)
		case strings.Contains(iss.Message, "no index"):
			dr.UnindexedFK = append(dr.UnindexedFK, iss)
		case strings.Contains(iss.Message, "columns"):
			dr.WideTables = append(dr.WideTables, iss)
		case strings.Contains(iss.Message, "timestamp"):
			dr.NoTimestamp = append(dr.NoTimestamp, iss)
		}
	}
	return dr
}

// ── Retrieval Chunk ──

// TableChunk is a single-table context for agent retrieval.
type TableChunk struct {
	Table       string           `json:"table"`
	Importance  float64          `json:"importance"`
	Comment     string           `json:"comment,omitempty"`
	Engine      string           `json:"engine,omitempty"`
	RowCount    int64            `json:"row_count,omitempty"`
	SizeBytes   int64            `json:"size_bytes,omitempty"`
	Columns     []ChunkColumn    `json:"columns"`
	Indexes     []ChunkIndex     `json:"indexes,omitempty"`
	ForeignKeys []ChunkFK        `json:"foreign_keys,omitempty"`
	Issues      []diagnostics.Issue `json:"issues,omitempty"`
}

// ChunkColumn is a column in a retrieval chunk.
type ChunkColumn struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Nullable       bool   `json:"nullable"`
	IsPrimary      bool   `json:"is_primary,omitempty"`
	IsUnique       bool   `json:"is_unique,omitempty"`
	IsIndex        bool   `json:"is_index,omitempty"`
	Comment        string `json:"comment,omitempty"`
}

// ChunkIndex is an index in a retrieval chunk.
type ChunkIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
}

// ChunkFK is a foreign key in a retrieval chunk.
type ChunkFK struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
}

// GenerateChunks creates retrieval chunks for top-ranked tables.
func GenerateChunks(result *analyze.Result, topN int, tableFilter string) []TableChunk {
	// Build a lookup of table importance scores
	type key struct{ inst, db, table string }
	scores := map[key]float64{}
	for _, r := range result.Ranks {
		scores[key{r.Instance, r.DB, r.Table}] = r.Score
	}

	// Build a lookup of issues per table
	tableIssues := map[key][]diagnostics.Issue{}
	for _, iss := range result.Issues {
		k := key{iss.Instance, iss.DB, iss.Table}
		tableIssues[k] = append(tableIssues[k], iss)
	}

	// Collect top-N tables from ranking
	var chunks []TableChunk
	for _, r := range result.Ranks {
		if len(chunks) >= topN {
			break
		}
		// When tableFilter is set, skip non-matching tables
		if tableFilter != "" && !strings.EqualFold(r.Table, tableFilter) {
			continue
		}
		// Find the schema table
		var t *schema.Table
		for _, inst := range result.Universe.Instances {
			if inst.Label != r.Instance {
				continue
			}
			for _, db := range inst.Databases {
				if db.Name != r.DB {
					continue
				}
				for _, tbl := range db.Tables {
					if tbl.Name == r.Table {
						t = tbl
						break
					}
				}
			}
		}
		if t == nil {
			continue
		}

		chunk := TableChunk{
			Table:      fmt.Sprintf("%s/%s/%s", r.Instance, r.DB, r.Table),
			Importance: r.Score,
			Comment:    t.Comment,
			Engine:     t.Engine,
			RowCount:   t.RowCount,
			SizeBytes:  t.SizeBytes,
		}

		for _, c := range t.Columns {
			chunk.Columns = append(chunk.Columns, ChunkColumn{
				Name:      c.Name,
				Type:      c.Type,
				Nullable:  c.Nullable,
				IsPrimary: c.IsPrimary,
				IsUnique:  c.IsUnique,
				IsIndex:   c.IsIndex,
				Comment:   c.Comment,
			})
		}

		for _, idx := range t.Indexes {
			chunk.Indexes = append(chunk.Indexes, ChunkIndex{
				Name:    idx.Name,
				Columns: idx.Columns,
				Unique:  idx.Unique,
			})
		}

		for _, fk := range t.ForeignKeys {
			chunk.ForeignKeys = append(chunk.ForeignKeys, ChunkFK{
				Columns:    fk.Columns,
				RefTable:   fmt.Sprintf("%s/%s/%s", fk.RefInstance, fk.RefDB, fk.RefTable),
				RefColumns: fk.RefColumns,
			})
		}

		k := key{r.Instance, r.DB, r.Table}
		chunk.Issues = tableIssues[k]

		chunks = append(chunks, chunk)
	}

	return chunks
}

// ── Markdown rendering for retrieval chunks ──

// RenderChunkMarkdown renders a single table chunk as Markdown.
func RenderChunkMarkdown(chunk *TableChunk) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", chunk.Table))
	if chunk.Comment != "" {
		b.WriteString(fmt.Sprintf("> %s\n\n", chunk.Comment))
	}

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString(fmt.Sprintf("| Importance | %.3f |\n", chunk.Importance))
	if chunk.Engine != "" {
		b.WriteString(fmt.Sprintf("| Engine | %s |\n", chunk.Engine))
	}
	if chunk.RowCount > 0 {
		b.WriteString(fmt.Sprintf("| Rows | %d |\n", chunk.RowCount))
	}
	if chunk.SizeBytes > 0 {
		b.WriteString(fmt.Sprintf("| Size | %s |\n", formatBytes(chunk.SizeBytes)))
	}
	b.WriteString("\n")

	// Columns
	b.WriteString("## Columns\n\n")
	b.WriteString("| Name | Type | Flags | Comment |\n")
	b.WriteString("|------|------|-------|----------|\n")
	for _, c := range chunk.Columns {
		flags := []string{}
		if c.IsPrimary {
			flags = append(flags, "PK")
		}
		if c.IsUnique {
			flags = append(flags, "UNI")
		}
		if c.IsIndex {
			flags = append(flags, "IDX")
		}
		if !c.Nullable && !c.IsPrimary {
			flags = append(flags, "NN")
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			c.Name, c.Type, strings.Join(flags, " "), c.Comment))
	}
	b.WriteString("\n")

	// Indexes
	if len(chunk.Indexes) > 0 {
		b.WriteString("## Indexes\n\n")
		for _, idx := range chunk.Indexes {
			uniq := ""
			if idx.Unique {
				uniq = " UNIQUE"
			}
			b.WriteString(fmt.Sprintf("- **%s**%s (%s)\n", idx.Name, uniq, strings.Join(idx.Columns, ", ")))
		}
		b.WriteString("\n")
	}

	// Foreign Keys
	if len(chunk.ForeignKeys) > 0 {
		b.WriteString("## Foreign Keys\n\n")
		b.WriteString("| Columns | References |\n")
		b.WriteString("|---------|------------|\n")
		for _, fk := range chunk.ForeignKeys {
			b.WriteString(fmt.Sprintf("| %s | %s(%s) |\n",
				strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", ")))
		}
		b.WriteString("\n")
	}

	// Issues
	if len(chunk.Issues) > 0 {
		b.WriteString("## Issues\n\n")
		for _, iss := range chunk.Issues {
			icon := "[!]"
			if iss.Severity == "info" {
				icon = "[i]"
			}
			b.WriteString(fmt.Sprintf("- %s %s\n", icon, iss.Message))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func formatBytes(b int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(b)
	for _, u := range units {
		if f < 1024 {
			return fmt.Sprintf("%.1f%s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1fPB", f)
}

// RenderChunksMarkdown renders all chunks as a single Markdown document.
func RenderChunksMarkdown(chunks []TableChunk) string {
	var b strings.Builder
	for i := range chunks {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		b.WriteString(RenderChunkMarkdown(&chunks[i]))
	}
	return b.String()
}
