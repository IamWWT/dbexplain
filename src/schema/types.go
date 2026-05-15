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