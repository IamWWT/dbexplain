package dsn

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"runtime"
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
	AllowedPath string // ?allowed_path=/data/ (DuckDB file access)
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
	case "csv":
		d.Kind = "csv"
	case "tsv":
		d.Kind = "csv" // tsv reuses csv connector with ?delimiter=\t
	case "xlsx":
		d.Kind = "xlsx"
	case "duckdb":
		d.Kind = "duckdb"
	case "prometheus":
		d.Kind = "prometheus"
	case "oracle", "oracles":
		if scheme == "oracles" {
			d.TLS = true
		}
		d.Kind = "oracle"
	case "hive", "hives":
		if scheme == "hives" {
			d.TLS = true
		}
		d.Kind = "hive"
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
	if v := u.Query().Get("allowed_path"); v != "" {
		d.AllowedPath = v
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

	// Reconstruct DSN with redacted credentials by finding the scheme prefix
	// and replacing the userinfo portion between "://" and the last "@".
	// Uses LastIndex to avoid false matches when the password contains '@'.
	schemeEnd := strings.Index(d.Raw, "://")
	if schemeEnd < 0 {
		return d.Raw
	}
	afterScheme := d.Raw[schemeEnd+3:]

	// Find the last '@' which separates userinfo from host
	lastAt := strings.LastIndex(afterScheme, "@")
	if lastAt < 0 {
		return d.Raw
	}

	// Rebuild: "scheme://{dbuser}:{dbpassword}@host:port/db..."
	// Use parsed User/Password fields instead of raw string parsing to handle
	// special characters (e.g. @ in password).
	redactedUserinfo := "{dbpassword}"
	if d.User != "" {
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
	unescaped, unescapeErr := url.PathUnescape(after)
	if unescapeErr != nil {
		log.Printf("[dsn] SQLitePath unescape %q: %v (using original)", after, unescapeErr)
	} else {
		after = unescaped
	}
	// Windows: sqlite:///C:/path → /C:/path → C:/path
	if runtime.GOOS == "windows" && len(after) >= 3 &&
		after[0] == '/' && after[2] == ':' {
		after = after[1:]
	}
	return filepath.FromSlash(after)
}

// FilePath extracts the filesystem path for file-based connectors (csv, xlsx).
func (d *DSN) FilePath() string {
	// Extract path from the raw DSN using "://" as separator (not d.Kind,
	// because tsv → kind="csv" but raw starts with "tsv://").
	schemeEnd := strings.Index(d.Raw, "://")
	if schemeEnd < 0 {
		return d.Raw
	}
	after := d.Raw[schemeEnd+3:]
	if i := strings.Index(after, "?"); i >= 0 {
		after = after[:i]
	}
	unescaped, unescapeErr := url.PathUnescape(after)
	if unescapeErr != nil {
		log.Printf("[dsn] FilePath unescape %q: %v (using original)", after, unescapeErr)
	} else {
		after = unescaped
	}
	// Windows: csv:///C:/path → /C:/path → C:/path
	if runtime.GOOS == "windows" && len(after) >= 3 &&
		after[0] == '/' && after[2] == ':' {
		after = after[1:]
	}
	return filepath.Clean(filepath.FromSlash(after))
}

// DSNParam returns the value of a query parameter from the raw DSN string.
func (d *DSN) DSNParam(key string) string {
	after := d.Raw
	if i := strings.Index(after, "?"); i >= 0 {
		q := after[i+1:]
		for _, pair := range strings.Split(q, "&") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 && kv[0] == key {
				v, _ := url.QueryUnescape(kv[1])
				return v
			}
		}
	}
	return ""
}