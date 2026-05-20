package analyze

import (
	"fmt"
	"strings"

	"dbexplain/capabilities"
	"dbexplain/diagnostics"
	"dbexplain/schema"
)

type Result struct {
	Universe *schema.Universe
	Refs     []*schema.Ref
	Groups   []TableGroup
	Issues   []diagnostics.Issue
	Ranks    []TableScore
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

func Analyze(u *schema.Universe, kindCaps map[string]*capabilities.Set) *Result {
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

	// 2. 运行基于能力的统一诊断
	r.Issues = diagnostics.NewRunner().Run(u, kindCaps)

	// 3. 确定性重要性评分
	r.Ranks = NewRanker().Rank(u, r.Refs)

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

