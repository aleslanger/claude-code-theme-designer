#!/bin/sh
# Install, update or uninstall claude-theme (Claude Code Theme Designer).
#
#   curl -fsSL https://raw.githubusercontent.com/aleslanger/claude-code-theme-designer/master/install.sh | sh
#
# Options:
#   --version <tag>   install a specific release (default: latest)
#   --bin-dir <dir>   install location (default: ~/.local/bin)
#   --uninstall       remove the claude-theme binary
#   --purge           with --uninstall: also remove ~/.config/claude-theme-designer
#                     (drafts, backups, config). Never touches ~/.claude/themes.
#   --yes             do not ask for confirmation
#
# Running the script again updates an existing installation in place.
# Every download is verified against the release's checksums.txt.
set -eu

REPO=aleslanger/claude-code-theme-designer
BASE_URL=${CLAUDE_THEME_BASE_URL:-https://github.com/$REPO/releases}
BIN_DIR=${HOME}/.local/bin
VERSION=latest
ACTION=install
PURGE=0
YES=0

say() { printf '%s\n' "$*"; }
die() { printf 'claude-theme installer: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
    case $1 in
        --version) [ $# -ge 2 ] || die "--version needs a value"; VERSION=$2; shift 2 ;;
        --bin-dir) [ $# -ge 2 ] || die "--bin-dir needs a value"; BIN_DIR=$2; shift 2 ;;
        --uninstall) ACTION=uninstall; shift ;;
        --purge) PURGE=1; shift ;;
        --yes|-y) YES=1; shift ;;
        -h|--help) sed -n '2,17p' "$0" 2>/dev/null || true; exit 0 ;;
        *) die "unknown option: $1" ;;
    esac
done

case $VERSION in
    latest) ;;
    v[0-9]*) ;;
    *) die "invalid version '$VERSION' (expected latest or vX.Y.Z)" ;;
esac
case $VERSION in *[!A-Za-z0-9._-]*) die "invalid version '$VERSION'" ;; esac

TARGET="$BIN_DIR/claude-theme"
CONFIG_DIR=${XDG_CONFIG_HOME:-$HOME/.config}/claude-theme-designer

confirm() {
    [ "$YES" = 1 ] && return 0
    # Opening proves there is a controlling terminal (-r alone is not enough).
    ( : < /dev/tty ) 2>/dev/null || die "$1 Re-run with --yes to confirm non-interactively."
    printf '%s [y/N] ' "$1" > /dev/tty
    read -r answer < /dev/tty || answer=
    case $answer in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

uninstall() {
    if [ -e "$TARGET" ] || [ -L "$TARGET" ]; then
        say "Will remove: $TARGET"
        confirm "Remove the claude-theme binary?" || die "cancelled"
        rm -f "$TARGET"
        say "Removed $TARGET"
    else
        say "claude-theme is not installed in $BIN_DIR"
    fi
    if [ "$PURGE" = 1 ] && [ -d "$CONFIG_DIR" ]; then
        say "Will remove designer data (drafts, backups, config): $CONFIG_DIR"
        say "Installed themes in ~/.claude/themes are NOT touched; remove them first with"
        say "  claude-theme uninstall <name>"
        confirm "Delete $CONFIG_DIR?" || die "cancelled"
        rm -rf "$CONFIG_DIR"
        say "Removed $CONFIG_DIR"
    fi
}

detect_target() {
    case $(uname -s) in
        Linux) os=linux ;;
        Darwin) os=darwin ;;
        *) die "unsupported OS $(uname -s); on Windows download the release archive or use: go install github.com/$REPO/cmd/claude-theme@latest" ;;
    esac
    case $(uname -m) in
        x86_64|amd64) arch=amd64 ;;
        arm64|aarch64) arch=arm64 ;;
        *) die "unsupported CPU architecture $(uname -m)" ;;
    esac
    ASSET="claude-theme_${os}_${arch}.tar.gz"
}

resolve_version() {
    [ "$VERSION" != latest ] && return 0
    url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$BASE_URL/latest") || die "cannot reach $BASE_URL"
    VERSION=${url##*/}
    case $VERSION in v[0-9]*) ;; *) die "could not determine the latest release (got '$VERSION')" ;; esac
}

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
    elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d' ' -f1
    else die "need sha256sum or shasum to verify the download"
    fi
}

install() {
    command -v curl >/dev/null 2>&1 || die "curl is required"
    command -v tar >/dev/null 2>&1 || die "tar is required"
    detect_target
    resolve_version

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT INT TERM

    say "Downloading claude-theme $VERSION ($ASSET)..."
    curl -fsSL "$BASE_URL/download/$VERSION/$ASSET" -o "$tmp/$ASSET" || die "download failed"
    curl -fsSL "$BASE_URL/download/$VERSION/checksums.txt" -o "$tmp/checksums.txt" || die "checksum download failed"

    expected=$(awk -v f="$ASSET" '$2 == f { print $1 }' "$tmp/checksums.txt")
    [ -n "$expected" ] || die "$ASSET is not listed in checksums.txt"
    actual=$(sha256_of "$tmp/$ASSET")
    [ "$expected" = "$actual" ] || die "checksum mismatch for $ASSET (expected $expected, got $actual); not installing"

    tar -xzf "$tmp/$ASSET" -C "$tmp"
    new="$tmp/claude-theme_${os}_${arch}/claude-theme"
    [ -f "$new" ] || die "archive does not contain claude-theme"

    previous=""
    [ -x "$TARGET" ] && previous=$("$TARGET" version 2>/dev/null || true)

    mkdir -p "$BIN_DIR"
    # Copy next to the target, then rename: the swap is atomic, and a running
    # claude-theme keeps its old inode.
    cp "$new" "$TARGET.new.$$"
    chmod 755 "$TARGET.new.$$"
    mv -f "$TARGET.new.$$" "$TARGET"

    if [ -n "$previous" ]; then
        say "Updated: $previous -> $("$TARGET" version)"
    else
        say "Installed: $("$TARGET" version) -> $TARGET"
    fi
    case ":$PATH:" in
        *":$BIN_DIR:"*) ;;
        *) say "Note: $BIN_DIR is not on your PATH. Add it, e.g.: export PATH=\"$BIN_DIR:\$PATH\"" ;;
    esac
    say "Run 'claude-theme' to start, 'claude-theme doctor' to check your setup."
}

case $ACTION in
    install) install ;;
    uninstall) uninstall ;;
esac
