# L9: CLI 帮助测试

> 验证所有 CLI 子命令、帮助手册和版本输出。

## 9.1 版本输出

```bash
cd src
go run . --version
```

实际输出:
```
dbexplain v0.0.9
```

## 9.2 简洁帮助

```bash
go run . -h
# 预期: 显示使用方式、全局标志、数据库类型、示例
```

## 9.3 完整手册

```bash
go run . all
# 预期: 中英文双语完整手册，包含所有子命令、DSN 格式、输出格式
```

```bash
go run . all --language en
# 预期: 英文版完整手册
```

## 9.4 数据库专项手册

```bash
for db in mysql postgres gaussdb clickhouse sqlite redis mongodb elasticsearch qdrant csv xlsx; do
  echo "=== $db ==="
  go run . $db 2>&1 | head -3
done
# 预期: 每个数据库/文件类型都输出对应的专项手册摘要
```

实际输出 (csv 示例):
```
=== csv ===
CSV/TSV Manual
==============
# CSV/TSV File Processing
...
```

## 9.5 数据库手册别名

```bash
go run . pg
# 预期: 同 postgres 手册

go run . ch
# 预期: 同 clickhouse 手册

go run . es
# 预期: 同 elasticsearch 手册

go run . sqlite3
# 预期: 同 sqlite 手册
```

## 9.6 列表子命令

```bash
go run . list -env
# 预期: 显示 15 个 DSN 的 INDEX/LABEL/KIND/HOST:PORT/DATABASE 映射表
```

## 9.7 Execute 子命令帮助

```bash
go run . execute -h
# 预期: 显示 execute 子命令的参数说明（--env、--label、--db、--dsn、--limit、--timeout、--explain、--human）
```

## 9.8 Encrypt 子命令帮助

```bash
go run . encrypt -h
# 预期: 显示加密子命令的参数说明
```

## 9.9 --filter 手册过滤

```bash
go run . all --filter redis
# 预期: 仅显示包含 "redis" 的手册内容

go run . all --filter "SSL"
# 预期: 仅显示包含 "SSL" 的手册内容（大小写不敏感）
```

## 9.10 --human 输出

```bash
go run . -env --label aiops-mysql --human
# 预期: 带上下文标记的人类友好输出
```

## 9.11 JSON 输出

```bash
go run . -env --label aiops-mysql --json 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print(f'kind: {d.get(\"kind\",\"?\")}')"
# 实际: kind: mysql
```

## 9.12 错误提示

```bash
# 无效子命令
go run . invalid_subcommand 2>&1
# 预期: 错误提示或帮助信息

# 无效 DSN
go run . -dsn "invalid://uri" 2>&1
# 预期: 错误提示
```
