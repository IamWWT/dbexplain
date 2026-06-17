# L2: 配置文件搜索优先级

> 验证 `FindConfigFile()` 的 7 级搜索优先级。从 v0.1.8 起 `-env` 参数已移除，始终自动加载。

## 前置条件

```bash
cd src
BIN=./dbexplain
```

## 24.1 搜索优先级定义

代码逻辑 (`src/internal/config/config.go:FindConfigFile()`):

| 优先级 | 位置 | 说明 |
|--------|------|------|
| L1 | `DBPROBE_ENV_FILE` 环境变量 | 显式覆盖 |
| L2 | `./.env.dbexplain` | 当前目录 |
| L3 | `./.env.dbexplain.enc` | 当前目录（加密） |
| L4 | `~/.config/dbexplain/.env.dbexplain` | 用户配置目录 |
| L5 | `~/.config/dbexplain/.env.dbexplain.enc` | 用户配置目录（加密） |
| L6 | `./.env` | 当前目录（旧版兼容） |
| L7 | `./.env.enc` | 当前目录（旧版加密兼容） |
| — | 无配置文件 | 返回空，打印 `no config file found` 提示 |

## 24.2 逐级独立存在测试

每个级别单独存在时工具应能正确发现和加载。

### 24.2.1 L1: DBPROBE_ENV_FILE

```bash
TMPENV=$(mktemp)
echo 'DB1=sqlite:///:memory:?label=L1-envvar' > "$TMPENV"
DBPROBE_ENV_FILE="$TMPENV" $BIN list 2>&1 | grep -E "Config source|L1-envvar"
rm -f "$TMPENV"
# 预期: 显示 Config source: <tmpfile> 和 L1-envvar 标签
```

### 24.2.2 L2: .env.dbexplain (CWD)

```bash
mkdir -p /tmp/test-l2 && cd /tmp/test-l2
echo 'DB1=sqlite:///:memory:?label=L2-cwd-dotenv' > .env.dbexplain
$BIN list 2>&1 | grep -E "Config source|L2-cwd-dotenv"
rm -f .env.dbexplain
# 预期: 显示 Config source: /tmp/test-l2/.env.dbexplain 和 L2-cwd-dotenv
```

### 24.2.3 L4: ~/.config/dbexplain/.env.dbexplain

```bash
mkdir -p "$HOME/.config/dbexplain"
echo 'DB1=sqlite:///:memory:?label=L4-userconfig' > "$HOME/.config/dbexplain/.env.dbexplain"
$BIN list 2>&1 | grep -E "Config source|L4-userconfig"
rm -f "$HOME/.config/dbexplain/.env.dbexplain"
# 预期: 显示 Config source: ~/.config/dbexplain/.env.dbexplain 和 L4-userconfig
```

### 24.2.4 L6: .env (CWD, legacy)

```bash
mkdir -p /tmp/test-l6 && cd /tmp/test-l6
echo 'DB1=sqlite:///:memory:?label=L6-legacy' > .env
$BIN list 2>&1 | grep -E "Config source|L6-legacy"
rm -f .env
# 预期: 显示 /tmp/test-l6/.env 和 L6-legacy
```

## 24.3 优先级覆盖测试

多级同时存在时，高优先级应胜出。

### 24.3.1 L2 > L4

```bash
mkdir -p /tmp/test-pri && cd /tmp/test-pri
echo 'DB1=sqlite:///:memory:?label=L2-WINS' > .env.dbexplain
mkdir -p "$HOME/.config/dbexplain"
echo 'DB1=sqlite:///:memory:?label=L4-should-not-appear' > "$HOME/.config/dbexplain/.env.dbexplain"
$BIN list 2>&1 | grep "L2-WINS\|L4-should-not-appear"
# 预期: L2-WINS (L2 优先于 L4)
rm -f .env.dbexplain "$HOME/.config/dbexplain/.env.dbexplain"
```

### 24.3.2 L1 > L2

```bash
cd /tmp/test-pri
echo 'DB1=sqlite:///:memory:?label=L2-should-not-appear' > .env.dbexplain
TMPENV=$(mktemp)
echo 'DB1=sqlite:///:memory:?label=L1-WINS' > "$TMPENV"
DBPROBE_ENV_FILE="$TMPENV" $BIN list 2>&1 | grep "L1-WINS\|L2-should-not-appear"
# 预期: L1-WINS (环境变量优先于 CWD)
rm -f .env.dbexplain "$TMPENV"
```

### 24.3.3 L4 > L5 (plaintext > encrypted in same dir)

```bash
echo 'DB1=sqlite:///:memory:?label=L4-plain-WINS' > "$HOME/.config/dbexplain/.env.dbexplain"
touch "$HOME/.config/dbexplain/.env.dbexplain.enc"
$BIN list 2>&1 | grep "L4-plain-WINS"
# 预期: L4-plain-WINS (plaintext 优先于 encrypted)
rm -f "$HOME/.config/dbexplain/.env.dbexplain" "$HOME/.config/dbexplain/.env.dbexplain.enc"
```

## 24.4 无配置兜底

没有任何配置文件时，输出友好提示（不 crash）：

```bash
cd /tmp && rm -f .env.dbexplain .env 2>/dev/null
$BIN list 2>&1 | head -3
# 预期: "no config file found. Create .env.dbexplain ..."
```

## 24.5 验证总结

| 用例 | 预期 | 实测 |
|------|------|------|
| 24.2.1 L1 DBPROBE_ENV_FILE | 加载该文件 | PASS |
| 24.2.2 L2 .env.dbexplain | 加载该文件 | PASS |
| 24.2.3 L4 ~/.config | 加载该文件 | PASS |
| 24.2.4 L6 .env legacy | 加载该文件 | PASS |
| 24.3.1 L2 > L4 | L2 胜出 | PASS |
| 24.3.2 L1 > L2 | L1 胜出 | PASS |
| 24.3.3 L4 > L5 | L4 胜出 | PASS |
| 24.4 无配置 | 友好提示 | PASS |
