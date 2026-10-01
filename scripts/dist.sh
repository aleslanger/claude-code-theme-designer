#!/bin/sh
# Builds release archives and checksums into dist/.
# Usage: scripts/dist.sh <version>     e.g. scripts/dist.sh v0.1.0
set -eu

VERSION=${1:?usage: scripts/dist.sh <version>}
MODULE=github.com/aleslanger/claude-code-theme-designer
TARGETS=${TARGETS:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
DIST=dist

rm -rf "$DIST"
mkdir -p "$DIST"

for target in $TARGETS; do
    os=${target%/*}
    arch=${target#*/}
    name="claude-theme_${os}_${arch}"
    bin=claude-theme
    [ "$os" = windows ] && bin=claude-theme.exe
    mkdir -p "$DIST/$name"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
        -ldflags "-s -w -X $MODULE/internal/cli.Version=$VERSION" \
        -o "$DIST/$name/$bin" ./cmd/claude-theme
    cp LICENSE README.md "$DIST/$name/"
    tar -C "$DIST" -czf "$DIST/$name.tar.gz" "$name"
    rm -rf "${DIST:?}/$name"
done

(cd "$DIST" && sha256sum ./*.tar.gz | sed 's| \./| |' > checksums.txt)
echo "Built $VERSION into $DIST/:"
ls -1 "$DIST"
