# 配置文件搜索机制

> dbexplain 如何找到 `.env` 配置文件，以及为什么搜索路径与二进制所在位置无关。

---

## 搜索优先级

`-env` 模式下，`findConfigFile()` 按以下顺序搜索配置文件，返回**第一个存在**的文件：

| 优先级 | 路径 | 说明 |
|--------|------|------|
| 1 | `$DBPROBE_ENV_FILE` 环境变量指向的路径 | 显式覆盖，可用于 CI/CD |
| 2 | `CWD/.env.dbexplain` | 当前目录，明文 |
| 3 | `CWD/.env.dbexplain.enc` | 当前目录，加密（自动解密） |
| 4 | `~/.config/dbexplain/.env.dbexplain` | 用户配置目录，明文 |
| 5 | `~/.config/dbexplain/.env.dbexplain.enc` | 用户配置目录，加密 |
| 6 | `CWD/.env` | 当前目录，向下兼容旧版 |
| 7 | `CWD/.env.enc` | 当前目录，加密旧版 |

> CWD = Current Working Directory，即你执行命令时所在的目录。

## 关键原则：搜索与二进制路径无关

`findConfigFile()` 的搜索逻辑**编译在二进制内**，运行时只依赖两个信息。**该规则适用于所有平台（Linux / macOS / Windows）**——`findConfigFile()` 本身没有平台分支，使用跨平台的 `os.Stat()`、`os.Getenv()`、`os.UserHomeDir()`，行为完全一致。

### 1. 当前工作目录（CWD）

```go
// 相对路径检查 — 依赖 CWD
os.Stat(".env.dbexplain")   // CWD/.env.dbexplain
os.Stat(".env")             // CWD/.env
```

CWD 是你**敲命令时所在的目录**，与二进制放在哪里无关：

```bash
# 全局安装，从项目目录运行
cd ~/projects/myapp
dbexplain -env              # 搜索 ~/projects/myapp/.env

# 相对路径二进制，从项目目录运行
cd ~/projects/myapp
./release/dbexplain-linux-amd64 -env   # 同样搜索 ~/projects/myapp/.env

# 绝对路径二进制，从项目目录运行
cd ~/projects/myapp
/usr/local/bin/dbexplain -env          # 同样搜索 ~/projects/myapp/.env
```

**三个命令结果完全一致**，因为 CWD 相同。

### 2. 用户家目录

```go
// 绝对路径检查 — 依赖 os.UserHomeDir()
homeDir, _ := os.UserHomeDir()
filepath.Join(homeDir, ".config", "dbexplain", ".env.dbexplain")
```

家目录由操作系统决定（`/etc/passwd` 或环境变量 `$HOME`），与二进制路径同样无关。

### 3. 环境变量（$DBPROBE_ENV_FILE）

环境变量是进程级别的属性，不因二进制位置不同而改变。

```bash
export DBPROBE_ENV_FILE=/etc/dbexplain/config.env
dbexplain -env              # 读取 /etc/dbexplain/config.env
../release/dbexplain -env   # 同样读取 /etc/dbexplain/config.env
```

---

## 源码位置

- `findConfigFile()` — `src/main.go`
- `loadEnvFile()` — `src/main.go`

## 常见问题

### Q: 我把二进制放在 `/usr/local/bin/`，它会去那里找 `.env` 吗？

**不会**。二进制搜索的是你**运行命令时的目录（CWD）**，而不是二进制所在的目录。

```bash
cd /home/user/data
dbexplain -env    # 搜索 /home/user/data/.env，而非 /usr/local/bin/.env
```

### Q: `schema` 采集模式（`dbexplain -env`）和 `execute` 模式（`dbexplain execute -env`）的搜索路径一样吗？

**完全一样**。两种模式调用同一个 `findConfigFile()` 函数，走同一套搜索逻辑。

### Q: 如何在非当前目录的路径运行？

两种方式：
1. 先 `cd` 到目标目录再运行
2. 使用 `DBPROBE_ENV_FILE` 环境变量指定配置文件绝对路径
