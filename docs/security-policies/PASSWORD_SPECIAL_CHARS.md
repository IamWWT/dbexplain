# DSN 密码特殊字符兼容性指南

> 哪些特殊字符可以直接用在 DSN 密码中，哪些需要百分号编码（URL encoding）。

---

## 1. 一句话结论

| 密码中的字符 | 需要转义？ | 说明 |
|-------------|-----------|------|
| `! @ # $ & ' ( ) * + , - . : ; = _ ~` | **不**需要 | 共 18 个字符可直接写在 DSN URL 中 |
| `" % / ? [ \ ] ^ \` { \| } 空格` | **必须**转义 | 共 13 个 ASCII 字符会被 Go 的 URL 解析器拒绝 |
| `～ ； ， 。 、`（全角/中文标点） | **必须**转义 | 共 5 个 Unicode 字符不被 URL userinfo 接受 |

## 2. 完整转义映射表

密码中有特殊字符时，在 DSN 中用百分号编码（`%XX`）替代原文。

| 原始字符 | 编码后 | 说明 |
|---------|--------|------|
| ` ` (空格) | `%20` | URL 不安全字符 |
| `"` | `%22` | URL 不安全字符 |
| `%` | `%25` | **特别注意** — `%` 后跟非十六进制字符会解析失败 |
| `/` | `%2F` | 路径分隔符 |
| `?` | `%3F` | Query 起始符 |
| `[` | `%5B` | RFC 3986 userinfo 禁止 |
| `\` | `%5C` | URL 不安全字符 |
| `]` | `%5D` | RFC 3986 userinfo 禁止 |
| `^` | `%5E` | URL 不安全字符 |
| `` ` `` | `%60` | URL 不安全字符 |
| `{` | `%7B` | URL 不安全字符 |
| `\|` | `%7C` | URL 不安全字符 |
| `}` | `%7D` | URL 不安全字符 |
| `<` | `%3C` | URL 不安全字符 |
| `>` | `%3E` | URL 不安全字符 |
| `～` | `%EF%BD%9E` | 全角波浪号，UTF-8 编码 |
| `；` | `%EF%BC%9B` | 全角分号 |
| `，` | `%EF%BC%8C` | 全角逗号 |
| `。` | `%E3%80%82` | 全角句号 |
| `、` | `%E3%80%81` | 全角顿号 |

### 举例

```
# 密码含 / → 写 %2F
mysql://user:pass%2Fword@host:3306/db
# 解析后 Password = "pass/word"

# 密码含空格
postgres://user:pass%20word@host:5432/db
# 解析后 Password = "pass word"

# 密码含 % → 必须写 %25
redis://:pass%25word@host:6379/0
# 解析后 Password = "pass%word"
```

## 3. 自动百分号编码支持

dbexplain 在 DSN 解析时**自动编码**不安全字符：

| 功能 | 说明 |
|------|------|
| `#`（井号） | 自动替换为 `%23` |
| 空格、`/`、`?`、`[`、`]`、`<`、`>`、`{`、`}`、`\|`、`\`、`^`、`` ` ``、`"`、中文标点等 | 自动百分号编码 |
| `%` 后接非十六进制字符 | 自动百分号编码为 `%25` |

所有不安全字符在 userinfo 部分都会被自动编码，**你不需要手动编码**：

```
# 直接写不安全字符，内部自动编码
mysql://user:pass#word@host:3306/db
# 内部: escapeUserinfo() 自动将 # 替换为 %23

mysql://user:pass word@host:3306/db
# 内部: escapeUserinfo() 自动将空格替换为 %20

postgres://user:pass[word@host:5432/db
# 内部: escapeUserinfo() 自动将 [ 替换为 %5B
```

> **注意**：如果用户自己在 DSN 中已做了百分号编码（如 `%23` 替代 `#`），结果是一样的：
> `mysql://user:pass%23word@host:3306/db` → Password = `pass#word`

### 会不会重复转义？

**不会。** 如果用户已经写了 `%23`，`escapeUserinfo()` 检测到 `%23` 是合法百分号编码序列，会原样保留，不会变成 `%2523`。`url.Parse` 将 `%23` 解码为 `#`，最终密码正确。

```
# 用户预编码: `%23` 是合法百分号序列 → escapeUserinfo 原样保留 → url.Parse 解码为 #
mysql://user:pass%23word@host:3306/db  →  Password = "pass#word"
#                                                         ↑ 完全相同
mysql://user:pass#word@host:3306/db    →  Password = "pass#word"
```

**预编码保护适用于所有不安全字符**，不仅限于 `#`：

```
# `%40` 是合法百分号序列 → escapeUserinfo 原样保留 → url.Parse 解码为 @
oracle://user%40domain:p%40ss@host:1521/XE  →  Password = "p@ss", User = "user@domain"
```

### 唯一需要避免的情况：双重编码

如果刻意把 `#` 写成 `%2523`（即在 `%23` 的基础上又编码了 `%`），结果会不正确：

```
mysql://user:pass%2523word@host:3306/db  →  Password = "pass%23word"  ❌
# 期望 pass#word，实际得到 pass%23word
```

**规则**：始终使用**单层百分号编码**。不要对已经编码的内容再编码。

## 4. `@` 在密码中

`@` 在密码中是安全的，不需要转义。Go 的 `url.Parse` 把 `@` 视为 userinfo 的合法字符，我们用 `LastIndex("@")` 找到最后一个 `@` 作为 userinfo 和 host 的分界：

```
# @ 在密码中 → 正常工作
mysql://user:pass@word@host:3306/db  →  Password = "pass@word"
```

如果手动将 `@` 编码为 `%40`，同样能正确解码：

```
mysql://user:pass%40word@host:3306/db  →  Password = "pass@word"
```

## 5. `%` 在密码中

`%` 后接两个十六进制字符（如 `%23`、`%40`）会被 Go 的 URL 解析器当作百分号编码解码。因此**密码中的字面 `%` 必须写为 `%25`**：

```
# ❌ 错误 — %wo 不是合法百分号编码
mysql://user:pass%word@host:3306/db

# ✅ 正确 — %25 解码为 %
mysql://user:pass%25word@host:3306/db  →  Password = "pass%word"
```

## 6. 各数据源密码风险等级

| 数据源 | 风险 | 原因 |
|--------|------|------|
| **MySQL** | 高 | `buildMySQLDSN()` 用 `fmt.Sprintf("...password=%s...")` 裸拼接，密码中部分字符可能破坏 MySQL DSN 语法格式 |
| **PostgreSQL** | 低 | `buildPGDSN()` 使用 URI 格式通过 `url.URL` 构建，驱动为 `pgx/v5/stdlib`，密码自动百分号编码 |
| **GaussDB** | 低 | `buildGaussDBDSN()` 使用独立 URI 格式通过 `url.URL` 构建，驱动为 `gaussdb-go/stdlib`（华为 pgx 分支），密码自动百分号编码 |
| **MongoDB** | 中 | `ApplyURI(d.Raw)` 将 Raw URL 直接传给驱动，仅 `#` 被预转义 |
| **Redis/ClickHouse/ES/Oracle/Hive** | 低 | 密码通过 HTTP Header / 结构体字段 / Base64 / `url.QueryEscape` 传递 |

> **建议**：即使 DSN 解析通过，**所有需要转义的字符都建议在 DSN 中百分号编码**，以避免 connector 层出现二次解析问题。

## 7. .env.dbexplain 文件注意事项

在 `.env.dbexplain` 配置文件中设置密码时，留意以下差异：

### DSN 格式 vs 原始密码

配置文件中填写的是**完整 DSN 字符串**，不是独立密码字段。因此密码中的特殊字符需遵循 URL 百分号编码规则：

```
# .env.dbexplain — DSN 中密码含 % 需写 %25
DB_DSN='postgres://user:pass%25word@host:5432/db'

# 等价的明文密码是: pass%word
```

### 不需要额外的 shell/INI 转义

`.env.dbexplain` 使用键值对格式，`key=value` 或 `key='value'`。除非你**手动开启**了 env 加密（`dbexplain encrypt`），否则不需要对密码做 shell 或 INI 级别的转义。

大多数情况下，直接在有特殊字符的密码外套上**单引号**即可，`#`、`$`、`!` 等不会被 shell 解释：

```
# 单引号保护，无需额外转义
DB_DSN='mysql://user:pass#word@host:3306/db'
DB_DSN='mysql://user:pass$word@host:3306/db'
DB_DSN='mysql://user:pass@word@host:3306/db'
# 全部正常——url.Parse 和 escapeUserinfo 会处理
```

> 注意：如果密码本身包含单引号，将单引号写为 `'\''`（结束单引号、转义单引号、恢复单引号），但更推荐直接对这个字符做 URL 编码：`'` → `%27`。

---

*技术背景：Go `url.Parse` 遵循 RFC 3986，userinfo 组件只允许 `unreserved` 字符（字母数字和 `-._~`）和 `sub-delims` 字符（``!$&'()*+,;= ``），以及百分号编码序列。其他字符均被拒绝。dbexplain 通过 `escapeUserinfo()` 自动编码所有不安全字符（保留已有 `%XX` 序列避免双编码）。*
