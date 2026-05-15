package connector

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"dbexplain/dsn"
	"dbexplain/schema"
)

type qdrantConnector struct{}

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
		return nil, fmt.Errorf("qdrant connect: %w", err)
	}
	defer client.Close()

	// 健康检查使用外部上下文，并加短超时
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := client.HealthCheck(healthCtx); err != nil {
		return nil, fmt.Errorf("qdrant health: %w", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "qdrant", Label: d.Label}

	// 列出集合
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	collections, err := client.ListCollections(listCtx)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}

	database := &schema.Database{Name: "default"}
	for _, collName := range collections {
		t := &schema.Table{
			Name:    collName,
			Comment: "vector collection",
		}
		// 获取集合信息（独立超时）
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