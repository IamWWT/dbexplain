# L3: CLI 帮助与子命令验证

> 验证 `--version`、`-h`、`dbexplain all`、子命令手册等 CLI 功能。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain-linux-amd64"
```

## 9.1 版本号

```bash
$BIN --version
# 预期: dbexplain v0.0.9
```

## 9.2 简短帮助

```bash
$BIN -h 2>&1 | head -20
# 预期: 包含 version v0.0.9、Usage、Database types（含 csv/tsv/xlsx）
```

## 9.3 完整手册 (中文)

```bash
$BIN all 2>&1 | head -20
# 预期: 手册版本 v0.0.9
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
for db in mysql postgres gaussdb clickhouse sqlite redis mongodb elasticsearch qdrant csv tsv xlsx; do
  echo "=== $db ==="
  $BIN "$db" 2>&1 | grep -m1 "v0.0.9"
done
# 预期: 12/12 全部包含 v0.0.9
```

## 9.7 别名子命令

```bash
for alias in pg postgresql ch sqlite3 es; do
  echo "=== $alias ==="
  $BIN "$alias" 2>&1 | grep -m1 "v0.0.9"
done
# 预期: 5/5 全部包含 v0.0.9
```

## 9.8 execute -h

```bash
$BIN execute -h 2>&1 | head -15
```

## 9.9 list -h

```bash
$BIN list -h 2>&1 | head -10
```

## 9.10 encrypt -h

```bash
$BIN encrypt -h 2>&1 | head -10
```
