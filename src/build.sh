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
#    elasticsearch qdrant csv xlsx duckdb
#    full   — all drivers except duckdb (duckdb requires CGO)
#
#  Note: duckdb tag is NOT included in "full". To build with DuckDB,
#  use "minimal" mode and explicitly include: bash build.sh minimal duckdb,mysql,postgres
#  DuckDB builds require CGO (C toolchain: gcc/clang on Linux/macOS, mingw on Windows).
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
#    csv         → csv,tsv         → csv://, tsv://
#    xlsx        → xlsx            → xlsx://
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
  sed -n '4,44p' "$0" | sed 's/^# //; s/^#\t/\t/; s/^#$//'
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
LDFLAGS="-s -w -X github.com/IamWWT/dbexplain/internal/version.Version=v0.1.3"
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

# ── CGO detection (duckdb tag) ────────────────────────────────
if [[ ",$TAGS," == *",duckdb,"* ]]; then
  CGO_ENABLED=1
  echo "[build] duckdb tag detected: CGO_ENABLED=1 (requires C toolchain)"
  echo "[build] WARNING: cross-compilation with CGO may fail; use native GOOS/GOARCH"
else
  CGO_ENABLED=0
fi

# ── Build loop ────────────────────────────────────────────────
for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  base="dbexplain-${GOOS}-${GOARCH}"
  out="$RELEASE_DIR/$base"
  [ "$GOOS" = "windows" ] && out+=".exe"

  echo ""
  echo "Building $base (GOOS=$GOOS GOARCH=$GOARCH, tags=$TAGS)..."

  CGO_ENABLED=$CGO_ENABLED GOOS=$GOOS GOARCH=$GOARCH go build \
    -tags "$TAGS" \
    -trimpath \
    -ldflags="$LDFLAGS" \
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
    # Check for actual dynamic library references (e.g., "libc.so.6 => /lib/...")
    if echo "$ldd_output" | grep -q "=> /"; then
      echo "WARNING: $out may not be statically linked!"
      echo "$ldd_output"
    fi
  elif [ "$GOOS" = "darwin" ]; then
    if command -v otool >/dev/null 2>&1; then
      if otool -L "$out" 2>&1 | grep -v ":$" | grep -v "/usr/lib/" | grep -v "/System/" | grep -q .; then
        echo "WARNING: $out may have non-system dynamic links!"
      fi
    fi
  fi

  # ── UPX compression ─────────────────────────────────────────
  # UPX (Ultimate Packer for eXecutables) compresses the binary by
  # appending a decompression stub that runs in-memory at startup.
  # The compressed binary is self-contained — users do NOT need upx
  # installed at runtime. CGO_ENABLED=0 + -s -w are prerequisites.
  #
  # Cross-compilation note: UPX supports Mach-O in general, but
  # Go cross-compiled Mach-O binaries (darwin targets built from
  # Linux) trigger CantUnpackException in UPX 5.0.0. The binary is
  # still valid and functional — just not UPX-compressed. Native
  # macOS builds with UPX work fine.
  #
  # Control (override auto-detect):
  #   --no-upx   skip even if upx is installed
  #   --upx      force (exit 1 if upx not found)
  #   (default)  auto: use upx in prod/minimal mode if found
  #
  should_upx=false
  if [ "$MODE" = "prod" ] || [ "$MODE" = "minimal" ]; then
    should_upx=true
  fi

  if [ "$UPX_MODE" = "skip" ]; then
    should_upx=false
    echo "  UPX: skipped via --no-upx"
  elif [ "$UPX_MODE" = "force" ]; then
    should_upx=true
    if ! command -v upx >/dev/null 2>&1; then
      echo "ERROR: --upx specified but upx not found in PATH" >&2
      exit 1
    fi
  fi

  if $should_upx && command -v upx >/dev/null 2>&1; then
      before=$(stat -c%s "$out" 2>/dev/null || stat -f%z "$out" 2>/dev/null)
      upx --best --lzma "$out" 2>&1 | tail -1
      after=$(stat -c%s "$out" 2>/dev/null || stat -f%z "$out" 2>/dev/null)
      if [ -n "$before" ] && [ -n "$after" ] && [ "$after" -gt 0 ]; then
        saved=$(( (before - after) * 100 / before ))
        echo "  UPX: ${before} → ${after} bytes (${saved}% saved)"
      fi
  elif $should_upx; then
      echo "  UPX not found — skipping compression"
  fi

  echo "Success: $out"
done

echo ""
echo "All binaries built into $RELEASE_DIR"
echo "Tags: $TAGS"
echo "Mode: $MODE"
