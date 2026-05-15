package dsn

import (
	"fmt"
	"net/url"
	"strings"
)

type DSN struct {
	Raw      string
	Kind     string // mysql, postgres, sqlite, clickhouse, redis
	User     string
	Password string
	Host     string
	Port     string
	DBName   string
	Label    string
}

// ParseDSN accepts: scheme://[user[:pass]@]host[:port][/dbname][?label=alias]
func ParseDSN(raw string) (*DSN, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %w", err)
	}
	d := &DSN{Raw: raw}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "mysql", "mariadb":
		d.Kind = "mysql"
	case "postgres", "postgresql", "pg":
		d.Kind = "postgres"
	case "gaussdb", "opengauss":
		d.Kind = "gaussdb"
	case "sqlite", "sqlite3":
		d.Kind = "sqlite"
	case "clickhouse", "ch":
		d.Kind = "clickhouse"
	case "redis", "rediss":
		d.Kind = "redis"
	case "mongodb":
		d.Kind = "mongodb"
	case "qdrant":
		d.Kind = "qdrant"
	case "elasticsearch", "es":
		d.Kind = "elasticsearch"
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}

	if u.User != nil {
		d.User = u.User.Username()
		d.Password, _ = u.User.Password()
	}
	d.Host = u.Hostname()
	d.Port = u.Port()
	d.DBName = strings.TrimPrefix(u.Path, "/")

	if v := u.Query().Get("label"); v != "" {
		d.Label = v
	} else {
		d.Label = labelFrom(d)
	}
	return d, nil
}

func labelFrom(d *DSN) string {
	host := d.Host
	if d.Port != "" {
		host += ":" + d.Port
	}
	if d.DBName != "" {
		return host + "/" + d.DBName
	}
	return host
}

func (d *DSN) Redacted() string {
	if d.Password == "" {
		return d.Raw
	}
	return strings.Replace(d.Raw, ":"+d.Password+"@", ":***@", 1)
}

// SQLitePath extracts file path for sqlite.
func (d *DSN) SQLitePath() string {
	after := strings.TrimPrefix(d.Raw, d.Kind+"://")
	after = strings.TrimPrefix(after, d.Kind+"3://")
	if i := strings.Index(after, "?"); i >= 0 {
		after = after[:i]
	}
	return after
}