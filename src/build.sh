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
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w -X main.version=v0.0.6" -o "$out" .
  
  # 校验架构正确性
  file "$out" | grep -q "$GOARCH" || {
    echo "ERROR: $out architecture mismatch!"
    file "$out"
    exit 1
  }
  echo "Success: $out"
done

echo "All binaries built into $RELEASE_DIR"
