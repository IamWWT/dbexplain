package render

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/IamWWT/dbexplain/internal/query"
)

// FormatExplainJSON parses MySQL EXPLAIN FORMAT=JSON output and renders it as a
// human-readable query plan summary. Falls back to FormatHuman for non-JSON plans.
func FormatExplainJSON(result *query.QueryResult) string {
	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 || result.Rows[0][0] == nil {
		return FormatHuman(result)
	}

	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(*result.Rows[0][0]), &plan); err != nil {
		return FormatHuman(result)
	}

	queryBlock, ok := plan["query_block"].(map[string]interface{})
	if !ok {
		return FormatHuman(result)
	}

	var out strings.Builder
	out.WriteString("EXPLAIN Query Plan:\n")
	renderExplainNode(&out, queryBlock, 0)

	// Append footer from original result
	rc := len(result.Rows)
	if result.Truncated {
		rc = int(^uint(0) >> 1) // approximate
	}
	out.WriteString(fmt.Sprintf("\n%d row(s) in set (%s)\n", rc, result.ExecutionTime))
	return out.String()
}

// renderExplainNode recursively renders a MySQL EXPLAIN FORMAT=JSON node.
func renderExplainNode(out *strings.Builder, node map[string]interface{}, depth int) {
	indent := strings.Repeat("  ", depth)

	// select_type, table, access_type, key, rows, filtered, Extra
	selectType := stringField(node, "select_type")
	table := stringField(node, "table_name")
	if table == "" {
		if t, ok := node["table"].(string); ok {
			table = t
		}
	}
	accessType := stringField(node, "access_type")
	key := stringField(node, "key")
	keyLen := stringField(node, "key_length")
	rows, _ := node["rows"].(float64)
	possibleKeys := stringSliceField(node, "possible_keys")

	// Build the summary line
	var parts []string
	if selectType != "" && selectType != "SIMPLE" {
		parts = append(parts, selectType)
	}
	if table != "" {
		parts = append(parts, table)
	}
	if accessType != "" {
		parts = append(parts, accessType)
	}
	summary := strings.Join(parts, ", ")
	out.WriteString(fmt.Sprintf("%s├─ %s\n", indent, summary))

	// Detail lines
	if key != "" {
		kl := ""
		if keyLen != "" {
			kl = "(" + keyLen + ")"
		}
		out.WriteString(fmt.Sprintf("%s   key: %s%s\n", indent, key, kl))
	}
	if rows > 0 {
		out.WriteString(fmt.Sprintf("%s   rows: %.0f\n", indent, rows))
	}
	if len(possibleKeys) > 0 {
		out.WriteString(fmt.Sprintf("%s   possible_keys: %s\n", indent, strings.Join(possibleKeys, ", ")))
	}
	if filtered := stringField(node, "filtered"); filtered != "" {
		if f, err := strconv.ParseFloat(filtered, 64); err == nil && f < 100 {
			out.WriteString(fmt.Sprintf("%s   filtered: %s%%\n", indent, filtered))
		}
	}
	if extra := stringField(node, "Extra"); extra != "" {
		out.WriteString(fmt.Sprintf("%s   Extra: %s\n", indent, extra))
	}

	// Nested: attached_subqueries
	if subqueries, ok := node["attached_subqueries"].([]interface{}); ok {
		for _, sq := range subqueries {
			if sqMap, ok := sq.(map[string]interface{}); ok {
				out.WriteString(fmt.Sprintf("%s   subquery:\n", indent))
				renderExplainNode(out, sqMap, depth+2)
			}
		}
	}

	// Nested: materialized_from_subquery
	if mat, ok := node["materialized_from_subquery"].(map[string]interface{}); ok {
		out.WriteString(fmt.Sprintf("%s   materialized from subquery:\n", indent))
		if sq, ok := mat["subquery"].(map[string]interface{}); ok {
			renderExplainNode(out, sq, depth+2)
		}
	}

	// Nested: union_result
	if union, ok := node["union_result"].(map[string]interface{}); ok {
		out.WriteString(fmt.Sprintf("%s   union_result:\n", indent))
		if tbls, ok := union["table_name"].([]interface{}); ok {
			for _, tbl := range tbls {
				if t, ok := tbl.(string); ok {
					out.WriteString(fmt.Sprintf("%s   - table: %s\n", indent, t))
				}
			}
		}
		// Recurse into tables
		if tbls, ok := union["tables"].([]interface{}); ok {
			for _, table := range tbls {
				if t, ok := table.(map[string]interface{}); ok {
					renderExplainNode(out, t, depth+2)
				}
			}
		}
	}

	// Nested: nested_loop (list of joined tables)
	if nestedLoop, ok := node["nested_loop"].([]interface{}); ok {
		for _, nl := range nestedLoop {
			if nlMap, ok := nl.(map[string]interface{}); ok {
				for _, tbl := range nlMap {
					if t, ok := tbl.(map[string]interface{}); ok {
						renderExplainNode(out, t, depth+1)
					}
				}
			}
		}
	}

	// Nested: table directly
	if tbl, ok := node["table"].(map[string]interface{}); ok {
		renderExplainNode(out, tbl, depth+1)
	}
}

// stringField extracts a string value from a map by key.
func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// stringSliceField extracts a []string from a map field that may be []interface{}.
func stringSliceField(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	interfaces, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, i := range interfaces {
		if s, ok := i.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
