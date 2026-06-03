#!/bin/bash
set -e

# ============================================================
# dbexplain v0.1.2 — Uninstaller (Linux / macOS)
# ============================================================
# Removes the dbexplain binary, config directory,
# and optionally legacy DBPROBE_ENV_FILE entries from shell profiles.
#
# Usage:
#   bash uninstall.sh               Interactive uninstall
#   bash uninstall.sh --all         Remove everything without confirmation
#   bash uninstall.sh --help        Show this help
# ============================================================

VERSION="v0.1.2"
TOOL_NAME="dbexplain"

SYSTEM_INSTALL_DIR="/usr/local/bin"
USER_INSTALL_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/dbexplain"

# ── Color output ──
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
step()  { echo -e "${CYAN}[*]${NC} $*"; }

# ── Help ──
print_help() {
    cat <<EOF
dbexplain ${VERSION} — Uninstaller (Linux/macOS)

Usage: bash uninstall.sh [OPTIONS]

Options:
  --all     Remove everything without confirmation prompts.
  --help    Show this help message and exit.

What gets removed:
  - Binary from /usr/local/bin/dbexplain or ~/.local/bin/dbexplain
  - Config directory: ~/.config/dbexplain/ (may contain .env.dbexplain, .enc files, .encryption_key)
  - DBPROBE_ENV_FILE from shell profiles (legacy cleanup, v0.0.6+ no longer required)

Warning: The config directory may contain credentials (.env.dbexplain, .enc, .encryption_key).
EOF
}

ALL_MODE=false
for arg in "$@"; do
    case "$arg" in
        --all)   ALL_MODE=true ;;
        --help)  print_help; exit 0 ;;
        *)       echo "Unknown option: $arg"; print_help; exit 1 ;;
    esac
done

echo ""
echo "  dbexplain ${VERSION} — Uninstaller"
echo ""

found=false

# ── Remove binary ──
for dir in "$SYSTEM_INSTALL_DIR" "$USER_INSTALL_DIR"; do
    bin_path="${dir}/${TOOL_NAME}"
    if [ -f "$bin_path" ]; then
        if [ "$ALL_MODE" = true ]; then
            rm -f "$bin_path"
            info "Removed ${bin_path}"
        else
            read -r -p "  Remove ${bin_path}? [Y/n] " answer
            if [ "$answer" != "n" ] && [ "$answer" != "N" ]; then
                rm -f "$bin_path"
                info "Removed ${bin_path}"
            else
                info "Kept ${bin_path}"
            fi
        fi
        found=true
    fi
done

if [ "$found" = false ]; then
    warn "No binary found in ${SYSTEM_INSTALL_DIR} or ${USER_INSTALL_DIR}"
fi

# ── Remove config ──
if [ -d "$CONFIG_DIR" ]; then
    echo ""
    if [ "$ALL_MODE" = true ]; then
        rm -rf "$CONFIG_DIR"
        info "Removed config directory ${CONFIG_DIR}"
    else
        warn "Config directory found: ${CONFIG_DIR}"
        warn "This may contain credentials (.env.dbexplain, .enc, .encryption_key)!"
        read -r -p "  Remove ${CONFIG_DIR}? [y/N] " answer
        if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
            rm -rf "$CONFIG_DIR"
            info "Removed ${CONFIG_DIR}"
        else
            info "Kept ${CONFIG_DIR}"
        fi
    fi
fi

# ── Remove legacy DBPROBE_ENV_FILE from shell profiles (v0.0.6+ no longer required) ──
echo ""
for rc in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
    if [ -f "$rc" ] && grep -q "DBPROBE_ENV_FILE" "$rc" 2>/dev/null; then
        if [ "$ALL_MODE" = true ]; then
            # Remove lines containing DBPROBE_ENV_FILE and the preceding comment
            sed -i '/# dbexplain config path/d;/DBPROBE_ENV_FILE/d' "$rc"
            info "Removed DBPROBE_ENV_FILE from ${rc}"
        else
            read -r -p "  Remove DBPROBE_ENV_FILE from ${rc}? [Y/n] " answer
            if [ "$answer" != "n" ] && [ "$answer" != "N" ]; then
                sed -i '/# dbexplain config path/d;/DBPROBE_ENV_FILE/d' "$rc"
                info "Removed from ${rc}"
            fi
        fi
    fi
done

echo ""
info "Uninstall complete."
echo ""
echo "  AI Agent skills (if installed) were not removed."
echo "  To uninstall skills, run:"
echo "    cd dbexplain-skill && bash scripts/uninstall-skill.sh"
echo ""
