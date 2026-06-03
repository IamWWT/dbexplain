#!/bin/bash
set -e

# ──────────────────────────────────────────────────────────────
#  dbexplain — Release build script
#
#  Produces two sets of binaries for each release:
#    1. Standard edition — 5-platform cross-compile, CGO_ENABLED=0, tags=full, UPX
#       Suffix: -std (pure Go, all drivers except DuckDB)
#    2. DuckDB edition  — current platform native compile, CGO_ENABLED=1,
#       tags=duckdb+all drivers, UPX
#       Suffix: -duckdb (all drivers including DuckDB)
#
#  Prerequisites:
#    - Go toolchain
#    - C toolchain (gcc/clang) for DuckDB build
#    - upx (optional, auto-detected)
#
#  Usage:
#    bash release.sh                    # full release
#    bash release.sh --no-upx           # skip UPX compression
#    bash release.sh --skip-duckdb      # skip DuckDB build (CGO not available)
#
#  Output in ../release/:
#    dbexplain-{GOOS}-{GOARCH}-std[.exe]       # 5 platform standard edition
#    dbexplain-{GOOS}-{GOARCH}-duckdb[.exe]    # current platform DuckDB edition
# ──────────────────────────────────────────────────────────────

RELEASE_DIR="../release"
mkdir -p "$RELEASE_DIR"

SKIP_DUCKDB=false
UPX_FLAG=""

for arg in "$@"; do
  case "$arg" in
    --skip-duckdb) SKIP_DUCKDB=true ;;
    --no-upx)      UPX_FLAG="--no-upx" ;;
    --help|-h)
      sed -n '4,28p' "$0" | sed 's/^# //; s/^#\t/\t/; s/^#$//'
      exit 0 ;;
    *)
      echo "Unknown option: $arg"
      echo "Usage: bash release.sh [--no-upx] [--skip-duckdb]"
      exit 1 ;;
  esac
done

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           dbexplain Release Build                       ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ── Phase 1: Standard edition (5-platform, CGO=0, -std suffix) ──
echo "━━━ Phase 1/2: Standard edition (5-platform, CGO=0, tags=full) ━━━"
bash build.sh prod $UPX_FLAG

# Rename all Phase 1 binaries to -std suffix
echo ""
echo "  Adding -std suffix to standard binaries..."
for f in "$RELEASE_DIR"/dbexplain-*; do
  base=$(basename "$f")
  # Skip if already has a suffix or is a symlink/backup
  case "$base" in
    *-std*|*-duckdb*|*.bak) continue ;;
  esac
  dir=$(dirname "$f")
  # dbexplain-linux-amd64 → dbexplain-linux-amd64-std
  # dbexplain-windows-amd64.exe → dbexplain-windows-amd64-std.exe
  name="${base%.exe}"
  ext=""
  [ "$base" != "$name" ] && ext=".exe"
  mv "$f" "$dir/${name}-std${ext}"
  echo "    $base → ${name}-std${ext}"
done

echo ""
echo "━━━ Phase 1 complete ─────────────────────────────────────"
echo ""

# ── Phase 2: DuckDB edition (current platform, CGO=1, -duckdb suffix) ──
if [ "$SKIP_DUCKDB" = true ]; then
  echo "━━━ Phase 2: SKIPPED (--skip-duckdb) ━━━"
else
  echo "━━━ Phase 2/2: DuckDB edition (CGO=1, native, tags=duckdb+all) ━━━"

  # Check for C toolchain
  if ! command -v gcc >/dev/null 2>&1 && ! command -v clang >/dev/null 2>&1; then
    echo "WARNING: No C compiler found (gcc/clang). DuckDB build requires CGO."
    echo "  Skipping DuckDB binary. Install gcc or clang and re-run,"
    echo "  or use '--skip-duckdb' to suppress this warning."
    echo ""
    exit 1
  fi

  DUCKDB_TAGS="duckdb,mysql,postgres,sqlite,clickhouse,redis,mongodb,elasticsearch,qdrant,csv,xlsx"

  GOOS="${GOOS:-$(go env GOOS)}"
  GOARCH="${GOARCH:-$(go env GOARCH)}"
  base_std="dbexplain-${GOOS}-${GOARCH}-std"
  base_duckdb="dbexplain-${GOOS}-${GOARCH}-duckdb"
  [ "$GOOS" = "windows" ] && base_std="${base_std}.exe"
  [ "$GOOS" = "windows" ] && base_duckdb="${base_duckdb}.exe"

  std_bin="$RELEASE_DIR/$base_std"
  duckdb_out="$RELEASE_DIR/${base_duckdb}"

  # Move standard binary aside before DuckDB build overwrites the base name
  if [ -f "$std_bin" ]; then
    cp "$std_bin" "${std_bin}.bak"
    echo "  Backed up: $(basename "$std_bin") → $(basename "$std_bin").bak"
  fi

  bash build.sh minimal "$DUCKDB_TAGS" $UPX_FLAG

  # Move DuckDB build to -duckdb name, restore standard binary
  base_nosuffix="dbexplain-${GOOS}-${GOARCH}"
  src="$RELEASE_DIR/$base_nosuffix"
  [ "$GOOS" = "windows" ] && src="${src}.exe"
  if [ -f "$src" ]; then
    mv "$src" "$duckdb_out"
    echo "  DuckDB: $(basename "$src") → $(basename "$duckdb_out")"
  fi
  if [ -f "${std_bin}.bak" ]; then
    mv "${std_bin}.bak" "$std_bin"
    echo "  Restored: $(basename "$std_bin").bak → $(basename "$std_bin")"
  fi

  echo ""
  echo "━━━ Phase 2 complete ─────────────────────────────────────"
fi

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  Release build complete                                 ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
ls -lh "$RELEASE_DIR"/
echo ""

# Collect sizes
echo "Binaries:"
total=0
for f in "$RELEASE_DIR"/dbexplain-*; do
  case "$f" in *.bak) continue ;; esac
  size=$(stat -c%s "$f" 2>/dev/null || stat -f%z "$f" 2>/dev/null)
  total=$((total + size))
  echo "  $(basename "$f")  ($(numfmt --to=iec-i --suffix=B "$size" 2>/dev/null || echo "$size bytes"))"
done
echo "  Total: $(numfmt --to=iec-i --suffix=B "$total" 2>/dev/null || echo "$total bytes")"
echo ""
