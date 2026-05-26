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
	"github.com/IamWWT/dbexplain/capabilities"
	"github.com/IamWWT/dbexplain/dsn"
	"github.com/IamWWT/dbexplain/query"
	"github.com/IamWWT/dbexplain/schema"
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

// ExecQuery implements query.Queryable for Elasticsearch via _sql endpoint.
// ES supports standard SQL since v6.3 — read-only-only, validated by sqlguard.
func (esConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	host := opts.DSN.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := opts.DSN.Port
	if port == "" {
		port = "9200"
	}
	scheme := "http"
	if opts.DSN.TLS {
		scheme = "https"
	}

	// Build _sql request
	sqlBody := map[string]interface{}{
		"query": opts.SQL,
	}
	if opts.MaxRows > 0 {
		sqlBody["fetch_size"] = opts.MaxRows
	}
	bodyBytes, _ := json.Marshal(sqlBody)

	reqURL := fmt.Sprintf("%s://%s:%s/_sql", scheme, host, port)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("es sql request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if opts.DSN.User != "" {
		httpReq.SetBasicAuth(opts.DSN.User, opts.DSN.Password)
	}

	httpCli := &http.Client{Timeout: time.Duration(opts.Timeout+5) * time.Second}
	if opts.DSN.TLS {
		httpCli.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	start := time.Now()
	resp, err := httpCli.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("es sql: %w", err)
	}
	defer resp.Body.Close()
	rbody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("es sql HTTP %d: %s", resp.StatusCode, string(rbody))
	}

	var esResp struct {
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Rows [][]interface{} `json:"rows"`
	}
	if err := json.Unmarshal(rbody, &esResp); err != nil {
		return nil, fmt.Errorf("es sql parse: %w\nbody: %s", err, string(rbody[:min(300, len(rbody))]))
	}

	result := &query.QueryResult{}
	for _, col := range esResp.Columns {
		result.Columns = append(result.Columns, query.ColumnInfo{Name: col.Name, Type: col.Type})
	}
	for i, row := range esResp.Rows {
		if i >= opts.MaxRows {
			result.Truncated = true
			break
		}
		sr := make([]*string, len(row))
		for j, v := range row {
			if v == nil {
				sr[j] = nil
			} else {
				s := fmt.Sprintf("%v", v)
				sr[j] = &s
			}
		}
		result.Rows = append(result.Rows, sr)
	}
	result.RowCount = len(result.Rows)
	if len(esResp.Rows) > opts.MaxRows {
		result.RowCount = opts.MaxRows
	}
	result.ExecutionTime = time.Since(start).String()
	return result, nil
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
