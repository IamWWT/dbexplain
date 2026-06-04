// Package metrics provides structured collection metrics for dbexplain.
//
// It aggregates per-DSN timing, success/failure status, and table counts
// from the schema collection pipeline, and can export them in JSON or
// Prometheus text format.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Snapshot captures the metrics for a single DSN collection attempt.
type Snapshot struct {
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
	NumDBs     int    `json:"num_databases"`
	NumTables  int    `json:"num_tables"`
	ErrorMsg   string `json:"error,omitempty"`
}

// Collector aggregates per-DSN collection metrics.
// All exported methods are thread-safe (can be called from multiple goroutines).
type Collector struct {
	mu        sync.Mutex
	snapshots []Snapshot
	startTime time.Time
}

// NewCollector returns an initialized Collector.
func NewCollector() *Collector {
	return &Collector{startTime: time.Now()}
}

// Record adds a single DSN collection result. Thread-safe.
func (c *Collector) Record(label, kind string, success bool, duration time.Duration, numDBs, numTables int, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshots = append(c.snapshots, Snapshot{
		Label:      label,
		Kind:       kind,
		Success:    success,
		DurationMs: duration.Milliseconds(),
		NumDBs:     numDBs,
		NumTables:  numTables,
		ErrorMsg:   errMsg,
	})
}

// Snapshots returns a copy of all recorded snapshots (sorted by label). Thread-safe.
func (c *Collector) Snapshots() []Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Snapshot, len(c.snapshots))
	copy(out, c.snapshots)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Label < out[j].Label
	})
	return out
}

// TotalDuration returns the wall-clock duration since collector creation.
func (c *Collector) TotalDuration() time.Duration {
	return time.Since(c.startTime)
}

// counts returns success, failure, and total table counts.
// Must be called with c.mu held.
func (c *Collector) counts() (success, failure, totalTables int) {
	for _, s := range c.snapshots {
		if s.Success {
			success++
			totalTables += s.NumTables
		} else {
			failure++
		}
	}
	return
}

// SuccessCount returns the number of successful collections. Thread-safe.
func (c *Collector) SuccessCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, _, _ := c.counts()
	return n
}

// FailureCount returns the number of failed collections. Thread-safe.
func (c *Collector) FailureCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, n, _ := c.counts()
	return n
}

// TotalTables returns the sum of all collected tables (successful only). Thread-safe.
func (c *Collector) TotalTables() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _, n := c.counts()
	return n
}

// escapePromLabel escapes a Prometheus label value per the exposition format spec:
//   " → \", \n → \\n, \ → \\
func escapePromLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// promLabel formats a Prometheus label pair with proper escaping.
func promLabel(k, v string) string {
	return fmt.Sprintf(`%s="%s"`, k, escapePromLabel(v))
}

// PrometheusText returns the metrics in Prometheus text format.
// All per-DSN metrics use gauge type (CLI is one-shot, not cumulative).
// Thread-safe.
func (c *Collector) PrometheusText() string {
	c.mu.Lock()
	snaps := make([]Snapshot, len(c.snapshots))
	copy(snaps, c.snapshots)
	totalSec := time.Since(c.startTime).Seconds()
	succ, fail, totalTbl := c.counts()
	c.mu.Unlock()

	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].Label < snaps[j].Label
	})

	var b strings.Builder

	// Per-DSN duration (gauge)
	b.WriteString("# HELP dbexplain_collect_duration_ms Collection duration per DSN\n")
	b.WriteString("# TYPE dbexplain_collect_duration_ms gauge\n")
	for _, s := range snaps {
		fmt.Fprintf(&b, "dbexplain_collect_duration_ms{%s,%s} %d\n",
			promLabel("label", s.Label), promLabel("kind", s.Kind), s.DurationMs)
	}

	// Per-DSN success (gauge, 1=success 0=failure)
	b.WriteString("# HELP dbexplain_collect_success Collection success (1=success, 0=failure)\n")
	b.WriteString("# TYPE dbexplain_collect_success gauge\n")
	for _, s := range snaps {
		val := 0
		if s.Success {
			val = 1
		}
		fmt.Fprintf(&b, "dbexplain_collect_success{%s,%s} %d\n",
			promLabel("label", s.Label), promLabel("kind", s.Kind), val)
	}

	// Per-DSN table count (gauge)
	b.WriteString("# HELP dbexplain_collect_tables_total Total tables collected per DSN\n")
	b.WriteString("# TYPE dbexplain_collect_tables_total gauge\n")
	for _, s := range snaps {
		fmt.Fprintf(&b, "dbexplain_collect_tables_total{%s,%s} %d\n",
			promLabel("label", s.Label), promLabel("kind", s.Kind), s.NumTables)
	}

	// Aggregate: total duration (gauge, not counter — CLI is one-shot)
	b.WriteString("# HELP dbexplain_collect_duration_seconds Collection wall-clock duration\n")
	b.WriteString("# TYPE dbexplain_collect_duration_seconds gauge\n")
	fmt.Fprintf(&b, "dbexplain_collect_duration_seconds %.3f\n", totalSec)

	// Aggregate: success/failure totals (gauge)
	b.WriteString("# HELP dbexplain_collect_success_total Total collections by status\n")
	b.WriteString("# TYPE dbexplain_collect_success_total gauge\n")
	fmt.Fprintf(&b, `dbexplain_collect_success_total{status="success"} %d`+"\n", succ)
	fmt.Fprintf(&b, `dbexplain_collect_success_total{status="failure"} %d`+"\n", fail)

	// Aggregate: total tables across all DSNs (gauge)
	b.WriteString("# HELP dbexplain_collect_tables_total_all Total tables across all DSNs\n")
	b.WriteString("# TYPE dbexplain_collect_tables_total_all gauge\n")
	fmt.Fprintf(&b, "dbexplain_collect_tables_total_all %d\n", totalTbl)

	return b.String()
}
