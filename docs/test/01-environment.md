# L1: 环境验证

> 验证编译环境、代码质量、单元测试和交叉编译能力。

## 1.1 Go 版本检查

```bash
go version
# 预期: go 1.26+
```

实际输出 (2026-05):
```
go version go1.26.2 linux/amd64
```

## 1.2 编译验证

```bash
cd src
go build ./...
# 预期: 无错误输出
```

注意: 如果遇到 proxy 超时，设置 `HTTPS_PROXY=http://127.0.0.1:7897/`:
```bash
HTTPS_PROXY=http://127.0.0.1:7897/ go build ./...
```

## 1.3 代码静态检查

```bash
cd src
go vet ./...
# 预期: 无警告输出
```

## 1.4 单元测试

```bash
cd src
go test ./... -v -count=1 2>&1 | tail -40
# 预期: 所有测试通过 (ok / PASS)
```

## 1.5 构建版本号验证

```bash
cd src
go run . --version
# 预期: dbexplain v0.0.9
```

## 1.6 交叉编译 (build.sh)

```bash
cd src
bash build.sh
# 预期: 5 个平台二进制成功生成到 ../release/
ls -la ../release/
# 预期输出:
#   dbexplain-linux-amd64
#   dbexplain-linux-arm64
#   dbexplain-darwin-amd64
#   dbexplain-darwin-arm64
#   dbexplain-windows-amd64.exe
```

## 1.7 数据结构验证

```bash
cd src
# 验证 DSN 解析
go test ./dsn/ -v -count=1
# 验证 sqlguard
go test ./sqlguard/ -v -count=1
# 验证策略引擎
go test ./policy/ -v -count=1
# 验证 CSV 连接器
go test ./connector/ -v -run CSV -count=1
```

## 1.8 代理环境说明

本机 Go 模块代理位于 `127.0.0.1:7897`，所有 `go mod tidy` / `go build` / `go vet` 命令需加环境变量：

```bash
HTTPS_PROXY=http://127.0.0.1:7897/ go mod tidy
HTTPS_PROXY=http://127.0.0.1:7897/ go build ./...
```
