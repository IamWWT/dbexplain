# 部署指南

`dbexplain` 提供**源码编译**、**二进制安装**、**AI Skill 部署**三种方式。本文档覆盖完整的部署流程。

---

## 目录

- [1. 源码编译](#1-源码编译)
- [2. 工具安装](#2-工具安装)
  - [Linux / macOS 在线安装](#linux--macos-在线安装)
  - [Linux / macOS 离线安装](#linux--macos-离线安装)
  - [Windows 安装](#windows-安装)
- [3. 安装后配置](#3-安装后配置)
  - [创建配置文件](#创建配置文件)
  - [加密配置文件](#加密配置文件)
- [4. AI Skill 部署](#4-ai-skill-部署)
  - [一键安装（工具 + Skill）](#一键安装工具--skill)
  - [单独安装 Skill](#单独安装-skill)
  - [更新 Skill](#更新-skill)
  - [卸载 Skill](#卸载-skill)
- [5. 验证](#5-验证)
- [6. 截图示例](#6-截图示例)

---

## 1. 源码编译

### 环境要求

- Go 1.26+
- Linux / macOS / Windows (WSL 或 Git Bash)

### 编译步骤

```bash
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
cd src && go mod tidy && bash build.sh
```

编译产物在 `release/` 目录，覆盖 5 个平台：

| 平台 | 文件名 |
|------|--------|
| Linux amd64 | `release/dbexplain-linux-amd64` |
| Linux arm64 | `release/dbexplain-linux-arm64` |
| macOS amd64 | `release/dbexplain-darwin-amd64` |
| macOS arm64 | `release/dbexplain-darwin-arm64` |
| Windows amd64 | `release/dbexplain-windows-amd64.exe` |

### 快速验证

```bash
echo "CREATE TABLE t(id int);" | sqlite3 /tmp/test.db
./release/dbexplain-linux-amd64 -dsn "sqlite:////tmp/test.db"
```

---

## 2. 工具安装

### Linux / macOS 在线安装

```bash
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
bash dbexplain-skill/scripts/install.sh            # 中文 Skill（默认）
bash dbexplain-skill/scripts/install.sh --lang en  # English Skill
```

脚本自动检测系统和架构，从 GitHub Releases 下载对应二进制并安装到 `/usr/local/bin`。

> ![安装流程](assets/install-offline-1.png)
> *图：install.sh 运行过程*

### Linux / macOS 离线安装

```bash
# 在有网络的机器上下载
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-linux-amd64

# 复制到离线环境后安装
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64

# 仅安装工具，不部署 Skill
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64 --no-skill
```

### Windows 安装

在 PowerShell 中运行：

```powershell
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
.\dbexplain-skill\scripts\install.ps1           # 中文 Skill
.\dbexplain-skill\scripts\install.ps1 -Lang en  # English Skill
```

脚本自动下载 `dbexplain-windows-amd64.exe` 到 `%LOCALAPPDATA%\dbexplain\`，添加到用户 PATH。

---

## 3. 安装后配置

### 创建配置文件

```bash
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
EOF
```

Windows 用户将配置文件放在 `%USERPROFILE%\.config\dbexplain\.env.dbexplain`。

运行验证：

```bash
dbexplain -env          # 终端格式化报告
dbexplain --version     # 查看版本
dbexplain list -env     # 列出已配置数据库
```

> ![终端运行示例](assets/usages.png)
> *图：dbexplain 运行效果（-env / all 等子命令输出）*

### 加密配置文件

防止明文密码泄露，支持机器指纹加密：

```bash
# 机器指纹加密（默认，无需密码）
dbexplain encrypt

# 密码 + 机器指纹双重保护
dbexplain encrypt --password

# 加密后直接运行（自动发现并解密 .enc 文件）
dbexplain -env
```

---

## 4. AI Skill 部署

### 一键安装（工具 + Skill）

安装脚本默认自动部署 Skill，无需额外操作：

```bash
bash dbexplain-skill/scripts/install.sh
bash dbexplain-skill/scripts/install.sh --lang en
```

### 单独安装 Skill

工具已安装时，单独部署 Skill：

```bash
# 交互安装（默认中文）
bash dbexplain-skill/scripts/install-skill.sh

# 英文版
bash dbexplain-skill/scripts/install-skill.sh --lang en
```

运行后交互选择安装目标：

| 选项 | 说明 |
|------|------|
| `[1] All platforms` | 安装到 `~/.agents/skills` 并 symlink 到其他平台（推荐） |
| `[2]` ~ `[5]` | 单个平台：Claude Code / DeepSeek / AixCoding / Agents |
| `[6] All project platforms` | 安装到当前项目的 `.claude/.deepseek/.aixcoding/.agents/skills` |
| `[7] Custom directory` | 指定任意目录 |

### 更新 Skill

```bash
# 更新所有已安装位置（SKILL.md + 二进制，.env 保留）
bash dbexplain-skill/scripts/install-skill.sh --update
```

### 卸载 Skill

```bash
bash dbexplain-skill/scripts/uninstall-skill.sh           # 交互选择
bash dbexplain-skill/scripts/uninstall-skill.sh --list    # 列出已安装位置
bash dbexplain-skill/scripts/uninstall-skill.sh --all     # 移除全部
```

---

## 5. 验证

### 验证工具可执行

```bash
dbexplain --version
dbexplain list -env
dbexplain all
```

![TOOL 验证输出](assets/install-offline-verify-2.png)

### 验证 Skill 安装

```bash
bash dbexplain-skill/scripts/install-skill.sh --verify
```

验证项：SKILL.md 存在与格式、二进制文件存在性与可执行权限、`--version` 烟雾测试。

> ![Skill 验证输出](assets/install-offline-verify-1.png)
> *图：--verify 闭环验证输出*

在 Claude Code 会话中执行 `/skills`，应列出 `dbexplain-skill`。输入触发词如"分析数据库结构"即可自动调用。

---

## 6. 图示例

以下为架构与部署关系图，由项目源码目录下的 drawio 文件导出生成：

| 图 | 描述 |
|------|------|
| ![架构总览](assets/architecture.drawio.png) | 4 阶段流水线：INPUT（连接配置）→ COLLECT（9 种 DB 模式抽取）→ ANALYZE（FK 推断/排序/诊断/IR Graph）→ OUTPUT（Markdown/JSON/上下文文件） |
| ![部署拓扑](assets/deployment.drawio.png) | 三步安装：GitHub Releases → install.sh → 三个目标（二进制/usr/local/bin + 配置~/.config + Skill~/.agents） |
| ![Skill 交互流程](assets/skill-interaction.drawio.png) | AI Agent 与 dbexplain 的边界：Agent 读取 SKILL.md → 调用 dbexplain 收集确定性事实 → Agent 解释结果给用户 |
| ![NL2SQL 架构决策](assets/nl2sql_architecture_decision.svg) | NL2SQL 场景下的架构决策示意图 |

---

> 各数据库专项手册见 [`docs/`](README.md#文档索引) 目录。安全检查清单见 [`docs/SECURITY_CHECKLIST.md`](SECURITY_CHECKLIST.md)。
