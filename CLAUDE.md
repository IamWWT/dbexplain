# dbexplain 项目宪法

> 项目核心约定与累积经验。所有贡献者应遵守。

## 防御编码（Bug Bash v0.1.6 精华）

### 类型断言
```go
// ✅ 正确 — 必须 comma-ok
if v, ok := x.(T); ok { ... }

// ❌ 错误 — 裸断言会 panic
v := x.(T)
```

### map 查指针
```go
// ✅ map[string]*T 必须双重检查
if v, ok := m[k]; ok && v != nil { ... }
```

### rows.Next() 后处理
```go
for rows.Next() { ... }
// ✅ 必须检查 rows.Err()
if err := rows.Err(); err != nil { ... }
```

### 循环变量取地址
```go
// ✅ 用索引取真实元素地址
for i := range xs { do(&xs[i]) }

// ❌ &x 取的是复用变量地址，所有迭代指向同一内存
for _, x := range xs { do(&x) }
```

### goroutine 清理
```go
go func() {
    defer r.Close()          // ✅ panic 时也要关闭资源
    defer func() { ... }()   // ✅ recover
    // ... 业务逻辑
}()
```

### Lock 空指针
```go
// ✅ 先判空再使用
if opts.Lock != nil {
    opts.Lock.Lock()
    defer opts.Lock.Unlock()
}
```

### break 在 switch+for 嵌套中只跳出 switch

```go
// ❌ break 跳出的是 switch，不是外层 for 循环
for i := range items {
    switch x {
    case 1:
        if done { break } // ❌ 只跳出 switch，for 循环继续
    }
}

// ✅ 使用 labeled break
loop:
for i := range items {
    switch x {
    case 1:
        if done { break loop } // ✅ 跳出 for 循环
    }
}
```

> v0.1.7 血泪教训：`WITH x AS (...) INSERT ...` 的 CTE 写检测修复中，```break``` 未跳出外层 ```for``` 导致 ```lastParenEnd``` 被后续 ```VALUES(1)``` 的 ```)``` 覆盖，主查询体检测始终看到空字符串。

### 数据库超时：lib/pq 上下文取消不可靠 — select+channel 兜底

PostgreSQL/GaussDB 的 `lib/pq` 驱动在数据库服务器无响应时，`context.WithTimeout` 取消不可靠（CancelRequest 走独立 TCP 连接也可能卡住）。

```go
// ✅ 子 goroutine + select + channel 三层超时防护
type result struct {
    inst *schema.Instance
    err  error
}
ch := make(chan result, 1)
go func() {
    inst, err := someLongOp(ctx)
    ch <- result{inst, err}
}()
select {
case res := <-ch:
    return res.inst, res.err
case <-ctx.Done():
    return nil, fmt.Errorf("TIMEOUT: operation exceeded %v", timeout)
}

// ❌ 仅依赖 lib/pq 的 context 取消
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()
result, err := someLongOp(ctx)  // 可能永远卡住
```

> 子 goroutine 可能泄露（lib/pq 内部线程卡住），但不影响主流程。collect、check、execute 三处均已应用此模式。

## 安全红线

### DSN 密码脱敏
```go
// ✅ 所有 DSN 输出必须用 Redacted()
fmt.Println(d.Redacted())          // 脱敏
fmt.Println(d.String())            // ✅ String() 自动返回 Redacted()

// ❌ 绝不输出 raw DSN
fmt.Println(d.Raw)
fmt.Println(d.Password)

// ✅ 错误信息脱敏
config.SanitizeErr(err)
```

> **SanitizeErr 死循环陷阱 (ISSUE-095)**: `d.Redacted()` 占位符 `gaussdb://{dbuser}:{dbpassword}@host/db` 仍匹配 URL 模式 `://user:pass@host`。SanitizeErr 第一遍把 `{dbpassword}` 替换为 `***`，第二遍把 `***` 再替换为 `***`（无操作）→ 死循环。**修复**: `newMsg == msg → break`，检测无变化即退出。**教训**: 已脱敏字符串不要再经过 SanitizeErr。

### panic recover 用解析后的 DSN
```go
parsed, err := dsn.ParseDSN(rawDSN)
// 先 Parse 再 defer recover
defer func() {
    if r := recover(); r != nil {
        log.Printf("PANIC: %s: %v", parsed.Redacted(), r)  // ✅
    }
}()
```

## DSL 架构规则

### Prometheus 特殊映射
- `FROM @label.metric` — metric 名 = 表名，labels = 动态列
- `FROM @label.promql(任意PromQL)` — 原始 PromQL 透传，不编译
- promql() 不支持 WHERE/GROUP BY — 过滤在表达式内联
- ORDER BY/LIMIT/OFFSET 在 Go 层后处理，非 PromQL 原生

### 预处理两趟扫描
1. 第一趟: `promql(...)` 提取（递归括号匹配）
2. 第二趟: `@label.table` 正则匹配
3. `IsRawPromQL` 标记贯穿 SourceRef → QueryIR → compiler → executor

### 联邦查询
- 物化别名必须用实际表名（非 placeholder），否则 JOIN 解析失败
- FROM 表数据必须排首位（allData 重排）

## 构建与发布

### 选择性编译
- 每个 connector 有独立 build tag: `//go:build prometheus || full`
- DuckDB 不包含在 `full` 中：`//go:build duckdb`（需 CGO）
- 测试: `go build -tags full; go test -tags full ./...`

### 发布前检查
参见 `docs/security-policies/SECURITY_CHECKLIST.md` §6 — 25+ 项（构建/测试/版本一致性/CHANGELOG 完整性/二进制冒烟/文档陈旧引用/安全）

## 文档义务
- CODE_MAP.md 随文件增删改同步更新
- README 中英双语同步更新
- CHANGELOG 中英双语同步更新
- 功能变更须更新对应 `docs/databases/` 手册
- 测试用例记入 `docs/test/`，非 mock E2E

## 验证闭环
每次功能变更后必须闭环验证：
```bash
cd src
go build -tags full ./cmd/dbexplain/   # 编译
go vet ./...                            # 静态分析
go test -tags full ./... -count=1       # 单元测试
bash build.sh minimal <affected_tags>   # 选择性编译验证
```
