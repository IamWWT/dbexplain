# L6: 文件处理 (CSV/TSV/XLSX)

> 验证 CSV、TSV、XLSX 文件的 Schema 采集和查询执行。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain-linux-amd64"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [test/README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../docs/CONFIG_SEARCH.md)。

## 5.1 CSV — Schema 采集

```bash
$BIN -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" --human 2>/dev/null | head -30
```

## 5.2 CSV — 查询全部

```bash
$BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --human
```

## 5.3 CSV — 带 LIMIT/OFFSET

```bash
$BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT * LIMIT 1 OFFSET 1" --human
```

## 5.4 CSV — 环境变量 DSN

```bash
$BIN execute -env --db 13 "SELECT *" --human
```

## 5.5 CSV — 大文件采样

```bash
$BIN execute -env --db 14 "SELECT * LIMIT 5" --human
```

## 5.6 TSV — Schema 采集

```bash
$BIN -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" --human 2>/dev/null | head -20
```

## 5.7 TSV — 查询

```bash
$BIN execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT *" --human
```

## 5.8 TSV — 环境变量 DSN

```bash
$BIN execute -env --db 15 "SELECT *" --human
```

## 5.9 XLSX — Schema 采集

```bash
$BIN execute -env --label tsf-xlsx "SELECT * LIMIT 5" --human
```

## 5.10 XLSX — 多 Sheet 验证

```bash
$BIN -env --label tsf-xlsx --json 2>/dev/null | python3 -c "
import json,sys; d=json.load(sys.stdin)
tables = d.get('databases',[{}])[0].get('tables',[])
for t in tables:
    print(f'  sheet={t[\"name\"]} columns={len(t.get(\"columns\",[]))} rows={t.get(\"row_count\",0)}')
"
```

## 5.11 XLSX — 另一文件

```bash
$BIN execute -env --label tdmq-xlsx "SELECT * LIMIT 3" --human
```

## 5.12 文件 — 不支持的查询拒绝

```bash
$BIN execute -env --db 13 "SELECT id FROM users"
# 预期: QUERY_ERROR (仅支持 SELECT *)
```
