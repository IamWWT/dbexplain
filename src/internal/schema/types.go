package schema

// Universe holds all collected metadata across multiple DB instances.
type Universe struct {
	Instances []*Instance
}

// Instance is one DSN / connection target.
type Instance struct {
	DSN       string // redacted
	Kind      string // mysql | postgres | sqlite | clickhouse | redis
	Label     string
	Databases []*Database
}

// Database is one logical database/schema.
type Database struct {
	Name   string
	Tables []*Table
}

// Table holds the full shape of one table (or Redis key family).
type Table struct {
	Name       string
	Comment    string
	Engine     string
	RowCount   int64
	SizeBytes  int64
	Columns    []*Column
	Indexes    []*Index
	ForeignKeys []*ForeignKey
	// ClickHouse extras
	PartitionKey string
	OrderByKey   string
	// Redis extras
	KeyPattern string
	DataType   string // string|hash|list|set|zset|stream
	// Operational stats (Phase 3) — reported by the database where available, nil if unsupported
	OpStats *OpStats `json:"op_stats,omitempty"`
}

// OpStats holds database-reported operational statistics.
// All fields are optional (zero = unknown/unavailable).
// This is deterministic ground truth, not AI inference.
type OpStats struct {
	// Table IO (PostgreSQL pg_stat_user_tables / MySQL performance_schema)
	SeqScan int64 `json:"seq_scan,omitempty"`
	IdxScan int64 `json:"idx_scan,omitempty"`
	NtupIns int64 `json:"n_tup_ins,omitempty"`
	NtupUpd int64 `json:"n_tup_upd,omitempty"`
	NtupDel int64 `json:"n_tup_del,omitempty"`

	// Query frequency (ClickHouse system.query_log)
	QueryCount    int64   `json:"query_count,omitempty"`
	AvgDurationMs float64 `json:"avg_duration_ms,omitempty"`

	// Redis INFO stats
	KeyspaceHits   int64 `json:"keyspace_hits,omitempty"`
	KeyspaceMisses int64 `json:"keyspace_misses,omitempty"`
	OpsPerSec      int64 `json:"ops_per_sec,omitempty"`
}

// WriteIntensity returns the write ratio (0-1), or 0 if unknown.
func (s *OpStats) WriteIntensity() float64 {
	total := s.NtupIns + s.NtupUpd + s.NtupDel
	if total == 0 {
		return 0
	}
	// Normalize to 0-1 range; 1 means exclusively writes
	return float64(total) / float64(total+1) // simplistic; refined in ranking
}

// ReadIntensity returns the read ratio (0-1), or 0 if unknown.
func (s *OpStats) ReadIntensity() float64 {
	if s.IdxScan == 0 && s.SeqScan == 0 {
		return 0
	}
	return float64(s.IdxScan) / float64(s.IdxScan+s.SeqScan+1)
}

// HitRate returns the keyspace hit rate (0-1), or 0 if unknown.
func (s *OpStats) HitRate() float64 {
	total := s.KeyspaceHits + s.KeyspaceMisses
	if total == 0 {
		return 0
	}
	return float64(s.KeyspaceHits) / float64(total)
}

type Column struct {
	Name            string
	Type            string
	Nullable        bool
	Default         string
	Comment         string
	IsPrimary       bool
	IsUnique        bool
	IsIndex         bool
	IsSortKey       bool // ClickHouse
	IsPartitionKey  bool // ClickHouse
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
	Type    string // BTREE, HASH, FULLTEXT ...
}

type ForeignKey struct {
	Name        string
	Columns     []string
	RefInstance string // empty = same instance
	RefDB       string
	RefTable    string
	RefColumns  []string
	OnDelete    string // CASCADE, SET NULL, RESTRICT, NO ACTION, etc.
	OnUpdate    string // CASCADE, SET NULL, RESTRICT, NO ACTION, etc.
}

// Ref is a resolved cross-table edge used in relationship graph.
type Ref struct {
	FromInstance string
	FromDB       string
	FromTable    string
	FromCol      string
	ToInstance   string
	ToDB         string
	ToTable      string
	ToCol        string
	Inferred     bool
	Confidence   int // 0-100
}