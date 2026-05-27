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
	Cluster  bool // ?cluster=true (Redis Cluster)
	TLS      bool // ?tls=true (ES/Redis HTTPS)
	SSLMode  string // ?sslmode=require|disable (PostgreSQL)
	TLSSkipVerify bool // ?tls-skip-verify=true (ES/Redis)
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
		if scheme == "rediss" {
			d.TLS = true
		}
		d.Kind = "redis"
	case "mongodb":
		d.Kind = "mongodb"
	case "qdrant":
		d.Kind = "qdrant"
	case "elasticsearch", "es", "elasticsearchs":
		if scheme == "elasticsearchs" {
			d.TLS = true
		}
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
	if v := u.Query().Get("cluster"); v == "true" || v == "1" {
		d.Cluster = true
	}
	if v := u.Query().Get("tls"); v == "true" || v == "1" {
		d.TLS = true
	}
	if v := u.Query().Get("tls-skip-verify"); v == "true" || v == "1" {
		d.TLSSkipVerify = true
	}
	if v := u.Query().Get("sslmode"); v != "" {
		d.SSLMode = v
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

	// Find the password in the raw DSN by locating the ":password@" pattern.
	// We search for both the decoded and raw forms to handle URL-encoded
	// characters (e.g. %23 vs #).
	//
	// Strategy: find the last '@' that separates userinfo from host,
	// then work backwards to find the ':' before the password.
	schemeEnd := strings.Index(d.Raw, "://")
	if schemeEnd < 0 {
		return d.Raw
	}
	afterScheme := d.Raw[schemeEnd+3:]

	// Find the last '@' (host separator)
	lastAt := strings.LastIndex(afterScheme, "@")
	if lastAt < 0 {
		return d.Raw
	}
	userinfo := afterScheme[:lastAt]

	// Find the last ':' in userinfo (password separator)
	colonPos := strings.LastIndex(userinfo, ":")
	if colonPos < 0 {
		return d.Raw // no password
	}

	// Redact credentials with descriptive placeholders
	redactedUserinfo := "{dbpassword}" // no username
	if colonPos > 0 {
		redactedUserinfo = "{dbuser}:{dbpassword}"
	}

	redacted := d.Raw[:schemeEnd+3] + redactedUserinfo + afterScheme[lastAt:]
	return redacted
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