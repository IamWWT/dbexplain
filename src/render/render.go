package render

import (
	"fmt"
	"os"
	"strings"

	"dbexplain/analyze"
	"dbexplain/schema"
)

var noColor = !isTerminal() || os.Getenv("NO_COLOR") != ""

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func color(code, s string) string {
	if noColor {
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

func section(title string) { fmt.Printf("\n%s\n", bold(cyan("▸ "+title))) }
func hr()                  { fmt.Println(dim(strings.Repeat("─", 72))) }

func Print(result *analyze.Result) {
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
			section(fmt.Sprintf("%s  /  %s", inst.Label, bold(db.Name)))
			for _, t := range db.Tables {
				printTable(inst, db, t)
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
				fmt.Printf("    • %s\n", dim(qt.Instance+"/"+qt.DB+"/"+qt.Table))
			}
		}
	}

	if len(result.Issues) > 0 {
		section(fmt.Sprintf("Issues (%d)", len(result.Issues)))
		for _, iss := range result.Issues {
			icon := "⚠"
			col := yellow
			if iss.Severity == "info" {
				icon = "ℹ"
				col = blue
			}
			fmt.Printf("  %s %s  %s\n",
				col(icon),
				dim(iss.Table.Instance+"/"+iss.Table.DB+"/"+iss.Table.Table),
				iss.Message,
			)
		}
	}
	fmt.Println()
}

func printTable(inst *schema.Instance, db *schema.Database, t *schema.Table) {
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
	fmt.Printf("\n  %s%s%s%s%s%s\n",
		bold(t.Name), dim(engine), dim(rows), dim(size), cmt, dim(keyPatternStr(t)),
	)
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
		dim(strings.Repeat("─", w0)), dim(strings.Repeat("─", w1)),
		dim(strings.Repeat("─", 12)), dim(strings.Repeat("─", 20)))

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
		arrow := "──FK──▶"
		col := green
		conf := ""
		if r.Inferred {
			arrow = "~~?~~~▶"
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
	if len(s) >= n { return s }
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n-1] + "…"
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
	fmt.Println("{")
	fmt.Println(`  "instances": [`)
	for ii, inst := range result.Universe.Instances {
		fmt.Printf(`    {"label":%q,"kind":%q,"databases":[`, inst.Label, inst.Kind)
		for di, db := range inst.Databases {
			fmt.Printf(`{"name":%q,"table_count":%d}`, db.Name, len(db.Tables))
			if di < len(inst.Databases)-1 { fmt.Print(",") }
		}
		fmt.Print("]}")
		if ii < len(result.Universe.Instances)-1 { fmt.Print(",") }
		fmt.Println()
	}
	fmt.Println("  ],")
	fmt.Println(`  "refs": [`)
	for i, r := range result.Refs {
		from := qualify(r.FromInstance, r.FromDB, r.FromTable, r.FromCol)
		to := qualify(r.ToInstance, r.ToDB, r.ToTable, r.ToCol)
		fmt.Printf(`    {"from":%q,"to":%q,"inferred":%v,"confidence":%d}`,
			from, to, r.Inferred, r.Confidence)
		if i < len(result.Refs)-1 { fmt.Print(",") }
		fmt.Println()
	}
	fmt.Println("  ],")
	fmt.Println(`  "issues": [`)
	for i, iss := range result.Issues {
		tbl := iss.Table.Instance + "/" + iss.Table.DB + "/" + iss.Table.Table
		fmt.Printf(`    {"severity":%q,"table":%q,"message":%q}`, iss.Severity, tbl, iss.Message)
		if i < len(result.Issues)-1 { fmt.Print(",") }
		fmt.Println()
	}
	fmt.Println("  ]")
	fmt.Println("}")
}