package connector

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"dbexplain/dsn"
	"dbexplain/schema"
)

func init() {
	Register("mongodb", func() Connector { return mongoConnector{} })
}

type mongoConnector struct{}

func (mongoConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	if d.DBName == "" {
		return nil, fmt.Errorf("mongodb: database name required in DSN (e.g. mongodb://.../mydb)")
	}

	logf(ctx, "[mongo] connect start: %s", d.Redacted())

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
			logf(ctx, "[mongo] disconnect error: %v", err)
		}
	}()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	logf(ctx, "[mongo] ping...")
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return nil, schema.NewDBError(d.Redacted(), d.DBName, "", "ping", err)
	}
	logf(ctx, "[mongo] ping ok")

	db := client.Database(d.DBName)

	colCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	logf(ctx, "[mongo] list collections: %s", d.DBName)
	collections, err := db.ListCollectionNames(colCtx, bson.D{})
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), d.DBName, "", "list collections", err)
	}
	logf(ctx, "[mongo] collections found: db=%s count=%d", d.DBName, len(collections))

	database := &schema.Database{Name: d.DBName}
	total := len(collections)
	for i, collName := range collections {
		logf(ctx, "[mongo] 采集集合 %d/%d: %s", i+1, total, collName)
		table := collectMongoCollectionMeta(ctx, db, collName)
		database.Tables = append(database.Tables, table)
	}

	inst := &schema.Instance{
		DSN:       d.Redacted(),
		Kind:      "mongodb",
		Label:     d.Label,
		Databases: []*schema.Database{database},
	}
	logf(ctx, "[mongo] collect done")
	return inst, nil
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
		logf(ctx, "[mongo] estimated count failed for %s.%s: %v", db.Name(), collName, err)
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