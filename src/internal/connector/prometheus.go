//go:build prometheus || full

package connector

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("prometheus", func() Connector { return promConnector{} })
}

type promConnector struct{}

func (promConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapRowCount,
		capabilities.CapPromQL,
	}
}

// ── HTTP 辅助 ──

func promHTTPClient(d *dsn.DSN, timeout int) *http.Client {
	cl := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	if d.TLS {
		cl.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: d.TLSSkipVerify},
		}
	}
	return cl
}

func promBaseURL(d *dsn.DSN) string {
	scheme := "http"
	if d.TLS {
		scheme = "https"
	}
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "9090"
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func doPromRequest(ctx context.Context, baseURL, path string, d *dsn.DSN, timeout int) ([]byte, error) {
	reqURL := baseURL + path
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("prometheus request: %w", err)
	}
	if d.User != "" {
		req.SetBasicAuth(d.User, d.Password)
	}
	cl := promHTTPClient(d, timeout)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prometheus HTTP: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("prometheus read body: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("prometheus HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ── Collect ──

func (promConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	baseURL := promBaseURL(d)
	timeout := promTimeout(d)

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "prometheus", Label: d.Label}
	db := &schema.Database{Name: "prometheus"}

	// 1. Labels
	if err := collectLabels(ctx, baseURL, d, timeout, db); err != nil {
		Logf(ctx, "[prometheus] labels collect warning: %v", err)
	}

	// 2. Metrics metadata
	if err := collectMetricsMeta(ctx, baseURL, d, timeout, db); err != nil {
		Logf(ctx, "[prometheus] metadata collect warning: %v", err)
	}

	inst.Databases = append(inst.Databases, db)
	return inst, nil
}

func promTimeout(d *dsn.DSN) int {
	if v := d.DSNParam("timeout"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10 // default 10s
}

// collectLabels 采集所有 label 名 → _labels 表（仅结构描述，实际值通过 PromQL 查询）
func collectLabels(ctx context.Context, baseURL string, d *dsn.DSN, timeout int, db *schema.Database) error {
	body, err := doPromRequest(ctx, baseURL, "/api/v1/labels", d, timeout)
	if err != nil {
		return err
	}
	var resp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parse labels: %w", err)
	}
	if resp.Status != "success" {
		return fmt.Errorf("labels API status: %s", resp.Status)
	}

	db.Tables = append(db.Tables, &schema.Table{
		Name:    "_labels",
		Engine:  "prometheus_meta",
		RowCount: int64(len(resp.Data)),
		Columns: []*schema.Column{
			{Name: "name", Type: "string", Comment: "label name"},
		},
		Comment: fmt.Sprintf("%d labels — query via PromQL: label_values()", len(resp.Data)),
		SampleRows: labelsToSampleRows(resp.Data),
	})
	return nil
}

// collectMetricsMeta 采集 metric 元数据 → _metrics 表
func collectMetricsMeta(ctx context.Context, baseURL string, d *dsn.DSN, timeout int, db *schema.Database) error {
	body, err := doPromRequest(ctx, baseURL, "/api/v1/metadata", d, timeout)
	if err != nil {
		return err
	}
	var resp struct {
		Status string `json:"status"`
		Data   map[string][]struct {
			Type string `json:"type"`
			Help string `json:"help"`
			Unit string `json:"unit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	if resp.Status != "success" {
		return fmt.Errorf("metadata API status: %s", resp.Status)
	}

	// 排序 metric 名
	names := make([]string, 0, len(resp.Data))
	for n := range resp.Data {
		names = append(names, n)
	}
	sort.Strings(names)

	type metricRow struct {
		Metric string `json:"metric"`
		Type   string `json:"type"`
		Help   string `json:"help"`
		Unit   string `json:"unit"`
	}
	rows := make([]metricRow, 0, len(names))
	for _, n := range names {
		entries := resp.Data[n]
		for _, e := range entries {
			rows = append(rows, metricRow{Metric: n, Type: e.Type, Help: e.Help, Unit: e.Unit})
		}
	}

	samples := make([]map[string]any, len(rows))
	for i, r := range rows {
		samples[i] = map[string]any{"metric": r.Metric, "type": r.Type, "help": r.Help, "unit": r.Unit}
	}

	db.Tables = append(db.Tables, &schema.Table{
		Name:    "_metrics",
		Engine:  "prometheus_meta",
		RowCount: int64(len(rows)),
		Columns: []*schema.Column{
			{Name: "metric", Type: "string"},
			{Name: "type", Type: "string", Comment: "counter / gauge / histogram / summary"},
			{Name: "help", Type: "string"},
			{Name: "unit", Type: "string"},
		},
		SampleRows: samples,
	})
	return nil
}



// labelsToSampleRows converts a label name list from Prometheus /api/v1/labels
// into schema.SampleRows format for the _labels meta table.
func labelsToSampleRows(data []string) []map[string]any {
	samples := make([]map[string]any, len(data))
	for i, label := range data {
		samples[i] = map[string]any{"name": label}
	}
	return samples
}
// ── ExecQuery (PromQL) ──

func (promConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	sql := strings.TrimSpace(opts.SQL)
	if sql == "" {
		return nil, fmt.Errorf("READ_ONLY_VIOLATION: empty PromQL query")
	}

	logSQL := TruncateSQL(opts.SQL)
	Logf(ctx, "[prometheus] [execute] %s", logSQL)

	d := opts.DSN
	baseURL := promBaseURL(d)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = promTimeout(d)
	}

	// GET /api/v1/query?query=<promql>
	path := fmt.Sprintf("/api/v1/query?query=%s", url.QueryEscape(sql))
	body, err := doPromRequest(ctx, baseURL, path, d, timeout)
	if err != nil {
		return nil, fmt.Errorf("prometheus query: %w", err)
	}

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string          `json:"resultType"`
			Result     json.RawMessage `json:"result"`
		} `json:"data"`
		Error   string `json:"error,omitempty"`
		Warnings []string `json:"warnings,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse PromQL response: %w", err)
	}
	if resp.Status != "success" {
		errMsg := resp.Error
		if errMsg == "" {
			errMsg = "unknown PromQL error"
		}
		return nil, fmt.Errorf("PromQL error: %s", errMsg)
	}

	start := time.Now()
	result := &query.QueryResult{}

	switch resp.Data.ResultType {
	case "vector":
		var items []promVectorItem
		if err := json.Unmarshal(resp.Data.Result, &items); err != nil {
			return nil, fmt.Errorf("parse vector result: %w", err)
		}
		promVectorResult(result, items, opts.MaxRows)

	case "matrix":
		var items []promMatrixItem
		if err := json.Unmarshal(resp.Data.Result, &items); err != nil {
			return nil, fmt.Errorf("parse matrix result: %w", err)
		}
		promMatrixResult(result, items, opts.MaxRows)

	case "scalar":
		var item []interface{}
		if err := json.Unmarshal(resp.Data.Result, &item); err != nil {
			return nil, fmt.Errorf("parse scalar result: %w", err)
		}
		promScalarResult(result, item)

	case "string":
		var item []interface{}
		if err := json.Unmarshal(resp.Data.Result, &item); err != nil {
			return nil, fmt.Errorf("parse string result: %w", err)
		}
		result.Columns = []query.ColumnInfo{{Name: "value", Type: "string"}}
		if len(item) == 2 {
			s := fmt.Sprintf("%v", item[1])
			result.Rows = append(result.Rows, []*string{&s})
		}

	default:
		return nil, fmt.Errorf("unsupported PromQL result type: %s", resp.Data.ResultType)
	}

	result.RowCount = len(result.Rows)
	result.ExecutionTime = time.Since(start).String()
	return result, nil
}

// ── PromQL result type handlers ──

type promVectorItem struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type promMatrixItem struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

// promVectorResult 处理即时向量结果
func promVectorResult(result *query.QueryResult, items []promVectorItem, maxRows int) {
	if len(items) == 0 {
		result.Columns = []query.ColumnInfo{{Name: "(empty)", Type: "string"}}
		return
	}

	labelKeys := collectLabelKeysFromItems(len(items), func(i int) map[string]string {
		return items[i].Metric
	})
	result.Columns = buildColumnsFromKeys(labelKeys)

	for i, item := range items {
		if i >= maxRows {
			result.Truncated = true
			break
		}
		row := buildRowFromLabels(labelKeys, item.Metric)
		if len(item.Value) >= 2 {
			ts := fmt.Sprintf("%v", item.Value[0])
			val := fmt.Sprintf("%v", item.Value[1])
			row = append(row, &ts, &val)
		}
		result.Rows = append(result.Rows, row)
	}
}

// promMatrixResult 处理范围向量结果（explode values）
func promMatrixResult(result *query.QueryResult, items []promMatrixItem, maxRows int) {
	if len(items) == 0 {
		result.Columns = []query.ColumnInfo{{Name: "(empty)", Type: "string"}}
		return
	}

	labelKeys := collectLabelKeysFromItems(len(items), func(i int) map[string]string {
		return items[i].Metric
	})
	result.Columns = buildColumnsFromKeys(labelKeys)

	rowCount := 0
	for _, item := range items {
		for _, val := range item.Values {
			if rowCount >= maxRows {
				result.Truncated = true
				break
			}
			row := buildRowFromLabels(labelKeys, item.Metric)
			if len(val) >= 2 {
				ts := fmt.Sprintf("%v", val[0])
				v := fmt.Sprintf("%v", val[1])
				row = append(row, &ts, &v)
			}
			result.Rows = append(result.Rows, row)
			rowCount++
		}
		if result.Truncated {
			break
		}
	}
}

// promScalarResult 处理标量结果
func promScalarResult(result *query.QueryResult, item []interface{}) {
	result.Columns = []query.ColumnInfo{
		{Name: "timestamp", Type: "float"},
		{Name: "value", Type: "float"},
	}
	if len(item) >= 2 {
		ts := fmt.Sprintf("%v", item[0])
		val := fmt.Sprintf("%v", item[1])
		result.Rows = append(result.Rows, []*string{&ts, &val})
	}
}

// collectLabelKeysFromItems 收集所有 result 中的 label key 集合
func collectLabelKeysFromItems(n int, getMetric func(i int) map[string]string) []string {
	keySet := make(map[string]bool)
	for i := 0; i < n; i++ {
		for k := range getMetric(i) {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildColumnsFromKeys(keys []string) []query.ColumnInfo {
	cols := make([]query.ColumnInfo, 0, len(keys)+2)
	for _, k := range keys {
		cols = append(cols, query.ColumnInfo{Name: k, Type: "string"})
	}
	cols = append(cols, query.ColumnInfo{Name: "timestamp", Type: "float"})
	cols = append(cols, query.ColumnInfo{Name: "value", Type: "string"})
	return cols
}

func buildRowFromLabels(keys []string, labels map[string]string) []*string {
	row := make([]*string, 0, len(keys)+2)
	for _, k := range keys {
		if v, ok := labels[k]; ok {
			val := v
			row = append(row, &val)
		} else {
			row = append(row, nil)
		}
	}
	return row
}
