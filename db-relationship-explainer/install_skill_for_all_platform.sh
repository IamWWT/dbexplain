#!/bin/bash
set -e

# ============================================================
# install_skill_for_all_platform.sh — db-relationship-explainer
# ============================================================
# Interactive installer that deploys the skill to one or more
# AI platform directories (Claude Code, DeepSeek, Agents, AiXCoding)
# or to a project-local or custom directory.
#
# Usage:
#   bash install_skill_for_all_platform.sh          # interactive mode
#   bash install_skill_for_all_platform.sh --verify # verify existing installation
#   bash install_skill_for_all_platform.sh --help    # show help
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="${SCRIPT_DIR}/tools"
SKILL_MD="${SCRIPT_DIR}/SKILL.md"
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE="${SCRIPT_DIR}/.env.example"
SKILL_NAME="db-relationship-explainer"
VERSION="v0.0.3"

# ─── Platform detection ────────────────────────────────────

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "WARNING: Unrecognized architecture '$arch', falling back to amd64"
      arch="amd64"
      ;;
  esac

  case "$os" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*)
      os="windows"
      ;;
    *)
      echo "ERROR: Unsupported OS '$os'. Supported: linux, darwin (macOS), windows (MSYS2/Git Bash)"
      exit 1
      ;;
  esac

  echo "${os}-${arch}"
}

PLATFORM="$(detect_platform)"
OS="${PLATFORM%-*}"
ARCH="${PLATFORM#*-}"

# Binary name: dbexplain-{os}-{arch}[.exe]
BINARY_SRC="dbexplain-${PLATFORM}"
[ "$OS" = "windows" ] && BINARY_SRC="${BINARY_SRC}.exe"

BINARY_SRC_PATH="${TOOLS_DIR}/${BINARY_SRC}"

# ─── Colour helpers ─────────────────────────────────────────

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
CYAN=$'\033[0;36m'; BOLD=$'\033[1m'; NC=$'\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()     { echo -e "${RED}[ERROR]${NC} $*"; }
header()  { echo -e "\n${BOLD}${CYAN}$*${NC}"; }
_step()   { echo -e "${CYAN}→${NC} $*"; }

# ─── Pre-flight checks ──────────────────────────────────────

run_preflight() {
  if [ ! -f "$SKILL_MD" ]; then
    err "SKILL.md not found at $SKILL_MD"
    exit 1
  fi

  if [ ! -f "$BINARY_SRC_PATH" ]; then
    err "Binary not found for platform ${PLATFORM}"
    echo ""
    echo "  Expected: ${BINARY_SRC_PATH}"
    echo ""
    echo "  Available binaries:"
    ls -1 "$TOOLS_DIR/" 2>/dev/null | sed 's/^/    /' || true
    echo ""
    echo "  Download from GitHub Releases:"
    echo "    https://github.com/IamWWT/understand_dbs_skills/releases/tag/${VERSION}"
    echo ""
    echo "  Then place the downloaded binary into:"
    echo "    ${TOOLS_DIR}/"
    echo "  And re-run this script."
    exit 1
  fi

  info "Detected platform : ${BOLD}${PLATFORM}${NC}"
  info "Binary to install : ${BOLD}${BINARY_SRC}${NC} ($(du -h "$BINARY_SRC_PATH" | cut -f1))"
}

# ─── Menu ───────────────────────────────────────────────────

choose_target() {
  header "Choose installation target"

  echo ""
  echo "  ${BOLD}Global (user-level) — available across all projects${NC}"
  echo "    [1] All platforms (install once, symlink everywhere)"
  echo "    [2] Claude Code       (~/.claude/skills)"
  echo "    [3] DeepSeek           (~/.deepseek/skills)"
  echo "    [4] AixCoding          (~/.aixcoding/skills)"
  echo "    [5] Agents             (~/.agents/skills)"
  echo ""
  echo "  ${BOLD}Local (project-level) — scoped to a single workspace${NC}"
  echo "    [6] All project platforms (.claude + .deepseek + .aixcoding + .agents)"
  echo ""
  echo "  ${BOLD}Custom${NC}"
  echo "    [7] Custom directory"
  echo ""

  local choice
  read -r -p "  Enter choice [1-7]: " choice

  case "$choice" in
    1) install_all_global ;;
    2) install_to "$HOME/.claude/skills" "claude" ;;
    3) install_to "$HOME/.deepseek/skills" "deepseek" ;;
    4) install_to "$HOME/.aixcoding/skills" "aixcoding" ;;
    5) install_to "$HOME/.agents/skills" "agents" ;;
    6) install_all_project ;;
    7) install_custom ;;
    *) err "Invalid choice: $choice"; exit 1 ;;
  esac
}

# ─── Install helpers ────────────────────────────────────────

install_to() {
  local target_dir="$1"
  local label="$2"

  header "Installing to ${label} (${target_dir})"

  mkdir -p "${target_dir}/${SKILL_NAME}/tools"

  # Copy SKILL.md
  cp "$SKILL_MD" "${target_dir}/${SKILL_NAME}/SKILL.md"
  _step "SKILL.md → ${target_dir}/${SKILL_NAME}/SKILL.md"

  # Copy .env.example (always safe)
  if [ -f "$ENV_EXAMPLE" ]; then
    cp "$ENV_EXAMPLE" "${target_dir}/${SKILL_NAME}/.env.example"
    _step ".env.example → ${target_dir}/${SKILL_NAME}/.env.example"
  fi

  # Copy .env if it exists (may contain credentials)
  if [ -f "$ENV_FILE" ]; then
    copy_env_if_wanted "${target_dir}/${SKILL_NAME}"
  fi

  # Copy binary
  cp "$BINARY_SRC_PATH" "${target_dir}/${SKILL_NAME}/tools/${BINARY_SRC}"
  chmod +x "${target_dir}/${SKILL_NAME}/tools/${BINARY_SRC}"
  _step "${BINARY_SRC}  → ${target_dir}/${SKILL_NAME}/tools/${BINARY_SRC}"

  info "Done — installed to ${target_dir}/${SKILL_NAME}/"
}

copy_env_if_wanted() {
  local dest_dir="$1"
  echo ""
  echo -e "  ${YELLOW}Source .env detected — may contain database credentials.${NC}"
  echo -n "  Copy it to the installation directory? [y/N] "
  read -r answer
  if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
    cp "$ENV_FILE" "${dest_dir}/.env"
    chmod 600 "${dest_dir}/.env"
    _step ".env → ${dest_dir}/.env (permissions: 600)"
  else
    info "Skipped .env — copy .env.example to .env and edit it manually."
  fi
}

create_symlink() {
  local link_dir="$1"
  local canonical_dir="$2"
  local platform_name="$3"

  mkdir -p "$link_dir"

  if [ -L "${link_dir}/${SKILL_NAME}" ]; then
    # Already a symlink — refresh it
    rm -f "${link_dir}/${SKILL_NAME}"
  elif [ -d "${link_dir}/${SKILL_NAME}" ]; then
    warn "${platform_name}: ${link_dir}/${SKILL_NAME} already exists as a directory, skipping symlink"
    return
  fi

  ln -s "${canonical_dir}/${SKILL_NAME}" "${link_dir}/${SKILL_NAME}" 2>/dev/null || {
    warn "Could not create symlink for ${platform_name}, falling back to full copy"
    install_to "$link_dir" "$platform_name"
    return
  }
  _step "Symlink: ${link_dir}/${SKILL_NAME} → ${canonical_dir}/${SKILL_NAME}"
}

install_all_global() {
  local canonical="${HOME}/.agents/skills"

  # Install to canonical location
  install_to "$canonical" "agents (canonical)"

  echo ""
  info "Creating symlinks from other platforms to canonical location..."

  create_symlink "$HOME/.claude/skills"    "$canonical" "claude"
  create_symlink "$HOME/.deepseek/skills"  "$canonical" "deepseek"
  create_symlink "$HOME/.aixcoding/skills" "$canonical" "aixcoding"

  echo ""
  info "All platforms configured — one real install + symlinks."
  print_summary_global
}

install_all_project() {
  local cwd="$(pwd)"

  header "Installing to project-local directories under ${cwd}"

  install_to "${cwd}/.claude/skills"    "claude (project)"
  install_to "${cwd}/.deepseek/skills"  "deepseek (project)"
  install_to "${cwd}/.aixcoding/skills" "aixcoding (project)"
  install_to "${cwd}/.agents/skills"    "agents (project)"

  echo ""
  info "All four project-local platforms installed."
}

install_custom() {
  local target_dir
  read -r -p "  Enter target directory path: " target_dir

  if [ -z "$target_dir" ]; then
    err "No directory provided."
    exit 1
  fi

  # Expand ~
  target_dir="${target_dir/#\~/$HOME}"

  if [ ! -d "$target_dir" ]; then
    echo -n "  Directory does not exist. Create it? [Y/n] "
    read -r create
    if [ "$create" != "n" ] && [ "$create" != "N" ]; then
      mkdir -p "$target_dir"
    else
      exit 0
    fi
  fi

  install_to "$target_dir" "custom"
}

# ─── Summary ────────────────────────────────────────────────

print_summary_global() {
  echo ""
  echo -e "${BOLD}${CYAN}═══ Installation Summary ═══${NC}"
  echo ""
  echo -e "  Canonical dir: ${BOLD}~/.agents/skills/${SKILL_NAME}${NC}"
  echo -e "  Symlinks:"
  echo -e "    ~/.claude/skills/${SKILL_NAME}"
  echo -e "    ~/.deepseek/skills/${SKILL_NAME}"
  echo -e "    ~/.aixcoding/skills/${SKILL_NAME}"
  echo ""
}

# ─── Verification ───────────────────────────────────────────

verify_installation() {
  header "Verification"

  local dirs_to_check=()

  if [ -n "$1" ] && [ "$1" != "--verify" ]; then
    # Specific directory provided
    dirs_to_check=("$1")
  elif [ "$1" = "--verify" ]; then
    # Check common locations
    dirs_to_check=(
      "$HOME/.claude/skills/${SKILL_NAME}"
      "$HOME/.deepseek/skills/${SKILL_NAME}"
      "$HOME/.agents/skills/${SKILL_NAME}"
      "$HOME/.aixcoding/skills/${SKILL_NAME}"
      "$(pwd)/.claude/skills/${SKILL_NAME}"
      "$(pwd)/.deepseek/skills/${SKILL_NAME}"
      "$(pwd)/.agents/skills/${SKILL_NAME}"
      "$(pwd)/.aixcoding/skills/${SKILL_NAME}"
    )
  fi

  local found_any=false
  local errors=0

  for dir in "${dirs_to_check[@]}"; do
    # Resolve symlinks so we don't double-check the same physical dir
    local real_dir
    real_dir="$(readlink -f "$dir" 2>/dev/null || echo "$dir")"

    if [ ! -d "$dir" ] && [ ! -L "$dir" ]; then
      continue
    fi

    found_any=true
    local label
    label="$(echo "$dir" | sed "s|$HOME|~|")"
    echo ""
    echo -e "${BOLD}── ${label} ──${NC}"

    # 1. Check SKILL.md
    if [ -f "${real_dir}/SKILL.md" ]; then
      echo -e "  ${GREEN}✓${NC} SKILL.md present"
      # Quick YAML frontmatter check: must have 'name:' field and '---' separator
      if grep -qE '^name:' "${real_dir}/SKILL.md" && grep -qE '^---' "${real_dir}/SKILL.md"; then
        echo -e "  ${GREEN}✓${NC} SKILL.md frontmatter format OK"
      else
        echo -e "  ${YELLOW}⚠${NC} SKILL.md may be missing YAML frontmatter"
      fi
    else
      echo -e "  ${RED}✗${NC} SKILL.md missing"
      ((errors++))
    fi

    # 2. Check tools directory and binary
    local bin_path="${real_dir}/tools/${BINARY_SRC}"
    if [ -f "$bin_path" ]; then
      if [ -x "$bin_path" ]; then
        echo -e "  ${GREEN}✓${NC} ${BINARY_SRC} present + executable"
      else
        echo -e "  ${YELLOW}⚠${NC} ${BINARY_SRC} present but NOT executable — fixing..."
        chmod +x "$bin_path"
      fi
    else
      # Check for any dbexplain binary
      local any_bin
      any_bin="$(ls "${real_dir}/tools"/dbexplain-* 2>/dev/null | head -1)"
      if [ -n "$any_bin" ]; then
        echo -e "  ${YELLOW}⚠${NC} Found alternative binary: $(basename "$any_bin") (expected: ${BINARY_SRC})"
      else
        echo -e "  ${RED}✗${NC} No binary found in tools/"
        ((errors++))
      fi
    fi

    # 3. Smoke-test the binary (--version, no DSN needed)
    local binary
    binary="${real_dir}/tools/${BINARY_SRC}"
    if [ -x "$binary" ]; then
      local ver_output
      if ver_output="$("$binary" --version 2>&1)"; then
        echo -e "  ${GREEN}✓${NC} Binary smoke test: ${ver_output}"
      else
        echo -e "  ${YELLOW}⚠${NC} Binary --version returned non-zero: ${ver_output}"
      fi
    fi
  done

  echo ""

  if [ "$found_any" = false ]; then
    warn "No installation found. Run without --verify to install."
    exit 1
  fi

  if [ "$errors" -eq 0 ]; then
    info "All checks passed."
  else
    warn "${errors} issue(s) found. Re-run the installer to fix."
    exit 1
  fi
}

# ─── Update ──────────────────────────────────────────────────

# Scan common locations for existing installations (mirrors uninstall script logic)
find_installations() {
  local dirs=()
  for base in "$HOME/.claude/skills" "$HOME/.deepseek/skills" \
              "$HOME/.agents/skills" "$HOME/.aixcoding/skills"; do
    local d="${base}/${SKILL_NAME}"
    [ -d "$d" ] || [ -L "$d" ] && dirs+=("$d")
  done
  for prefix in ".claude" ".deepseek" ".aixcoding" ".agents"; do
    local d="$(pwd)/${prefix}/skills/${SKILL_NAME}"
    [ -d "$d" ] || [ -L "$d" ] && dirs+=("$d")
  done
  printf '%s\n' "${dirs[@]}"
}

update_installations() {
  local dirs=()
  IFS=$'\n' read -r -d '' -a dirs < <(find_installations && printf '\0')

  if [ "${#dirs[@]}" -eq 0 ]; then
    warn "No existing installations found. Run without --update to install."
    exit 0
  fi

  header "Updating ${#dirs[@]} installation(s)"

  for dir in "${dirs[@]}"; do
    local real_dir
    real_dir="$(readlink -f "$dir" 2>/dev/null || echo "$dir")"

    local short
    short="$(echo "$dir" | sed "s|$HOME|~|")"
    _step "Updating ${short} ..."

    mkdir -p "${real_dir}/tools"

    # Overwrite SKILL.md (new version)
    cp "$SKILL_MD" "${real_dir}/SKILL.md"
    echo -e "      ${GREEN}✓${NC} SKILL.md"

    # Overwrite .env.example (new template)
    if [ -f "$ENV_EXAMPLE" ]; then
      cp "$ENV_EXAMPLE" "${real_dir}/.env.example"
      echo -e "      ${GREEN}✓${NC} .env.example"
    fi

    # Preserve .env — only mention it exists
    if [ -f "${real_dir}/.env" ]; then
      echo -e "      ${YELLOW}○${NC} .env (preserved, unchanged)"
    fi

    # Overwrite binary
    cp "$BINARY_SRC_PATH" "${real_dir}/tools/${BINARY_SRC}"
    chmod +x "${real_dir}/tools/${BINARY_SRC}"
    echo -e "      ${GREEN}✓${NC} ${BINARY_SRC}"
  done

  echo ""
  info "Updated ${#dirs[@]} installation(s)."
  echo ""
  echo -e "  Run ${BOLD}bash install_skill_for_all_platform.sh --verify${NC} to confirm."
}

# ─── Help ────────────────────────────────────────────────────

show_help() {
  echo "db-relationship-explainer Skill Installer  ${VERSION}"
  echo ""
  echo "Usage:"
  echo "  bash install_skill_for_all_platform.sh              Interactive install"
  echo "  bash install_skill_for_all_platform.sh --update     Update SKILL.md + binary in all found installations"
  echo "  bash install_skill_for_all_platform.sh --verify     Verify existing install(s)"
  echo "  bash install_skill_for_all_platform.sh --help       This help"
  echo ""
  echo "The script auto-detects your OS and architecture, then interactively"
  echo "asks where to install the skill: globally (~/.claude/skills, etc.),"
  echo "per-project, or a custom directory."
  echo ""
  echo "--update overwrites SKILL.md and the binary in every found"
  echo "installation. The .env file is always preserved."
}

# ─── Main ────────────────────────────────────────────────────

main() {
  echo ""
  echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${NC}"
  echo -e "${BOLD}${CYAN}║  db-relationship-explainer  Installer    ║${NC}"
  echo -e "${BOLD}${CYAN}║  ${VERSION}                                ║${NC}"
  echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${NC}"

  case "${1:-}" in
    --help|-h)
      show_help
      exit 0
      ;;
    --update)
      run_preflight
      update_installations
      exit 0
      ;;
    --verify)
      run_preflight
      verify_installation "--verify"
      exit 0
      ;;
    "")
      run_preflight
      choose_target
      echo ""
      info "All done!"
      echo ""
      echo -e "  ${BOLD}Next steps:${NC}"
      echo -e "    1. Configure .env — edit the .env file and fill in your real DB credentials"
      echo -e "       (or copy .env.example to .env if it doesn't exist yet)"
      echo -e "    2. Verify —— ${BOLD}bash install_skill_for_all_platform.sh --verify${NC}"
      echo ""
      ;;
    *)
      err "Unknown argument: $1"
      show_help
      exit 1
      ;;
  esac
}

main "$@"
