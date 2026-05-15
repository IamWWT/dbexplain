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

type mongoConnector struct{}

func (mongoConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	if d.DBName == "" {
		return nil, fmt.Errorf("mongodb: database name required in DSN (e.g. mongodb://.../mydb)")
	}

	logf(ctx, "[mongo] connect start: %s", d.Redacted())

	// 使用 CSOT 统一超时，禁止重试，避免驱动内部循环卡死
	clientOpts := options.Client().
		ApplyURI(d.Raw).
		SetTimeout(10 * time.Second). // 覆盖所有操作的超时
		SetRetryReads(false).
		SetRetryWrites(false)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	defer func() {
		disCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := client.Disconnect(disCtx); err != nil {
			logf(ctx, "[mongo] disconnect error: %v", err)
		}
	}()

	// Ping 使用独立超时（从外部 ctx 派生，受 CSOT 和外部双重保护）
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	logf(ctx, "[mongo] ping...")
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	logf(ctx, "[mongo] ping ok")

	db := client.Database(d.DBName)

	// 列出集合
	colCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	logf(ctx, "[mongo] list collections: %s", d.DBName)
	collections, err := db.ListCollectionNames(colCtx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list collections for db %s: %w", d.DBName, err)
	}
	logf(ctx, "[mongo] collections found: db=%s count=%d", d.DBName, len(collections))

	database := &schema.Database{Name: d.DBName}
	for _, collName := range collections {
		logf(ctx, "[mongo] collection stats: %s.%s", d.DBName, collName)
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

// collectMongoCollectionMeta 只获取近似文档数，不采样任何文档，确保零风险
func collectMongoCollectionMeta(ctx context.Context, db *mongo.Database, collName string) *schema.Table {
	t := &schema.Table{
		Name:   collName,
		Engine: "WiredTiger",
	}

	coll := db.Collection(collName)

	// 获取近似文档数（独立超时）
	estCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if n, err := coll.EstimatedDocumentCount(estCtx); err != nil {
		logf(ctx, "[mongo] estimated count failed for %s.%s: %v", db.Name(), collName, err)
	} else {
		t.RowCount = n
	}

	// 仅添加虚拟主键列，不做任何文档采样
	t.Columns = append(t.Columns, &schema.Column{
		Name:    "_id",
		Type:    "objectId",
		Comment: "mongodb document primary key",
	})
	return t
}