#!/bin/bash
set -e

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
  out="dbexplain-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && out+=".exe"
  
  echo "Building $out (GOOS=$GOOS GOARCH=$GOARCH)..."
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$out" .
  
  # 校验架构正确性
  file "$out" | grep -q "$GOARCH" || {
    echo "ERROR: $out architecture mismatch!"
    file "$out"
    exit 1
  }
  echo "Success: $out"
done
