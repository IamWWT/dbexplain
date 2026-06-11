# L0 / L4 / L7: 回归测试

> 版本升级对比、全量 Schema 采集回归、功能检查清单、版本一致性验证、性能基准。

---

## 前置条件

```bash
cd src
BIN="../release/dbexplain"
```

> **配置优先级**：运行 `-env` 前确保 CWD 中有 `.env.dbexplain` 或设置 `DBPROBE_ENV_FILE=.env`。
> 详见 [README.md](README.md#配置优先级说明) 和 [docs/CONFIG_SEARCH.md](../CONFIG_SEARCH.md)。

## 10.0 版本升级流程

跨版本回归时，构建上一版本进行对比：

```bash
# 检出上一版本
git worktree add /tmp/build-prev v0.0.8
cd /tmp/build-prev/src && go build -ldflags="-s -w" -o /tmp/dbexplain-prev .
cd -

# 构建当前版本
cd src && go build -ldflags="-s -w -X github.com/IamWWT/dbexplain/internal/version.Version=v0.1.5" -o /tmp/dbexplain-curr .

# 对比
/tmp/dbexplain-prev -env -timeout 30s --json > /tmp/prev.json 2>/dev/null
/tmp/dbexplain-curr -env -timeout 30s --json > /tmp/curr.json 2>/dev/null

# 清理
git worktree remove --force /tmp/build-prev
```

## 10.1 全量 Schema 采集回归

```bash
$BIN -env -timeout 30s 2>&1
# 预期: 全部 15 个 DSN 采集成功，总耗时 < 60s
```

## 10.2 功能回归检查清单

| 功能 | 版本引入 | 验证方式 |
|------|---------|---------|
| Schema 采集 (全部类型) | v0.0.4+ | 02-schema-collection.md |
| 跨库关系推断 | v0.0.4+ | 02-schema-collection.md |
| JSON/--human 输出 | v0.0.4+ | 02-schema-collection.md |
| -include/-exclude 过滤 | v0.0.5+ | 02-schema-collection.md |
| --context AI 上下文 | v0.0.5+ | 10-regression.md |
| -cache 增量扫描 | v0.0.5+ | 10-regression.md |
| 多 DSN 并发采集 | v0.0.5+ | 02-schema-collection.md |
| execute 只读查询 | v0.0.7+ | 03-execute-sql.md, 04-execute-nosql.md, 05-file-processing.md |
| sqlguard 三层校验 | v0.0.7+ | 06-security-sqlguard.md |
| 并发控制 | v0.0.7+ | 08-concurrent-limit.md |
| 安全策略引擎 | v0.0.8+ | 07-policy-engine.md |
| CSV/TSV/XLSX 文件 | v0.1.0 | 05-file-processing.md |
| 单二进制架构 | v0.1.0 | 01-environment.md |
| CONFIG_SEARCH 文档 | v0.1.0 | — |

## 10.3 版本一致性验证

```bash
# 检查全部文件中是否无旧版本残留
grep -rn "v0\.0\.8" --include="*.md" --include="*.sh" --include="*.ps1" --include="*.go" . | grep -v CHANGELOG | grep -v "_v0.0.8" | grep -v TEST_v0.0.8
# 预期: 空（无输出）
```

## 10.4 安全审计 — 敏感文件保护

```bash
# 确认 .env 文件未被 Git 追踪
git ls-files | grep -c "\.env$"
# 预期: 0

# 确认 logs/ 未被 Git 追踪
git ls-files | grep -c "logs/"
# 预期: 0

# 确认 .enc 文件未被 Git 追踪
git ls-files | grep -c "\.enc$"
# 预期: 0
```

## 10.5 性能基准测试

```bash
# 全量采集耗时（15 DSN）
time $BIN -env -timeout 60s --json 2>/dev/null > /dev/null
```

### 性能基线

| 数据源数 | v0.0.8 耗时 | v0.1.0 预期 |
|---------|-----------|------------|
| 9 DSN | ~15s | — |
| 15 DSN | — | < 60s |

> v0.1.0 继承并扩展了 v0.0.9 的文件处理能力（CSV/TSV/XLSX），文件处理为纯本地操作，新增耗时可忽略。主要耗时仍在远程数据库连接上。

## 10.6 发现并修复的问题

| ID | 问题 | 修复 | 版本 |
|----|------|------|------|
| FIX-001 | `extractColumnRefs` 无法匹配 `schema.table.column` 三节名 | 改为 `\w+(?:\.\w+)+` | v0.0.8 |
| FIX-002 | `handleFileExecute` 遗漏 `tsv` 路由 | 补充 `parsed.Kind == "tsv"` 检查 | v0.0.9 |
| FIX-003 | 旧双二进制架构（build_excel.sh + 子模块） | 合并为单二进制 | v0.0.9 |

## 10.7 后续改进建议

| 建议 | 优先级 |
|------|--------|
| CI 流水线 — GitHub Actions (go build + vet + test + 交叉编译) | 高 |
| analyze/connector 包补充单元测试 | 中 |
| Windows/macOS 实机验证 | 中 |
| CSV/XLSX 大文件性能测试 | 低 |
