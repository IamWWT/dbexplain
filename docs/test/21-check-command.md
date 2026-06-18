# 配置检查 — `dbexplain check`

> 验证配置文件格式、DSN 语法正确性、数据库连通性。

> **v0.1.8 变更**: `-env` 参数已移除，始终自动加载配置文件。
> **v0.1.9 变更**: 并发检测 + 流式输出（结果按完成先后逐行打印）；默认超时 10s → 20s。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> 运行前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE`。
> 详见 [CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

## 21.0 帮助信息

```bash
$BIN -h
# 预期: 输出包含 "check" 子命令
```

## 21.1 无参数运行（自动发现 .env 文件）

```bash
$BIN check
# 预期: 自动发现 .env.dbexplain，列出所有 DSN 的语法和连通性检查结果
# 每行格式: No. EnvKey Label Kind Host:Port Syntax Connect
```

## 21.2 检查默认超时（20s）

```bash
$BIN check --timeout 20s
# 预期: 全部 DSN 显示 ✅ OK 或 ❌ FAIL，含连接延迟 (ms)
# 退出码: 0（全部成功）或 1（有失败）
```

## 21.3 检查单个 DSN

```bash
$BIN check --dsn 'sqlite:///tmp/test.db?label=my-sqlite'
# 预期: 语法 ✅ OK, 连接 ✅ OK (SQLite 自动创建文件)
```

## 21.4 无效 DSN 语法

```bash
$BIN check --dsn 'invalid-dsn'
# 预期: Syntax ❌ FAIL, Connect ⏭ skipped
# 错误信息: unsupported scheme ""
```

## 21.5 无法连接的 DSN

```bash
$BIN check --dsn 'redis://:pass@localhost:1/0?label=bad' --timeout 3s
# 预期: Syntax ✅ OK, Connect ❌ FAIL
# 错误详情: connect: dial tcp 127.0.0.1:1: connect: connection refused
```

## 21.6 JSON 配置文件

```bash
echo '["sqlite:///tmp/a.db","sqlite:///tmp/b.db"]' > /tmp/test-config.json
$BIN check --config /tmp/test-config.json
# 预期: 两个 DSN 均 ✅ OK
rm -f /tmp/test-config.json
```

## 21.7 --dsn 与自动 .env 同时加载

```bash
$BIN check --dsn 'sqlite:///tmp/extra.db?label=extra' --timeout 5s
# 预期: .env 中的 DSN + 额外 DSN 一起检查
```

## 21.8 超时机制

```bash
$BIN check --dsn 'mysql://u:p@192.0.2.1:3306/x?label=unreachable' --timeout 5s
# 预期: 5s 后显示 ⏱ timeout (5000ms) 或连接失败
```

## 21.9 凭证安全

```bash
$BIN check --dsn 'postgres://admin:secret123@localhost:5432/mydb?label=test-pg' --timeout 3s
# 预期: 错误信息中密码部分显示为 *** 或 {dbpassword}
# 绝不泄漏明文密码
```

## 21.10 混合结果

```bash
$BIN check --dsn 'redis://:pass@localhost:6379/0?label=good' \
           --dsn 'bad-dsn' \
           --dsn 'mysql://u:p@localhost:1/x?label=fail-mysql' \
           --timeout 3s
# 预期:
#   good     — Syntax ✅ OK,  Connect ✅ OK 或 ❌
#   bad-dsn  — Syntax ❌ FAIL, Connect ⏭ skipped
#   fail-mysql — Syntax ✅ OK, Connect ❌ FAIL
# 退出码: 1（有失败项）
```

## 21.11 自动加载配置文件（v0.1.8+）

验证 `dbexplain check` 自动加载 `.env.dbexplain`（无需传参）。

```bash
# 创建临时 .env 文件
echo 'DBTEST=sqlite:///:memory:?label=auto-env-test' > /tmp/.env-check-test

# 在临时目录运行 check（自动加载配置文件）
cd /tmp && ../src/dbexplain check 2>&1 | grep -c "auto-env-test"
# 预期: 1（DSN 列表中包含 auto-env-test）

rm -f /tmp/.env-check-test
```

## 21.12 并发流式输出（v0.1.9+）

验证结果按完成先后逐行打印，而非一次性输出。

```bash
$BIN check --dsn 'sqlite:///tmp/a.db?label=a' \
           --dsn 'sqlite:///tmp/b.db?label=b' \
           --dsn 'sqlite:///tmp/c.db?label=c'
# 预期: 每行输出带序号（No.），各结果在各自检测完成后立即打印，
#       而非等全部完成才统一输出。最终有 Summary 行。
```

---

## 检查清单

| # | 测试项 | 预期 | 状态 |
|---|--------|------|------|
| 21.0 | 帮助信息 | check 出现在子命令列表中 | |
| 21.1 | 自动 .env 发现 | 加载配置文件 | |
| 21.2 | 默认超时 20s | 每行显示语法/连接状态 | |
| 21.3 | 单个 DSN | SQLite 自动创建并连接 | |
| 21.4 | 无效语法 | Syntax ❌ FAIL + 错误原因 | |
| 21.5 | 无法连接 | Syntax ✅ OK + Connect ❌ FAIL + 错误详情 | |
| 21.6 | JSON 配置 | 文件 DSN 正常检查 | |
| 21.7 | 混合 DSN 源 | .env + --dsn 合并 | |
| 21.8 | 超时机制 | 超时后显示 ⏱ timeout | |
| 21.9 | 凭证安全 | 密码脱敏显示 | |
| 21.10 | 混合结果 | 退出码反映失败项 | |
| 21.11 | 自动加载配置文件 | 无参数自动加载 .env | |
| 21.12 | 并发流式输出 | 结果逐行打印，非一次性 | |
