# L1: 环境验证与静态分析

---

## 1.1 Go 版本

```bash
go version
# 预期: go 1.26+
```

## 1.2 编译验证

```bash
cd src
go build ./... && echo "build: OK" || echo "build: FAIL"
go vet ./... && echo "vet: OK" || echo "vet: FAIL"
```

## 1.3 单元测试

```bash
cd src && go test ./... -v -count=1 2>&1 | tail -20
```

### DSN 解析 (dsn 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestParseDSN_Schemes` | 19 | 全部数据库类型 + alias + TLS + unsupported |
| `TestParseDSN_QueryParams` | 8 | label/sslmode/cluster/tls/中文 |
| `TestParseDSN_AutoLabel` | 1 | 无 label 自动生成 |
| `TestRedacted` | 10 | 密码脱敏（含 @/URL编码/空密码/占位符） |
| `TestParseDSN_EdgeCases` | 1 | 边界情况 |

### 字段推断 (schema 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestInferComment` | 43 | 标识符/名称/时间/金额/状态/布尔/邮箱/电话/IP/URL/密钥等 |
| `TestInferComment_Ordering` | 1 | 规则优先级验证 |

### 安全策略 (policy 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| 19 测试函数 | 45 | 全局/按DSN/表级/列级/语句级/原生/Mongo/Qdrant/Redis key/DSL per-DSN策略/MongoDB $out拒绝 |

### SQL 只读校验 (sqlguard 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| 15 测试函数 | 32 | 读动词/写动词(含KILL/SET/FLUSH)/空查询/多语句/自动LIMIT(含LIMIT())/分号/首词提取 |

### 查询并发控制 (query 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| 9 测试函数 | 15 | Lock/Unlock/并发/多标签 |

## 1.4 交叉编译

```bash
cd src && bash build.sh
# 预期: 5 个 GOOS/GOARCH 全部成功
#   linux/amd64 + linux/arm64 + darwin/amd64 + darwin/arm64 + windows/amd64

cd src && bash build.sh prod --no-upx
# 预期: 同上 5 个 GOOS/GOARCH, 跳过 UPX 压缩

cd src && bash build.sh prod --upx
# 预期: 同上 5 个 GOOS/GOARCH, 强制 UPX (未安装时 exit 1)
```

## 1.5 按需编译

```bash
cd src && bash build.sh dev && echo "dev: OK"          # 当前 GOOS/GOARCH 全驱动
cd src && bash build.sh dev --no-upx && echo "no-upx: OK"  # dev 跳过 UPX
cd src && bash build.sh minimal mysql,postgres && echo "minimal: OK"  # 按需驱动
cd src && bash build.sh minimal mysql,postgres --upx && echo "upx-force: OK"  # 强制 UPX
```
验证各模式二进制工作正常:

```bash
../release/dbexplain-linux-amd64 --version
# 预期: dbexplain v0.1.4
```

## 1.6 安全审计 — 敏感文件不被 Git 追踪

```bash
# .env 文件不在 Git 中
git ls-files src/.env
# 预期: 空（无输出）

# logs/ 目录不在 Git 中
git ls-files src/logs/
# 预期: 空

# 加密文件不在 Git 中
git ls-files '*.enc'
# 预期: 空
```

## 1.7 Shell 脚本语法检查

```bash
bash -n dbexplain-skill/scripts/install.sh && echo "install.sh OK"
bash -n dbexplain-skill/scripts/uninstall.sh && echo "uninstall.sh OK"
bash -n dbexplain-skill/scripts/install-skill.sh && echo "install-skill OK"
bash -n dbexplain-skill/scripts/uninstall-skill.sh && echo "uninstall-skill OK"
```

## 1.8 二进制版本确认

```bash
./dbexplain --version
# 预期: dbexplain v0.1.4
```

## 1.9 构建模式因素影响分析

`build.sh` 4 种模式 + 构建标签（Build Tags）在不同维度上的影响分析。

### 1.9.1 编译时间

| 模式 | GOOS/GOARCH 数 | UPX | Race | 相对耗时 | 典型场景 |
|------|---------------|-----|------|---------|---------|
| dev | 1 (当前, 如 linux/amd64) | 否 | 否 | **1×** (基准) | 本地开发迭代，快速验证 |
| test | 1 (当前, 如 linux/amd64) | 否 | 是 | **2-3×** | CI 测试，竞态检测 |
| minimal | 1 (当前, 如 linux/amd64) | 是 | 否 | **1.5-2×** (含 UPX) | 定制化精简分发 |
| prod | **5** (linux/darwin/windows × amd64/arm64) | 是 | 否 | **5-8×** | 发布，跨平台全驱动 |

**规律**: 编译时间主要受三因素影响 — GOOS/GOARCH 数（线性）、UPX（+50-100% per binary）、race detector（+100-200%）。

### 1.9.2 二进制体积

> UPX（Ultimate Packer for eXecutables）是构建时可执行文件压缩工具，对 Go 静态二进制可达 60-80% 压缩率。
> 压缩后的二进制为独立自解压文件，**用户运行时不需要安装 UPX**，无任何额外依赖。

| 配置 | 标签 | 无 UPX | UPX 后 | 压缩比 |
|------|------|--------|--------|--------|
| 仅基础 | 无标签 | — | — | — |
| 仅文件 | csv,xlsx | 6.2 MB | **1.9 MB** | 69% |
| SQL 数据库 | mysql,postgres,clickhouse,sqlite | 12 MB | **3.6 MB** | 70% |
| NoSQL 数据库 | redis,mongodb,es,qdrant | 35 MB | **7.0 MB** | 80% |
| SQL + NoSQL（无文件） | 全部远程库 | 40 MB | **8.5 MB** | 79% |
| 全驱动 | full | 42 MB | **9.1 MB** | 78% |

**规律**:
- 基础设施约 1-2 MB（UPX 后），包含 CLI 框架、DSN 解析、Schema 处理、DSL 引擎、安全策略等
- 各驱动体积差异大：SQL 驱动轻量（每个 0.5-1 MB UPX 后），NoSQL 驱动较重（ES/MongoDB 各 ~2 MB UPX 后）
- 纯文件场景体积最小（1.9 MB），无网络依赖，适合离线环境
- UPX 对 Go 静态二进制压缩效果极佳（平均 70-80%），远优于通用压缩工具

### 1.9.3 功能完整性

| 标签 | 驱动数 | 支持的数据源 | 限制 |
|------|--------|-------------|------|
| full (prod/dev/test) | 11 | mysql,postgres,clickhouse,sqlite,redis,mongodb,es,qdrant,csv,tsv,xlsx | 无 |
| mysql,postgres,sqlite,clickhouse | 4 | SQL 数据库 | NoSQL/文件报 `no scanner configured` |
| redis,mongodb,es,qdrant | 4 | NoSQL 数据库 | SQL/文件报 `no scanner configured` |
| csv,xlsx | 2 | csv,tsv,xlsx | 仅文件查询；`ResolveJoinSources` 返回 nil（stub） |
| 无标签 | 0 | 无 | registry 返回 `no connector for %q` 友好错误 |

**Stub 机制**: 当 csv/xlsx 未编译时，`queryutil/join_resolve_stub.go`（`//go:build !csv && !xlsx && !full`）提供无操作实现，`ResolveJoinSources()` 返回 `nil, nil`，避免编译错误。

### 1.9.4 UPX 压缩影响

| 指标 | 无 UPX | UPX --best --lzma | 影响程度 |
|------|--------|-------------------|---------|
| 磁盘体积 | 26 MB | 15 MB | -42% |
| 启动时间 (首次) | 8-12 ms | 40-80 ms | +3-5× |
| 启动时间 (缓存后) | — | 15-25 ms | +1-2×（OS 页缓存） |
| 内存占用 (RSS) | ~12 MB | ~20 MB | +60-70%（解压后实际内存映射） |
| 安全 | 可 strings 分析 | 轻度混淆（非加密） | 防随意审查，不防逆向 |
| 兼容性 | 无 | 部分 UPX 版本对特定平台兼容性需验证 | 低风险（主流平台已验证） |

**结论**: 分发和二进制的体积优先使用 UPX；本地开发和 CI 测试时可跳过 UPX 以加快编译速度。

### 1.9.5 安全属性

| 特性 | 效果 | dev | test | prod | minimal |
|------|------|-----|------|------|---------|
| 符号表剥离 (`-s -w`) | 体积减小 ~30%，DWARF 调试信息移除，`strings` 分析难度略增 | ✓ | ✓ | ✓ | ✓ |
| 编译路径移除 (`-trimpath`) | 不暴露本地文件系统路径，构建可复现 | ✓ | ✓ | ✓ | ✓ |
| CGO_ENABLED=0 (静态链接) | 无动态库依赖，单文件可部署至任意同架构 Linux 环境 | ✓ | ✓ | ✓ | ✓ |
| DuckDB CGO + 静态链接 | `duckdb` tag 启用 CGO=1 + `-extldflags=-static` (Linux) 或 `-static-libgcc -static-libstdc++` (macOS)，零运行时 ldd 依赖 | ✗ | ✗ | ✗ | minimal 含 duckdb 时 |
| UPX 轻度混淆 | 体积再降 70-80%（实测全驱动 42 MB → 9.1 MB），检查/审计工具输出不可读；启动延迟 +3-5× | ✗ | ✗ | ✓ | ✓ |
| Race detector | 运行时检测并发数据竞争；体积约 2×，运行性能下降 5-10× | ✗ | ✓ | ✗ | ✗ |

**静态链接保障**: `CGO_ENABLED=0` 确保纯 Go 静态链接，二进制运行时无动态库依赖（除 Linux 内核系统调用接口外）。`ldd` 检查确认无 `=> /` 动态引用。
**DuckDB 静态链接**: `duckdb` tag 使用 `CGO_ENABLED=1` + `-extldflags=-static`（Linux），`ldd` 显示 "not a dynamic executable"，零运行时依赖。macOS 使用 `-static-libgcc -static-libstdc++`，仅保留系统必有的 `/usr/lib/libSystem.B.dylib`。

**prod 模式默认 `full` 标签的设计考量**:

`bash build.sh`（prod）默认 `tags=full` 打包全部 11 种驱动，UPX 压缩后二进制 9.1 MB。理由是：

- **通用性优先**：发布版是"下载一次，到处能用"的通用二进制。用户只需一个文件就能连接任意支持的数据库类型（mysql/postgres/clickhouse/sqlite/redis/mongodb/es/qdrant/csv/tsv/xlsx），无需事先了解构建标签机制
- **Skill 集成依赖全量支持**：`dbexplain-skill` 文档列出的所有数据库类型在 prod 二进制中均可用，LLM Agent 在未被告知构建配置时也不会意外报 "no scanner configured"
- **15 MB 已是合理体积上限**：UPX 压缩后 15 MB 在当今网络环境下属可接受范围；如需进一步压缩，使用 `minimal` 模式定制

用户明确知晓部署环境仅需特定数据库时，应使用 `minimal` 模式获得更小体积。

### 1.9.6 场景推荐

| 场景 | 推荐命令 | 驱动数 | 体积(UPX) | 理由 |
|------|---------|--------|----------|------|
| 日常开发调试 | `bash build.sh dev` | 11 | 42 MB | 当前 GOOS/GOARCH 快速编译，覆盖全部类型，无 UPX 等待 |
| CI 单元测试 | `bash build.sh test` | 11 | ~80 MB | race detector 捕获并发数据竞争 |
| 全功能发布 | `bash build.sh` (prod) | 11 | 9.1 MB × 5 | 5 个 GOOS/GOARCH + 全驱动 + UPX，通用发布包 |
| 仅 SQL 数据库 (含 CH) | `bash build.sh minimal mysql,postgres,sqlite,clickhouse` | 4 | 3.6 MB | 关系型+分析型，最精简 SQL 场景 |
| 仅 MySQL + PG | `bash build.sh minimal mysql,postgres` | 2 | ~2.5 MB | 传统 OLTP 专用 |
| 全部 NoSQL | `bash build.sh minimal redis,mongodb,es,qdrant` | 4 | 7.0 MB | 非 SQL 数据源全量覆盖 |
| 仅 Redis + MongoDB | `bash build.sh minimal redis,mongodb` | 2 | ~4 MB | KV + 文档数据库专用 |
| 仅 ES 搜索 | `bash build.sh minimal elasticsearch` | 1 | ~3 MB | 搜索分析专用 |
| 仅 Qdrant 向量 | `bash build.sh minimal qdrant` | 1 | ~1.5 MB | 向量检索专用 |
| 纯文件查询 | `bash build.sh minimal csv,xlsx` | 2 | 1.9 MB | 离线环境，无网络依赖 |
| 文件 + SQL 混合分析 | `bash build.sh minimal csv,xlsx,mysql,postgres,sqlite` | 5 | ~4 MB | 文件 JOIN SQL 数据库 |
| 全远程数据库 (无文件) | `bash build.sh minimal mysql,postgres,sqlite,clickhouse,redis,mongodb,es,qdrant` | 8 | 8.5 MB | 所有在线 DB，仅去掉文件驱动 |

### 1.9.7 构建产物安全验证

UPX 压缩后的二进制为独立自解压文件，**用户不需要安装 UPX**，零额外依赖。验证流程：

```bash
# ── 静态链接验证 ──────────────────────────────────────────
file ../release/dbexplain-linux-amd64
# 预期: "ELF ... statically linked"

ldd ../release/dbexplain-linux-amd64
# 预期: "不是动态可执行文件" 或 "statically linked"
# 确认: 无 "=> /" 动态引用

nm -D ../release/dbexplain-linux-amd64
# 预期: 空输出 (无动态符号) 或 "无符号" / "no symbols"

# ── UPX 完整性验证 ────────────────────────────────────────
upx -t ../release/dbexplain-linux-amd64
# 预期: "Passed 1 format test" / "一切正常. 通过1个格式测试."

# ── 运行时独立验证 ────────────────────────────────────────
# 确认二进制不依赖任何外部文件
../release/dbexplain-linux-amd64 --version
# 预期: "dbexplain v0.1.4"

# 在隔离环境运行 (无 PATH 依赖)
env -i HOME=/nonexistent PATH=/usr/bin:/bin \
  ../release/dbexplain-linux-amd64 --version
# 预期: "dbexplain v0.1.4" (无额外依赖)
```

**验证结果 (v0.1.4, Linux amd64)**:

| 检查项 | 命令 | 预期 | 结果 |
|--------|------|------|------|
| 链接方式 | `file` | `statically linked` | PASS |
| 动态引用 | `ldd` | 无 `=> /` 动态加载 | PASS |
| 动态符号 | `nm -D` | 空 (无动态符号) | PASS |
| UPX 完整性 | `upx -t` | `Passed 1 format test` | PASS |
| 版本输出 | `--version` | `v0.1.4` | PASS |
| 隔离运行 | `env -i PATH=... --version` | `v0.1.4` | PASS |

**UPX 零运行时依赖确认**: UPX 在构建时将解压 stub 附加到二进制的末尾。运行时，stub 将原始程序解压到内存然后跳转到入口点。此过程完全在进程内完成，不调用外部程序或加载动态库。因此，UPX 压缩后的二进制是**完全自包含的**。

**动态 UPX 决策 (`--no-upx` / `--upx`)**:

build.sh 支持通过命令行参数动态控制 UPX:

| 参数 | 效果 | 适用场景 |
|------|------|---------|
| (无参数) | auto: prod/minimal 模式检测 upx 可用性 | 日常构建 |
| `--no-upx` | 强制跳过 UPX，即使已安装 | CI 加速、调试构建 |
| `--upx` | 强制使用 UPX，未安装时 exit 1 | 发布流程保证压缩一致性 |

参数可从命令行任意位置传入，与 mode 和 tags 顺序无关。

### 1.9.8 UPX 安全性结论

基于 §1.9.2-§1.9.7 全部实测结果，UPX 用于本项目的结论如下：

**安全性**:
- 零运行时依赖 — 构建时一次压缩，用户无需安装 UPX，无外部库调用
- 静态链接保障 — CGO_ENABLED=0 纯 Go 二进制，`ldd`/`nm -D` 确认无动态引用
- 完整性验证 — `upx -t` 确认压缩格式正确
- 代码安全 — 纯 Go 实现，无 `os/exec` 或外部程序调用
- UPX 自身为1996年至今的开源项目，业界广泛使用

**功能影响**: **无**。UPX 在 Go 运行时启动前完成内存解压，不影响任何运行时行为（goroutine/signal/文件操作/panic traceback）。仅增加一次性启动延迟约 30-70ms。

**跨平台限制**: UPX 支持 Mach-O 格式，但 Go 交叉编译的 Mach-O 二进制（Linux 编译 darwin 目标）触发 UPX 5.0.0 的 `CantUnpackException`，无法压缩。二进制依然有效且功能完整。macOS 原生编译时 UPX 正常工作。`build.sh` 已显式检测并跳过 darwin/arm64（UPX 无支持）和 darwin/amd64 交叉编译两种场景，打印具体跳过原因。

**UPX 不是加密**: 压缩 + 轻度混淆，不阻止逆向工程。若需防逆向应使用商业加壳工具，但会引入运行时依赖，违反"零依赖"原则。

**结论**: UPX 安全可用，体积减少 78%（43 MB → 9.5 MB），零运行时依赖，功能完全不受影响。
