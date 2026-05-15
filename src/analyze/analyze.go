package analyze

import (
	"fmt"
	"strings"

	"dbexplain/schema"
)

type Result struct {
	Universe *schema.Universe
	Refs     []*schema.Ref
	Groups   []TableGroup
	Issues   []Issue
}

type TableGroup struct {
	Name   string
	Tables []QTable
}

type QTable struct {
	Instance string
	DB       string
	Table    string
}

type Issue struct {
	Severity string // warn | info
	Table    QTable
	Message  string
}

func Analyze(u *schema.Universe) *Result {
	r := &Result{Universe: u}

	// 1. explicit FKs
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				for _, fk := range t.ForeignKeys {
					ref := &schema.Ref{
						FromInstance: inst.Label,
						FromDB:       db.Name,
						FromTable:    t.Name,
						FromCol:      strings.Join(fk.Columns, ","),
						ToInstance:   fk.RefInstance,
						ToDB:         fk.RefDB,
						ToTable:      fk.RefTable,
						ToCol:        strings.Join(fk.RefColumns, ","),
						Inferred:     false,
						Confidence:   100,
					}
					if ref.ToInstance == "" {
						ref.ToInstance = inst.Label
					}
					if ref.ToDB == "" {
						ref.ToDB = db.Name
					}
					r.Refs = append(r.Refs, ref)
				}
			}
		}
	}

	allTables := collectAllTables(u)
	r.Refs = append(r.Refs, inferRefs(allTables, r.Refs)...)
	r.Groups = clusterGroups(allTables, r.Refs)

	// 2. 构建 instanceKinds 映射
	instanceKinds := make(map[string]string)
	for _, inst := range u.Instances {
		instanceKinds[inst.Label] = inst.Kind
	}
	r.Issues = detectIssues(allTables, r.Refs, instanceKinds)
	return r
}

type tableEntry struct {
	inst   string
	db     string
	table  *schema.Table
	colSet map[string]*schema.Column
}

func collectAllTables(u *schema.Universe) []tableEntry {
	var entries []tableEntry
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				cs := map[string]*schema.Column{}
				for _, c := range t.Columns {
					cs[strings.ToLower(c.Name)] = c
				}
				entries = append(entries, tableEntry{
					inst:   inst.Label,
					db:     db.Name,
					table:  t,
					colSet: cs,
				})
			}
		}
	}
	return entries
}

func inferRefs(tables []tableEntry, existing []*schema.Ref) []*schema.Ref {
	type key struct{ inst, db, table string }
	pkIndex := map[key]string{}
	for _, e := range tables {
		for _, c := range e.table.Columns {
			if c.IsPrimary {
				pkIndex[key{e.inst, e.db, strings.ToLower(e.table.Name)}] = strings.ToLower(c.Name)
				break
			}
		}
		if _, ok := pkIndex[key{e.inst, e.db, strings.ToLower(e.table.Name)}]; !ok {
			if _, hasID := e.colSet["id"]; hasID {
				pkIndex[key{e.inst, e.db, strings.ToLower(e.table.Name)}] = "id"
			}
		}
	}

	existingSet := map[string]bool{}
	for _, r := range existing {
		existingSet[r.FromInstance+r.FromDB+r.FromTable+r.FromCol] = true
	}

	var inferred []*schema.Ref
	for _, src := range tables {
		for _, col := range src.table.Columns {
			cname := strings.ToLower(col.Name)
			if col.IsPrimary {
				continue
			}
			var stem string
			switch {
			case strings.HasSuffix(cname, "_id"):
				stem = strings.TrimSuffix(cname, "_id")
			case strings.HasSuffix(cname, "id") && len(cname) > 2:
				stem = strings.TrimSuffix(cname, "id")
				stem = strings.TrimRight(stem, "_-")
			case strings.HasSuffix(cname, "_fk"):
				stem = strings.TrimSuffix(cname, "_fk")
			default:
				continue
			}
			if stem == "" {
				continue
			}
			dedupKey := src.inst + src.db + src.table.Name + col.Name
			if existingSet[dedupKey] {
				continue
			}
			for _, tgt := range tables {
				tname := strings.ToLower(tgt.table.Name)
				if tname != stem && tname != stem+"s" && tname != stem+"es" {
					continue
				}
				pkCol, ok := pkIndex[key{tgt.inst, tgt.db, tname}]
				if !ok {
					continue
				}
				confidence := 70
				if tgt.inst == src.inst && tgt.db == src.db {
					confidence = 85
				}
				if tgt.inst != src.inst {
					confidence = 55
				}
				inferred = append(inferred, &schema.Ref{
					FromInstance: src.inst,
					FromDB:       src.db,
					FromTable:    src.table.Name,
					FromCol:      col.Name,
					ToInstance:   tgt.inst,
					ToDB:         tgt.db,
					ToTable:      tgt.table.Name,
					ToCol:        pkCol,
					Inferred:     true,
					Confidence:   confidence,
				})
				existingSet[dedupKey] = true
				break
			}
		}
	}
	return inferred
}

func clusterGroups(tables []tableEntry, refs []*schema.Ref) []TableGroup {
	type nodeKey = string
	nodeOf := func(inst, db, table string) nodeKey { return inst + "\x00" + db + "\x00" + table }

	parent := map[nodeKey]nodeKey{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] == "" || parent[x] == x {
			parent[x] = x
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	union := func(a, b string) {
		pa, pb := find(a), find(b)
		if pa != pb {
			parent[pa] = pb
		}
	}

	for _, e := range tables {
		n := nodeOf(e.inst, e.db, e.table.Name)
		parent[n] = n
	}
	for _, r := range refs {
		union(nodeOf(r.FromInstance, r.FromDB, r.FromTable),
			nodeOf(r.ToInstance, r.ToDB, r.ToTable))
	}

	groups := map[nodeKey][]QTable{}
	for _, e := range tables {
		n := nodeOf(e.inst, e.db, e.table.Name)
		root := find(n)
		groups[root] = append(groups[root], QTable{e.inst, e.db, e.table.Name})
	}

	var result []TableGroup
	for _, members := range groups {
		name := groupName(members)
		result = append(result, TableGroup{Name: name, Tables: members})
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if len(result[j].Tables) > len(result[i].Tables) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func groupName(members []QTable) string {
	if len(members) == 1 {
		return members[0].Table
	}
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = m.Table
	}
	prefix := longestCommonPrefix(names)
	if prefix != "" {
		return prefix + "* cluster"
	}
	return fmt.Sprintf("%d-table cluster", len(members))
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	for len(prefix) > 0 {
		last := prefix[len(prefix)-1]
		if last == '_' || last == '-' {
			prefix = prefix[:len(prefix)-1]
			break
		}
		prefix = prefix[:len(prefix)-1]
	}
	return prefix
}

func detectIssues(tables []tableEntry, refs []*schema.Ref, instanceKinds map[string]string) []Issue {
	var issues []Issue

	for _, e := range tables {
		kind, exists := instanceKinds[e.inst]
		if !exists {
			kind = ""
		}

		// 1. 外键列无索引 (所有数据库都可能存在显式外键定义)
		fkCols := map[string]bool{}
		for _, fk := range e.table.ForeignKeys {
			for _, c := range fk.Columns {
				fkCols[strings.ToLower(c)] = true
			}
		}
		indexedCols := map[string]bool{}
		for _, idx := range e.table.Indexes {
			for _, c := range idx.Columns {
				indexedCols[strings.ToLower(c)] = true
			}
		}
		for col := range fkCols {
			if !indexedCols[col] {
				issues = append(issues, Issue{
					Severity: "warn",
					Table:    QTable{e.inst, e.db, e.table.Name},
					Message:  fmt.Sprintf("FK column %q has no index — full scan risk", col),
				})
			}
		}

		// 2. 宽表 (通用)
		if len(e.table.Columns) > 30 {
			issues = append(issues, Issue{
				Severity: "info",
				Table:    QTable{e.inst, e.db, e.table.Name},
				Message:  fmt.Sprintf("%d columns — consider vertical partitioning", len(e.table.Columns)),
			})
		}

		// 3. 无主键 —— 仅对传统 SQL 数据库检测，NoSQL 豁免
		if !isNoSQLKind(kind) {
			hasPK := false
			for _, c := range e.table.Columns {
				if c.IsPrimary {
					hasPK = true
					break
				}
			}
			if !hasPK && len(e.table.Columns) > 0 {
				issues = append(issues, Issue{
					Severity: "warn",
					Table:    QTable{e.inst, e.db, e.table.Name},
					Message:  "no primary key defined",
				})
			}
		}

		// 4. 无时间戳列 —— 对 NoSQL 和 ClickHouse 豁免 (可按需扩展)
		if !isNoSQLKind(kind) && kind != "clickhouse" {
			if len(e.table.Columns) >= 3 {
				hasTime := false
				for col := range e.colSet {
					if strings.Contains(col, "created") || strings.Contains(col, "updated") ||
						strings.Contains(col, "timestamp") || strings.Contains(col, "time") {
						hasTime = true
						break
					}
				}
				if !hasTime {
					issues = append(issues, Issue{
						Severity: "info",
						Table:    QTable{e.inst, e.db, e.table.Name},
						Message:  "no timestamp column — audit trail gap",
					})
				}
			}
		}
	}

	return issues
}

// isNoSQLKind 判断数据库类型是否属于 NoSQL（键值、文档、向量等），这些通常没有传统“表”的主键概念。
func isNoSQLKind(kind string) bool {
    switch kind {
    case "redis", "mongodb", "qdrant", "elasticsearch":
        return true
    }
    return false
}