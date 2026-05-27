#!/bin/bash
# ============================================================
# uninstall-skill.sh — dbexplain-skill
# ============================================================
# Removes the skill from global, project-local, or custom
# directories. Detects all installations and asks before removal.
#
# Usage:
#   bash uninstall-skill.sh          # interactive mode
#   bash uninstall-skill.sh --help    # show help
#   bash uninstall-skill.sh --list    # list found installations
# ============================================================

SKILL_NAME="dbexplain-skill"
VERSION="v0.0.8"

RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
CYAN=$'\033[0;36m'; BOLD=$'\033[1m'; NC=$'\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()     { echo -e "${RED}[ERROR]${NC} $*"; }
header()  { echo -e "\n${BOLD}${CYAN}$*${NC}"; }
_step()   { echo -e "${CYAN}→${NC} $*"; }

# ─── Discovery ──────────────────────────────────────────────

find_installations() {
  local dirs=()

  # Global
  for base in "$HOME/.claude/skills" "$HOME/.deepseek/skills" \
              "$HOME/.agents/skills" "$HOME/.aixcoding/skills"; do
    local d="${base}/${SKILL_NAME}"
    if [ -d "$d" ] || [ -L "$d" ]; then
      dirs+=("$d")
    fi
  done

  # Project-local (scan current directory tree up to 2 levels)
  local cwd="$(pwd)"
  for prefix in ".claude" ".deepseek" ".aixcoding" ".agents"; do
    local d="${cwd}/${prefix}/skills/${SKILL_NAME}"
    if [ -d "$d" ] || [ -L "$d" ]; then
      dirs+=("$d")
    fi
  done

  printf '%s\n' "${dirs[@]}"
}

describe_installation() {
  local dir="$1"
  local short
  short="$(echo "$dir" | sed "s|$HOME|~|")"

  if [ -L "$dir" ]; then
    local target
    target="$(readlink "$dir")"
    echo -e "  ${YELLOW}symlink${NC}  ${short} → ${target}"
  else
    local has_env=""
    [ -f "${dir}/.env.dbexplain" ] && has_env=" + .env.dbexplain"
    [ -f "${dir}/.env.dbexplain.enc" ] && has_env="${has_env} + .env.dbexplain.enc"
    echo -e "  ${GREEN}directory${NC} ${short}${has_env}"
  fi
}

# ─── Removal ────────────────────────────────────────────────

remove_one() {
  local dir="$1"
  local short
  short="$(echo "$dir" | sed "s|$HOME|~|")"

  if [ -L "$dir" ]; then
    _step "Removing symlink: ${short}"
    rm "$dir"
    info "Removed ${short}"
  elif [ -d "$dir" ]; then
    # Check for .env.dbexplain / .env.dbexplain.enc and warn about credentials
    if [ -f "${dir}/.env.dbexplain" ] || [ -f "${dir}/.env.dbexplain.enc" ]; then
      echo ""
      echo -e "  ${YELLOW}⚠  This installation contains config files${NC}"
      if [ -f "${dir}/.env.dbexplain" ]; then
        echo -e "  ${YELLOW}   .env.dbexplain (plaintext credentials)${NC}"
      fi
      if [ -f "${dir}/.env.dbexplain.enc" ]; then
        echo -e "  ${YELLOW}   .env.dbexplain.enc (encrypted credentials)${NC}"
      fi
      echo -e "  ${YELLOW}   Removing will delete them permanently.${NC}"
      echo ""
    fi
    echo -n "  Remove ${short} and all its contents? [y/N] "
    read -r answer
    if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
      rm -rf "$dir"
      info "Removed ${short}"
    else
      info "Skipped ${short}"
    fi
  fi
}

remove_all() {
  local dirs=()
  IFS=$'\n' read -r -d '' -a dirs < <(find_installations && printf '\0')

  if [ "${#dirs[@]}" -eq 0 ]; then
    info "No installations found."
    return
  fi

  echo ""
  for d in "${dirs[@]}"; do
    remove_one "$d"
  done
}

remove_interactive() {
  local dirs=()
  IFS=$'\n' read -r -d '' -a dirs < <(find_installations && printf '\0')

  if [ "${#dirs[@]}" -eq 0 ]; then
    info "No installations found."
    exit 0
  fi

  header "Found ${#dirs[@]} installation(s)"

  for i in "${!dirs[@]}"; do
    echo -n "  [${i}] "
    describe_installation "${dirs[$i]}"
  done
  echo ""

  echo "  [a] Remove ALL"
  echo "  [q] Quit without removing"
  echo ""

  read -r -p "  Enter choice: " choice

  case "$choice" in
    q|Q) info "Exiting without changes."; exit 0 ;;
    a|A)
      echo ""
      for d in "${dirs[@]}"; do
        remove_one "$d"
      done
      ;;
    *)
      if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -lt "${#dirs[@]}" ]; then
        echo ""
        remove_one "${dirs[$choice]}"
      else
        err "Invalid choice: $choice"
        exit 1
      fi
      ;;
  esac
}

list_installations() {
  local dirs=()
  IFS=$'\n' read -r -d '' -a dirs < <(find_installations && printf '\0')

  if [ "${#dirs[@]}" -eq 0 ]; then
    echo "No installations found."
    exit 0
  fi

  echo "Found ${#dirs[@]} installation(s):"
  for d in "${dirs[@]}"; do
    describe_installation "$d"
  done
}

# ─── Help ────────────────────────────────────────────────────

show_help() {
  echo "dbexplain-skill Skill Uninstaller  ${VERSION}"
  echo ""
  echo "Usage:"
  echo "  bash uninstall-skill.sh           Interactive removal"
  echo "  bash uninstall-skill.sh --list    List found installations"
  echo "  bash uninstall-skill.sh --all     Remove ALL found installations"
  echo "  bash uninstall-skill.sh --help    This help"
  echo ""
  echo "Scans: ~/.claude/skills, ~/.deepseek/skills, ~/.agents/skills,"
  echo "       ~/.aixcoding/skills, and current project directories."
  echo ""
  echo "About .env.dbexplain/.env.dbexplain.enc:"
  echo "  If the installed skill directory contains an .env.dbexplain or encrypted"
  echo "  .env.dbexplain.enc file (which may hold database credentials), you will be"
  echo "  warned before removal. Backup these files before uninstalling if needed."
}

# ─── Main ────────────────────────────────────────────────────

main() {
  echo ""
  echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${NC}"
  echo -e "${BOLD}${CYAN}║  dbexplain-skill  Uninstaller            ║${NC}"
  echo -e "${BOLD}${CYAN}║  ${VERSION}                                ║${NC}"
  echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${NC}"

  case "${1:-}" in
    --help|-h)
      show_help
      exit 0
      ;;
    --list)
      list_installations
      exit 0
      ;;
    --all)
      remove_all
      exit 0
      ;;
    "")
      remove_interactive
      ;;
    *)
      err "Unknown argument: $1"
      show_help
      exit 1
      ;;
  esac
}

main "$@"
