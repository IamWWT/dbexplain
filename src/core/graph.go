package core

import (
	ir "github.com/IamWWT/dbexplain/ir"
	"github.com/IamWWT/dbexplain/schema"
)

// BuildGraph converts a schema.Instance into an IR Graph (nodes + columns + edges).
// Declared foreign keys are mapped to EdgeDeclaredFK edges with confidence 100.
func BuildGraph(inst *schema.Instance) *ir.Graph {
	g := &ir.Graph{}

	instID := inst.Label
	if instID == "" {
		instID = inst.DSN
	}
	instNode := ir.Node{
		ID:     instID,
		Kind:   ir.KindInstance,
		Label:  inst.Label,
		Engine: inst.Kind,
	}
	g.Nodes = append(g.Nodes, instNode)

	for _, db := range inst.Databases {
		dbID := instID + "/" + db.Name
		g.Nodes = append(g.Nodes, ir.Node{
			ID:    dbID,
			Kind:  ir.KindDatabase,
			Label: db.Name,
		})

		for _, t := range db.Tables {
			tableID := dbID + "/" + t.Name
			tableMeta := map[string]any{
				"row_count":    t.RowCount,
				"engine":       t.Engine,
				"partition_by": t.PartitionKey,
				"order_by":     t.OrderByKey,
			}
			g.Nodes = append(g.Nodes, ir.Node{
				ID:       tableID,
				Kind:     ir.KindTable,
				Label:    t.Name,
				Engine:   t.Engine,
				Metadata: tableMeta,
			})

			for _, col := range t.Columns {
				colID := tableID + "/" + col.Name
				g.Columns = append(g.Columns, ir.Column{
					ID:             colID,
					Name:           col.Name,
					DataType:       col.Type,
					Nullable:       col.Nullable,
					IsPrimary:      col.IsPrimary,
					IsUnique:       col.IsUnique,
					IsIndex:        col.IsIndex,
					IsSortKey:      col.IsSortKey,
					IsPartitionKey: col.IsPartitionKey,
					DefaultValue:   col.Default,
					Comment:        col.Comment,
				})
			}

			for _, fk := range t.ForeignKeys {
				refTableID := instID + "/"
				if fk.RefDB != "" {
					refTableID += fk.RefDB + "/"
				} else {
					refTableID += db.Name + "/"
				}
				refTableID += fk.RefTable

				for i, col := range fk.Columns {
					sourceID := tableID + "/" + col
					var targetID string
					if i < len(fk.RefColumns) {
						targetID = refTableID + "/" + fk.RefColumns[i]
					} else {
						targetID = refTableID + "/" + col
					}
					g.Edges = append(g.Edges, ir.Edge{
						Source:     sourceID,
						Target:     targetID,
						Type:       ir.EdgeDeclaredFK,
						Confidence: 100,
						Metadata: map[string]any{
							"constraint_name": fk.Name,
							"on_delete":       fk.OnDelete,
							"on_update":       fk.OnUpdate,
						},
					})
				}
			}
		}
	}

	return g
}
