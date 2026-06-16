//go:build clickhouse || full

package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("clickhouse", func() Connector { return clickhouseConnector{} })
}

type clickhouseConnector struct{}

func (clickhouseConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapSampling,
		capabilities.CapRowCount,
		capabilities.CapPartition,
		capabilities.CapSQL,
	}
}

func (clickhouseConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "8123"
	}
	base := fmt.Sprintf("http://%s:%s", host, port)
	cli := &chHTTP{
		base:    base,
		user:    d.User,
		pass:    d.Password,
		httpCli: &http.Client{Timeout: 10 * time.Second},
	}

	if err := cli.ping(ctx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "connect", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "clickhouse", Label: d.Label}

	var dbNames []string
	if d.DBName != "" && d.DBName != "default" {
		dbNames = []string{d.DBName}
	} else {
		Logf(ctx, "[clickhouse] [collect] %s", "SELECT name FROM system.databases WHERE name NOT IN ('system','information_schema','INFORMATION_SCHEMA') ORDER BY name")
		rows, err := cli.queryRows(ctx, "SELECT name FROM system.databases WHERE name NOT IN ('system','information_schema','INFORMATION_SCHEMA') ORDER BY name")
		if err != nil {
			return nil, schema.NewDBError(d.Redacted(), "", "", "list databases", err)
		}
		for _, r := range rows {
			dbNames = append(dbNames, r[0])
		}
	}

	for _, dbName := range dbNames {
		Logf(ctx, "[clickhouse] collecting database %s", dbName)
		database, err := collectCHDB(ctx, cli, dbName, d.Redacted())
		if err != nil {
			Logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectCHDB(ctx context.Context, cli *chHTTP, dbName, redactedDSN string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}
	Logf(ctx, "[clickhouse] [collect] %s", "SELECT name, engine, toUInt64(total_rows), toUInt64(total_bytes), comment FROM system.tables WHERE database='%s' AND engine NOT LIKE '%%View%%' ORDER BY name")
	rows, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT name, engine, toUInt64(total_rows), toUInt64(total_bytes), comment
		FROM system.tables WHERE database='%s' AND engine NOT LIKE '%%View%%'
		ORDER BY name`, escCH(dbName)))
	if err != nil {
		return nil, schema.NewDBError(redactedDSN, dbName, "", "query tables", err)
	}
	var tables []*schema.Table
	for _, r := range rows {
		t := &schema.Table{Name: r[0], Engine: r[1], Comment: r[4]}
		fmt.Sscan(r[2], &t.RowCount)
		fmt.Sscan(r[3], &t.SizeBytes)
		tables = append(tables, t)
	}
	total := len(tables)
	for i, t := range tables {
		Logf(ctx, "[%s] 采集表 %d/%d: %s", dbName, i+1, total, t.Name)
		fillCHTable(ctx, cli, dbName, t, redactedDSN)
		database.Tables = append(database.Tables, t)
	}

	// 操作语义采集 (Phase 3) — system.query_log 始终可用
	collectCHOpStats(ctx, cli, dbName, database.Tables)

	return database, nil
}

// collectCHOpStats 从 system.query_log 获取每表查询频率统计。
// ClickHouse 的 tables 字段是内核解析数组，无需正则提取，无文本解析误差。
func collectCHOpStats(ctx context.Context, cli *chHTTP, dbName string, tables []*schema.Table) {
	// 批量查询所有表的 query_count（最近 7 天）
	Logf(ctx, "[clickhouse] [collect] %s", "SELECT table_name, count() AS cnt, avg(query_duration_ms) AS avg_ms FROM system.query_log ARRAY JOIN tables AS table_name WHERE type = 'QueryFinish' AND event_time > now() - INTERVAL 7 DAY AND tables IS NOT NULL AND database = '%s' GROUP BY table_name")
	rows, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT table_name, count() AS cnt, avg(query_duration_ms) AS avg_ms
		FROM system.query_log
		ARRAY JOIN tables AS table_name
		WHERE type = 'QueryFinish'
		  AND event_time > now() - INTERVAL 7 DAY
		  AND tables IS NOT NULL
		  AND database = '%s'
		GROUP BY table_name`, escCH(dbName)))
	if err != nil {
		Logf(ctx, "[clickhouse] query_log unavailable for %s: %v", dbName, err)
		return
	}

	// 分配到各表
	statsMap := make(map[string]*schema.OpStats, len(rows))
	for _, r := range rows {
		var queryCount int64
		var avgMs float64
		fmt.Sscan(r[1], &queryCount)
		fmt.Sscan(r[2], &avgMs)
		statsMap[r[0]] = &schema.OpStats{QueryCount: queryCount, AvgDurationMs: avgMs}
	}

	for _, t := range tables {
		if s, ok := statsMap[t.Name]; ok {
			t.OpStats = s
		}
	}
}

func fillCHTable(ctx context.Context, cli *chHTTP, dbName string, t *schema.Table, redactedDSN string) {
	Logf(ctx, "[clickhouse] [collect] %s", "SELECT name, type, default_kind, default_expression, comment, is_in_primary_key, is_in_sorting_key, is_in_partition_key FROM system.columns WHERE database='%s' AND table='%s' ORDER BY position")
	rows, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT name, type, default_kind, default_expression, comment,
		       is_in_primary_key, is_in_sorting_key, is_in_partition_key
		FROM system.columns WHERE database='%s' AND table='%s'
		ORDER BY position`, escCH(dbName), escCH(t.Name)))
	if err != nil {
		Logf(ctx, "[clickhouse] columns error %s: %v", t.Name, err)
		return
	}
	var colsWithoutComment []*schema.Column
	for _, r := range rows {
		c := &schema.Column{
			Name:    r[0],
			Type:    r[1],
			Comment: r[4],
		}
		if r[2] != "" {
			c.Default = r[2] + " " + r[3]
		}
		c.IsPrimary = r[5] == "1"
		c.IsSortKey = r[6] == "1"
		c.IsPartitionKey = r[7] == "1"
		t.Columns = append(t.Columns, c)
		if c.Comment == "" {
			colsWithoutComment = append(colsWithoutComment, c)
		}
	}

	// 推断注释
	if len(colsWithoutComment) > 0 && t.RowCount > 0 {
		sample, err := fetchCHSampleRow(ctx, cli, dbName, t.Name)
		if err == nil {
			for _, c := range colsWithoutComment {
				if val, ok := sample[c.Name]; ok {
					c.Comment = schema.InferComment(c.Name, c.Type, val)
				}
			}
		} else {
			Logf(ctx, "[clickhouse] sample row failed for %s.%s: %v", dbName, t.Name, err)
		}
	}

	// 表元数据
	Logf(ctx, "[clickhouse] [collect] %s", "SELECT partition_key, sorting_key, primary_key FROM system.tables WHERE database='%s' AND name='%s'")
	meta, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT partition_key, sorting_key, primary_key
		FROM system.tables WHERE database='%s' AND name='%s'`, escCH(dbName), escCH(t.Name)))
	if err == nil && len(meta) > 0 {
		t.PartitionKey = meta[0][0]
		t.OrderByKey = meta[0][1]
	}
}

func fetchCHSampleRow(ctx context.Context, cli *chHTTP, dbName, table string) (map[string]string, error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 1", escCH(dbName), escCH(table))
	rows, err := cli.queryRows(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows")
	}
	// 需要获取列名
	meta, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT name FROM system.columns WHERE database='%s' AND table='%s' ORDER BY position`, escCH(dbName), escCH(table)))
	if err != nil {
		return nil, err
	}
	var cols []string
	for _, r := range meta {
		cols = append(cols, r[0])
	}
	if len(rows[0]) != len(cols) {
		return nil, fmt.Errorf("column count mismatch")
	}
	result := make(map[string]string)
	for i, col := range cols {
		if i < len(rows[0]) {
			result[col] = rows[0][i]
		}
	}
	return result, nil
}

// ── chHTTP helper 保持不变，略
type chHTTP struct {
	base    string
	user    string
	pass    string
	httpCli *http.Client
}

func (c *chHTTP) ping(ctx context.Context) error {
	_, err := c.query(ctx, "SELECT 1")
	return err
}

func (c *chHTTP) queryRows(ctx context.Context, sql string) ([][]string, error) {
	body, err := c.query(ctx, sql+" FORMAT JSONCompact")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data [][]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ch json parse: %w\nbody: %s", err, string(body[:min(200, len(body))]))
	}
	var out [][]string
	for _, row := range resp.Data {
		var sr []string
		for _, v := range row {
			sr = append(sr, fmt.Sprintf("%v", v))
		}
		out = append(out, sr)
	}
	return out, nil
}

func (c *chHTTP) query(ctx context.Context, sql string) ([]byte, error) {
	u := c.base + "/"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(sql))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	if c.user != "" {
		req.Header.Set("X-ClickHouse-User", c.user)
		if c.pass != "" {
			req.Header.Set("X-ClickHouse-Key", c.pass)
		}
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("clickhouse read response body: %w", readErr)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("clickhouse HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ExecQuery implements query.Queryable for ClickHouse via HTTP.
func (clickhouseConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	host := opts.DSN.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := opts.DSN.Port
	if port == "" {
		port = "8123"
	}
	base := fmt.Sprintf("http://%s:%s", host, port)
	cli := &chHTTP{
		base:    base,
		user:    opts.DSN.User,
		pass:    opts.DSN.Password,
		httpCli: &http.Client{Timeout: time.Duration(opts.Timeout+5) * time.Second},
	}

	// Build query with optional max_execution_time
	sql := opts.SQL
	logSQL := TruncateSQL(opts.SQL)
	Logf(ctx, "[clickhouse] [execute] %s", logSQL)
	if opts.Timeout > 0 {
		sql = fmt.Sprintf("%s SETTINGS max_execution_time=%d", sql, opts.Timeout)
	}

	start := time.Now()
	result := &query.QueryResult{}

	// Execute as FORMAT JSON (returns meta+data+rows structure)
	body, err := cli.query(ctx, sql+" FORMAT JSON")
	if err != nil {
		return nil, fmt.Errorf("clickhouse query: %w", err)
	}

	var resp struct {
		Meta []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"meta"`
		Data []map[string]interface{} `json:"data"`
		Rows uint64                   `json:"rows"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("clickhouse response parse: %w\nbody: %s", err, string(body[:min(300, len(body))]))
	}

	for _, m := range resp.Meta {
		result.Columns = append(result.Columns, query.ColumnInfo{Name: m.Name, Type: m.Type})
	}

	colNames := make([]string, len(result.Columns))
	for i, c := range result.Columns {
		colNames[i] = c.Name
	}

	total := int(resp.Rows)
	for i, row := range resp.Data {
		if i >= opts.MaxRows {
			result.Truncated = true
			break
		}
		sr := make([]*string, len(colNames))
		for j, col := range colNames {
			v, ok := row[col]
			if !ok || v == nil {
				sr[j] = nil
			} else {
				s := fmt.Sprintf("%v", v)
				sr[j] = &s
			}
		}
		result.Rows = append(result.Rows, sr)
	}
	result.RowCount = total
	if total > opts.MaxRows {
		result.RowCount = opts.MaxRows
	}
	result.ExecutionTime = time.Since(start).String()
	return result, nil
}

func escCH(s string) string {
	// ClickHouse follows SQL standard: single quotes escaped by doubling
	return strings.ReplaceAll(s, "'", "''")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}