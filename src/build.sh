#!/bin/bash
PLATFORMS=(
  "linux/amd64" "linux/arm64"
  "darwin/amd64" "darwin/arm64"
  "windows/amd64"
)
for platform in "${PLATFORMS[@]}"; do
  GOOS=${platform%/*} 
  GOARCH=${platform#*/}
  out="dbexplain-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && out+=".exe"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o "$out" .
done