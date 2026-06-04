#!/bin/bash
set -e

# ──────────────────────────────────────────────────────────────
#  dbexplain — Official release build script
#
#  Zero-flag command: produces ALL platform/edition/UPX variant
#  binaries and tarballs in one run. No flags needed.
#
#  See build.sh for developer builds (4 modes, custom tags).
#
#  Output in ../release/:
#    Binaries:
#      dbexplain-{GOOS}-{GOARCH}-std[.exe]          # 5 platform standard
#      dbexplain-{GOOS}-{GOARCH}-duckdb              # DuckDB edition
#      dbexplain-linux-arm64-duckdb                  # DuckDB arm64 cross
#
#    Tarballs (per platform × per UPX variant):
#      dbexplain-{VERSION}-{plat}-std-upx.tar.gz     # UPX compressed
#      dbexplain-{VERSION}-{plat}-std-noupx.tar.gz   # original (no UPX)
#      dbexplain-{VERSION}-{plat}-duckdb-upx.tar.gz
#      dbexplain-{VERSION}-{plat}-duckdb-noupx.tar.gz
#
#  Prerequisites:
#    - Go toolchain
#    - C toolchain (gcc/clang) for DuckDB build
#    - upx (optional — UPX tarballs skipped automatically if absent)
# ──────────────────────────────────────────────────────────────

RELEASE_DIR="../release"
mkdir -p "$RELEASE_DIR"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           dbexplain Release Build                       ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ──────────────────────────────────────────────────────────────
#  Helper: determine if UPX works for a (platform, edition)
#  Returns 0 (yes) or 1 (no)
# ──────────────────────────────────────────────────────────────
upx_works_for() {
  local plat="$1"
  local edition="$2"
  local os="${plat%-*}"
  local arch="${plat#*-}"

  # UPX must be installed
  command -v upx &>/dev/null || return 1

  case "$os" in
    darwin)
      return 1  # cross-compiled Mach-O: CantUnpackException
      ;;
    *)
      return 0  # linux/*, windows/* all UPX-capable
      ;;
  esac
}

# ──────────────────────────────────────────────────────────────
#  Helper: create UPX-compressed copy of a binary
#  Returns 0 on success, 1 on failure (caller handles skip)
# ──────────────────────────────────────────────────────────────
upx_copy() {
  local src="$1"
  local dst="$2"
  cp "$src" "$dst"
  if upx --best --lzma "$dst" &>/dev/null; then
    return 0
  else
    rm -f "$dst"
    return 1
  fi
}

# ──────────────────────────────────────────────────────────────
#  Helper: pack a single binary into a per-platform tarball
#  Tarball name: dbexplain-${VERSION}-${plat}-${edition}-${upx_flag}.tar.gz
#  Inside dir:   dbexplain-${VERSION}-${plat}-${edition}-${upx_flag}/
# ──────────────────────────────────────────────────────────────
VERSION=""
resolve_version() {
  VERSION=$(grep 'Version=v' "$(dirname "$0")/build.sh" | sed 's/.*Version=//; s/"//')
}

pack_tar() {
  local plat="$1"
  local edition="$2"
  local upx_flag="$3"
  local os="${plat%-*}"
  local arch="${plat#*-}"

  local binary="dbexplain-${plat}-${edition}"
  [ "$os" = "windows" ] && binary="${binary}.exe"

  local src="$RELEASE_DIR/$binary"
  [ -f "$src" ] || return 0

  local target_dir="dbexplain-${VERSION}-${plat}-${edition}-${upx_flag}"
  local tarball="${target_dir}.tar.gz"
  local tmpdir
  tmpdir="$(mktemp -d)"

  mkdir -p "$tmpdir/$target_dir"
  cp "$src" "$tmpdir/$target_dir/"
  tar -czf "$RELEASE_DIR/$tarball" -C "$tmpdir" "$target_dir"
  rm -rf "$tmpdir"

  local size
  size=$(stat -c%s "$RELEASE_DIR/$tarball" 2>/dev/null || stat -f%z "$RELEASE_DIR/$tarball" 2>/dev/null)
  echo "  $tarball  ($(numfmt --to=iec-i --suffix=B "$size" 2>/dev/null || echo "$size bytes"))"
}

# ──────────────────────────────────────────────────────────────
#  Phase 1: Standard edition (5-platform, CGO=0, --no-upx)
#  Build WITHOUT UPX first, then create UPX copies later
# ──────────────────────────────────────────────────────────────
echo "━━━ Phase 1/3: Standard edition (5-platform, CGO=0, tags=full, --no-upx) ━━━"
bash build.sh prod --no-upx

# Rename to -std suffix
echo ""
echo "  Adding -std suffix to standard binaries..."
for f in "$RELEASE_DIR"/dbexplain-*; do
  base=$(basename "$f")
  case "$base" in
    *-std*|*-duckdb*|*.bak) continue ;;
  esac
  dir=$(dirname "$f")
  name="${base%.exe}"
  ext=""
  [ "$base" != "$name" ] && ext=".exe"
  mv "$f" "$dir/${name}-std${ext}"
  echo "    $base → ${name}-std${ext}"
done

echo ""
echo "━━━ Phase 1 complete ─────────────────────────────────────"
echo ""

# ──────────────────────────────────────────────────────────────
#  Phase 2: DuckDB edition (CGO=1, --no-upx)
#  Build native + arm64 cross, both without UPX
# ──────────────────────────────────────────────────────────────
echo "━━━ Phase 2/3: DuckDB edition (CGO=1, --no-upx) ━━━"
DUCKDB_TAGS="duckdb,mysql,postgres,sqlite,clickhouse,redis,mongodb,elasticsearch,qdrant,csv,xlsx,prometheus"

if command -v gcc &>/dev/null || command -v clang &>/dev/null; then
  # Native platform DuckDB
  GOOS="${GOOS:-$(go env GOOS)}"
  GOARCH="${GOARCH:-$(go env GOARCH)}"
  base_std="dbexplain-${GOOS}-${GOARCH}-std"
  [ "$GOOS" = "windows" ] && base_std="${base_std}.exe"
  std_bin="$RELEASE_DIR/$base_std"

  # Back up std binary
  if [ -f "$std_bin" ]; then
    cp "$std_bin" "${std_bin}.bak"
  fi

  bash build.sh minimal "$DUCKDB_TAGS" --no-upx

  # Move duckdb binary, restore std
  base_nosuffix="dbexplain-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && base_nosuffix="${base_nosuffix}.exe"
  src="$RELEASE_DIR/$base_nosuffix"
  duckdb_out="$RELEASE_DIR/dbexplain-${GOOS}-${GOARCH}-duckdb"
  if [ -f "$src" ]; then
    mv "$src" "$duckdb_out"
    echo "  DuckDB native: $(basename "$duckdb_out")"
  fi
  if [ -f "${std_bin}.bak" ]; then
    mv "${std_bin}.bak" "$std_bin"
  fi

  # Cross-compile DuckDB for linux/arm64
  if command -v aarch64-linux-gnu-gcc &>/dev/null; then
    echo ""
    echo "  Cross-compiling DuckDB for linux/arm64..."
    GOOS=linux GOARCH=arm64 bash build.sh minimal "$DUCKDB_TAGS" --no-upx

    if [ -f "$RELEASE_DIR/dbexplain-linux-arm64" ]; then
      mv "$RELEASE_DIR/dbexplain-linux-arm64" "$RELEASE_DIR/dbexplain-linux-arm64-duckdb"
      echo "    → dbexplain-linux-arm64-duckdb"
    fi
  else
    echo ""
    echo "  WARNING: aarch64-linux-gnu-gcc not found — skipping linux/arm64 DuckDB"
  fi
else
  echo "  WARNING: No C compiler found — skipping DuckDB build entirely"
  echo "  Install gcc/clang and re-run for DuckDB support."
fi

echo ""
echo "━━━ Phase 2 complete ─────────────────────────────────────"
echo ""

# ──────────────────────────────────────────────────────────────
#  Phase 3: Create UPX copies + Package ALL tarballs
#  For each (platform, edition) binary:
#    1. Always create -noupx tarball from original
#    2. If UPX-capable, create temp UPX copy → -upx tarball → clean
# ──────────────────────────────────────────────────────────────
resolve_version
echo "━━━ Phase 3/3: UPX copies + tarballs (${VERSION}) ─━━"
echo ""

pack_dual_tars() {
  local plat="$1"
  local edition="$2"
  local os="${plat%-*}"

  local binary="dbexplain-${plat}-${edition}"
  [ "$os" = "windows" ] && binary="${binary}.exe"
  local src="$RELEASE_DIR/$binary"

  [ -f "$src" ] || return 0

  # Always create -noupx tarball from original (uncompressed) binary
  pack_tar "$plat" "$edition" "noupx"

  # If UPX works, create -upx tarball from compressed copy
  if upx_works_for "$plat" "$edition"; then
    local upx_bin="${src}.upxtmp"
    if upx_copy "$src" "$upx_bin"; then
      # Temporarily replace src for pack_tar
      mv "$src" "${src}.noupxbak"
      mv "$upx_bin" "$src"
      pack_tar "$plat" "$edition" "upx"
      # Restore original
      mv "$src" "$upx_bin" 2>/dev/null || true
      mv "${src}.noupxbak" "$src"
      rm -f "$upx_bin"
    fi
  fi
}

# Standard edition: all 5 platforms, each with upx+noupx variants
for plat in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64; do
  pack_dual_tars "$plat" "std"
done

# DuckDB edition: linux only
for plat in linux-amd64 linux-arm64; do
  pack_dual_tars "$plat" "duckdb"
done

echo ""

# ──────────────────────────────────────────────────────────────
#  Summary
# ──────────────────────────────────────────────────────────────
echo "Binaries:"
total_bin=0
for f in "$RELEASE_DIR"/dbexplain-*; do
  case "$f" in *.bak|*.tar.gz|*.upxtmp|*.noupxbak) continue ;; esac
  size=$(stat -c%s "$f" 2>/dev/null || stat -f%z "$f" 2>/dev/null)
  total_bin=$((total_bin + size))
  echo "  $(basename "$f")  ($(numfmt --to=iec-i --suffix=B "$size" 2>/dev/null || echo "$size bytes"))"
done
echo "  Total: $(numfmt --to=iec-i --suffix=B "$total_bin" 2>/dev/null || echo "$total_bin bytes")"
echo ""
echo "Tarballs:"
total_tar=0
for f in "$RELEASE_DIR"/dbexplain-*.tar.gz; do
  [ -f "$f" ] || continue
  size=$(stat -c%s "$f" 2>/dev/null || stat -f%z "$f" 2>/dev/null)
  total_tar=$((total_tar + size))
  echo "  $(basename "$f")  ($(numfmt --to=iec-i --suffix=B "$size" 2>/dev/null || echo "$size bytes"))"
done
echo "  Total: $(numfmt --to=iec-i --suffix=B "$total_tar" 2>/dev/null || echo "$total_tar bytes")"
echo ""
