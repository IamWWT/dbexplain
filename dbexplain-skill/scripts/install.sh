#!/bin/bash
set -e

# ============================================================
# dbexplain v0.1.6 — One-click installer (Linux / macOS)
# ============================================================
# Installs the dbexplain binary system-wide and optionally
# deploys the AI Agent skill to supported platforms.
#
# Distribution format: tarball (.tar.gz) from GitHub Releases.
# The installer auto-detects platform → correct tarball → extract.
#
# Usage:
#   bash install.sh                     Interactive install
#   bash install.sh --offline [PATH]    Offline mode (binary or tarball)
#   bash install.sh --no-skill          Skip skill installation
#   bash install.sh --update            Overwrite existing installation
#   bash install.sh --lang en           Install with English skill
#   bash install.sh --edition std       Install standard edition (default)
#   bash install.sh --edition duckdb    Install DuckDB edition (requires CGO build)
#   bash install.sh --help              Show this help
# ============================================================

VERSION="v0.1.6"
REPO="IamWWT/dbexplain"
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
OFFLINE_PATH="" # optional pre-placed binary/tarball path for --offline
LANG_SKILL=""   # empty means interactive (ask user)
INSTALL_DIR=""  # resolved later
EDITION=""      # empty means interactive (ask user); std or duckdb
BINARY_SUFFIX="std"  # default fallback

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
  --offline [PATH]   Offline mode. If PATH is given, install that specific
                     binary or .tar.gz file directly. Accepts both raw
                     binaries and tarballs. If omitted, prompt user.
  --no-skill         Skip AI Agent skill installation (tool only).
  --update           Update mode: overwrite existing binary and skill files
                     without touching config.
  --lang zh|en       Skill language: zh=中文 (default), en=English.
  --edition std|duckdb  Edition to install: std (pure Go, default) or
                     duckdb (requires CGO, current platform only).
  --help             Show this help message and exit.

Examples:
  bash install.sh                          # Full interactive install
  bash install.sh --lang en                # Full install with English skill
  bash install.sh --no-skill               # Tool only, no skill
  bash install.sh --edition duckdb         # Install DuckDB edition
  bash install.sh --offline                # Offline: you provide the binary
  bash install.sh --offline ./dbexplain-v0.1.5-linux-amd64-std-upx.tar.gz  # Tarball
  bash install.sh --offline ./dbexplain-linux-amd64-std         # Raw binary
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
        --edition)
            if [ "$2" = "std" ] || [ "$2" = "duckdb" ]; then
                EDITION="$2"
                shift
            else
                echo "Unknown --edition value: $2 (expected std or duckdb)"
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
    if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
        rm -rf "$TMP_DIR"
    fi
}
trap cleanup EXIT INT TERM HUP

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

    info "Detected platform: ${OS}/${ARCH}"
}

# ── Tarball name resolution ──
# Per-platform tarball naming (UPX flag encoded in name):
#   dbexplain-${VERSION}-${OS}-${ARCH}-${edition}-{upx,noupx}.tar.gz
#   └─ dbexplain-${VERSION}-${OS}-${ARCH}-${edition}-{upx,noupx}/
#        └─ dbexplain-${OS}-${ARCH}-${edition}[.exe]
#
# UPX mapping:
#   linux/*  (std) → upx     (well supported)
#   darwin/* (std) → noupx   (cross-compiled or no avail)
#   windows  (std) → upx
#   duckdb/linux-amd64 → upx   (native)
#   duckdb/linux-arm64 → noupx (CGO cross-compiled)
get_upx_flag() {
    if [ "$BINARY_SUFFIX" = "duckdb" ] && [ "$ARCH" = "arm64" ]; then
        echo "noupx"
    elif [ "$OS" = "darwin" ]; then
        echo "noupx"
    else
        echo "upx"
    fi
}

get_tarball_name() {
    local edition="$1"
    local upx_flag
    upx_flag="$(get_upx_flag)"
    echo "dbexplain-${VERSION}-${OS}-${ARCH}-${edition}-${upx_flag}.tar.gz"
}

# Binary name inside tarball
get_binary_name() {
    local edition="$1"
    local name="dbexplain-${OS}-${ARCH}-${edition}"
    [ "$OS" = "windows" ] && name="${name}.exe"
    echo "$name"
}

# Tarball internal directory prefix (same as tarball name minus .tar.gz)
get_tarball_dirname() {
    local edition="$1"
    local upx_flag
    upx_flag="$(get_upx_flag)"
    echo "dbexplain-${VERSION}-${OS}-${ARCH}-${edition}-${upx_flag}"
}

# ── Edition selection ──
select_edition() {
    if [ -n "$EDITION" ]; then
        BINARY_SUFFIX="$EDITION"
        info "Edition: ${EDITION}"
    else
        echo ""
        step "Select edition:"
        echo "  1) Standard edition (-std) — pure Go, no DuckDB, zero runtime deps"
        echo "  2) DuckDB edition (-duckdb) — includes DuckDB, requires C libs (system pre-installed)"
        echo ""
        echo "  Note: DuckDB edition enables Parquet/JSON/CSV file analysis via DuckDB engine."
        echo "        Only available for the current platform (no cross-compile)."
        echo ""
        read -r -p "  Choose [1/2] (default: 1): " choice
        case "$choice" in
            2|duckdb) BINARY_SUFFIX="duckdb" ;;
            *)        BINARY_SUFFIX="std" ;;
        esac
        info "Selected edition: ${BINARY_SUFFIX}"
    fi

    BINARY_NAME="$(get_binary_name "$BINARY_SUFFIX")"
    TARBALL_NAME="$(get_tarball_name "$BINARY_SUFFIX")"
    TARBALL_DIR="$(get_tarball_dirname "$BINARY_SUFFIX")"
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

# ── Online install: download tarball from GitHub, extract binary ──
install_online() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL_NAME}"
    TMP_DIR="$(mktemp -d)"
    local tarball_path="${TMP_DIR}/${TARBALL_NAME}"

    step "Downloading ${TARBALL_NAME} ..."
    if command -v curl &>/dev/null; then
        curl -L --progress-bar -o "$tarball_path" "$DOWNLOAD_URL"
    elif command -v wget &>/dev/null; then
        wget -q --show-progress -O "$tarball_path" "$DOWNLOAD_URL"
    else
        err "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    # Validate tarball integrity
    if [ ! -s "$tarball_path" ]; then
        err "Downloaded file is empty: ${tarball_path}"
        exit 1
    fi

    step "Extracting ${BINARY_NAME} from tarball ..."
    if ! tar -tzf "$tarball_path" &>/dev/null; then
        err "Downloaded tarball is corrupt or invalid: ${TARBALL_NAME}"
        exit 1
    fi

    if ! tar -xzf "$tarball_path" -C "$TMP_DIR" "${TARBALL_DIR}/${BINARY_NAME}"; then
        err "Failed to extract ${BINARY_NAME} from tarball"
        err "  Expected path inside tarball: ${TARBALL_DIR}/${BINARY_NAME}"
        err "  Available files:"
        tar -tzf "$tarball_path" | sed 's/^/    /'
        exit 1
    fi

    local extracted="${TMP_DIR}/${TARBALL_DIR}/${BINARY_NAME}"
    if [ ! -f "$extracted" ]; then
        err "Extracted binary not found: ${extracted}"
        exit 1
    fi

    chmod +x "$extracted"
    info "Downloaded and extracted: ${BINARY_NAME}"
    move_binary "$extracted"
}

# ── Offline install: use provided path, tarball, or prompt user ──
# Accepts both:
#   - Raw binary:  dbexplain-linux-amd64-std
#   - Tarball:     dbexplain-v0.1.6-std-upx.tar.gz (auto-detected by .tar.gz suffix)
install_offline() {
    echo ""

    # Internal helper: extract binary from a tarball
    extract_from_tarball() {
        local tarball_path="$1"
        local tmp
        tmp="$(mktemp -d)"

        # Validate tarball integrity
        if ! tar -tzf "$tarball_path" &>/dev/null; then
            rm -rf "$tmp"
            return 1
        fi

        # Try to extract the expected binary for this platform
        if tar -xzf "$tarball_path" -C "$tmp" "${TARBALL_DIR}/${BINARY_NAME}" 2>/dev/null; then
            local extracted="${tmp}/${TARBALL_DIR}/${BINARY_NAME}"
            if [ -f "$extracted" ]; then
                chmod +x "$extracted"
                copy_binary "$extracted"
                rm -rf "$tmp"
                return 0
            fi
        fi

        # Fallback: scan tarball for any dbexplain binary
        info "Scanning tarball for dbexplain binary..."
        local found
        found="$(tar -tzf "$tarball_path" | grep -E 'dbexplain(-[^-]+){2,}(\.exe)?$' | grep -v '/$' | head -1)"
        if [ -n "$found" ]; then
            tar -xzf "$tarball_path" -C "$tmp" "$found"
            local extracted="${tmp}/${found}"
            chmod +x "$extracted"
            info "Found binary in tarball: ${found}"
            copy_binary "$extracted"
            rm -rf "$tmp"
            return 0
        fi

        rm -rf "$tmp"
        return 1
    }

    # ── Specific path provided ──
    if [ -n "$OFFLINE_PATH" ]; then
        if [ ! -f "$OFFLINE_PATH" ]; then
            err "Specified file not found: ${OFFLINE_PATH}"
            exit 1
        fi

        case "$OFFLINE_PATH" in
            *.tar.gz|*.tgz)
                step "Offline mode: installing from tarball ${OFFLINE_PATH}"
                if extract_from_tarball "$OFFLINE_PATH"; then
                    return
                fi
                err "Failed to extract binary from tarball: ${OFFLINE_PATH}"
                exit 1
                ;;
            *)
                step "Offline mode: using specified binary ${OFFLINE_PATH}"
                chmod +x "$OFFLINE_PATH"
                copy_binary "$OFFLINE_PATH"
                return
                ;;
        esac
    fi

    # ── Interactive offline mode ──
    TARBALL_URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL_NAME}"
    step "Offline mode: please obtain the binary or tarball manually."
    echo ""
    echo "  Download URL (tarball):"
    echo "    ${TARBALL_URL}"
    echo ""
    echo "  Then pass it to install.sh:"
    echo "    bash install.sh --offline /path/to/${TARBALL_NAME}"
    echo "    bash install.sh --offline /path/to/${BINARY_NAME}"
    echo ""
    echo "  Or place the tarball in the current directory and press Enter."
    echo ""

    read -r -p "  Press Enter once the file is in place... " _unused

    # Check for tarball in current directory first
    if [ -f "./${TARBALL_NAME}" ]; then
        if extract_from_tarball "./${TARBALL_NAME}"; then
            return
        fi
    fi

    # Check for raw binary in current directory
    if [ -f "./${BINARY_NAME}" ]; then
        chmod +x "./${BINARY_NAME}"
        copy_binary "./${BINARY_NAME}"
        return
    fi

    # Check destination path
    if [ -f "${DEST_BIN}" ]; then
        info "Found binary at ${DEST_BIN}"
        return
    fi

    err "Binary/tarball not found. Please re-run with --offline [PATH]"
    exit 1
}

# ── macOS Gatekeeper: remove quarantine attribute ──
remove_quarantine() {
    if [ "$OS" = "darwin" ]; then
        xattr -d com.apple.quarantine "$DEST_BIN" 2>/dev/null || true
    fi
}

# ── Copy binary to install dir (offline: preserve user's original file) ──
copy_binary() {
    local src="$1"
    if [ "$USE_SUDO" = "sudo" ]; then
        sudo mkdir -p "$INSTALL_DIR"
        sudo cp "$src" "$DEST_BIN"
        sudo chmod +x "$DEST_BIN"
    else
        mkdir -p "$INSTALL_DIR"
        cp "$src" "$DEST_BIN"
        chmod +x "$DEST_BIN"
    fi
    remove_quarantine
    info "Binary copied to ${DEST_BIN}"
    info "Original file preserved at ${src}"
}

# ── Move binary to install dir (online: temp file, no need to keep) ──
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
    remove_quarantine
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
        echo "  cd dbexplain-skill && bash scripts/install-skill.sh"
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
    select_edition
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
