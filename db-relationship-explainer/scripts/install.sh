#!/bin/bash
set -e

# ============================================================
# dbexplain v0.0.7 — One-click installer (Linux / macOS)
# ============================================================
# Installs the dbexplain binary system-wide and optionally
# deploys the AI Agent skill to supported platforms.
#
# Usage:
#   bash install.sh                     Interactive install
#   bash install.sh --offline [PATH]    Offline mode (manual binary, optional path)
#   bash install.sh --no-skill          Skip skill installation
#   bash install.sh --update            Overwrite existing installation
#   bash install.sh --lang en           Install with English skill
#   bash install.sh --help              Show this help
# ============================================================

VERSION="v0.0.7"
REPO="IamWWT/understand_dbs_skills"
TOOL_NAME="dbexplain"

# ── Paths ──
SYSTEM_INSTALL_DIR="/usr/local/bin"
USER_INSTALL_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/dbexplain"
ENV_FILE="${CONFIG_DIR}/.env.dbexplain"
SKILL_SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── Flags ──
OFFLINE_MODE=false
SKIP_SKILL=false
UPDATE_MODE=false
OFFLINE_PATH="" # optional pre-placed binary path for --offline
LANG_SKILL=""   # empty means interactive (ask user)
INSTALL_DIR=""  # resolved later

# ── Color output ──
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
err()   { echo -e "${RED}[x]${NC} $*"; }
step()  { echo -e "${CYAN}[*]${NC} $*"; }

# ── Help ──
print_help() {
    cat <<EOF
dbexplain ${VERSION} — One-Click Installer (Linux/macOS)

Usage: bash install.sh [OPTIONS]

Options:
  --offline [PATH]   Offline mode. If PATH is given, install that specific binary
                     file directly. If omitted, prompt the user to manually place
                     the binary, then complete config and skill setup.
  --no-skill         Skip AI Agent skill installation (tool only).
  --update           Update mode: overwrite existing binary and skill files
                     without touching config.
  --lang zh|en       Skill language: zh=中文 (default), en=English.
  --help             Show this help message and exit.

Examples:
  bash install.sh                          # Full interactive install
  bash install.sh --lang en                # Full install with English skill
  bash install.sh --no-skill               # Tool only, no skill
  bash install.sh --offline                # Offline: you provide the binary
  bash install.sh --offline ./dbexplain    # Offline: use specified binary
  bash install.sh --update                 # Update to latest version

After install:
  Binary : ${SYSTEM_INSTALL_DIR}/dbexplain  (or ${USER_INSTALL_DIR}/dbexplain)
  Config : ${CONFIG_DIR}/.env.dbexplain

  Quick test:  dbexplain --version
  Edit config: nano ${CONFIG_DIR}/.env.dbexplain
  Run:         dbexplain -env
EOF
}

# ── Parse flags ──
while [ $# -gt 0 ]; do
    case "$1" in
        --offline)
            OFFLINE_MODE=true
            if [ -n "${2:-}" ] && [ "${2:0:1}" != "-" ]; then
                OFFLINE_PATH="$2"
                shift
            fi
            ;;
        --no-skill)  SKIP_SKILL=true ;;
        --update)    UPDATE_MODE=true ;;
        --lang)
            if [ "$2" = "zh" ] || [ "$2" = "en" ]; then
                LANG_SKILL="$2"
                shift
            else
                echo "Unknown --lang value: $2 (expected zh or en)"
                print_help; exit 1
            fi
            ;;
        --help)      print_help; exit 0 ;;
        *)           echo "Unknown option: $1"; print_help; exit 1 ;;
    esac
    shift
done

# ── Cleanup on exit ──
cleanup() {
    if [ -n "${TMP_BIN:-}" ] && [ -f "$TMP_BIN" ]; then
        rm -f "$TMP_BIN"
    fi
}
trap cleanup EXIT INT

# ── Platform detection ──
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            err "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)   OS="linux" ;;
        darwin)  OS="darwin" ;;
        *)
            err "Unsupported OS: $OS"
            exit 1
            ;;
    esac

    BINARY_DOWNLOAD="dbexplain-${OS}-${ARCH}"
    info "Detected platform: ${OS}/${ARCH}"
}

# ── Resolve install directory ──
resolve_install_dir() {
    if [ "$OS" = "darwin" ]; then
        # macOS: prefer /usr/local/bin, fallback to user-local
        if [ -w "$SYSTEM_INSTALL_DIR" ] || [ "$EUID" = "0" ]; then
            INSTALL_DIR="$SYSTEM_INSTALL_DIR"
        else
            warn "/usr/local/bin not writable — will install to ${USER_INSTALL_DIR}"
            INSTALL_DIR="$USER_INSTALL_DIR"
            mkdir -p "$INSTALL_DIR"
        fi
    else
        # Linux: try /usr/local/bin with sudo, fallback to ~/.local/bin
        if [ -w "$SYSTEM_INSTALL_DIR" ] || [ "$EUID" = "0" ]; then
            INSTALL_DIR="$SYSTEM_INSTALL_DIR"
        elif command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
            INSTALL_DIR="$SYSTEM_INSTALL_DIR"
            USE_SUDO="sudo"
        else
            warn "/usr/local/bin not writable and no passwordless sudo — will install to ${USER_INSTALL_DIR}"
            INSTALL_DIR="$USER_INSTALL_DIR"
            mkdir -p "$INSTALL_DIR"
        fi
    fi

    DEST_BIN="${INSTALL_DIR}/${TOOL_NAME}"
}

# ── Online install: download from GitHub ──
install_online() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_DOWNLOAD}"
    TMP_BIN="$(mktemp)"

    step "Downloading ${BINARY_DOWNLOAD} ..."
    if command -v curl &>/dev/null; then
        curl -L --progress-bar -o "$TMP_BIN" "$DOWNLOAD_URL"
    elif command -v wget &>/dev/null; then
        wget -q --show-progress -O "$TMP_BIN" "$DOWNLOAD_URL"
    else
        err "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    chmod +x "$TMP_BIN"
    move_binary "$TMP_BIN"
}

# ── Offline install: use provided path or prompt user ──
install_offline() {
    echo ""

    # If a specific path was given, use it directly
    if [ -n "$OFFLINE_PATH" ]; then
        if [ ! -f "$OFFLINE_PATH" ]; then
            err "Specified binary not found: ${OFFLINE_PATH}"
            exit 1
        fi
        step "Offline mode: using specified binary ${OFFLINE_PATH}"
        chmod +x "$OFFLINE_PATH"
        move_binary "$OFFLINE_PATH"
        return
    fi

    # Interactive offline mode
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_DOWNLOAD}"
    step "Offline mode: please obtain the binary manually."
    echo ""
    echo "  Download URL:"
    echo "    ${DOWNLOAD_URL}"
    echo ""
    echo "  Then place it at:"
    echo "    ${INSTALL_DIR}/${TOOL_NAME}"
    echo ""
    echo "  Or place it in the current directory as:"
    echo "    ${BINARY_DOWNLOAD}"
    echo ""

    read -r -p "  Press Enter once the binary is in place... " _unused

    # Check destination first
    if [ -f "${INSTALL_DIR}/${TOOL_NAME}" ]; then
        info "Found binary at ${INSTALL_DIR}/${TOOL_NAME}"
        return
    fi

    # Check current dir with download name
    if [ -f "./${BINARY_DOWNLOAD}" ]; then
        chmod +x "./${BINARY_DOWNLOAD}"
        move_binary "./${BINARY_DOWNLOAD}"
        return
    fi

    err "Binary not found at either location. Please re-run with --offline [PATH] specifying the binary location."
    exit 1
}

# ── Move binary to install dir ──
move_binary() {
    local src="$1"
    if [ "$USE_SUDO" = "sudo" ]; then
        sudo mkdir -p "$INSTALL_DIR"
        sudo mv "$src" "$DEST_BIN"
        sudo chmod +x "$DEST_BIN"
    else
        mkdir -p "$INSTALL_DIR"
        mv "$src" "$DEST_BIN"
        chmod +x "$DEST_BIN"
    fi
    info "Binary installed to ${DEST_BIN}"
}

# ── Config setup ──
setup_config() {
    if [ "$UPDATE_MODE" = true ]; then
        info "Update mode: skipping config setup."
        return
    fi

    mkdir -p "$CONFIG_DIR"

    if [ -f "$ENV_FILE" ]; then
        info "Config already exists at ${ENV_FILE} — skipping."
    else
        if [ -f "${SKILL_SRC_DIR}/.env.dbexplain.example" ]; then
            cp "${SKILL_SRC_DIR}/.env.dbexplain.example" "$ENV_FILE"
            info "Config template created at ${ENV_FILE}"
        else
            # Create a minimal template
            cat > "$ENV_FILE" << 'EOFTPL'
# dbexplain configuration file
# Format: DB<n>=<DSN>
#
# Examples:
# DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql
# DB2=redis://:password@127.0.0.1:6379/0?label=my-redis
# DB3=postgres://user:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable
EOFTPL
            info "Minimal config template created at ${ENV_FILE}"
        fi
        warn "Please edit ${ENV_FILE} and fill in your database connections."
    fi

    # No longer prompt for DBPROBE_ENV_FILE — config auto-discovery
    # in findConfigFile() handles both plaintext and encrypted files.
}

# ── Skill installation ──
install_skill() {
    if [ "$SKIP_SKILL" = true ]; then
        info "Skipping skill installation (--no-skill)."
        return
    fi

    echo ""
    step "Installing AI Agent skill ..."

    local skill_installer="${SKILL_SRC_DIR}/scripts/install-skill.sh"

    if [ ! -f "$skill_installer" ]; then
        warn "Skill installer not found at ${skill_installer}"
        warn "You can install the skill manually later:"
        echo "  cd db-relationship-explainer && bash scripts/install-skill.sh"
        return
    fi

    # Run skill installer (already interactive)
    # Pass --lang if specified, otherwise let install-skill.sh ask interactively
    if [ -n "$LANG_SKILL" ]; then
        bash "$skill_installer" --lang "$LANG_SKILL"
    else
        bash "$skill_installer"
    fi
}

# ── PATH check ──
check_path() {
    if [ "$INSTALL_DIR" = "$USER_INSTALL_DIR" ]; then
        if ! echo "$PATH" | tr ':' '\n' | grep -qx "$USER_INSTALL_DIR"; then
            warn "${USER_INSTALL_DIR} is not in your PATH."
            echo "  Add this to your shell profile:"
            echo "    export PATH=\"\${HOME}/.local/bin:\${PATH}\""
        fi
    fi
}

# ── Print success ──
print_success() {
    echo ""
    echo "============================================"
    info "dbexplain ${VERSION} installation complete!"
    echo "============================================"
    echo ""
    echo "  Binary : ${DEST_BIN}"
    echo "  Config : ${ENV_FILE}"
    echo ""
    echo "  Quick test : dbexplain --version"
    echo "  List DBs  : dbexplain list -env"
    echo "  Edit config: nano ${ENV_FILE}"
    echo "  Run        : dbexplain -env"
    echo ""
    echo "  Secure your config (recommended):"
    echo "    dbexplain encrypt ${ENV_FILE}"
    echo "    rm ${ENV_FILE}"
    echo "    dbexplain -env"
    echo ""
    echo "  Full manual: dbexplain all"

    if [ "$INSTALL_DIR" = "$USER_INSTALL_DIR" ]; then
        warn "Binary is in ${USER_INSTALL_DIR}. Make sure it's in your PATH."
    fi

    if [ "$SKIP_SKILL" = true ]; then
        echo "  To install the AI Agent skill later:"
        echo "    bash scripts/install-skill.sh           # 中文"
        echo "    bash scripts/install-skill.sh --lang en # English"
        echo ""
    fi
}

# ── Main ──
main() {
    echo ""
    echo "  dbexplain ${VERSION} — One-Click Installer"
    echo "  ${REPO}"
    echo ""

    detect_platform
    resolve_install_dir

    if [ "$UPDATE_MODE" = true ]; then
        info "Update mode: will overwrite existing installation."
        SKIP_SKILL=false  # in update mode, also update skill
    fi

    if [ "$OFFLINE_MODE" = true ]; then
        install_offline
    else
        install_online
    fi

    setup_config
    check_path
    install_skill
    print_success
}

main
