# ============================================================
# dbexplain v0.1.5 — One-click installer (Windows PowerShell)
# ============================================================
# Installs the dbexplain binary and optionally deploys
# the AI Agent skill.
#
# Distribution format: tarball (.tar.gz) from GitHub Releases.
# The installer auto-detects platform → correct tarball → extract.
#
# Usage:
#   .\install.ps1                   Interactive install
#   .\install.ps1 -Offline [PATH]   Offline mode (binary or .tar.gz)
#   .\install.ps1 -NoSkill          Skip skill installation
#   .\install.ps1 -Update           Overwrite existing installation
#   .\install.ps1 -Lang en           Install with English skill
#   .\install.ps1 -Edition duckdb   Install DuckDB edition
#   .\install.ps1 -Help             Show this help
# ============================================================

param(
    [string]$Offline = "",      # Offline mode with optional path (binary or tarball)
    [switch]$NoSkill,
    [switch]$Update,
    [ValidateSet("zh", "en")]
    [string]$Lang = "",
    [ValidateSet("std", "duckdb")]
    [string]$Edition = "",
    [switch]$Help
)

$VERSION = "v0.1.5"
$REPO = "IamWWT/dbexplain"
$TOOL_NAME = "dbexplain"
$EditionSuffix = if ($Edition) { $Edition } else { "" }  # resolved below
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
  -Offline [PATH] Offline mode. If PATH is given, install that specific
                  binary (.exe) or .tar.gz file. If omitted, prompt user.
  -NoSkill        Skip AI Agent skill installation (tool only).
  -Update         Update mode: overwrite existing binary and skill files
                  without touching config.
  -Lang zh|en     Skill language: zh=中文 (default), en=English.
  -Edition std|duckdb  Edition: std (pure Go, default) or duckdb (requires CGO).
  -Help           Show this help message and exit.

Examples:
  .\install.ps1                  # Full interactive install
  .\install.ps1 -Lang en          # Full install with English skill
  .\install.ps1 -NoSkill          # Tool only, no skill
  .\install.ps1 -Edition duckdb   # Install DuckDB edition
  .\install.ps1 -Offline          # Offline: you provide the binary
  .\install.ps1 -Offline "C:\downloads\dbexplain-v0.1.5-windows-amd64-std-upx.tar.gz"  # Tarball
  .\install.ps1 -Offline "C:\downloads\dbexplain-windows-amd64-std.exe"   # Raw exe
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

# ── Edition selection ──
if ($Edition -eq "") {
    Write-Host ""
    Write-Step "Select edition:"
    Write-Host "  1) Standard edition (-std) — pure Go, no DuckDB, zero runtime deps"
    Write-Host "  2) DuckDB edition (-duckdb) — includes DuckDB, requires CGO build"
    Write-Host ""
    $choice = Read-Host "  Choose [1/2] (default: 1)"
    if ($choice -eq "2" -or $choice -eq "duckdb") {
        $EditionSuffix = "duckdb"
    } else {
        $EditionSuffix = "std"
    }
} else {
    $EditionSuffix = $Edition
}
Write-Info "Selected edition: $EditionSuffix"

# ── Tarball name resolution ──
# Per-platform tarball: dbexplain-${VERSION}-windows-amd64-${EditionSuffix}.tar.gz
$TARBALL_NAME = "dbexplain-${VERSION}-windows-amd64-${EditionSuffix}-upx.tar.gz"
$TARBALL_DIR = "dbexplain-${VERSION}-windows-amd64-${EditionSuffix}-upx"
$BINARY_NAME = "dbexplain-windows-amd64-${EditionSuffix}.exe"

# ── Resolve install directory ──
$DestBin = Join-Path $InstallDir $BINARY_DEST

if ($Update) {
    Write-Info "Update mode: will overwrite existing installation."
}

# ── Check tar availability (required for tarball extraction) ──
$TarAvailable = $null -ne (Get-Command tar -ErrorAction SilentlyContinue)
if (-not $TarAvailable) {
    Write-Warn "'tar' command not found. Windows 10+/Server 2016+ include tar built-in."
    Write-Warn "Falling back to raw binary mode — you must provide the .exe directly."
}

# ── Helper: extract a single file from tarball ──
function Extract-FromTarball {
    param([string]$TarballPath, [string]$OutputDir)
    if (-not $TarAvailable) { return $false }
    $targetFile = "${TARBALL_DIR}/${BINARY_NAME}"
    try {
        tar -xzf "$TarballPath" -C "$OutputDir" $targetFile 2>$null
        $extracted = Join-Path $OutputDir $targetFile
        return (Test-Path $extracted)
    } catch {
        return $false
    }
}

# ── Install binary ──
$IsOffline = $Offline -ne "" -or $PSBoundParameters.ContainsKey('Offline')

if (-not $IsOffline) {
    # Online mode: download tarball from GitHub, extract .exe
    $DownloadUrl = "https://github.com/$REPO/releases/download/$VERSION/$TARBALL_NAME"
    $TmpDir = Join-Path $env:TEMP "dbexplain-install"
    $TarballPath = Join-Path $TmpDir $TARBALL_NAME
    $null = New-Item -ItemType Directory -Path $TmpDir -Force

    Write-Step "Downloading $TARBALL_NAME ..."
    try {
        if (Get-Command Start-BitsTransfer -ErrorAction SilentlyContinue) {
            Start-BitsTransfer -Source $DownloadUrl -Destination $TarballPath
        } else {
            Invoke-WebRequest -Uri $DownloadUrl -OutFile $TarballPath
        }
        Write-Info "Download complete ($((Get-Item $TarballPath).Length) bytes)."
    } catch {
        Write-Err "Download failed: $_"
        exit 1
    }

    # Extract binary from tarball
    if ($TarAvailable) {
        Write-Step "Extracting $BINARY_NAME from tarball ..."
        if (Extract-FromTarball $TarballPath $TmpDir) {
            $Extracted = Join-Path $TmpDir "${TARBALL_DIR}/${BINARY_NAME}"
        } else {
            Write-Err "Failed to extract $BINARY_NAME from tarball."
            Write-Step "Contents of tarball:"
            tar -tzf $TarballPath | ForEach-Object { Write-Host "    $_" }
            exit 1
        }
    } else {
        Write-Err "Cannot extract tarball: 'tar' command not found."
        Write-Warn "Install the binary manually:"
        Write-Warn "  1. Download: $DownloadUrl"
        Write-Warn "  2. Extract $BINARY_NAME using 7-Zip or another tool"
        Write-Warn "  3. Place it at: $DestBin"
        exit 1
    }

    # Move .exe to install dir
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    Move-Item -Force $Extracted $DestBin
    Write-Info "Binary installed to $DestBin"

    # Cleanup temp
    Remove-Item -Force -Recurse $TmpDir -ErrorAction SilentlyContinue

} else {
    # Offline mode
    $OfflinePath = if ($Offline -ne "") { $Offline } else { $null }
    $TarballUrl = "https://github.com/$REPO/releases/download/$VERSION/$TARBALL_NAME"

    if ($OfflinePath) {
        # Specific path provided
        if (-not (Test-Path $OfflinePath)) {
            Write-Err "File not found: $OfflinePath"
            exit 1
        }
        if ($OfflinePath -like "*.tar.gz" -or $OfflinePath -like "*.tgz") {
            # Tarball provided
            Write-Step "Offline mode: installing from tarball $OfflinePath"
            $OfflineTmp = Join-Path $env:TEMP "dbexplain-offline"
            $null = New-Item -ItemType Directory -Path $OfflineTmp -Force
            if (Extract-FromTarball $OfflinePath $OfflineTmp) {
                $Extracted = Join-Path $OfflineTmp "${TARBALL_DIR}/${BINARY_NAME}"
                if (-not (Test-Path $InstallDir)) {
                    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
                }
                Copy-Item -Force $Extracted $DestBin
                Write-Info "Binary installed to $DestBin (from tarball)"
                Remove-Item -Force -Recurse $OfflineTmp -ErrorAction SilentlyContinue
            } else {
                Write-Err "Failed to extract from tarball: $OfflinePath"
                exit 1
            }
        } else {
            # Raw .exe provided
            Write-Step "Offline mode: using specified binary $OfflinePath"
            if (-not (Test-Path $InstallDir)) {
                New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
            }
            Copy-Item -Force $OfflinePath $DestBin
            Write-Info "Binary installed to $DestBin"
        }
    } else {
        # Interactive offline mode
        Write-Host ""
        Write-Step "Offline mode: please obtain the binary or tarball manually."
        Write-Host ""
        Write-Host "  Download URL (tarball):"
        Write-Host "    $TarballUrl"
        Write-Host ""
        Write-Host "  Then run:"
        Write-Host "    .\install.ps1 -Offline 'C:\path\to\$TARBALL_NAME'"
        Write-Host "    .\install.ps1 -Offline 'C:\path\to\$BINARY_NAME'"
        Write-Host ""

        $null = Read-Host "  Press Enter once the file is in place"

        if (-not (Test-Path $DestBin)) {
            # Check current dir
            $curTarball = Join-Path (Get-Location) $TARBALL_NAME
            $curExe = Join-Path (Get-Location) $BINARY_NAME
            if (Test-Path $curTarball) {
                Write-Step "Found tarball, extracting..."
                $OfflineTmp = Join-Path $env:TEMP "dbexplain-offline"
                $null = New-Item -ItemType Directory -Path $OfflineTmp -Force
                if (Extract-FromTarball $curTarball $OfflineTmp) {
                    $Extracted = Join-Path $OfflineTmp "${TARBALL_DIR}/${BINARY_NAME}"
                    if (-not (Test-Path $InstallDir)) {
                        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
                    }
                    Copy-Item -Force $Extracted $DestBin
                    Remove-Item -Force -Recurse $OfflineTmp -ErrorAction SilentlyContinue
                }
            } elseif (Test-Path $curExe) {
                if (-not (Test-Path $InstallDir)) {
                    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
                }
                Copy-Item -Force $curExe $DestBin
                Write-Info "Binary copied to $DestBin (from current directory)"
            } else {
                Write-Err "Binary/tarball not found."
                Write-Err "Re-run with -Offline 'C:\path\to\file'"
                exit 1
            }
        } else {
            Write-Info "Found binary at $DestBin"
        }
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

    # No longer prompt for DBPROBE_ENV_FILE — config auto-discovery
    # in findConfigFile() handles both plaintext and encrypted files.
}

# ── Skill installation ──
if (-not $NoSkill) {
    Write-Host ""
    Write-Step "Skill installation ..."

    $skillInstaller = Join-Path $ScriptDir "install-skill.sh"

    if (-not (Test-Path $skillInstaller)) {
        Write-Warn "Skill installer not found at $skillInstaller"
        Write-Warn "To install the skill manually:"
        Write-Host "  cd dbexplain-skill && bash scripts/install-skill.sh"
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
            Write-Host "    Claude Code : %USERPROFILE%\.claude\skills\dbexplain-skill"
            Write-Host "    DeepSeek    : %USERPROFILE%\.deepseek\skills\dbexplain-skill"
            Write-Host "    AiXCoding   : %USERPROFILE%\.aixcoding\skills\dbexplain-skill"
            Write-Host "    Agents      : %USERPROFILE%\.agents\skills\dbexplain-skill"
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
Write-Host "  List DBs  : dbexplain list -env"
Write-Host "  Edit config: notepad $EnvFilePath"
Write-Host "  Run        : dbexplain -env"
Write-Host ""
Write-Host "  Secure your config (recommended):"
Write-Host "    dbexplain encrypt $EnvFilePath"
Write-Host "    Remove-Item $EnvFilePath"
Write-Host "    dbexplain -env"
Write-Host ""
Write-Host "  Full manual: dbexplain all"
Write-Host ""
Write-Warn "IMPORTANT: Restart your terminal for PATH changes to take effect."
Write-Host ""

if ($NoSkill) {
    Write-Host "  To install the AI Agent skill later:"
    Write-Host "    cd dbexplain-skill && bash scripts/install-skill.sh"
    Write-Host ""
}
