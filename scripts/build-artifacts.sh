#!/usr/bin/env bash
set -euo pipefail

out="${1:-dist}"
version="${AKTION_VERSION:-${GITHUB_REF_NAME:-dev}}"

rm -rf "$out"
mkdir -p "$out"

ldflags="-s -w -X main.version=${version}"

manager_targets=(
  linux/amd64
  linux/arm64
  windows/amd64
  windows/arm64
  darwin/amd64
  darwin/arm64
)

for target in "${manager_targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  ext=""
  if [[ "$os" == "windows" ]]; then
    ext=".exe"
  fi

  echo "==> manager: ${os}/${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "$ldflags" \
    -o "$out/aktion-${os}-${arch}${ext}" ./cmd/aktion
done

for arch in amd64 arm64; do
  echo "==> listener: linux/${arch}"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
    go build -trimpath -ldflags "$ldflags" \
    -o "$out/aktion-listener-linux-${arch}" ./cmd/aktion-listener
done


echo
echo "Artifacts:"
ls -lh "$out"
