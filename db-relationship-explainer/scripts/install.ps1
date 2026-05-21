# ============================================================
# dbexplain v0.0.5 — One-click installer (Windows PowerShell)
# ============================================================
# Installs the dbexplain binary and optionally deploys
# the AI Agent skill.
#
# Usage:
#   .\install.ps1                   Interactive install
#   .\install.ps1 -Offline          Offline mode (manual binary placement)
#   .\install.ps1 -NoSkill          Skip skill installation
#   .\install.ps1 -Update           Overwrite existing installation
#   .\install.ps1 -Lang en           Install with English skill
#   .\install.ps1 -Help             Show this help
# ============================================================

param(
    [switch]$Offline,
    [switch]$NoSkill,
    [switch]$Update,
    [ValidateSet("zh", "en")]
    [string]$Lang = "",
    [switch]$Help
)

$VERSION = "v0.0.5"
$REPO = "IamWWT/understand_dbs_skills"
$TOOL_NAME = "dbexplain"
$BINARY_DOWNLOAD = "dbexplain-windows-amd64.exe"
$BINARY_DEST = "dbexplain.exe"

$InstallDir = "$env:LOCALAPPDATA\dbexplain"
$ConfigDir = "$env:USERPROFILE\.config\dbexplain"
$EnvFilePath = "$ConfigDir\.env.dbexplain"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SkillSrcDir = Join-Path $ScriptDir ".."

# ── Colors ──
function Write-Info  { Write-Host "[+] $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "[!] $args" -ForegroundColor Yellow }
function Write-Err   { Write-Host "[x] $args" -ForegroundColor Red }
function Write-Step  { Write-Host "[*] $args" -ForegroundColor Cyan }

# ── Help ──
if ($Help) {
    @"
dbexplain $VERSION — One-Click Installer (Windows)

Usage: .\install.ps1 [OPTIONS]

Options:
  -Offline    Offline mode: prompt user to manually place the binary,
              then complete config and skill setup.
  -NoSkill    Skip AI Agent skill installation (tool only).
  -Update     Update mode: overwrite existing binary and skill files
              without touching config.
  -Lang zh|en Skill language: zh=中文 (default), en=English.
  -Help       Show this help message and exit.

Examples:
  .\install.ps1                  # Full interactive install
  .\install.ps1 -Lang en          # Full install with English skill
  .\install.ps1 -NoSkill          # Tool only, no skill
  .\install.ps1 -Offline          # Offline: you provide the binary
  .\install.ps1 -Update           # Update to latest version

After install:
  Binary : $InstallDir\$BINARY_DEST
  Config : $EnvFilePath

  Quick test:  dbexplain --version
  Edit config: notepad $EnvFilePath
  Run:         dbexplain -env
"@
    exit 0
}

# ── Check PowerShell version ──
if ($PSVersionTable.PSVersion.Major -lt 5) {
    Write-Err "PowerShell 5.0 or later required."
    exit 1
}

# ── Main ──
Write-Host ""
Write-Host "  dbexplain $VERSION — One-Click Installer (Windows)"
Write-Host "  $REPO"
Write-Host ""

# ── Detect architecture ──
$is64 = [Environment]::Is64BitOperatingSystem
if (-not $is64) {
    Write-Err "Only 64-bit Windows is supported."
    exit 1
}
Write-Info "Detected platform: windows/amd64"

# ── Resolve install directory ──
$DestBin = Join-Path $InstallDir $BINARY_DEST

if ($Update) {
    Write-Info "Update mode: will overwrite existing installation."
}

# ── Install binary ──
if (-not $Offline) {
    # Online mode: download from GitHub
    $DownloadUrl = "https://github.com/$REPO/releases/download/$VERSION/$BINARY_DOWNLOAD"
    $TmpBin = Join-Path $env:TEMP $BINARY_DOWNLOAD

    Write-Step "Downloading $BINARY_DOWNLOAD ..."
    try {
        # Use BITS transfer if available (faster, resumable), fallback to Invoke-WebRequest
        if (Get-Command Start-BitsTransfer -ErrorAction SilentlyContinue) {
            Start-BitsTransfer -Source $DownloadUrl -Destination $TmpBin
        } else {
            Invoke-WebRequest -Uri $DownloadUrl -OutFile $TmpBin
        }
        Write-Info "Download complete."
    } catch {
        Write-Err "Download failed: $_"
        Write-Warn "Try offline mode: .\install.ps1 -Offline"
        exit 1
    }

    # Move to install dir
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    Move-Item -Force $TmpBin $DestBin
    Write-Info "Binary installed to $DestBin"
} else {
    # Offline mode
    $DownloadUrl = "https://github.com/$REPO/releases/download/$VERSION/$BINARY_DOWNLOAD"
    Write-Host ""
    Write-Step "Offline mode: please obtain the binary manually."
    Write-Host ""
    Write-Host "  Download URL:"
    Write-Host "    $DownloadUrl"
    Write-Host ""
    Write-Host "  Then place it at (rename to dbexplain.exe):"
    Write-Host "    $DestBin"
    Write-Host ""

    $null = Read-Host "  Press Enter once the binary is in place"

    if (-not (Test-Path $DestBin)) {
        # Check current dir
        $curBin = Join-Path (Get-Location) $BINARY_DOWNLOAD
        if (Test-Path $curBin) {
            if (-not (Test-Path $InstallDir)) {
                New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
            }
            Move-Item -Force $curBin $DestBin
            Write-Info "Binary moved to $DestBin"
        } else {
            Write-Err "Binary not found at $DestBin or current directory."
            exit 1
        }
    } else {
        Write-Info "Found binary at $DestBin"
    }
}

# ── Add to user PATH ──
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    Write-Step "Adding $InstallDir to user PATH ..."
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    # Also update current session
    $env:Path += ";$InstallDir"
    Write-Info "Added to user PATH."
} else {
    Write-Info "Install directory already in PATH."
}

# ── Setup config ──
if (-not $Update) {
    if (-not (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }

    if (-not (Test-Path $EnvFilePath)) {
        $examplePath = Join-Path $SkillSrcDir ".env.example"
        if (Test-Path $examplePath) {
            Copy-Item $examplePath $EnvFilePath
            Write-Info "Config template created at $EnvFilePath"
        } else {
            @"
# dbexplain configuration file
# Format: DB<n>=<DSN>
#
# Examples:
# DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql
# DB2=redis://:password@127.0.0.1:6379/0?label=my-redis
# DB3=postgres://user:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable
"@ | Out-File -FilePath $EnvFilePath -Encoding UTF8
            Write-Info "Minimal config template created at $EnvFilePath"
        }
        Write-Warn "Please edit $EnvFilePath and fill in your database connections."
    } else {
        Write-Info "Config already exists at $EnvFilePath — skipping."
    }

    # Offer to set DBPROBE_ENV_FILE
    Write-Host ""
    $setEnv = Read-Host "  Set DBPROBE_ENV_FILE environment variable? [y/N]"
    if ($setEnv -eq "y" -or $setEnv -eq "Y") {
        [Environment]::SetEnvironmentVariable("DBPROBE_ENV_FILE", $EnvFilePath, "User")
        $env:DBPROBE_ENV_FILE = $EnvFilePath
        Write-Info "DBPROBE_ENV_FILE set to $EnvFilePath"
    }
}

# ── Skill installation ──
if (-not $NoSkill) {
    Write-Host ""
    Write-Step "Skill installation ..."

    $skillInstaller = Join-Path $ScriptDir "install-skill.sh"

    if (-not (Test-Path $skillInstaller)) {
        Write-Warn "Skill installer not found at $skillInstaller"
        Write-Warn "To install the skill manually:"
        Write-Host "  cd db-relationship-explainer && bash scripts/install-skill.sh"
    } else {
        # Check if bash is available (Git Bash / MSYS2 / WSL)
        if (Get-Command bash -ErrorAction SilentlyContinue) {
            Write-Info "Running skill installer via bash ..."
            if ($Lang -ne "") {
                bash $skillInstaller --lang $Lang
            } else {
                bash $skillInstaller
            }
        } else {
            Write-Warn "bash not found in PATH."
            Write-Warn "The skill installer requires Git Bash or MSYS2."
            Write-Warn "Install Git for Windows (https://git-scm.com) and re-run,"
            Write-Warn "or install the skill manually:"
            Write-Host ""
            Write-Host "  Skill directories:"
            Write-Host "    Claude Code : %USERPROFILE%\.claude\skills\db-relationship-explainer"
            Write-Host "    DeepSeek    : %USERPROFILE%\.deepseek\skills\db-relationship-explainer"
            Write-Host "    AiXCoding   : %USERPROFILE%\.aixcoding\skills\db-relationship-explainer"
            Write-Host "    Agents      : %USERPROFILE%\.agents\skills\db-relationship-explainer"
            Write-Host ""
            Write-Host "  Copy SKILL.md, .env.example, and create tools\dbexplain.exe symlink"
            Write-Host "  pointing to $DestBin."
        }
    }
} else {
    Write-Info "Skipping skill installation (-NoSkill)."
}

# ── Success ──
Write-Host ""
Write-Host "============================================"
Write-Info "dbexplain $VERSION installation complete!"
Write-Host "============================================"
Write-Host ""
Write-Host "  Binary : $DestBin"
Write-Host "  Config : $EnvFilePath"
Write-Host ""
Write-Host "  Quick test : dbexplain --version"
Write-Host "  Edit config: notepad $EnvFilePath"
Write-Host "  Run        : dbexplain -env"
Write-Host "  Full manual: dbexplain --manual"
Write-Host ""
Write-Warn "IMPORTANT: Restart your terminal for PATH changes to take effect."
Write-Host ""

if ($NoSkill) {
    Write-Host "  To install the AI Agent skill later:"
    Write-Host "    cd db-relationship-explainer && bash scripts/install-skill.sh"
    Write-Host ""
}
