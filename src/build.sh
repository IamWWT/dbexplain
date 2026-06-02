#!/bin/bash
set -e

# 目标输出目录（项目根目录下的 release/）
RELEASE_DIR="../release"
mkdir -p "$RELEASE_DIR"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  base="dbexplain-${GOOS}-${GOARCH}"
  out="$RELEASE_DIR/$base"
  [ "$GOOS" = "windows" ] && out+=".exe"

  echo "Building $base (GOOS=$GOOS GOARCH=$GOARCH)..."
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
    -ldflags="-s -w -X github.com/IamWWT/dbexplain/internal/version.Version=v0.1.1" -o "$out" ./cmd/dbexplain

  # 校验架构正确性
  file "$out" | grep -q "$GOARCH" || {
    echo "ERROR: $out architecture mismatch!"
    file "$out"
    exit 1
  }

  # 校验静态链接（CGO_ENABLED=0 保证所有平台零动态依赖）
  if [ "$GOOS" = "linux" ]; then
    # ldd 对静态链接二进制返回 exit 1 + "not a dynamic executable"（locale 无关）
    # 对动态链接二进制返回 exit 0（需要告警）
    ldd_output=$(ldd "$out" 2>&1) && {
      echo "WARNING: $out may not be statically linked!"
      echo "$ldd_output"
    } || true
  elif [ "$GOOS" = "darwin" ]; then
    if command -v otool >/dev/null 2>&1; then
      if otool -L "$out" 2>&1 | grep -v ":$" | grep -v "/usr/lib/" | grep -v "/System/" | grep -q .; then
        echo "WARNING: $out may have non-system dynamic links!"
      fi
    fi
  fi
  # Windows: CGO_ENABLED=0 PE binaries have no dynamic dependencies, skip check
  echo "Success: $out"
done

echo ""
echo "All binaries built into $RELEASE_DIR"
echo ""
echo "Single binary per platform — includes all database types + xlsx support."
