//go:build mongodb || full

package connector

import (
	"context"
	"fmt"
	"time"

	"encoding/json"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("mongodb", func() Connector { return mongoConnector{} })
}

type mongoConnector struct{}

func (mongoConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapRowCount,
	}
}

func (mongoConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	if d.DBName == "" {
		return nil, fmt.Errorf("mongodb: database name required in DSN (e.g. mongodb://.../mydb)")
	}

	Logf(ctx, "[mongo] connect start: %s", d.Redacted())

	clientOpts := options.Client().
		ApplyURI(d.Raw).
		SetTimeout(10 * time.Second).
		SetRetryReads(false).
		SetRetryWrites(false)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "connect", err)
	}
	defer func() {
		disCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := client.Disconnect(disCtx); err != nil {
			Logf(ctx, "[mongo] disconnect error: %v", err)
		}
	}()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	Logf(ctx, "[mongo] ping...")
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return nil, schema.NewDBError(d.Redacted(), d.DBName, "", "ping", err)
	}
	Logf(ctx, "[mongo] ping ok")

	db := client.Database(d.DBName)

	colCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	Logf(ctx, "[mongo] list collections: %s", d.DBName)
	collections, err := db.ListCollectionNames(colCtx, bson.D{})
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), d.DBName, "", "list collections", err)
	}
	Logf(ctx, "[mongo] collections found: db=%s count=%d", d.DBName, len(collections))

	database := &schema.Database{Name: d.DBName}
	total := len(collections)
	for i, collName := range collections {
		Logf(ctx, "[mongo] 采集集合 %d/%d: %s", i+1, total, collName)
		table := collectMongoCollectionMeta(ctx, db, collName)
		database.Tables = append(database.Tables, table)
	}

	inst := &schema.Instance{
		DSN:       d.Redacted(),
		Kind:      "mongodb",
		Label:     d.Label,
		Databases: []*schema.Database{database},
	}
	Logf(ctx, "[mongo] collect done")
	return inst, nil
}

// mongoQuerySpec describes a read-only MongoDB query.
// Accepted operations: "find", "aggregate"
type mongoQuerySpec struct {
	Find      string                   `json:"find"`
	Aggregate string                   `json:"aggregate"`
	Filter    map[string]interface{}   `json:"filter"`
	Pipeline  []map[string]interface{} `json:"pipeline"`
	Limit     int64                    `json:"limit"`
}

// ExecQuery implements query.Queryable for MongoDB.
// Accepts JSON describing read-only operations:
//
//	{"find": "collection", "filter": {...}, "limit": 100}
//	{"aggregate": "collection", "pipeline": [...], "limit": 100}
func (mongoConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	var spec mongoQuerySpec
	if err := json.Unmarshal([]byte(opts.SQL), &spec); err != nil {
		return nil, fmt.Errorf("mongo query: invalid JSON — expected {\"find\": \"...\", \"filter\": {...}} or {\"aggregate\": \"...\", \"pipeline\": [...]}: %w", err)
	}

	if spec.Find == "" && spec.Aggregate == "" {
		return nil, fmt.Errorf("READ_ONLY_VIOLATION: mongo query must specify \"find\" or \"aggregate\"")
	}
	if spec.Find != "" && spec.Aggregate != "" {
		return nil, fmt.Errorf("READ_ONLY_VIOLATION: specify either \"find\" or \"aggregate\", not both")
	}

	logSQL := TruncateSQL(opts.SQL)
	Logf(ctx, "[mongo] [execute] %s", logSQL)

	if opts.DSN.DBName == "" {
		return nil, fmt.Errorf("mongo query: database name required in DSN")
	}

	// Connect
	clientOpts := options.Client().
		ApplyURI(opts.DSN.Raw).
		SetTimeout(time.Duration(opts.Timeout+5) * time.Second).
		SetRetryReads(false).
		SetRetryWrites(false)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(opts.DSN.DBName)
	limit := spec.Limit
	if limit == 0 {
		limit = int64(opts.MaxRows)
	}

	start := time.Now()
	var docs []bson.M

	if spec.Find != "" {
		filter := spec.Filter
		if filter == nil {
			filter = bson.M{}
		}
		findOpts := options.Find().SetLimit(limit)
		cursor, err := db.Collection(spec.Find).Find(ctx, filter, findOpts)
		if err != nil {
			return nil, fmt.Errorf("mongo find: %w", err)
		}
		defer cursor.Close(ctx)
		if err := cursor.All(ctx, &docs); err != nil {
			return nil, fmt.Errorf("mongo find decode: %w", err)
		}
	} else {
		pipeline := spec.Pipeline
		if pipeline == nil {
			pipeline = []map[string]interface{}{}
		}
		// Reject write stages in aggregation pipeline (with $facet recursion)
		mongoWriteStages := map[string]bool{
			"$out": true, "$merge": true, "$indexStats": true,
		}
		var checkStage func(stage map[string]interface{}) error
		checkStage = func(stage map[string]interface{}) error {
			for key, val := range stage {
				if mongoWriteStages[strings.ToLower(key)] {
					return fmt.Errorf("READ_ONLY_VIOLATION: write stage %q is not allowed in aggregation pipeline", key)
				}
				// $facet contains nested sub-pipelines; recursively check each one
				if strings.ToLower(key) == "$facet" {
					if facetMap, ok := val.(map[string]interface{}); ok {
						for _, facetVal := range facetMap {
							subPipeline, ok := facetVal.([]interface{})
							if !ok {
								continue
							}
							for _, rawSubStage := range subPipeline {
								subStage, ok := rawSubStage.(map[string]interface{})
								if !ok {
									continue
								}
								if err := checkStage(subStage); err != nil {
									return err
								}
							}
						}
					}
				}
			}
			return nil
		}
		for _, stage := range pipeline {
			if err := checkStage(stage); err != nil {
				return nil, err
			}
		}
		// Append $limit to pipeline if specified
		if limit > 0 {
			pipeline = append(pipeline, map[string]interface{}{"$limit": limit})
		}
		aggPipeline := make(mongo.Pipeline, 0, len(pipeline))
		for _, stage := range pipeline {
			raw, err := bson.Marshal(stage)
			if err != nil {
				return nil, fmt.Errorf("mongo marshal stage: %w", err)
			}
			var doc bson.D
			if err := bson.Unmarshal(raw, &doc); err != nil {
				return nil, fmt.Errorf("mongo unmarshal stage: %w", err)
			}
			aggPipeline = append(aggPipeline, doc)
		}
		cursor, err := db.Collection(spec.Aggregate).Aggregate(ctx, aggPipeline)
		if err != nil {
			return nil, fmt.Errorf("mongo aggregate: %w", err)
		}
		defer cursor.Close(ctx)
		if err := cursor.All(ctx, &docs); err != nil {
			return nil, fmt.Errorf("mongo aggregate decode: %w", err)
		}
	}

	// Convert to QueryResult — collect all unique keys as columns
	result := &query.QueryResult{}
	colSet := map[string]int{} // name → index
	var colOrder []string

	// First pass: collect all column names
	for _, doc := range docs {
		for k := range doc {
			if _, exists := colSet[k]; !exists {
				colSet[k] = len(colOrder)
				colOrder = append(colOrder, k)
			}
		}
	}

	// Build columns
	for _, name := range colOrder {
		result.Columns = append(result.Columns, query.ColumnInfo{Name: name, Type: "bson"})
	}

	// Build rows
	for i, doc := range docs {
		if i >= opts.MaxRows {
			result.Truncated = true
			break
		}
		row := make([]*string, len(colOrder))
		for j, name := range colOrder {
			if v, ok := doc[name]; ok {
				s := stringifyVal(v)
				row[j] = &s
			}
		}
		result.Rows = append(result.Rows, row)
	}
	result.RowCount = len(result.Rows)
	result.ExecutionTime = time.Since(start).String()
	return result, nil
}

// stringifyVal converts a BSON value to string representation.
func stringifyVal(v interface{}) string {
	if v == nil {
		return "null"
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

func collectMongoCollectionMeta(ctx context.Context, db *mongo.Database, collName string) *schema.Table {
	t := &schema.Table{
		Name:   collName,
		Engine: "WiredTiger",
	}

	coll := db.Collection(collName)

	estCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if n, err := coll.EstimatedDocumentCount(estCtx); err != nil {
		Logf(ctx, "[mongo] estimated count failed for %s.%s: %v", db.Name(), collName, err)
	} else {
		t.RowCount = n
	}

	t.Columns = append(t.Columns, &schema.Column{
		Name:    "_id",
		Type:    "objectId",
		Comment: "mongodb document primary key",
	})
	return t
}