package connector

import (
	"context"
	"fmt"
	"runtime/debug"

	"dbexplain/dsn"
	"dbexplain/schema"
)

// CollectSafe 包装采集调用，捕获 panic 并转换为错误
func CollectSafe(ctx context.Context, c Connector, d *dsn.DSN) (inst *schema.Instance, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in connector %s: %v\n%s", d.Kind, r, debug.Stack())
			// 记录 panic 日志
			logf(ctx, "CRITICAL PANIC: %v", err)
		}
	}()
	return c.Collect(ctx, d)
}