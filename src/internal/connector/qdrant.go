//go:build qdrant || full

package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("qdrant", func() Connector { return qdrantConnector{} })
}

type qdrantConnector struct{}

func (qdrantConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapVector,
		capabilities.CapRowCount,
	}
}

func (qdrantConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	host := d.Host
	if host == "" {
		host = "localhost"
	}
	intPort, err := strconv.Atoi(d.Port)
	if err != nil || intPort == 0 {
		intPort = 6334
	}

	cfg := &qdrant.Config{
		Host:   host,
		Port:   intPort,
		APIKey: d.Password,
		UseTLS: false,
	}
	client, err := qdrant.NewClient(cfg)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "connect", err)
	}
	defer client.Close()

	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := client.HealthCheck(healthCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "health check", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "qdrant", Label: d.Label}

	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	collections, err := client.ListCollections(listCtx)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "list collections", err)
	}

	database := &schema.Database{Name: "default"}
	total := len(collections)
	for i, collName := range collections {
		Logf(ctx, "[qdrant] 采集集合 %d/%d: %s", i+1, total, collName)
		t := &schema.Table{
			Name:    collName,
			Comment: "vector collection",
		}
		infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		info, err := client.GetCollectionInfo(infoCtx, collName)
		cancel()
		if err == nil {
			t.RowCount = int64(info.GetPointsCount())
			t.Columns = append(t.Columns, &schema.Column{
				Name:    "vector",
				Type:    "float[]",
				Comment: "embedding vector",
			})
			t.Engine = "qdrant"
		}
		database.Tables = append(database.Tables, t)
	}

	inst.Databases = append(inst.Databases, database)
	return inst, nil
}

// qdrantQuerySpec describes a read-only Qdrant operation.
type qdrantQuerySpec struct {
	Scroll string `json:"scroll"` // collection name to scroll points from
	Count  string `json:"count"`  // collection name to count points
	Limit  int    `json:"limit"`  // max points to return (for scroll)
}

// ExecQuery implements query.Queryable for Qdrant.
// Accepts JSON describing read-only operations:
//
//	{"scroll": "collection_name", "limit": 100}
//	{"count": "collection_name"}
func (qdrantConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	var spec qdrantQuerySpec
	if err := json.Unmarshal([]byte(opts.SQL), &spec); err != nil {
		return nil, fmt.Errorf("qdrant query: invalid JSON — expected {\"scroll\": \"...\", \"limit\": N} or {\"count\": \"...\"}: %w", err)
	}

	if spec.Scroll == "" && spec.Count == "" {
		return nil, fmt.Errorf("READ_ONLY_VIOLATION: qdrant query must specify \"scroll\" or \"count\"")
	}

	logSQL := TruncateSQL(opts.SQL)
	Logf(ctx, "[qdrant] [execute] %s", logSQL)

	host := opts.DSN.Host
	if host == "" {
		host = "localhost"
	}
	intPort, err := strconv.Atoi(opts.DSN.Port)
	if err != nil || intPort == 0 {
		intPort = 6334
	}

	cfg := &qdrant.Config{Host: host, Port: intPort, APIKey: opts.DSN.Password, UseTLS: false}
	client, err := qdrant.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("qdrant connect: %w", err)
	}
	defer client.Close()

	start := time.Now()
	result := &query.QueryResult{}

	if spec.Count != "" {
		infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		info, err := client.GetCollectionInfo(infoCtx, spec.Count)
		if err != nil {
			return nil, fmt.Errorf("qdrant count: %w", err)
		}
		count := fmt.Sprintf("%d", info.GetPointsCount())
		result.Columns = []query.ColumnInfo{{Name: "collection", Type: "string"}, {Name: "points_count", Type: "int64"}}
		collName := spec.Count
		result.Rows = [][]*string{{&collName, &count}}
		result.RowCount = 1
	} else {
		// Scroll: use ListCollections + GetCollectionInfo per collection
		listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		collections, err := client.ListCollections(listCtx)
		if err != nil {
			return nil, fmt.Errorf("qdrant list: %w", err)
		}

		result.Columns = []query.ColumnInfo{
			{Name: "collection", Type: "string"},
			{Name: "points_count", Type: "int64"},
		}
		for _, collName := range collections {
			if spec.Scroll != "" && collName != spec.Scroll {
				continue
			}
			infoCtx, c2 := context.WithTimeout(ctx, 3*time.Second)
			info, err := client.GetCollectionInfo(infoCtx, collName)
			c2()
			if err != nil {
				continue
			}
			count := fmt.Sprintf("%d", info.GetPointsCount())
			name := collName
			result.Rows = append(result.Rows, []*string{&name, &count})
			if spec.Scroll != "" {
				break
			}
		}
		result.RowCount = len(result.Rows)
	}
	result.ExecutionTime = time.Since(start).String()
	return result, nil
}