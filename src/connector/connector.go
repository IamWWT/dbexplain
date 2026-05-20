package connector

import (
	"context"
	"log"

	"dbexplain/capabilities"
	"dbexplain/dsn"
	"dbexplain/schema"
)

type LoggerKey struct{}

func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey{}, logger)
}

func logf(ctx context.Context, format string, args ...interface{}) {
	if logger, ok := ctx.Value(LoggerKey{}).(*log.Logger); ok {
		logger.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
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