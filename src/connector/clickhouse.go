package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dbexplain/dsn"
	"dbexplain/schema"
)

type clickhouseConnector struct{}

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
		return nil, fmt.Errorf("clickhouse connect: %w", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "clickhouse", Label: d.Label}

	var dbNames []string
	if d.DBName != "" && d.DBName != "default" {
		dbNames = []string{d.DBName}
	} else {
		rows, err := cli.queryRows(ctx, "SELECT name FROM system.databases WHERE name NOT IN ('system','information_schema','INFORMATION_SCHEMA') ORDER BY name")
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			dbNames = append(dbNames, r[0])
		}
	}

	for _, dbName := range dbNames {
		database, err := collectCHDB(ctx, cli, dbName)
		if err != nil {
			return nil, fmt.Errorf("ch db %s: %w", dbName, err)
		}
		inst.Databases = append(inst.Databases, database)
	}
	return inst, nil
}

func collectCHDB(ctx context.Context, cli *chHTTP, dbName string) (*schema.Database, error) {
	database := &schema.Database{Name: dbName}
	rows, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT name, engine, toUInt64(total_rows), toUInt64(total_bytes), comment
		FROM system.tables WHERE database='%s' AND engine NOT LIKE '%%View%%'
		ORDER BY name`, escCH(dbName)))
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		t := &schema.Table{Name: r[0], Engine: r[1], Comment: r[4]}
		fmt.Sscan(r[2], &t.RowCount)
		fmt.Sscan(r[3], &t.SizeBytes)
		fillCHTable(ctx, cli, dbName, t)
		database.Tables = append(database.Tables, t)
	}
	return database, nil
}

func fillCHTable(ctx context.Context, cli *chHTTP, dbName string, t *schema.Table) {
	rows, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT name, type, default_kind, default_expression, comment,
		       is_in_primary_key, is_in_sorting_key, is_in_partition_key
		FROM system.columns WHERE database='%s' AND table='%s'
		ORDER BY position`, escCH(dbName), escCH(t.Name)))
	if err != nil {
		return
	}
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
	}
	meta, err := cli.queryRows(ctx, fmt.Sprintf(`
		SELECT partition_key, sorting_key, primary_key
		FROM system.tables WHERE database='%s' AND name='%s'`, escCH(dbName), escCH(t.Name)))
	if err == nil && len(meta) > 0 {
		t.PartitionKey = meta[0][0]
		t.OrderByKey = meta[0][1]
	}
}

// ── HTTP helper ────────────────────────────────────────────────────────────

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
	if c.user != "" {
		params := url.Values{}
		params.Set("user", c.user)
		if c.pass != "" {
			params.Set("password", c.pass)
		}
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(sql))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("clickhouse HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func escCH(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}