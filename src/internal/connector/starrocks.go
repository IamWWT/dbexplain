//go:build mysql || full

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("starrocks", func() Connector { return starrocksConnector{} })
}

type starrocksConnector struct{}

func (starrocksConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{
		capabilities.CapSQL,
		capabilities.CapRowCount,
		capabilities.CapIndex,
		capabilities.CapPartition, // OLAP 分区
		capabilities.CapSampling,
	}
}

func (starrocksConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	// StarRocks FE MySQL 协议默认端口 9030（非 MySQL 的 3306）
	if d.Port == "" {
		d.Port = "9030"
	}
	db, err := openMySQL(d)
	if err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "open", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "ping", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "starrocks", Label: d.Label}

	var dbNames []string
	if d.DBName != "" {
		dbNames = []string{d.DBName}
	} else {
		Logf(ctx, "[starrocks] [collect] %s", "SHOW DATABASES")
		rows, err := db.QueryContext(ctx, "SHOW DATABASES")
		if err != nil {
			return nil, schema.NewDBError(d.Redacted(), "", "", "list databases", err)
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				if !isStarRocksSystemDB(n) {
					dbNames = append(dbNames, n)
				}
			}
		}
		if err := rows.Err(); err != nil {
			Logf(ctx, "[starrocks] rows iteration: %v", err)
		}
	}

	for _, dbName := range dbNames {
		Logf(ctx, "[starrocks] collecting database %s", dbName)
		database, err := collectMySQLDB(ctx, db, dbName, d.Redacted())
		if err != nil {
			Logf(ctx, "error in db %s: %v", dbName, err)
			continue
		}
		// StarRocks 表的 Engine 字段修正：StarRocks 表引擎为 OLAP/MySQL 等，
		// collectMySQLDB 已通过 information_schema.TABLES 采集了 ENGINE 列。
		// OLAP 元数据（分区键/分布键）暂通过 PartitionKey 字段存储。
		for _, t := range database.Tables {
			enrichStarRocksTable(ctx, db, dbName, t)
		}
		inst.Databases = append(inst.Databases, database)
	}

	return inst, nil
}

// enrichStarRocksTable 查询 StarRocks 特有的 OLAP 元数据（分区键、分布键）。
// 通过 SHOW CREATE TABLE 解析 PARTITION BY / DISTRIBUTED BY 子句。
func enrichStarRocksTable(ctx context.Context, db *sql.DB, dbName string, t *schema.Table) {
	q := fmt.Sprintf("SHOW CREATE TABLE %s.%s", quoteMySQL(dbName), quoteMySQL(t.Name))
	Logf(ctx, "[starrocks] [collect] %s", q)
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		Logf(ctx, "[starrocks] SHOW CREATE TABLE %s.%s failed: %v", dbName, t.Name, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			continue
		}
		// 解析 PARTITION BY (...) 子句
		if partKey := extractStarRocksPartitionKey(ddl); partKey != "" {
			if t.PartitionKey == "" {
				t.PartitionKey = partKey
			}
		}
		// 解析 DISTRIBUTED BY HASH(...) 子句 — 存入 OrderByKey 字段复用
		if distKey := extractStarRocksDistributedKey(ddl); distKey != "" {
			if t.OrderByKey == "" {
				t.OrderByKey = distKey
			}
		}
	}
	if err := rows.Err(); err != nil {
		Logf(ctx, "[starrocks] SHOW CREATE TABLE %s.%s rows iteration: %v", dbName, t.Name, err)
	}
}

// extractStarRocksPartitionKey 从 DDL 中提取 PARTITION BY 后的列表达式。
// 支持: PARTITION BY RANGE(`col`) / PARTITION BY date_trunc('day', `col`) /
// PARTITION BY `col` / PARTITION BY (col1, col2)
//
// 采用括号深度追踪：遇到顶层关键字或括号闭合到深度 0 即停止，
// 避免 RANGE(`col`)(PARTITION ...) 场景下误切到分区定义中的右括号。
func extractStarRocksPartitionKey(ddl string) string {
	upper := strings.ToUpper(ddl)
	idx := strings.Index(upper, "PARTITION BY")
	if idx < 0 {
		return ""
	}
	rest := ddl[idx+len("PARTITION BY"):]
	rest = strings.TrimLeft(rest, " \t\n\r")

	depth := 0
	end := 0
	for end < len(rest) {
		ch := rest[end]
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end++ // 包含闭合括号
				goto done
			}
		default:
			if depth == 0 {
				upperRest := strings.ToUpper(rest[end:])
				if strings.HasPrefix(upperRest, "DISTRIBUTED") ||
					strings.HasPrefix(upperRest, "PROPERTIES") ||
					strings.HasPrefix(upperRest, "ENGINE") ||
					strings.HasPrefix(upperRest, "ORDER BY") ||
					strings.HasPrefix(upperRest, "PRIMARY KEY") ||
					strings.HasPrefix(upperRest, "COMMENT") {
					goto done
				}
			}
		}
		end++
	}
done:
	expr := strings.TrimSpace(rest[:end])
	// PARTITION BY (col1, col2) 形式：剥掉外层括号
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") &&
		strings.Count(expr, "(") == 1 && strings.Count(expr, ")") == 1 {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	return expr
}

// extractStarRocksDistributedKey 从 DDL 中提取 DISTRIBUTED BY HASH(...) 后的列名。
func extractStarRocksDistributedKey(ddl string) string {
	upper := strings.ToUpper(ddl)
	idx := strings.Index(upper, "DISTRIBUTED BY HASH")
	if idx < 0 {
		// 也尝试 DISTRIBUTED BY RANDOM（无哈希列）
		if strings.Contains(upper, "DISTRIBUTED BY RANDOM") {
			return "RANDOM"
		}
		return ""
	}
	rest := ddl[idx+len("DISTRIBUTED BY HASH"):]
	rest = strings.TrimLeft(rest, " \t\n\r")
	if !strings.HasPrefix(rest, "(") {
		return ""
	}
	// 找到匹配的右括号
	depth := 0
	for i, ch := range rest {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return strings.TrimSpace(rest[1:i])
			}
		}
	}
	return ""
}

// isStarRocksSystemDB 过滤 StarRocks 系统库。
// StarRocks 系统库：information_schema, _statistics_, _sys, _independent_stats, starrocks, sys, mysql
func isStarRocksSystemDB(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "information_schema", "_statistics_", "_sys", "_independent_stats",
		"starrocks", "sys", "mysql", "performance_schema":
		return true
	}
	return false
}

// ExecQuery implements query.Queryable for StarRocks (MySQL protocol).
func (starrocksConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	if opts.DSN.Port == "" {
		opts.DSN.Port = "9030"
	}
	db, err := openMySQL(opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("starrocks open: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// StarRocks 可能不支持 max_execution_time（MySQL 系统变量），
	// 尝试设置但忽略错误（与 MySQL 连接器一致的做法）。
	if opts.Timeout > 0 {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("SET SESSION max_execution_time=%d", opts.Timeout*1000)); err != nil {
			Logf(ctx, "[starrocks] set max_execution_time failed: %v (query will still run without timeout guard)", err)
		}
	}

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	result, err := executeSQLQuery(runCtx, db, opts.SQL, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("starrocks query: %w", err)
	}
	return result, nil
}
