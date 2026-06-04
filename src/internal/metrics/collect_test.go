package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector() returned nil")
	}
	if c.SuccessCount() != 0 {
		t.Errorf("expected 0 successes, got %d", c.SuccessCount())
	}
	if c.FailureCount() != 0 {
		t.Errorf("expected 0 failures, got %d", c.FailureCount())
	}
}

func TestRecordAndSnapshots(t *testing.T) {
	c := NewCollector()
	c.Record("my-mysql", "mysql", true, 100*time.Millisecond, 1, 15, "")
	c.Record("my-redis", "redis", false, 50*time.Millisecond, 0, 0, "timeout")
	c.Record("my-pg", "postgres", true, 200*time.Millisecond, 2, 30, "")

	snaps := c.Snapshots()
	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snaps))
	}

	// Verify sort order (by label)
	if snaps[0].Label != "my-mysql" {
		t.Errorf("expected first snapshot label 'my-mysql', got %q", snaps[0].Label)
	}
	if snaps[1].Label != "my-pg" {
		t.Errorf("expected second snapshot label 'my-pg', got %q", snaps[1].Label)
	}
	if snaps[2].Label != "my-redis" {
		t.Errorf("expected third snapshot label 'my-redis', got %q", snaps[2].Label)
	}

	// Verify values
	if snaps[0].Kind != "mysql" || snaps[0].NumTables != 15 || !snaps[0].Success {
		t.Errorf("unexpected mysql snapshot: %+v", snaps[0])
	}
	if snaps[1].Kind != "postgres" || snaps[1].NumTables != 30 || !snaps[1].Success {
		t.Errorf("unexpected pg snapshot: %+v", snaps[1])
	}
	if snaps[2].Kind != "redis" || snaps[2].Success || snaps[2].ErrorMsg != "timeout" {
		t.Errorf("unexpected redis snapshot: %+v", snaps[2])
	}
}

func TestSuccessFailureCounts(t *testing.T) {
	c := NewCollector()
	c.Record("a", "mysql", true, 10*time.Millisecond, 1, 5, "")
	c.Record("b", "redis", false, 10*time.Millisecond, 0, 0, "err")
	c.Record("c", "pg", true, 10*time.Millisecond, 1, 3, "")
	c.Record("d", "es", false, 10*time.Millisecond, 0, 0, "err")

	if c.SuccessCount() != 2 {
		t.Errorf("expected 2 successes, got %d", c.SuccessCount())
	}
	if c.FailureCount() != 2 {
		t.Errorf("expected 2 failures, got %d", c.FailureCount())
	}
	if c.TotalTables() != 8 {
		t.Errorf("expected 8 total tables, got %d", c.TotalTables())
	}
}

func TestTotalDuration(t *testing.T) {
	c := NewCollector()
	c.Record("a", "mysql", true, 10*time.Millisecond, 1, 5, "")
	d := c.TotalDuration()
	if d <= 0 {
		t.Errorf("expected positive total duration, got %v", d)
	}
}

func TestEmptyCollector(t *testing.T) {
	c := NewCollector()
	snaps := c.Snapshots()
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots from empty collector, got %d", len(snaps))
	}
	if c.SuccessCount() != 0 {
		t.Errorf("expected 0 successes")
	}
	if c.FailureCount() != 0 {
		t.Errorf("expected 0 failures")
	}
	if c.TotalTables() != 0 {
		t.Errorf("expected 0 tables")
	}
}

func TestPrometheusText(t *testing.T) {
	c := NewCollector()
	c.Record("my-mysql", "mysql", true, 100*time.Millisecond, 1, 15, "")
	c.Record("my-redis", "redis", false, 50*time.Millisecond, 0, 0, "timeout")

	output := c.PrometheusText()

	// Check required sections
	checks := []string{
		"# HELP dbexplain_collect_duration_ms",
		"# TYPE dbexplain_collect_duration_ms gauge",
		`label="my-mysql"`,
		`dbexplain_collect_success{label="my-mysql",kind="mysql"} 1`,
		`dbexplain_collect_success{label="my-redis",kind="redis"} 0`,
		"# HELP dbexplain_collect_tables_total",
		`dbexplain_collect_tables_total{label="my-mysql",kind="mysql"} 15`,
		"# HELP dbexplain_collect_duration_seconds",
		"# TYPE dbexplain_collect_duration_seconds gauge",
		`dbexplain_collect_success_total{status="success"} 1`,
		`dbexplain_collect_success_total{status="failure"} 1`,
		"dbexplain_collect_tables_total_all",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("PrometheusText missing expected content: %q\nFull output:\n%s", check, output)
		}
	}
}

func TestPrometheusLabelEscaping(t *testing.T) {
	c := NewCollector()
	c.Record(`my "prod" db`, `"mysql"`, true, 10*time.Millisecond, 1, 5, "")

	output := c.PrometheusText()

	// The label value should be escaped in Prometheus format
	if !strings.Contains(output, `label="my \"prod\" db"`) {
		t.Errorf("expected escaped label in Prometheus output\nFull output:\n%s", output)
	}
	if !strings.Contains(output, `kind="\"mysql\""`) {
		t.Errorf("expected escaped kind in Prometheus output\nFull output:\n%s", output)
	}
}

func TestPrometheusTextEmpty(t *testing.T) {
	c := NewCollector()
	output := c.PrometheusText()
	// Empty collector should still produce HELP/TYPE lines but no per-DSN data lines
	if !strings.Contains(output, "# HELP dbexplain_collect_duration_ms") {
		t.Errorf("expected HELP line even for empty collector")
	}
	// Should still have aggregate metrics (with zero values)
	if !strings.Contains(output, `dbexplain_collect_success_total{status="success"} 0`) {
		t.Errorf("expected zero success count for empty collector")
	}
	if !strings.Contains(output, "dbexplain_collect_tables_total_all 0") {
		t.Errorf("expected zero total tables for empty collector")
	}
}
