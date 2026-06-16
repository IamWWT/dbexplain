package connector

import "testing"

func TestTruncateDefault(t *testing.T) {
	if MaxSQLLogLen != 5000 {
		t.Errorf("default MaxSQLLogLen = %d, want 5000", MaxSQLLogLen)
	}
}

func TestTruncateSQL(t *testing.T) {
	// Under limit — no truncation
	short := "SELECT 1"
	got := TruncateSQL(short)
	if got != short {
		t.Errorf("short SQL truncated: got %q", got)
	}

	// Over limit — truncate
	long := ""
	for i := 0; i < 6000; i++ {
		long += "x"
	}
	got = TruncateSQL(long)
	if len(got) != 5000+len("...(truncated)") {
		t.Errorf("truncated length = %d, want %d", len(got), 5000+len("...(truncated)"))
	}
	if got[5000:] != "...(truncated)" {
		t.Errorf("missing truncation suffix: got %q", got[5000:])
	}
}

func TestTruncateSQLCustom(t *testing.T) {
	old := MaxSQLLogLen
	MaxSQLLogLen = 100
	defer func() { MaxSQLLogLen = old }()

	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	got := TruncateSQL(long)
	if len(got) != 100+len("...(truncated)") {
		t.Errorf("truncated length = %d, want %d", len(got), 100+len("...(truncated)"))
	}
}
