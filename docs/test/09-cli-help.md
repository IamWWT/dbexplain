# L3: CLI 帮助与子命令验证

> 验证 `--version`、`-h`、`dbexplain all`、子命令手册等 CLI 功能。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

## 9.1 版本号

```bash
$BIN --version
# 预期: dbexplain v0.1.7
```

## 9.2 简短帮助

```bash
$BIN -h 2>&1 | head -20
# 预期: 包含 version v0.1.7、Usage、Database types（含 csv/tsv/xlsx）
```

## 9.3 完整手册 (中文)

```bash
$BIN all 2>&1 | head -20
# 预期: 手册版本 v0.1.7
```

## 9.4 完整手册 (英文)

```bash
$BIN all --language en 2>&1 | head -20
```

## 9.5 手册关键字过滤

```bash
$BIN all --filter redis 2>&1 | head -10
# 预期: 仅显示 Redis 相关章节
```

## 9.6 全部数据库子命令

```bash
for db in mysql postgres gaussdb clickhouse sqlite redis mongodb elasticsearch qdrant csv tsv xlsx oracle hive; do
  echo "=== $db ==="
  $BIN "$db" 2>&1 | grep -m1 "v0.1.7"
done
# 预期: 14/14 全部包含 v0.1.7
```

## 9.7 别名子命令

```bash
for alias in pg postgresql ch sqlite3 es oracles hives; do
  echo "=== $alias ==="
  $BIN "$alias" 2>&1 | grep -m1 "v0.1.7"
done
# 预期: 7/7 全部包含 v0.1.7
```

## 9.8 execute -h

```bash
$BIN execute -h 2>&1 | head -15
```

## 9.9 list -h

```bash
$BIN list -h 2>&1 | head -10
```

## 9.10 encrypt 子命令

```bash
# encrypt -h 帮助
$BIN encrypt -h 2>&1 | head -10
# 预期: 包含 Usage of encrypt

# encrypt 加密文件
echo 'DBTEST=sqlite:///:memory:?label=test' > /tmp/test_encrypt.env
$BIN encrypt /tmp/test_encrypt.env 2>&1 | head -5
# 预期: 显示加密成功或密码提示
rm -f /tmp/test_encrypt.env
```

## 9.11 collect 子命令

```bash
# collect -h 帮助
$BIN collect -h 2>&1 | head -10
# 预期: 包含 Usage of collect 和 -dsn / -human 等标志

# collect 无参（无 DSN）
$BIN collect 2>&1 | head -3
# 预期: 提示无 DSN 的错误消息

# collect 带 DSN（文件数据源，无需外部数据库）
$BIN collect -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" --human 2>&1 | head -15
# 预期: 显示 Schema 采集结果，包含 csv-users 的数据库/表信息

# collect 向后兼容（无子命令 = collect）
$BIN -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" --human 2>&1 | head -5
# 预期: 与 collect --human 输出一致
```

## 9.12 repl 子命令

```bash
# repl -h 帮助
$BIN repl -h 2>&1 | head -10
# 预期: 包含 Usage of repl

# repl 无参（无 DSN）
$BIN repl 2>&1 | head -3
# 预期: No DSN entries 错误消息

# repl 直连 (Ctrl+D 退出)
echo "" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" 2>&1
# 预期: 显示 dbexplain REPL 提示符并正常退出

# .help 命令
echo -e ".help\n.exit" | $BIN repl 2>&1
# 预期: 显示命令列表（.conn / .dsn / .list / .databases / .help / .exit / Ctrl+D）

# .list 列出所有数据源
echo -e ".list\n.exit" | $BIN repl 2>&1
# 预期: 显示 Configured databases: 表头及所有 DSN 条目列表（含 # Label Kind Status 列）

# .databases 别名
echo -e ".databases\n.exit" | $BIN repl 2>&1
# 预期: 与 .list 输出一致

# .conn 切换数据源 + SQL 查询
echo -e ".conn aiops-clickhouse\nSELECT 1 AS test\n.exit" | $BIN repl 2>&1
# 预期: Switched to: aiops-clickhouse → 返回 test=1

# 写操作拒绝（DROP）
echo -e ".conn aiops-mysql\nDROP TABLE iplist\n.exit" | $BIN repl 2>&1
# 预期: READ_ONLY_VIOLATION: write operation "DROP"

# 写操作拒绝（INSERT）
echo -e "INSERT INTO iplist VALUES(1)\n.exit" | $BIN repl 2>&1
# 预期: READ_ONLY_VIOLATION: write operation "INSERT"

# DENY_TABLES
echo -e "SELECT * FROM information_schema.tables LIMIT 1\n.exit" | $BIN repl 2>&1
# 预期: ACCESS_DENIED: table "information_schema"

# 无效 label
echo -e ".conn nonexistent\n.exit" | $BIN repl 2>&1
# 预期: No DSN with label "nonexistent" found

# 未知命令
echo -e ".unknown\n.exit" | $BIN repl 2>&1
# 预期: Unknown command: .unknown (try .help)

# 空输入（应忽略不报错）
echo -e "\n.exit" | $BIN repl 2>&1
# 预期: 空行不产生错误，正常退出

# .dsn 别名
echo -e ".dsn openim-redis\nPING\n.exit" | $BIN repl 2>&1
# 预期: Switched to: openim-redis → PONG

# .exit 退出
echo -e ".exit" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" 2>&1
# 预期: Goodbye.

# .quit 退出
echo -e ".quit" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" 2>&1
# 预期: Goodbye.

# --limit 自定义行数
echo -e "SELECT 1\n.exit" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" --limit 500 2>&1
# 预期: REPL 正常启动，limit 生效

# --timeout 自定义超时
echo -e "SELECT 1\n.exit" | $BIN repl --dsn "sqlite:////tmp/test.db?label=test" --timeout 15 2>&1
# 预期: REPL 正常启动，timeout 生效

# ClickHouse 带分号（已修复，自动去尾部 ;）
echo -e ".conn aiops-clickhouse\nSHOW TABLES;\n.exit" | $BIN repl 2>&1
# 预期: 不再报多语句错误，正常返回

# Elasticsearch JSON 原生查询 → 友好提示
echo -e ".conn aiops-es\n{\"query\":{\"match_all\":{}}}\n.exit" | $BIN repl 2>&1
# 预期: ES JSON native queries not supported in REPL
```
