package connector

import (
	"context"
	"log"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/schema"
)

// MaxSQLLogLen controls the maximum length (in characters, NOT rows) of SQL text
// written to log files. SQL longer than this is truncated with "...(truncated)" suffix.
// Default is 5000 characters. Override via --sql-log-max-len flag.
var MaxSQLLogLen = 5000

// TruncateSQL truncates sql to MaxSQLLogLen characters for safe logging.
// Returns the original string if within limit, or truncated with "...(truncated)" suffix.
// This is about text length (characters), NOT row count — see query.QueryResult.Truncated for row truncation.
func TruncateSQL(sql string) string {
	if len(sql) > MaxSQLLogLen {
		return sql[:MaxSQLLogLen] + "...(truncated)"
	}
	return sql
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

// --- collect option context keys ---

type ctxKeyNoSample struct{}
type ctxKeySkipOpstats struct{}

// WithNoSample returns a context that tells collectors to skip sample row fetching.
func WithNoSample(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyNoSample{}, true)
}

// IsNoSample reports whether the context has the no-sample flag set.
func IsNoSample(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyNoSample{}).(bool)
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

type Connector interface {
	Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error)
	Capabilities() []capabilities.Capability
}

// Collect 主入口，解析 DSN、获取连接器并安全调用
func Collect(ctx context.Context, rawDSN string) (*schema.Instance, error) {
	d, err := dsn.ParseDSN(rawDSN)
	if err != nil {
		return nil, err
	}
	c, err := GetConnector(d.Kind)
	if err != nil {
		return nil, err
	}
	return CollectSafe(ctx, c, d)
}