package connector

import (
    "context"
    "fmt"
    "log"
    "dbexplain/dsn"
    "dbexplain/schema"
)


// LoggerKey 是 context 中存储 logger 的键
type loggerKey struct{}

// WithLogger 将 logger 注入 context
func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
    return context.WithValue(ctx, loggerKey{}, logger)
}

// logf 从 context 获取 logger 并输出，若不存在则使用标准 log
func logf(ctx context.Context, format string, args ...interface{}) {
    if logger, ok := ctx.Value(loggerKey{}).(*log.Logger); ok {
        logger.Printf(format, args...)
    } else {
        log.Printf(format, args...)
    }
}

type Connector interface {
    Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error)
}

func Collect(ctx context.Context, rawDSN string) (*schema.Instance, error) {
    d, err := dsn.ParseDSN(rawDSN)
    if err != nil {
        return nil, err
    }
    var c Connector
    switch d.Kind {
    case "mysql":
        c = mysqlConnector{}
    case "postgres", "gaussdb":
        c = postgresConnector{}
    case "sqlite":
        c = sqliteConnector{}
    case "clickhouse":
        c = clickhouseConnector{}
    case "redis":
        c = redisConnector{}
    case "mongodb":
        c = mongoConnector{}
    case "qdrant":
        c = qdrantConnector{}
    case "elasticsearch", "es":
        c = esConnector{}
    default:
        return nil, fmt.Errorf("no connector for %q", d.Kind)
    }
    return c.Collect(ctx, d)   // 传入 ctx
}