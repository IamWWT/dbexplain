# L5: 并发控制验证

> 验证同一 label 的并发查询互斥机制。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

## 8.1 并发互斥

```bash
# 在后台启动慢查询
$BIN execute --db 6 --timeout 60 "SELECT pg_sleep(10)" &
sleep 1

# 同时发起第二个查询
$BIN execute --db 6 "SELECT 1"
# 预期: CONCURRENT_LIMIT: a query is already running for label "video-pg"

# 等待后台完成
wait
```

## 8.2 不同 label 可并发

```bash
# 同时查询不同数据库
$BIN execute --db 1 --timeout 60 "SELECT 1" &
$BIN execute --db 6 --timeout 60 "SELECT 1" &
# 两者都应成功（不同 label 不互斥）
wait
```
