# 安全检查手册 （Security Checklist）

> 发布前必读。每项必须确认通过，发现新问题及时追加到本手册。

---

## 1. 凭证保护（第一要义）

**任何代码路径都不得将数据库密码写入标准输出、标准错误、日志文件或任何持久化存储。**

### 检查项

- [ ] **DSN 输出** — 所有终端输出、日志、错误消息中的 DSN 必须经过 `Redacted()` 脱敏
  ```go
  // 正确
  log.Printf("skipping %s", parsed.Redacted())
  // 错误
  log.Printf("skipping %s", e.raw)
  ```
- [ ] **Error 消息** — `log.Printf`/`log.Fatalf`/`fmt.Errorf` 的格式化参数不得直接引用原始 DSN 字符串
- [ ] **第三方库错误** — `godotenv` 等第三方库的错误消息可能包含原始文件内容，必须经过 `sanitizeErr()` 处理
- [ ] **JSON 输出** — `-json` 输出永不包含连接字符串
- [ ] **日志文件** — `logs/<label>.log` 中不得出现原始密码
- [ ] **标准输出** — 所有 DSN 回显均经过 `Redacted()`
- [ ] **`.env` 文件** — 确认 `.gitignore` 包含 `src/.env`、`.env`、`.env.dbexplain`

### 历史上的坑

| Issue | 问题 | 修复 |
|-------|------|------|
| ISSUE-040 | `.env` 真实凭证提交到 Git | `.gitignore` 增加规则 |
| ISSUE-041 | `src/logs/` 暴露数据库名 | `.gitignore` 增加规则 |
| ISSUE-052 | godotenv 错误消息泄露完整 DSN | `sanitizeErr()` 脱敏处理 |
| v0.0.4 bug | `filterDSNs` skip 消息用 `e.raw` 泄漏密码 | 改用 `parsed.Redacted()` |
| v0.0.8 A4 | `os.Setenv` 传递 DSN 密码到进程环境 | `loadEnvFile()` 直接返回 `[]dsnEntry`，消除 OS env 中间人 |
| v0.0.8 C9 | ClickHouse URL 查询参数传密码，HTTP 日志可能泄露 | 改为 `X-ClickHouse-User`/`X-ClickHouse-Key` 请求头 |
| v0.0.8 A1 | DSN 解析错误消息含未脱敏密码 | 新增 `sanitizeErr()` 统一脱敏 |

---

## 2. 文件编码与平台兼容

### 检查项

- [ ] **UTF-8 BOM 处理** — 读取配置文件时必须先剥离 BOM (`EF BB BF`)
  - `.env.dbexplain` 通过 `loadEnvFile()` 读取，已内置 BOM 剥离
  - JSON 配置文件：`json.Decoder` 自动处理 BOM
  - 其他文本文件：读取后 `bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})`
- [ ] **Windows GBK** — `-o` 文本输出文件正常（中文系统自动转 GBK，英文系统 UTF-8 BOM）
- [ ] **JSON 输出不加 BOM** — `-json -o` 输出纯 UTF-8，不加 BOM
- [ ] **ASCII 安全渲染** — Unicode 制表符、项目符号等替换为 ASCII 等效字符

### 历史上的坑

| Issue | 问题 | 修复 |
|-------|------|------|
| ISSUE-051 | JSON 输出含 BOM，标准解析器报错 | `-json -o` 不再加 BOM |
| ISSUE-052 | UTF-8 BOM 导致 `.env.dbexplain` 解析失败 | `loadEnvFile()` 剥离 BOM |
| v0.0.4 | Windows 记事本/CMD 中文乱码 | ACP 检测 + GBK 转换 |

---

## 3. 输入验证

### 检查项

- [ ] **DSN 解析** — 恶意构造的 DSN 不会导致 panic 或注入
- [ ] **配置文件** — 畸形 JSON/ENV 文件有友好错误提示，不暴露系统路径
- [ ] **命令行参数** — `--include`/`--exclude` 等过滤参数对特殊字符安全（逗号分隔正常）
- [ ] **数据库名/表名** — 包含特殊字符的标识符正确处理（反引号/双引号转义）
- [ ] **SQL 注释剥离** — `extractTableNames()`/`extractColumnRefs()` 前必须先 `stripSQLComments()` 去除 `--` 和 `/* */` 注释，防止注释内嵌敏感表名绕过检测
- [ ] **引用标识符归一化** — `normalizeIdentifiers()` 剥离反引号/双引号/方括号引用后提取，防止引用标识符绕过策略匹配
- [ ] **空白字符归一化** — `CheckSQL()`/`CheckNative()` 使用 `normalizeWhitespace()` 折叠所有空白后匹配，防止变体空白绕过语句级策略
- [ ] **子查询 LIMIT 绕过** — `AutoLimit()` 使用 `hasOuterLimit()` 剥离括号内容后检测 LIMIT，防止子查询内部 LIMIT 绕过自动注入
- [ ] **Redis 通配符** — `globMatch()` 替代 `filepath.Match`，确保 `/` 不截断 `*` 通配符匹配
- [ ] **文件路径** — `--log-dir`、`--context`、`--cache`、`-o` 等路径参数防止路径遍历
- [ ] **DuckDB 文件访问控制** — `read_parquet`/`read_csv_auto`/`read_json` 等文件函数受 `allowed_path` DSN 参数限制；未配置时拒绝所有文件读取函数调用；已配置时路径必须在前缀范围内（`filepath.Clean` + `strings.HasPrefix`），多路径逗号分隔合法

---

## 4. 安全传输

### 检查项

- [ ] **TLS/SSL** — MySQL/PostgreSQL/ClickHouse/ES 的 TLS 配置正确
  - MySQL: `tls=<config>` 参数生效
  - PostgreSQL: `sslmode` 参数生效
  - ES/Redis: `?tls=true` 启用 HTTPS
- [ ] **ES 证书验证** — `?tls-skip-verify=true` 需显式启用（不再默认跳过），仅限诊断环境
- [ ] **ClickHouse 鉴权** — 使用 `X-ClickHouse-User`/`X-ClickHouse-Key` 请求头而非 URL 参数，避免密码在 HTTP 日志中泄露

---

## 5. 运行时安全

### 检查项

- [ ] **只读操作** — 所有 Connector 仅使用 `SELECT`/`SHOW`/`SCAN`/`PRAGMA` 等只读操作
- [ ] **采样上限** — Redis ≤ 2000 key、≤ 5 字段、≤ 512 字节
- [ ] **连接隔离** — 每实例独立连接，单实例 panic 不影响其他实例
- [ ] **超时控制** — 所有数据库连接有合理的连接/读写超时
- [ ] **并发安全** — sync.Map 或 mutex 保护共享状态
- [ ] **终端注入防御** — `formatHuman()` 中单元格值经过 `sanitizeCell()` 剥离 ANSI 转义序列和控制字符（仅 `--human` 输出，覆盖全部 14 种数据源（含可选 DuckDB 和 Prometheus）；JSON 输出经 Go `json.Encoder` 原生转义，无需额外处理）
- [ ] **列宽上限** — `formatHuman()` 列宽 cap 于 256 字符，超长 cell 截断并追加 `…`（仅 `--human` 输出，覆盖全部 14 种数据源（含可选 DuckDB 和 Prometheus））

---

## 5a. 提交前快速检查

每次 `git commit` 前，至少确认以下 5 项通过（约 30 秒）：

```
[ ] go build ./... 无错误
[ ] go vet ./...   无警告
[ ] go test ./... -count=1 全部通过
[ ] grep -rn "v0\.1\.[01]" --include="*.md" --include="*.sh" --include="*.ps1" . \
      | grep -v CHANGELOG | grep -v RELEASE_WECHAT | grep -v "history\|introduced in\|requires\|已完成\|v0.1.[01] landed\|v0.1.[01] 新增\|v0.1.[01] 已\|已迁移\|Phase.*v0.1.[01]" \
      | grep -v "\.git/" | grep -v testdata/
    # 确认无残留旧版本号（只允许历史记录/CHANGELOG中的引用）
[ ] issues.json 有效性: python3 -m json.tool issues.json > /dev/null
```

> 完整发布检查见 §6，耗时约 3-5 分钟。

---

## 6. 发布前快速检查

每次发布前，逐项执行：

```
[ ] go build 无错误
[ ] go vet  无警告
[ ] go test ./... 全部通过
[ ] 版本一致性: version.go / build.sh ldflags / CHANGELOG.md 版本号一致
[ ] CHANGELOG 完整性: 当前版本所有已关闭 Issue 在 CHANGELOG 列出
[ ] CHANGELOG 中英文同步: CHANGELOG_EN.md 条目与 CHANGELOG.md 保持一致
[ ] issues.json 有效性: python3 -m json.tool issues.json 无语法错误
[ ] 交叉编译 5 平台全部成功（bash build.sh）
[ ] release/ 目录下 6 平台二进制存在（含 dev 版本），file 命令确认架构正确
[ ] 全平台版本一致性: 5 平台二进制各自 --version 全部输出 v0.x.x
[ ] dev 二进制使用 -tags full 构建（42MB 全驱动，非 3.7MB 精简版）
[ ] 二进制冒烟测试: --version 输出正确版本号，基本查询正常
[ ] .gitignore 包含: src/.env, .env, .env.dbexplain, src/logs/
[ ] grep -r "e\.raw" src/        # 确认无原始 DSN 输出
[ ] grep -r "log.*Printf.*%s.*err" src/  # 确认错误消息经过脱敏
[ ] grep -r "\.env" src/ --include="*.go" | grep -v test | grep -v example  # 检查引用
[ ] 以 UTF-8 BOM 编码的 .env.dbexplain 测试自动加载正常
[ ] 文档陈旧引用检查: grep 在 docs/ 中查找已删除文件路径的引用
[ ] 文档版本一致性: grep -rn "v0\.[01]\.[01]" docs/test/*.md 确认测试预期均更新为本版本
[ ] 脚本版本一致性: grep "\$VERSION\s*=" dbexplain-skill/scripts/*.{sh,ps1} 确认版本号最新
[ ] 脚本头部版本: head -5 dbexplain-skill/scripts/*.{sh,ps1} 确认注释中版本号最新
[ ] Markdown 链接有效性: find docs/ -name "*.md" -exec grep -oP '\[.+?\]\([^)]+\)' {} \; \
      | grep -v "http" | grep -v "#" | grep -v "\.\./" \
      | while read -r link; do echo "CHECK: $link"; done
    # 确认相对链接指向的文件存在（排除外部URL和锚点）
[ ] 确认 CHANGELOG.md 列出所有安全问题
[ ] 确认 SECURITY_CHECKLIST.md 追加新发现的问题
[ ] 加密输出文件权限为 0600
[ ] 密码输入无回显 (term.ReadPassword)
[ ] 加密文件解密失败不暴露内部错误细节
[ ] .gitignore 包含 *.enc
```

### 历史上的坑

| 版本 | 问题 | 修复 |
|------|------|------|
| v0.1.2 | 10 个测试文档 `../docs/CONFIG_SEARCH.md` 链接多一层 `docs/` 路径（应为 `../CONFIG_SEARCH.md`） | §5a 新增版本一致性 grep + §6 新增文档版本一致性检查 |
| v0.1.2 | PowerShell 脚本 `$VERSION = "v0.1.0"` 未随版本更新 | §6 新增脚本版本一致性 + 脚本头部版本检查 |
| v0.1.2 | 测试文档中版本预期（09-cli-help.md）残留 v0.1.1 | §5a 新增旧版本号 grep 检查 |
| v0.1.2 | dev 二进制未加 `-tags full` 仅 3.7MB（精简版） | §6 新增 dev 二进制 -tags full 检查 |
| v0.1.2 | `docs/file_index.md` 引用已删除的 RELEASE_WECHAT_v0.1.1.md | §6 新增 Markdown 链接有效性检查 |

---

## 7. 配置加密检查

### 检查项

- [ ] **encrypt 子命令** — `go run . encrypt` 和 `go run . encrypt --password` 均正常生成加密文件
- [ ] **文件格式** — 加密文件以正确模式字节开头（0x00 机器模式, 0x01 密码模式）
- [ ] **机器指纹确定性** — 同一台机器多次运行 `MachineID()` 返回相同值
- [ ] **跨机器验证** — 不同机器上加密的文件无法解密（指纹不匹配）
- [ ] **密码模式** — `--password` 加密的文件在无 `APP_ENCRYPTION_KEY` 且无 `~/.config/dbexplain/.encryption_key` 时解密失败
- [ ] **自动解密** — `dbexplain` 自动发现 `~/.config/dbexplain/.env.dbexplain.enc` 并解密加载配置（无需环境变量）
- [ ] **文件权限** — 加密输出文件权限为 `0600`
- [ ] **密码输入安全** — 密码输入无回显，确认密码流程正确
- [ ] **BOM 兼容** — 带 BOM 的 `.env` 文件加密后解密内容正确（BOM 在加密前已剥离）
- [ ] **错误信息安全** — 解密失败不暴露加密算法内部细节
- [ ] **`.gitignore`** — 确认 `*.enc` 已加入 `.gitignore`

### 历史上的坑

| Issue | 问题 | 修复 |
|-------|------|------|
| - | - | - |

---

## 8. 新增安全问题流程

发现新安全问题时：

1. **记录 ISSUE** — 创建 GitHub Issue，标记 bug 标签
2. **追加本章** — 在对应检查项的"历史上的坑"表格中追加
3. **更新 CHANGELOG** — 在对应版本的安全/修复章节中追加
4. **团队通报** — Release 描述中重点标注安全问题

---

## 相关文档

- [ARCHITECTURE.md - 第 9 章 安全性](./ARCHITECTURE.md)
- [CHANGELOG.md](../CHANGELOG.md) — 每版本安全问题汇总
- [DEPLOY.md](./DEPLOY.md) — 部署安全配置说明
