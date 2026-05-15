package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"dbexplain/dsn"
	"dbexplain/schema"
)

type esConnector struct{}

func (esConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{fmt.Sprintf("http://%s:%s", d.Host, d.Port)},
		Username:  d.User,
		Password:  d.Password,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 2,
		},
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("es client: %w", err)
	}

	// 健康检查
	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := client.Info(client.Info.WithContext(infoCtx))
	if err != nil {
		return nil, fmt.Errorf("es info: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("es unhealthy: %s", string(body))
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "elasticsearch", Label: d.Label}

	// 获取所有索引
	catCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err = client.Cat.Indices(
		client.Cat.Indices.WithContext(catCtx),
		client.Cat.Indices.WithFormat("json"),
	)
	if err != nil {
		return nil, fmt.Errorf("cat indices: %w", err)
	}
	defer res.Body.Close()
	var indices []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return nil, fmt.Errorf("parse indices: %w", err)
	}

	database := &schema.Database{Name: "elasticsearch"}
	for _, idx := range indices {
		indexName := idx["index"].(string)
		if strings.HasPrefix(indexName, ".") {
			continue
		}
		t := &schema.Table{
			Name:   indexName,
			Engine: "elasticsearch",
		}
		// 获取映射
		mapping, err := getESMapping(ctx, client, indexName)
		if err == nil {
			for field, props := range mapping {
				if field == "_all" || field == "_source" {
					continue
				}
				c := &schema.Column{
					Name:    field,
					Type:    fmt.Sprintf("%v", props["type"]),
					Comment: "es field",
				}
				t.Columns = append(t.Columns, c)
			}
		}
		database.Tables = append(database.Tables, t)
	}

	inst.Databases = append(inst.Databases, database)
	return inst, nil
}

func getESMapping(ctx context.Context, client *elasticsearch.Client, indexName string) (map[string]map[string]interface{}, error) {
	res, err := client.Indices.GetMapping(
		client.Indices.GetMapping.WithContext(ctx),
		client.Indices.GetMapping.WithIndex(indexName),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}
	indexData, ok := result[indexName].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected mapping format")
	}
	mappings, ok := indexData["mappings"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no mappings")
	}
	props, ok := mappings["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no properties")
	}
	fields := make(map[string]map[string]interface{})
	for name, raw := range props {
		if propMap, ok := raw.(map[string]interface{}); ok {
			fields[name] = propMap
		}
	}
	return fields, nil
}