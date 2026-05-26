// Package diagnostics provides a unified, capability-driven diagnostic layer.
//
// Rather than embedding risk detection inside individual connectors
// (e.g., Redis big-key detection in redis.go) or hardcoding DB-type
// branches in analyze.go, diagnostics are defined as independent rules
// triggered by capabilities.
//
// Design principles:
//   - Capability-driven: rules check Has(CapForeignKey), not kind=="mysql"
//   - Normalized output: all diagnostics produce the same Issue type
//   - Deterministic only: no AI inference, only observable facts
//   - Extensible: new rules only need a capability condition
package diagnostics

import (
	"fmt"
	"strings"

	"github.com/IamWWT/dbexplain/capabilities"
	"github.com/IamWWT/dbexplain/schema"
)

// Severity classifies the importance of a diagnostic finding.
type Severity string

const (
	SeverityWarn Severity = "warn" // actionable issue (missing PK, unindexed FK)
	SeverityInfo Severity = "info" // informational (no timestamp, wide table)
)

// Issue is a normalized diagnostic finding.
type Issue struct {
	Severity Severity
	Instance string
	DB       string
	Table    string
	Message  string
}

// Format returns an "instance/db/table" qualified string.
func (i Issue) Format() string {
	return fmt.Sprintf("%s/%s/%s", i.Instance, i.DB, i.Table)
}

// Runner executes diagnostic rules against a universe.
type Runner struct {
	rules []Rule
}

// Rule is a diagnostic check triggered by capability.
// It receives the full universe and the capability set of the current instance,
// and returns any issues found.
type Rule struct {
	Name     string
	Requires capabilities.Capability // empty = always run
	Check    func(table *schema.Table, instance, db string, caps *capabilities.Set) []Issue
}

// NewRunner creates a Runner with the standard diagnostic rules.
func NewRunner() *Runner {
	return &Runner{rules: standardRules()}
}

// Run executes all rules against the universe and returns all findings.
func (r *Runner) Run(u *schema.Universe, kindCaps map[string]*capabilities.Set) []Issue {
	var all []Issue
	for _, inst := range u.Instances {
		caps, ok := kindCaps[inst.Kind]
		if !ok {
			caps = capabilities.NewSet()
		}
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				for _, rule := range r.rules {
					if rule.Requires != "" && !caps.Has(rule.Requires) {
						continue
					}
						issues := rule.Check(t, inst.Label, db.Name, caps)
						all = append(all, issues...)
				}
			}
		}
	}
	return all
}

// ── Standard diagnostic rules ──

func standardRules() []Rule {
	return []Rule{
		{
			Name:     "unindexed_fk",
			Requires: capabilities.CapForeignKey,
			Check:    checkUnindexedFK,
		},
		{
			Name:     "wide_table",
			Requires: "", // universal
			Check:    checkWideTable,
		},
		{
			Name:     "missing_pk",
			Requires: capabilities.CapForeignKey, // only traditional SQL databases
			Check:    checkMissingPK,
		},
		{
			Name:     "no_timestamp",
			Requires: "", // universal but skip NoSQL + ClickHouse
			Check:    checkNoTimestamp,
		},
	}
}

// ── Rule implementations ──

func checkUnindexedFK(t *schema.Table, inst, db string, caps *capabilities.Set) []Issue {
	var issues []Issue
	fkCols := make(map[string]bool)
	for _, fk := range t.ForeignKeys {
		for _, c := range fk.Columns {
			fkCols[strings.ToLower(c)] = true
		}
	}
	indexedCols := make(map[string]bool)
	for _, idx := range t.Indexes {
		for _, c := range idx.Columns {
			indexedCols[strings.ToLower(c)] = true
		}
	}
	for col := range fkCols {
		if !indexedCols[col] {
			issues = append(issues, Issue{
				Severity: SeverityWarn,
				Instance: inst,
				DB:       db,
				Table:    t.Name,
				Message:  fmt.Sprintf("FK column %q has no index — full scan risk", col),
			})
		}
	}
	return issues
}

func checkWideTable(t *schema.Table, inst, db string, caps *capabilities.Set) []Issue {
	if len(t.Columns) > 30 {
		return []Issue{{
			Severity: SeverityInfo,
			Instance: inst,
			DB:       db,
			Table:    t.Name,
			Message:  fmt.Sprintf("%d columns — consider vertical partitioning", len(t.Columns)),
		}}
	}
	return nil
}

func checkMissingPK(t *schema.Table, inst, db string, caps *capabilities.Set) []Issue {
	for _, c := range t.Columns {
		if c.IsPrimary {
			return nil
		}
	}
	if len(t.Columns) == 0 {
		return nil
	}
	return []Issue{{
		Severity: SeverityWarn,
		Instance: inst,
		DB:       db,
		Table:    t.Name,
		Message:  "no primary key defined",
	}}
}

func checkNoTimestamp(t *schema.Table, inst, db string, caps *capabilities.Set) []Issue {
	// Skip NoSQL (no FK capability) and ClickHouse (has partition capability)
	if !caps.Has(capabilities.CapForeignKey) || caps.Has(capabilities.CapPartition) {
		return nil
	}
	if len(t.Columns) < 3 {
		return nil
	}
	for _, c := range t.Columns {
		name := strings.ToLower(c.Name)
		if strings.Contains(name, "created") || strings.Contains(name, "updated") ||
			strings.Contains(name, "timestamp") || strings.Contains(name, "time") {
			return nil
		}
	}
	return []Issue{{
		Severity: SeverityInfo,
		Instance: inst,
		DB:       db,
		Table:    t.Name,
		Message:  "no timestamp column — audit trail gap",
	}}
}
