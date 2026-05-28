# L8: 并发限制测试

> 验证 `--conn` 参数的并发控制功能。

## 8.1 串行采集

```bash
cd src
dbexplain -env -timeout 60s --conn 1 --json
# 预期: 所有数据源依次采集，无死锁，全部成功（15 个 DSN 逐一完成）
```

## 8.2 默认并发

```bash
dbexplain -env -timeout 30s --json 2>&1 | head -5
# 预期: 默认并发 10，多个数据源并行采集
```

## 8.3 高并发

```bash
dbexplain -env -timeout 30s --conn 20 --json 2>&1 | head -5
# 预期: 更高并发（20），无资源竞争
```

## 8.4 Execute 并发锁

```bash
cd src
# 同时发起两个查询到同一 DSN
dbexplain execute -env --db 1 "SELECT SLEEP(5)" &
PID1=$!
sleep 1
dbexplain execute -env --db 1 "SELECT 1" &
PID2=$!
wait $PID1 $PID2 2>/dev/null
# 第二个查询应提示 CONCURRENT_LIMIT 或等第一个完成后执行
```

## 8.5 跨标签并发

```bash
# 不同 DSN 可同时查询
dbexplain execute -env --db 1 "SELECT SLEEP(3)" &
PID1=$!
dbexplain execute -env --db 2 "SELECT 1" --human &
PID2=$!
wait $PID1 $PID2 2>/dev/null
# 两个查询应同时完成，ClickHouse 查询不等 MySQL
```
