package connector

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/schema"
)

// MaxSQLLogLen controls the maximum length (in characters, NOT rows) of SQL text
// written to log files. SQL longer than this is truncated with "...(truncated)" suffix.
// Default is 5000 characters. Override via --sql-log-max-len flag.
var MaxSQLLogLen = 5000

// Verbose controls whether [DEBUG] level logs are written to dbexplain.log.
// Default is false. Override via --verbose flag.
var Verbose bool

// TruncateSQL truncates sql to MaxSQLLogLen characters for safe logging.
// Returns the original string if within limit, or truncated with "...(truncated)" suffix.
// This is about text length (characters), NOT row count — see query.QueryResult.Truncated for row truncation.
func TruncateSQL(sql string) string {
	if len(sql) > MaxSQLLogLen {
		return sql[:MaxSQLLogLen] + "...(truncated)"
	}
	return sql
}

// isPermissionErr detects database permission-denied errors for fallback logic.
// Used by PG, MySQL, and other connectors for collection-level degradation.
func isPermissionErr(err error) bool {
	return strings.Contains(err.Error(), "permission denied")
}

type LoggerKey struct{}

func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey{}, logger)
}

func Logf(ctx context.Context, format string, args ...interface{}) {
	if logger, ok := ctx.Value(LoggerKey{}).(*log.Logger); ok {
		logger.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Debugf logs a [DEBUG] level message only when Verbose is true.
// Use this for detailed diagnostic info that is too noisy for normal operation.
func Debugf(ctx context.Context, format string, args ...interface{}) {
	if Verbose {
		Logf(ctx, format, args...)
	}
}

// --- collect option context keys ---

type ctxKeySample struct{}
type ctxKeySkipOpstats struct{}
type ctxKeyGaussDBCompat struct{}

// WithSample returns a context that tells collectors to fetch sample rows
// for comment inference. By default, sample rows are NOT fetched.
func WithSample(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeySample{}, true)
}

// IsSample reports whether the context has the sample flag set.
func IsSample(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeySample{}).(bool)
	return v
}

// WithSkipOpstats returns a context that tells MySQL collector to skip op-stats queries.
func WithSkipOpstats(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeySkipOpstats{}, true)
}

// IsSkipOpstats reports whether the context has the skip-opstats flag set.
func IsSkipOpstats(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeySkipOpstats{}).(bool)
	return v
}

// WithGaussDBCompat returns a context that tells the PG collector to use
// information_schema.COLUMNS instead of pg_catalog functions for GaussDB Oracle
// compatibility mode, which does not support pg_catalog.format_type(), pg_get_expr(),
// or col_description().
func WithGaussDBCompat(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyGaussDBCompat{}, true)
}

// IsGaussDBCompat reports whether the context indicates GaussDB Oracle compatibility mode.
func IsGaussDBCompat(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyGaussDBCompat{}).(bool)
	return v
}

type ctxKeyTableFilter struct{}

// WithTableFilter returns a context that tells collectors to only collect
// the specified table names. An empty or nil slice means no filtering.
func WithTableFilter(ctx context.Context, names []string) context.Context {
	return context.WithValue(ctx, ctxKeyTableFilter{}, names)
}

// GetTableFilter returns the table name filter from the context.
// Returns nil when no filter is set (collect all tables).
func GetTableFilter(ctx context.Context) []string {
	v, _ := ctx.Value(ctxKeyTableFilter{}).([]string)
	return v
}

type Connector interface {
	Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error)
	Capabilities() []capabilities.Capability
}

// Collect is the main entry point: parse DSN, get connector, call safely.
func Collect(ctx context.Context, rawDSN string) (*schema.Instance, error) {
	d, err := dsn.ParseDSN(rawDSN)
	if err != nil {
		return nil, err
	}
	c, err := GetConnector(d.Kind)
	if err != nil {
		return nil, err
	}
	Logf(ctx, "[connect] %s ...", d.Redacted())
	start := time.Now()
	inst, err := CollectSafe(ctx, c, d)
	elapsed := time.Since(start)
	if err != nil {
		Logf(ctx, "[connect fail] %s (%v): %v", d.Redacted(), elapsed, err)
	} else {
		Logf(ctx, "[connect ok] %s (%v)", d.Redacted(), elapsed)
	}
	return inst, err
}