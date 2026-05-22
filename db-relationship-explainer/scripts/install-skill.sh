#!/bin/bash
set -e

# ============================================================
# install-skill.sh — db-relationship-explainer Skill Deployer
# ============================================================
# Interactive installer that deploys the skill to one or more
# AI platform directories (Claude Code, DeepSeek, Agents, AiXCoding)
# or to a project-local or custom directory.
#
# Usage:
#   bash install-skill.sh          # interactive mode
#   bash install-skill.sh --verify # verify existing installation
#   bash install-skill.sh --update # update existing installations
#   bash install-skill.sh --help   # show help
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TOOLS_DIR="${SKILL_DIR}/tools"
SKILL_MD_ZH="${SKILL_DIR}/SKILL_ZH.md"
SKILL_MD_EN="${SKILL_DIR}/SKILL_EN.md"
ENV_EXAMPLE="${SKILL_DIR}/.env.dbexplain.example"
SKILL_NAME="db-relationship-explainer"
VERSION="v0.0.6"
LANG="zh"        # default: Chinese
LANG_VIA_CLI=""  # non-empty when --lang was passed on command line

# Returns the path to the language-specific SKILL.md source
skill_md_path() {
  if [ "$LANG" = "en" ]; then
    echo "$SKILL_MD_EN"
  else
    echo "$SKILL_MD_ZH"
  fi
}

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

# ─── Colour helpers ─────────────────────────────────────────

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
CYAN=$'\033[0;36m'; BOLD=$'\033[1m'; NC=$'\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()     { echo -e "${RED}[ERROR]${NC} $*"; }
header()  { echo -e "\n${BOLD}${CYAN}$*${NC}"; }
_step()   { echo -e "${CYAN}→${NC} $*"; }

# ─── Pre-flight checks ──────────────────────────────────────

# Track whether we're using a system-installed dbexplain or a local binary
SYSTEM_DBEXPLAIN=""
BINARY_SRC_PATH=""

run_preflight() {
  local src_md
  src_md="$(skill_md_path)"
  if [ ! -f "$src_md" ]; then
    err "SKILL.md not found at $src_md"
    exit 1
  fi

  # Check for system-installed dbexplain first (in PATH)
  if command -v dbexplain &>/dev/null; then
    SYSTEM_DBEXPLAIN="$(command -v dbexplain)"
    BINARY_SRC_PATH="$SYSTEM_DBEXPLAIN"
    info "Detected platform : ${BOLD}${PLATFORM}${NC}"
    info "Using system binary: ${BOLD}${SYSTEM_DBEXPLAIN}${NC}"
    return
  fi

  # system dbexplain not found
  err "dbexplain not found in PATH"
  echo ""
  echo "  Run the tool installer first:"
  echo ""
  echo "    bash install.sh"
  echo ""
  echo "  Or download from GitHub Releases:"
  echo "    https://github.com/IamWWT/understand_dbs_skills/releases"
  exit 1
}

# ─── Menu ───────────────────────────────────────────────────

choose_language() {
  header "Choose SKILL language"
  echo ""
  echo "  [1] 中文 (简体)"
  echo "  [2] English"
  echo ""
  local choice
  read -r -p "  Enter choice [1-2, default: 1]: " choice
  case "${choice:-1}" in
    2) LANG="en" ;;
    *) LANG="zh" ;;
  esac
  info "Language: ${LANG}"
}

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

  # Copy language-specific SKILL.md to target as SKILL.md
  local src_md
  src_md="$(skill_md_path)"
  cp "$src_md" "${target_dir}/${SKILL_NAME}/SKILL.md"
  _step "$(basename "$src_md") → ${target_dir}/${SKILL_NAME}/SKILL.md"

  # Copy .env.dbexplain.example (always safe)
  if [ -f "$ENV_EXAMPLE" ]; then
    cp "$ENV_EXAMPLE" "${target_dir}/${SKILL_NAME}/.env.dbexplain.example"
    _step ".env.dbexplain.example → ${target_dir}/${SKILL_NAME}/.env.dbexplain.example"
  fi

  # Install binary (as "dbexplain" — platform-agnostic name)
  local dest_bin="${target_dir}/${SKILL_NAME}/tools/dbexplain"

  if [ -n "$SYSTEM_DBEXPLAIN" ]; then
    # System binary available — create symlink
    rm -f "$dest_bin"  # remove any old copy/symlink
    if ln -s "$SYSTEM_DBEXPLAIN" "$dest_bin" 2>/dev/null; then
      _step "Symlink: ${dest_bin} → ${SYSTEM_DBEXPLAIN}"
    else
      # Symlink failed — fall back to copy
      cp "$BINARY_SRC_PATH" "$dest_bin"
      chmod +x "$dest_bin"
      _step "Copied:  ${BINARY_SRC_PATH} → ${dest_bin}"
    fi
  else
    # Local binary — copy with platform-agnostic name
    cp "$BINARY_SRC_PATH" "$dest_bin"
    chmod +x "$dest_bin"
    _step "Copied:  ${BINARY_SRC_PATH} → ${dest_bin}"
  fi

  info "Done — installed to ${target_dir}/${SKILL_NAME}/"
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
    # Specific directory provided as $1 (legacy path)
    dirs_to_check=("$1")
  elif [ -n "${2:-}" ]; then
    # Custom dir passed as $2 (with --verify flag as $1)
    dirs_to_check=("$2")
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

    # 2. Check tools directory and binary (look for "dbexplain" first, then legacy platform names)
    local bin_path="${real_dir}/tools/dbexplain"
    if [ -f "$bin_path" ]; then
      if [ -x "$bin_path" ]; then
        if [ -L "$bin_path" ]; then
          local target
          target="$(readlink -f "$bin_path" 2>/dev/null || readlink "$bin_path")"
          echo -e "  ${GREEN}✓${NC} dbexplain present (symlink → ${target})"
        else
          echo -e "  ${GREEN}✓${NC} dbexplain present + executable"
        fi
      else
        echo -e "  ${YELLOW}⚠${NC} dbexplain present but NOT executable — fixing..."
        chmod +x "$bin_path"
      fi
    else
      # Check for any dbexplain binary
      local any_bin
      any_bin="$(ls "${real_dir}/tools"/dbexplain* 2>/dev/null | head -1)"
      if [ -n "$any_bin" ]; then
        echo -e "  ${YELLOW}⚠${NC} Found alternative binary: $(basename "$any_bin")"
        bin_path="$any_bin"
      else
        echo -e "  ${RED}✗${NC} No binary found in tools/"
        ((errors++))
      fi
    fi

    # 3. Smoke-test the binary (--version, no DSN needed)
    local binary
    binary="$bin_path"
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

update_single_dir() {
  local real_dir
  real_dir="$(readlink -f "$1" 2>/dev/null || echo "$1")"

  # Validate: must contain SKILL.md or tools/ to be a skill dir
  if [ ! -f "${real_dir}/SKILL.md" ] && [ ! -d "${real_dir}/tools" ]; then
    warn "Not a skill installation directory: $1 (SKILL.md or tools/ not found)"
    return 1
  fi

  local short
  short="$(echo "$1" | sed "s|$HOME|~|")"
  _step "Updating ${short} ..."

  mkdir -p "${real_dir}/tools"

  local src_md
  src_md="$(skill_md_path)"
  cp "$src_md" "${real_dir}/SKILL.md"
  echo -e "      ${GREEN}✓${NC} SKILL.md ($LANG)"

  if [ -f "$ENV_EXAMPLE" ]; then
    cp "$ENV_EXAMPLE" "${real_dir}/.env.dbexplain.example"
    echo -e "      ${GREEN}✓${NC} .env.dbexplain.example"
  fi

  if [ -f "${real_dir}/.env.dbexplain" ]; then
    echo -e "      ${YELLOW}○${NC} .env.dbexplain (preserved, unchanged)"
  fi

  # Update binary (platform-agnostic name: dbexplain)
  local dest_bin="${real_dir}/tools/dbexplain"
  if [ -n "$SYSTEM_DBEXPLAIN" ]; then
    rm -f "$dest_bin"
    ln -s "$SYSTEM_DBEXPLAIN" "$dest_bin" 2>/dev/null || {
      cp "$BINARY_SRC_PATH" "$dest_bin"
      chmod +x "$dest_bin"
    }
  else
    cp "$BINARY_SRC_PATH" "$dest_bin"
    chmod +x "$dest_bin"
  fi
  local method="copied"
  [ -L "$dest_bin" ] && method="symlink"
  echo -e "      ${GREEN}✓${NC} dbexplain (${method})"
  return 0
}

update_installations() {
  local dirs=()
  local custom_dir="${1:-}"

  if [ -n "$custom_dir" ]; then
    # Expand ~
    custom_dir="${custom_dir/#\~/$HOME}"
    dirs=("${custom_dir}/${SKILL_NAME}")
  else
    IFS=$'\n' read -r -d '' -a dirs < <(find_installations && printf '\0')
  fi

  if [ "${#dirs[@]}" -eq 0 ]; then
    warn "No existing installations found. Run without --update to install."
    exit 0
  fi

  if [ -n "$custom_dir" ]; then
    header "Updating custom installation"
  else
    header "Updating ${#dirs[@]} installation(s)"
  fi

  local updated=0
  for dir in "${dirs[@]}"; do
    if update_single_dir "$dir"; then
      ((updated++))
    fi
  done

  echo ""
  info "Updated ${updated} installation(s)."
  echo ""
  echo -e "  Run ${BOLD}bash install_skill_for_all_platform.sh --verify${NC} to confirm."
}

# ─── Help ────────────────────────────────────────────────────

show_help() {
  echo "db-relationship-explainer Skill Installer  ${VERSION}"
  echo ""
  echo "Usage:"
  echo "  bash install_skill_for_all_platform.sh              Interactive install"
  echo "  bash install_skill_for_all_platform.sh --update            Update all found installations"
  echo "  bash install_skill_for_all_platform.sh --update <dir>      Update a specific installation directory"
  echo "  bash install_skill_for_all_platform.sh --verify            Verify all found install(s)"
  echo "  bash install_skill_for_all_platform.sh --verify <dir>      Verify a specific installation"
  echo "  bash install_skill_for_all_platform.sh --help              This help"
  echo ""
  echo "Options:"
  echo "  --lang zh|en    SKILL language: zh=中文 (default), en=English"
  echo ""
  echo "The script auto-detects your OS and architecture, then interactively"
  echo "asks where to install the skill: globally (~/.claude/skills, etc.),"
  echo "per-project, or a custom directory."
  echo ""
  echo "--update scans standard locations (global + project-local) by default."
  echo "Add a directory path to update a custom installation instead."
  echo "The .env.dbexplain file is always preserved."
}

# ─── Main ────────────────────────────────────────────────────

main() {
  # Parse --lang from args
  local args=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --lang)
        if [ "$2" = "zh" ] || [ "$2" = "en" ]; then
          LANG="$2"
          LANG_VIA_CLI="1"
          shift 2
        else
          err "--lang must be 'zh' or 'en', got: ${2:-}"
          exit 1
        fi
        ;;
      *)
        args+=("$1")
        shift
        ;;
    esac
  done
  set -- "${args[@]}"

  echo ""
  echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${NC}"
  echo -e "${BOLD}${CYAN}║  db-relationship-explainer  Installer    ║${NC}"
  echo -e "${BOLD}${CYAN}║  ${VERSION}  lang=${LANG}                      ║${NC}"
  echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${NC}"

  case "${1:-}" in
    --help|-h)
      show_help
      exit 0
      ;;
    --update)
      run_preflight
      update_installations "${2:-}"
      exit 0
      ;;
    --verify)
      run_preflight
      verify_installation "--verify" "${2:-}"
      exit 0
      ;;
    "")
      run_preflight
      # Skip interactive language pick if --lang was passed via CLI
      if [ -z "$LANG_VIA_CLI" ]; then
        choose_language
      fi
      choose_target
      echo ""
      info "All done!"
      echo ""
      echo -e "  ${BOLD}Next steps:${NC}"
      echo -e "    1. Create config: cp .env.dbexplain.example ~/.config/dbexplain/.env.dbexplain"
      echo -e "       Then edit ~/.config/dbexplain/.env.dbexplain with your real DB credentials."
      echo -e "    2. Encrypt config (recommended):"
      echo -e "         dbexplain encrypt ~/.config/dbexplain/.env.dbexplain"
      echo -e "         rm ~/.config/dbexplain/.env.dbexplain"
      echo -e "         dbexplain -env"
      echo -e "    3. Verify —— ${BOLD}bash install_skill_for_all_platform.sh --verify${NC}"
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
