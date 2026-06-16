#!/bin/bash
set -e

# ──────────────────────────────────────────────────────────────
#  dbexplain — Cross‑platform build script
#
#  Usage: bash build.sh [mode] [tags...] [--no-upx | --upx]
#
#  Modes (platform = GOOS/GOARCH):
#    prod     linux/amd64 + linux/arm64 + darwin/amd64 + darwin/arm64 + windows/amd64
#             tags=full, UPX lzma (default)
#    dev      current GOOS/GOARCH only (e.g. linux/amd64), tags=full, fast, no UPX
#    test     current GOOS/GOARCH only (e.g. linux/amd64), tags=full, -race, no UPX
#    minimal  current GOOS/GOARCH only (e.g. linux/amd64), custom tags, UPX lzma
#
#  UPX control (override default auto-detect):
#    --no-upx   Skip UPX compression even if upx is installed
#    --upx      Force UPX compression (exit with error if not found)
#
#
#  Examples:
#    bash build.sh                                    # prod (linux/darwin/windows × amd64/arm64, full)
#    bash build.sh dev                                # current GOOS/GOARCH only, fast
#    bash build.sh test                               # current GOOS/GOARCH only, race detector
#    bash build.sh prod --no-upx                      # prod without UPX
#    bash build.sh minimal mysql,postgres             # 2 drivers only
#    bash build.sh minimal mysql,postgres --no-upx    # 2 drivers, no UPX
#    bash build.sh minimal csv,xlsx                   # file-query only
#
#  Tag reference — each db type can be included individually:
#    mysql postgres sqlite clickhouse redis mongodb
#    elasticsearch qdrant csv xlsx duckdb prometheus
#    oracle hive
#    full   — all drivers except duckdb (duckdb requires CGO)
#
#  Note: duckdb tag is NOT included in "full". To build with DuckDB,
#  use "minimal" mode and explicitly include: bash build.sh minimal duckdb,mysql,postgres
#  DuckDB builds require CGO (C toolchain: gcc/clang on Linux/macOS, mingw on Windows).
#  DuckDB linux/amd64 builds use -extldflags=-static for a fully self-contained binary.
#  DuckDB linux/arm64 native builds use -static (full static); cross-compiled arm64
#    uses -static-libgcc -static-libstdc++ (Ubuntu 22.04 cross toolchain glibc has
#    R_AARCH64_LD64_GOTPAGE_LO15 GOT overflow with -static).
#  DuckDB darwin builds retain /usr/lib/libSystem.B.dylib (macOS cannot fully static link).
#
#  Tag → Kind → DSN scheme mapping:
#    mysql       → mysql           → mysql://, mariadb://
#    postgres    → postgres,gaussdb → postgres://, pg://, gaussdb://, opengauss://
#    sqlite      → sqlite          → sqlite://, sqlite3://
#    clickhouse  → clickhouse      → clickhouse://, ch://
#    redis       → redis           → redis://, rediss://
#    mongodb     → mongodb         → mongodb://
#    elasticsearch → elasticsearch → elasticsearch://, es://, elasticsearchs://
#    qdrant      → qdrant          → qdrant://
#    prometheus  → prometheus      → prometheus://
#    oracle      → oracle          → oracle://, oracles://
#    hive        → hive            → hive://, hives://
#    csv         → csv,tsv         → csv://, tsv://
#    xlsx        → xlsx            → xlsx://
#
#  UPX compression notes:
#    - linux/amd64, linux/arm64, windows/amd64: fully supported
#    - darwin/amd64 (native macOS build): supported, UPX ≤ 4.x required
#    - darwin/amd64 (cross-compiled from Linux): SKIPPED — UPX 5.x CantUnpackException
#    - darwin/arm64: ALWAYS SKIPPED — UPX has no arm64 Mach-O support in any version
#    For compressed darwin/amd64 binaries, build natively on macOS with UPX ≤ 4.x.
# ──────────────────────────────────────────────────────────────

RELEASE_DIR="../release"
mkdir -p "$RELEASE_DIR"

# ── Parse UPX flags (before mode, from anywhere in args) ──────
UPX_MODE="auto"  # auto | force | skip
FILTERED_ARGS=()
for arg in "$@"; do
  case "$arg" in
    --no-upx) UPX_MODE="skip" ;;
    --upx)    UPX_MODE="force" ;;
    *)        FILTERED_ARGS+=("$arg") ;;
  esac
done
set -- "${FILTERED_ARGS[@]}"

# ── Help flag ──────────────────────────────────────────────────
if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  # Extract and print the header comment block (lines 4-44)
  # Remove leading "# " / "#" so output is clean markdown
  sed -n '4,57p' "$0" | sed 's/^# //; s/^#\t/\t/; s/^#$//'
  exit 0
fi

# ── Parse mode ────────────────────────────────────────────────
MODE="${1:-prod}"
CUSTOM_TAGS=""

if [ "$MODE" = "prod" ] || [ "$MODE" = "dev" ] || [ "$MODE" = "test" ]; then
  TAGS="full"
  shift 2>/dev/null || true
  if [ "$MODE" = "prod" ] && [ -n "${1:-}" ]; then
    echo "WARNING: prod mode ignores custom tags — use 'minimal' for selective builds" >&2
  fi
elif [ "$MODE" = "minimal" ]; then
  TAGS="${2:-}"
  if [ -z "${2:-}" ]; then
    echo "WARNING: minimal mode with no tags — building with zero drivers (infra only)" >&2
  fi
  shift 2 2>/dev/null || shift 1 2>/dev/null || true
else
  echo "Unknown mode: $MODE"
  echo "Usage: bash build.sh [prod|dev|test|minimal] [tags...]"
  exit 1
fi

# ── go mod tidy (prod only) ──────────────────────────────────
if [ "$MODE" = "prod" ]; then
  echo "[build] go mod tidy..."
  go mod tidy
fi

# ── Common ldflags ────────────────────────────────────────────
LDFLAGS="-s -w -X github.com/IamWWT/dbexplain/internal/version.Version=v0.1.7"
EXTRALDFLAGS=""

# ── Mode-specific flags ───────────────────────────────────────
case "$MODE" in
  prod)
    PLATFORMS=(
      "linux/amd64"
      "linux/arm64"
      "darwin/amd64"
      "darwin/arm64"
      "windows/amd64"
    )
    ;;
  dev|minimal)
    GOOS="${GOOS:-$(go env GOOS)}"
    GOARCH="${GOARCH:-$(go env GOARCH)}"
    PLATFORMS=("${GOOS}/${GOARCH}")
    ;;
  test)
    GOOS="${GOOS:-$(go env GOOS)}"
    GOARCH="${GOARCH:-$(go env GOARCH)}"
    PLATFORMS=("${GOOS}/${GOARCH}")
    EXTRALDFLAGS="-race"
    ;;
esac

# ── CGO / DuckDB detection ────────────────────────────────────
# DuckDB ships pre-built static libduckdb.a via duckdb-go-bindings.
# To produce a fully self-contained binary without ldd dependencies:
#   Linux:   -extldflags=-static  → links libc, libstdc++, libgcc all statically
#   Darwin:  -extldflags='-static-libgcc -static-libstdc++'
#            (macOS forbids fully static linking; /usr/lib/libSystem.B.dylib is
#            always present on any Mac and is the only remaining dependency)
#   Windows: static linking handled by mingw-w64 toolchain by default
CGO_ENABLED=0
DUCKDB_EXTLDFLAGS=""
IS_DUCKDB=false

if [[ ",$TAGS," == *",duckdb,"* ]]; then
  IS_DUCKDB=true
  CGO_ENABLED=1
  echo "[build] duckdb tag detected: CGO_ENABLED=1 (requires C toolchain)"
  echo "[build] DuckDB linux builds: -extldflags=-static (zero ldd dependencies)"
  echo "[build] DuckDB darwin builds: -static-libgcc -static-libstdc++ (system libs only)"
  echo "[build] WARNING: cross-compilation with CGO may fail; use native GOOS/GOARCH"
fi

# ── Build loop ────────────────────────────────────────────────
HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"
# ── Edition suffix ────────────────────────────────────────────
# Standard build (tags=full) gets -std suffix.
# DuckDB build gets -duckdb suffix.
# Custom/minimal builds get no suffix (user chose specific drivers).
EDITION_SUFFIX=""
if [ "$TAGS" = "full" ]; then
  EDITION_SUFFIX="-std"
elif $IS_DUCKDB; then
  EDITION_SUFFIX="-duckdb"
fi

for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  base="dbexplain-${GOOS}-${GOARCH}${EDITION_SUFFIX}"
  out="$RELEASE_DIR/$base"
  [ "$GOOS" = "windows" ] && out+=".exe"

  echo ""
  echo "Building $base (GOOS=$GOOS GOARCH=$GOARCH, tags=$TAGS)..."

  # Per-platform DuckDB extldflags
  DUCKDB_EXTLDFLAGS=""
  if $IS_DUCKDB; then
    case "$GOOS/$GOARCH" in
      linux/amd64)
        # -static links everything (libc, libstdc++, libgcc)
        # Result: zero dynamic dependencies, ldd shows "not a dynamic executable"
        DUCKDB_EXTLDFLAGS="-extldflags=-static"
        ;;
      linux/arm64)
        if [ "$GOOS/$GOARCH" = "$HOST_GOOS/$HOST_GOARCH" ]; then
          # Native ARM64 build: use native gcc, full -static works
          DUCKDB_EXTLDFLAGS="-extldflags=-static"
        else
          # Cross-compilation from another arch (e.g. x86_64):
          # Ubuntu 22.04 cross toolchain glibc has GOT overflow with -static.
          # -static-libgcc -static-libstdc++ avoids glibc static linking but keeps
          # C++ runtime static. glibc .so is required at runtime (present on all Linux).
          # NOTE: entire -extldflags=... is quoted so Go 1.26+ quoted.Split treats it
          # as a single field (quotes at field start, not mid-field).
          DUCKDB_EXTLDFLAGS="'-extldflags=-static-libgcc -static-libstdc++'"
        fi
        ;;
      darwin/*)
        # macOS has no static libSystem — fully static impossible.
        # -static-libgcc -static-libstdc++ eliminates C++ runtime deps.
        # Remaining: /usr/lib/libSystem.B.dylib (always present on any Mac).
        # NOTE: entire -extldflags=... is quoted for Go 1.26+ quoted.Split compat.
        DUCKDB_EXTLDFLAGS="'-extldflags=-static-libgcc -static-libstdc++'"
        ;;
      windows/*)
        # mingw-w64 links statically by default; no extra flags needed.
        DUCKDB_EXTLDFLAGS=""
        ;;
    esac
  fi

  # C cross-compiler for DuckDB (CGO) cross-platform builds
  CC=""
  if $IS_DUCKDB; then
    case "$GOOS/$GOARCH" in
      linux/arm64)
        if [ "$GOOS/$GOARCH" = "$HOST_GOOS/$HOST_GOARCH" ]; then
          # Native ARM64 build: use native gcc
          CC="gcc"
        else
          # Cross-compilation from another arch (e.g. x86_64 → ARM64)
          CC="aarch64-linux-gnu-gcc"
        fi
        ;;
      # windows/amd64)  CC="x86_64-w64-mingw32-gcc-posix" ;;  # uncomment when ready
    esac
  fi

  # Combine ldflags: base + EXTRALDFLAGS (race) + DUCKDB_EXTLDFLAGS
  BUILD_LDFLAGS="$LDFLAGS"
  [ -n "$EXTRALDFLAGS" ]      && BUILD_LDFLAGS="$BUILD_LDFLAGS $EXTRALDFLAGS"
  [ -n "$DUCKDB_EXTLDFLAGS" ] && BUILD_LDFLAGS="$BUILD_LDFLAGS $DUCKDB_EXTLDFLAGS"

  CGO_ENABLED=$CGO_ENABLED GOOS=$GOOS GOARCH=$GOARCH CC="$CC" go build \
    -tags "$TAGS" \
    -trimpath \
    -ldflags="$BUILD_LDFLAGS" \
    -o "$out" ./cmd/dbexplain

  # ── Architecture check ──────────────────────────────────────
  file "$out" | grep -q "$GOARCH" || {
    echo "ERROR: $out architecture mismatch!"
    file "$out"
    exit 1
  }

  # ── Static link check ───────────────────────────────────────
  if [ "$GOOS" = "linux" ]; then
    ldd_output=$(ldd "$out" 2>&1) || true
    if echo "$ldd_output" | grep -q "=> /"; then
      echo "WARNING: $out may not be statically linked!"
      echo "$ldd_output"
    elif $IS_DUCKDB; then
      echo "  static check: OK (no dynamic library references)"
    fi
  elif [ "$GOOS" = "darwin" ]; then
    if command -v otool >/dev/null 2>&1; then
      non_system=$(otool -L "$out" 2>&1 | grep -v ":$" | grep -v "/usr/lib/" | grep -v "/System/")
      if [ -n "$non_system" ]; then
        echo "WARNING: $out has non-system dynamic links:"
        echo "$non_system"
      elif $IS_DUCKDB; then
        echo "  dynamic check: OK (system libs only)"
        otool -L "$out" 2>/dev/null | grep -v ":$" | sed 's/^/    /'
      fi
    fi
  fi

  # ── UPX compression ─────────────────────────────────────────
  # UPX compresses the binary by appending a self-decompression stub.
  # Users do NOT need UPX installed at runtime — the binary is self-contained.
  #
  # Platform support matrix:
  #   linux/amd64     ✓ fully supported
  #   linux/arm64     ✓ fully supported (UPX 3.96+)
  #   windows/amd64   ✓ fully supported
  #   darwin/amd64    ✓ on native macOS build only (UPX ≤ 4.x; 5.x breaks Go Mach-O)
  #   darwin/arm64    ✗ UNSUPPORTED — UPX has no arm64 Mach-O support in any version
  #
  # Control: --no-upx skips, --upx forces (exits if upx not found), default=auto
  #
  should_upx=false
  upx_skip_reason=""

  if [ "$MODE" = "prod" ] || [ "$MODE" = "minimal" ]; then
    should_upx=true
  fi

  if [ "$UPX_MODE" = "skip" ]; then
    should_upx=false
    upx_skip_reason="--no-upx flag"
  elif [ "$UPX_MODE" = "force" ]; then
    should_upx=true
    if ! command -v upx >/dev/null 2>&1; then
      echo "ERROR: --upx specified but upx not found in PATH" >&2
      exit 1
    fi
  fi

  # darwin/arm64: UPX has no arm64 Mach-O support in any version
  if [ "$GOOS" = "darwin" ] && [ "$GOARCH" = "arm64" ]; then
    should_upx=false
    upx_skip_reason="darwin/arm64 — UPX has no arm64 Mach-O support"
  fi

  # darwin/amd64 cross-compiled from non-darwin: UPX 5.x breaks Go Mach-O
  # Native macOS builds work; cross-compiled ones do not.
  if [ "$GOOS" = "darwin" ] && [ "$GOARCH" = "amd64" ] && [ "$(uname -s)" != "Darwin" ]; then
    should_upx=false
    upx_skip_reason="darwin/amd64 cross-compiled — build natively on macOS for UPX compression (UPX ≤ 4.x)"
  fi

  if ! $should_upx; then
    [ -n "$upx_skip_reason" ] && echo "  UPX: skipped ($upx_skip_reason)"
    echo "Success: $out"
    continue
  fi

  if command -v upx >/dev/null 2>&1; then
    before=$(stat -c%s "$out" 2>/dev/null || stat -f%z "$out" 2>/dev/null)
    # Capture UPX exit code properly — do not pipe through tail (hides failures)
    upx_out=$(upx --best --lzma "$out" 2>&1)
    upx_exit=$?
    if [ $upx_exit -ne 0 ]; then
      echo "  UPX: compression failed (binary is valid but uncompressed)"
      echo "  UPX output: $(echo "$upx_out" | tail -3)"
    else
      after=$(stat -c%s "$out" 2>/dev/null || stat -f%z "$out" 2>/dev/null)
      if [ -n "$before" ] && [ -n "$after" ] && [ "$after" -gt 0 ]; then
        saved=$(( (before - after) * 100 / before ))
        echo "  UPX: ${before} → ${after} bytes (${saved}% saved)"
      fi
    fi
  else
    echo "  UPX not found — skipping compression"
  fi

  echo "Success: $out"
done

echo ""
echo "All binaries built into $RELEASE_DIR"
echo "Tags: $TAGS"
echo "Mode: $MODE"
