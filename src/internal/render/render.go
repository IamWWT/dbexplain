package render

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/IamWWT/dbexplain/internal/analyze"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func noColor() bool {
	return !isTerminal() || os.Getenv("NO_COLOR") != ""
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func color(code, s string) string {
	if noColor() {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

var (
	bold   = func(s string) string { return color("1", s) }
	dim    = func(s string) string { return color("2", s) }
	cyan   = func(s string) string { return color("96", s) }
	yellow = func(s string) string { return color("93", s) }
	green  = func(s string) string { return color("92", s) }
	red    = func(s string) string { return color("91", s) }
	blue   = func(s string) string { return color("94", s) }
)

func section(title string) { fmt.Printf("\n%s\n", bold(cyan("> "+title))) }
func hr()                  { fmt.Println(dim(strings.Repeat("-", 72))) }

func Print(result *analyze.Result, human bool) {
	u := result.Universe

	section(fmt.Sprintf("Instances (%d)", len(u.Instances)))
	for _, inst := range u.Instances {
		totalTables := 0
		for _, db := range inst.Databases {
			totalTables += len(db.Tables)
		}
		fmt.Printf("  %s  %s  %s\n",
			cyan(fmt.Sprintf("%-30s", inst.Label)),
			yellow(inst.Kind),
			dim(fmt.Sprintf("%d db(s), %d tables", len(inst.Databases), totalTables)),
		)
	}

	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			section(formatDBContext(inst, db, human))
			for _, t := range db.Tables {
				printTable(inst, db, t, human)
			}
		}
	}

	explicit := filterRefs(result.Refs, false)
	inferred := filterRefs(result.Refs, true)
	section(fmt.Sprintf("Relationships  (%d explicit FK, %d inferred)", len(explicit), len(inferred)))
	if len(result.Refs) == 0 {
		fmt.Println(dim("  no relationships found"))
	} else {
		printRefs(result.Refs)
	}

	if len(result.Groups) > 0 {
		section(fmt.Sprintf("Clusters (%d)", len(result.Groups)))
		for _, g := range result.Groups {
			if len(g.Tables) < 2 {
				continue
			}
			fmt.Printf("  %s\n", bold(g.Name))
			for _, qt := range g.Tables {
				fmt.Printf("    * %s\n", dim(qt.Instance+"/"+qt.DB+"/"+qt.Table))
			}
		}
	}

	if len(result.Issues) > 0 {
		section(fmt.Sprintf("Issues (%d)", len(result.Issues)))
		for _, iss := range result.Issues {
			icon := "[!]"
			col := yellow
			if iss.Severity == "info" {
				icon = "[i]"
				col = blue
			}
			fmt.Printf("  %s %s  %s\n",
				col(icon),
				dim(iss.Instance+"/"+iss.DB+"/"+iss.Table),
				iss.Message,
			)
		}
	}
	fmt.Println()
}

// formatDBContext builds the section header for a database context.
// Uses human-friendly labels when human mode is on; compact format for AI.
func formatDBContext(inst *schema.Instance, db *schema.Database, human bool) string {
	if human || inst.Kind == "" {
		return fmt.Sprintf("[instance=%s] [database=%s] kind=%s", inst.Label, bold(db.Name), inst.Kind)
	}
	return fmt.Sprintf("%s  /  %s", inst.Label, bold(db.Name))
}

// tableLabel returns the type-specific label for a table based on its parent instance kind.
func tableLabel(inst *schema.Instance, name string) string {
	switch inst.Kind {
	case "redis":
		return fmt.Sprintf("pattern=%s", name)
	case "mongodb":
		return fmt.Sprintf("collection=%s", name)
	case "elasticsearch":
		return fmt.Sprintf("index=%s", name)
	case "qdrant":
		return fmt.Sprintf("collection=%s", name)
	default:
		return fmt.Sprintf("table=%s", name)
	}
}

func printTable(inst *schema.Instance, db *schema.Database, t *schema.Table, human bool) {
	size := ""
	if t.SizeBytes > 0 {
		size = " " + fmtSize(t.SizeBytes)
	}
	rows := ""
	if t.RowCount > 0 {
		rows = fmt.Sprintf(" ~%s rows", fmtInt(t.RowCount))
	}
	engine := ""
	if t.Engine != "" {
		engine = " [" + t.Engine + "]"
	}
	cmt := ""
	if t.Comment != "" {
		cmt = "  " + dim(t.Comment)
	}
	if human {
		label := tableLabel(inst, t.Name)
		fmt.Printf("\n  [%s]%s%s%s%s%s\n",
			bold(label), dim(engine), dim(rows), dim(size), cmt, dim(keyPatternStr(t)),
		)
	} else {
		fmt.Printf("\n  %s%s%s%s%s%s\n",
			bold(t.Name), dim(engine), dim(rows), dim(size), cmt, dim(keyPatternStr(t)),
		)
	}
	hr()

	if len(t.Columns) == 0 {
		fmt.Println(dim("    (no columns)"))
		return
	}

	w0, w1 := 4, 4
	for _, c := range t.Columns {
		if len(c.Name) > w0 { w0 = len(c.Name) }
		if len(c.Type) > w1 { w1 = len(c.Type) }
	}
	if w0 > 40 { w0 = 40 }
	if w1 > 40 { w1 = 40 }

	fmt.Printf("  %s  %s  %s  %s\n",
		bold(pad("name", w0)), bold(pad("type", w1)), bold(pad("flags", 12)), bold("comment"))
	fmt.Printf("  %s  %s  %s  %s\n",
		dim(strings.Repeat("-", w0)), dim(strings.Repeat("-", w1)),
		dim(strings.Repeat("-", 12)), dim(strings.Repeat("-", 20)))

	for _, c := range t.Columns {
		flags := buildFlags(c)
		fmt.Printf("  %s  %s  %s  %s\n",
			colorizeName(c, pad(truncate(c.Name, w0), w0)),
			dim(pad(truncate(c.Type, w1), w1)),
			pad(flags, 12),
			dim(c.Comment),
		)
	}

	if len(t.Indexes) > 0 {
		parts := []string{}
		for _, idx := range t.Indexes {
			s := "IDX(" + strings.Join(idx.Columns, ",") + ")"
			if idx.Unique {
				s = "UNI(" + strings.Join(idx.Columns, ",") + ")"
			}
			if idx.Name == "PRIMARY" {
				continue
			}
			parts = append(parts, s)
		}
		if len(parts) > 0 {
			fmt.Printf("  %s %s\n", dim("indexes:"), dim(strings.Join(parts, "  ")))
		}
	}

	if t.PartitionKey != "" {
		fmt.Printf("  %s %s\n", dim("PARTITION BY"), blue(t.PartitionKey))
	}
	if t.OrderByKey != "" {
		fmt.Printf("  %s    %s\n", dim("ORDER BY"), blue(t.OrderByKey))
	}
}

func printRefs(refs []*schema.Ref) {
	for _, r := range refs {
		arrow := "--FK-->"
		col := green
		conf := ""
		if r.Inferred {
			arrow = "~~?~~~>"
			col = dim
			conf = fmt.Sprintf(" (inferred, %d%%)", r.Confidence)
		}
		from := qualify(r.FromInstance, r.FromDB, r.FromTable, r.FromCol)
		to := qualify(r.ToInstance, r.ToDB, r.ToTable, r.ToCol)
		fmt.Printf("  %s %s %s%s\n", cyan(from), col(arrow), cyan(to), dim(conf))
	}
}

func qualify(inst, db, table, col string) string {
	return fmt.Sprintf("%s/%s.%s(%s)", inst, db, table, col)
}

func filterRefs(refs []*schema.Ref, inferred bool) []*schema.Ref {
	var out []*schema.Ref
	for _, r := range refs {
		if r.Inferred == inferred {
			out = append(out, r)
		}
	}
	return out
}

func buildFlags(c *schema.Column) string {
	var parts []string
	if c.IsPrimary { parts = append(parts, yellow("PK")) }
	if c.IsUnique  { parts = append(parts, green("UNI")) }
	if c.IsIndex   { parts = append(parts, blue("IDX")) }
	if !c.Nullable && !c.IsPrimary { parts = append(parts, dim("NN")) }
	if c.IsSortKey     { parts = append(parts, blue("SORT")) }
	if c.IsPartitionKey { parts = append(parts, green("PART")) }
	return strings.Join(parts, " ")
}

func colorizeName(c *schema.Column, s string) string {
	if c.IsPrimary { return yellow(s) }
	return s
}

func keyPatternStr(t *schema.Table) string {
	if t.KeyPattern != "" {
		return "  key=" + t.KeyPattern
	}
	return ""
}

func pad(s string, n int) string {
	if visualWidth(s) >= n { return s }
	// fmt padding uses byte count, compensate for CJK
	padByte := n - visualWidth(s)
	return s + strings.Repeat(" ", padByte)
}

func truncate(s string, n int) string {
	if visualWidth(s) <= n { return s }
	// Truncate by visual width, not byte length
	runes := []rune(s)
	w := 0
	i := 0
	for _, r := range runes {
		rw := 1
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0xAC00 && r <= 0xD7AF) ||
			(r >= 0x3000 && r <= 0x303F) || (r >= 0xFF01 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6) || r >= 0x20000 {
			rw = 2
		}
		if w+rw > n-2 {
			break
		}
		w += rw
		i++
	}
	return string(runes[:i]) + "..."
}

func fmtSize(b int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(b)
	for _, u := range units {
		if f < 1024 { return fmt.Sprintf("%.1f%s", f, u) }
		f /= 1024
	}
	return fmt.Sprintf("%.1fPB", f)
}

func fmtInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 { out = append(out, ',') }
		out = append(out, byte(c))
	}
	return string(out)
}

func PrintJSON(result *analyze.Result) {
	// 使用 json.MarshalIndent 输出完整结构化数据（含列、索引、外键等元数据）
	out, err := json.MarshalIndent(buildJSONResult(result), "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	fmt.Println(string(out))
}

// ── JSON 序列化辅助类型 ──

type jsonResult struct {
	Instances []jsonInstance `json:"instances"`
	Refs      []jsonRef      `json:"refs"`
	Groups    []jsonGroup    `json:"groups,omitempty"`
	Issues    []jsonIssue    `json:"issues"`
}

type jsonInstance struct {
	Label     string       `json:"label"`
	Kind      string       `json:"kind"`
	Databases []jsonDB     `json:"databases"`
}

type jsonDB struct {
	Name       string      `json:"name"`
	TableCount int         `json:"table_count"`
	Tables     []jsonTable `json:"tables"`
}

type jsonTable struct {
	Name         string       `json:"name"`
	Comment      string       `json:"comment,omitempty"`
	Engine       string       `json:"engine,omitempty"`
	RowCount     int64        `json:"row_count,omitempty"`
	SizeBytes    int64        `json:"size_bytes,omitempty"`
	Columns      []jsonColumn `json:"columns,omitempty"`
	Indexes      []jsonIndex  `json:"indexes,omitempty"`
	ForeignKeys  []jsonFK     `json:"foreign_keys,omitempty"`
	PartitionKey string       `json:"partition_key,omitempty"`
	OrderByKey   string       `json:"order_by_key,omitempty"`
	KeyPattern   string       `json:"key_pattern,omitempty"`
	DataType     string       `json:"data_type,omitempty"`
	OpStats      *jsonOpStats `json:"op_stats,omitempty"`
}

type jsonOpStats struct {
	SeqScan         int64   `json:"seq_scan,omitempty"`
	IdxScan         int64   `json:"idx_scan,omitempty"`
	NtupIns         int64   `json:"n_tup_ins,omitempty"`
	NtupUpd         int64   `json:"n_tup_upd,omitempty"`
	NtupDel         int64   `json:"n_tup_del,omitempty"`
	QueryCount      int64   `json:"query_count,omitempty"`
	AvgDurationMs   float64 `json:"avg_duration_ms,omitempty"`
	KeyspaceHits    int64   `json:"keyspace_hits,omitempty"`
	KeyspaceMisses  int64   `json:"keyspace_misses,omitempty"`
	OpsPerSec       int64   `json:"ops_per_sec,omitempty"`
}

type jsonColumn struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Nullable       bool   `json:"nullable"`
	Default        string `json:"default,omitempty"`
	Comment        string `json:"comment,omitempty"`
	IsPrimary      bool   `json:"is_primary,omitempty"`
	IsUnique       bool   `json:"is_unique,omitempty"`
	IsIndex        bool   `json:"is_index,omitempty"`
	IsSortKey      bool   `json:"is_sort_key,omitempty"`
	IsPartitionKey bool   `json:"is_partition_key,omitempty"`
}

type jsonIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
	Type    string   `json:"type,omitempty"`
}

type jsonFK struct {
	Name        string   `json:"name,omitempty"`
	Columns     []string `json:"columns"`
	RefInstance string   `json:"ref_instance,omitempty"`
	RefDB       string   `json:"ref_db,omitempty"`
	RefTable    string   `json:"ref_table"`
	RefColumns  []string `json:"ref_columns"`
	OnDelete    string   `json:"on_delete,omitempty"`
	OnUpdate    string   `json:"on_update,omitempty"`
}

type jsonRef struct {
	// deprecating — use structured fields below
	From       string `json:"from"`
	To         string `json:"to"`
	// structured fields
	FromInstance string `json:"from_instance"`
	FromDB       string `json:"from_db"`
	FromTable    string `json:"from_table"`
	FromCol      string `json:"from_col"`
	ToInstance   string `json:"to_instance"`
	ToDB         string `json:"to_db"`
	ToTable      string `json:"to_table"`
	ToCol        string `json:"to_col"`
	Inferred     bool   `json:"inferred"`
	Confidence   int    `json:"confidence"`
}

type jsonGroup struct {
	Name   string        `json:"name"`
	Tables []jsonGroupTbl `json:"tables"`
}

type jsonGroupTbl struct {
	Instance string `json:"instance"`
	DB       string `json:"db"`
	Table    string `json:"table"`
}

type jsonIssue struct {
	Severity string `json:"severity"`
	Table    string `json:"table"`
	Message  string `json:"message"`
}

func buildJSONResult(r *analyze.Result) *jsonResult {
	jr := &jsonResult{}

	for _, inst := range r.Universe.Instances {
		ji := jsonInstance{Label: inst.Label, Kind: inst.Kind}
		for _, db := range inst.Databases {
			jd := jsonDB{Name: db.Name, TableCount: len(db.Tables)}
			for _, t := range db.Tables {
				jt := jsonTable{
					Name:         t.Name,
					Comment:      t.Comment,
					Engine:       t.Engine,
					RowCount:     t.RowCount,
					SizeBytes:    t.SizeBytes,
					PartitionKey: t.PartitionKey,
					OrderByKey:   t.OrderByKey,
					KeyPattern:   t.KeyPattern,
					DataType:     t.DataType,
				}
				if t.OpStats != nil {
					jt.OpStats = &jsonOpStats{
						SeqScan:        t.OpStats.SeqScan,
						IdxScan:        t.OpStats.IdxScan,
						NtupIns:        t.OpStats.NtupIns,
						NtupUpd:        t.OpStats.NtupUpd,
						NtupDel:        t.OpStats.NtupDel,
						QueryCount:     t.OpStats.QueryCount,
						AvgDurationMs:  t.OpStats.AvgDurationMs,
						KeyspaceHits:   t.OpStats.KeyspaceHits,
						KeyspaceMisses: t.OpStats.KeyspaceMisses,
						OpsPerSec:      t.OpStats.OpsPerSec,
					}
				}
				for _, c := range t.Columns {
					jt.Columns = append(jt.Columns, jsonColumn{
						Name: c.Name, Type: c.Type,
						Nullable: c.Nullable, Default: c.Default,
						Comment: c.Comment,
						IsPrimary: c.IsPrimary, IsUnique: c.IsUnique,
						IsIndex: c.IsIndex, IsSortKey: c.IsSortKey,
						IsPartitionKey: c.IsPartitionKey,
					})
				}
				for _, idx := range t.Indexes {
					jt.Indexes = append(jt.Indexes, jsonIndex{
						Name: idx.Name, Columns: idx.Columns,
						Unique: idx.Unique, Type: idx.Type,
					})
				}
				for _, fk := range t.ForeignKeys {
					jt.ForeignKeys = append(jt.ForeignKeys, jsonFK{
						Name: fk.Name, Columns: fk.Columns,
						RefInstance: fk.RefInstance, RefDB: fk.RefDB,
						RefTable: fk.RefTable, RefColumns: fk.RefColumns,
						OnDelete: fk.OnDelete, OnUpdate: fk.OnUpdate,
					})
				}
				jd.Tables = append(jd.Tables, jt)
			}
			ji.Databases = append(ji.Databases, jd)
		}
		jr.Instances = append(jr.Instances, ji)
	}

	for _, ref := range r.Refs {
		jr.Refs = append(jr.Refs, jsonRef{
			From:         qualify(ref.FromInstance, ref.FromDB, ref.FromTable, ref.FromCol),
			To:           qualify(ref.ToInstance, ref.ToDB, ref.ToTable, ref.ToCol),
			FromInstance: ref.FromInstance,
			FromDB:       ref.FromDB,
			FromTable:    ref.FromTable,
			FromCol:      ref.FromCol,
			ToInstance:   ref.ToInstance,
			ToDB:         ref.ToDB,
			ToTable:      ref.ToTable,
			ToCol:        ref.ToCol,
			Inferred:     ref.Inferred, Confidence: ref.Confidence,
		})
	}

	for _, g := range r.Groups {
		jg := jsonGroup{Name: g.Name}
		for _, qt := range g.Tables {
			jg.Tables = append(jg.Tables, jsonGroupTbl{qt.Instance, qt.DB, qt.Table})
		}
		jr.Groups = append(jr.Groups, jg)
	}

	for _, iss := range r.Issues {
		jr.Issues = append(jr.Issues, jsonIssue{
			Severity: string(iss.Severity),
			Table:    iss.Format(),
			Message:  iss.Message,
		})
	}

	return jr
}