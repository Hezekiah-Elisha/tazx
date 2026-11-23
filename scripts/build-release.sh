#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-v1.1.0}"
OUT_DIR="dist"

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

echo "Building Tazx ${VERSION} release binaries..."
for target in "${TARGETS[@]}"; do
    os="${target%/*}"
    arch="${target#*/}"
    out_file="${OUT_DIR}/tazx-${os}-${arch}"
    echo "  -> ${out_file}"
    CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build \
        -trimpath \
        -ldflags "-s -w -X github.com/Hezekiah-Elisha/tazx/cmd.AppVersion=${VERSION}" \
        -o "${out_file}" .
done

(
    cd "${OUT_DIR}"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum tazx-* > checksums.txt
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 tazx-* > checksums.txt
    else
        echo "Error: sha256sum or shasum is required to generate checksums.txt" >&2
        exit 1
    fi
)

echo "Release artifacts written to ${OUT_DIR}/:"
ls -lh "${OUT_DIR}"
