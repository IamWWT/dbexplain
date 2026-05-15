package connector

import (
	"context"
	"strconv"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"dbexplain/dsn"
	"dbexplain/schema"
)

func init() {
	Register("qdrant", func() Connector { return qdrantConnector{} })
}

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
		logf(ctx, "[qdrant] 采集集合 %d/%d: %s", i+1, total, collName)
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