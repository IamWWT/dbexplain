# ISSUE-062: 策略引擎全字段查询绕过修复

> 修复 `DENY_TABLES`/`DENY_COLUMNS` 在 `SELECT *` 和原生全字段查询时的绕过问题。

---

## 背景

v0.0.8 新增的安全策略引擎有三层检查：语句级、表级、列级。但在实际测试中发现，某些查询模式能绕过列级和表级禁止。

## 问题 1: `DENY_TABLES=information_schema` 不生效

### 根因
`extractTableNames()` 的正则 `(?:\w+\.)?(\w+)` 只捕获了表名部分（`TABLES`），丢弃了 schema 前缀（`information_schema.`）。

```go
// 修复前 regex — (\w+) 只捕获表名
re := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|UPDATE|INTO|TABLE)\s+(?:\w+\.)?(\w+)`)
// SELECT * FROM information_schema.TABLES → 提取到 ["TABLES"]
// DENY_TABLES=information_schema 匹配不上
```

### 修复
捕获全限定名并拆分出 schema、table 两部分：

```go
// 修复后 regex — (\w+(?:\.\w+)?) 捕获 schema.table
re := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|UPDATE|INTO|TABLE)\s+(\w+(?:\.\w+)?)`)
// SELECT * FROM information_schema.TABLES → 提取到 ["information_schema.TABLES", "TABLES", "information_schema"]
// DENY_TABLES=information_schema 匹配 information_schema ✓
```

### 影响
所有 SQL 数据库（MySQL、PostgreSQL、ClickHouse、SQLite、Elasticsearch）。

---

## 问题 2: `DENY_COLUMNS=iplist.owner` 不拦截 `SELECT * FROM iplist`

### 根因
列级检查通过 `extractColumnRefs()` 找 SQL 中的 `table.column` 模式。`SELECT *` 没有显式列引用，`extractColumnRefs` 提取不到任何匹配。

### 修复
在 `CheckSQL()` 列级检查末尾新增 `matchStarSelect()` 逻辑：

```go
normalized := normalizeWhitespace(sql)
if matchStarSelect(normalized) {
    tables := extractTableNames(sql)
    for _, denied := range c.DenyColumns {
        if deniedTable, _, ok := strings.Cut(denied, "."); ok {
            for _, t := range tables {
                if strings.EqualFold(t, deniedTable) {
                    return &ErrDenied{Level: "column", ...}
                }
            }
        }
    }
}
```

### 验证

| 查询 | 结果 |
|------|------|
| `SELECT * FROM iplist` | `ACCESS_DENIED: column "iplist.owner"` ✓ |
| `SELECT ID, hostip FROM iplist` | 正常返回 ✓ |
| `SELECT ID, iplist.owner FROM iplist` | `ACCESS_DENIED` ✓（显式引用） |

### 影响
所有 SQL 数据库。

---

## 问题 3: MongoDB/Qdrant 原生全字段查询绕过

### 根因
`CheckNative()` 注释写明 "Column-level is skipped for native queries"。MongoDB 的 `{"find":"credential"}` 默认返回所有字段，类似 SQL 的 `SELECT *`。

### 修复
在 `CheckNative()` 中新增列级检查：

```go
// 对于 MongoDB/Qdrant: 检查 DENY_COLUMNS=collection.field
for _, col := range collections {
    for _, denied := range c.DenyColumns {
        deniedCol, field, hasField := strings.Cut(denied, ".")
        if !hasField { continue }
        if !strings.EqualFold(col, deniedCol) { continue }
        // 投影明确排除了该字段 → 放行
        if hasProjection(query) && fieldIsProjectedOut(query, field) {
            continue
        }
        return &ErrDenied{...}
    }
}
```

### 验证

| 查询 | 结果 |
|------|------|
| `{"find":"credential"}` + `DENY_COLUMNS=credential.account` | `ACCESS_DENIED` ✓ |
| `{"find":"credential","projection":{"account":0}}` | 放行 ✓（明确排除） |
| `{"scroll":"runbooks"}` + `DENY_COLUMNS=runbooks.title` | `ACCESS_DENIED` ✓ |

### 影响
MongoDB、Qdrant。

---

## 问题 4: `--human` 放查询语句后不生效

### 根因
Go `flag.FlagSet` 遇到第一个非 flag 参数（SQL 字符串）即停止解析。`execute "SELECT 1" --human` 中 `--human` 被忽略。

### 修复
`fs.Parse()` 后扫描 `fs.Args()` 查找 `--human`：

```go
for _, a := range fs.Args() {
    if a == "--human" {
        *human = true
    }
}
```

### 影响
`execute` 子命令用户。

---

## 修复文件

| 文件 | 修改 |
|------|------|
| `src/policy/policy.go` | `extractTableNames()` regex 修复 + `matchStarSelect()` 新增 + `CheckNative()` 列级检查 + `hasProjection()`/`fieldIsProjectedOut()` 辅助函数 |
| `src/execute.go` | `--human` 旗标后置支持 |
| `src/main.go` | `--label` 全局标志 + 日志路由 + `log.SetOutput()` 重定向 |

## 跟踪

- 报告人: 用户实测
- 版本: v0.0.9
- 状态: 已修复
- 关联: ISSUE-061（v0.0.8 策略引擎原始实现）
