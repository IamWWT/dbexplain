# ============================================================
# dbexplain v0.0.7 — Uninstaller (Windows PowerShell)
# ============================================================
# Removes the dbexplain binary, config directory,
# and optionally legacy DBPROBE_ENV_FILE from user environment.
#
# Usage:
#   .\uninstall.ps1                 Interactive uninstall
#   .\uninstall.ps1 -All            Remove everything without confirmation
#   .\uninstall.ps1 -Help           Show this help
# ============================================================

param(
    [switch]$All,
    [switch]$Help
)

$VERSION = "v0.0.7"
$InstallDir = "$env:LOCALAPPDATA\dbexplain"
$ConfigDir = "$env:USERPROFILE\.config\dbexplain"
$DestBin = Join-Path $InstallDir "dbexplain.exe"

# ── Colors ──
function Write-Info  { Write-Host "[+] $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "[!] $args" -ForegroundColor Yellow }

# ── Help ──
if ($Help) {
    @"
dbexplain $VERSION — Uninstaller (Windows)

Usage: .\uninstall.ps1 [OPTIONS]

Options:
  -All     Remove everything without confirmation prompts.
  -Help    Show this help message and exit.

What gets removed:
  - Binary: $DestBin
  - Install directory: $InstallDir
  - Config directory: $ConfigDir (may contain .env.dbexplain, .enc files, .encryption_key)
  - DBPROBE_ENV_FILE user environment variable (legacy cleanup, v0.0.6+ no longer required)
  - Install directory from user PATH

Warning: The config directory may contain credentials (.env.dbexplain, .enc, .encryption_key).
"@
    exit 0
}

Write-Host ""
Write-Host "  dbexplain $VERSION — Uninstaller (Windows)"
Write-Host ""

$found = $false

# ── Remove binary ──
if (Test-Path $DestBin) {
    if ($All) {
        Remove-Item -Force $DestBin
        Write-Info "Removed $DestBin"
    } else {
        $answer = Read-Host "  Remove $DestBin? [Y/n]"
        if ($answer -ne "n" -and $answer -ne "N") {
            Remove-Item -Force $DestBin
            Write-Info "Removed $DestBin"
        } else {
            Write-Info "Kept $DestBin"
        }
    }
    $found = $true
}

# ── Remove install directory if empty ──
if (Test-Path $InstallDir) {
    $contents = Get-ChildItem -Path $InstallDir -ErrorAction SilentlyContinue
    if (-not $contents) {
        if ($All) {
            Remove-Item -Force -Recurse $InstallDir
        } else {
            $answer = Read-Host "  Remove empty directory $InstallDir? [Y/n]"
            if ($answer -ne "n" -and $answer -ne "N") {
                Remove-Item -Force -Recurse $InstallDir
            }
        }
    }
}

if (-not $found) {
    Write-Warn "No binary found at $DestBin"
}

# ── Remove from PATH ──
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -like "*$InstallDir*") {
    Write-Host ""
    if ($All) {
        $newPath = ($userPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Info "Removed $InstallDir from user PATH"
    } else {
        $answer = Read-Host "  Remove $InstallDir from user PATH? [Y/n]"
        if ($answer -ne "n" -and $answer -ne "N") {
            $newPath = ($userPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
            Write-Info "Removed from user PATH"
        }
    }
}

# ── Remove config ──
if (Test-Path $ConfigDir) {
    Write-Host ""
    if ($All) {
        Remove-Item -Force -Recurse $ConfigDir
        Write-Info "Removed config directory $ConfigDir"
    } else {
        Write-Warn "Config directory found: $ConfigDir"
        Write-Warn "This may contain credentials (.env.dbexplain, .enc, .encryption_key)!"
        $answer = Read-Host "  Remove $ConfigDir? [y/N]"
        if ($answer -eq "y" -or $answer -eq "Y") {
            Remove-Item -Force -Recurse $ConfigDir
            Write-Info "Removed $ConfigDir"
        } else {
            Write-Info "Kept $ConfigDir"
        }
    }
}

# ── Remove legacy DBPROBE_ENV_FILE (v0.0.6+ no longer required) ──
$current = [Environment]::GetEnvironmentVariable("DBPROBE_ENV_FILE", "User")
if ($current) {
    Write-Host ""
    if ($All) {
        [Environment]::SetEnvironmentVariable("DBPROBE_ENV_FILE", $null, "User")
        Write-Info "Removed DBPROBE_ENV_FILE user environment variable"
    } else {
        $answer = Read-Host "  Remove DBPROBE_ENV_FILE=$current from user environment? [Y/n]"
        if ($answer -ne "n" -and $answer -ne "N") {
            [Environment]::SetEnvironmentVariable("DBPROBE_ENV_FILE", $null, "User")
            Write-Info "Removed DBPROBE_ENV_FILE"
        }
    }
}

Write-Host ""
Write-Info "Uninstall complete."
Write-Host ""
Write-Host "  AI Agent skills (if installed) were not removed."
Write-Host "  To uninstall skills, run:"
Write-Host "    cd db-relationship-explainer && bash scripts/uninstall-skill.sh"
Write-Host ""

if (-not $All) {
    Write-Warn "Restart your terminal for PATH changes to fully take effect."
}
