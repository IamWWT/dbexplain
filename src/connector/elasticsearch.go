package connector

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"dbexplain/capabilities"
	"dbexplain/dsn"
	"dbexplain/schema"
)

func init() {
	Register("elasticsearch", func() Connector { return esConnector{} })
	Register("es", func() Connector { return esConnector{} })
}

type esConnector struct{}

func (esConnector) Capabilities() []capabilities.Capability {
	return nil
}

func (esConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "9200"
	}

	scheme := "http"
	transport := &http.Transport{
		MaxIdleConnsPerHost: 2,
	}
	if d.TLS {
		scheme = "https"
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // 诊断工具可接受
		}
	}

	cfg := elasticsearch.Config{
		Addresses: []string{fmt.Sprintf("%s://%s:%s", scheme, host, port)},
		Username:  d.User,
		Password:  d.Password,
		Transport: transport,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "create client", err)
	}

	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := client.Info(client.Info.WithContext(infoCtx))
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "info", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, schema.NewDBError(d.Redacted(), "", "", "unhealthy", fmt.Errorf("%s", string(body)))
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "elasticsearch", Label: d.Label}

	catCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err = client.Cat.Indices(
		client.Cat.Indices.WithContext(catCtx),
		client.Cat.Indices.WithFormat("json"),
	)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "cat indices", err)
	}
	defer res.Body.Close()
	var indices []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "parse indices", err)
	}

	database := &schema.Database{Name: "elasticsearch"}
	total := len(indices)
	count := 0
	for _, idx := range indices {
		indexName, ok := idx["index"].(string)
		if !ok || strings.HasPrefix(indexName, ".") {
			continue
		}
		count++
		logf(ctx, "[es] 采集索引 %d/%d: %s", count, total, indexName)
		t := &schema.Table{
			Name:   indexName,
			Engine: "elasticsearch",
		}
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
