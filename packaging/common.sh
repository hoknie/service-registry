# Sourced by the packaging scripts: the version, the release folder and the archive layout.
# shellcheck shell=bash

cd "$(dirname "${BASH_SOURCE[0]}")/.."
ROOT="$PWD"
OUT="${OUT:-$ROOT/dist}"
RELEASE="${RELEASE:-$OUT/release}"
PROGRAM="svc-registry"
VERSION_FILE="$ROOT/internal/app/version.go"

VERSION="${VERSION:-$(awk -F'"' '/^const Version = "/ { print $2; exit }' "$VERSION_FILE")}"
[ -n "$VERSION" ] || { echo "packaging: cannot read the version out of internal/app/version.go" >&2; exit 2; }
export VERSION

say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
die() { printf 'packaging: %s\n' "$*" >&2; exit 1; }

require_interface() {
    [ -f "$ROOT/web/out/en/login.html" ] \
        || die "web/out is missing; the packages ship the interface beside the binary, so run 'just web-build' (or 'just web-dist') first"
}

# install_interface <folder>: the built interface (web/out) in <folder>, directories 0755, files 0644.
install_interface() {
    local folder="$1"
    require_interface
    install -d -m 0755 "$folder"
    cp -R "$ROOT/web/out/." "$folder/"
    find "$folder" -type d -exec chmod 0755 {} +
    find "$folder" -type f -exec chmod 0644 {} +
}

# install_docs <folder>: README.md, docs/*.md and the full environment reference.
install_docs() {
    local dir="$1"
    install -d -m 0755 "$dir/docs"
    install -m 0644 "$ROOT/README.md" "$dir/README.md"
    install -m 0644 "$ROOT"/docs/*.md "$dir/docs/"
    install -m 0644 "$ROOT/.env.example" "$dir/docs/env.example"
}

# archive <binary> <platform> <arch> <service files from packaging/...>
# Writes $RELEASE/svc-registry_<version>.<platform>.<arch>.tar.gz: the binary, the interface in web/,
# svc-registry.env.example, the service files, README.md, docs/ and LICENSE.
archive() {
    local binary="$1" platform="$2" arch="$3"
    shift 3
    local name="${PROGRAM}_${VERSION}.${platform}.${arch}"
    local work; work="$(mktemp -d)"
    local tree="$work/$name"

    install -d -m 0755 "$tree"
    install -m 0755 "$binary" "$tree/$PROGRAM"
    install_interface "$tree/web"
    install -m 0644 "$ROOT/packaging/svc-registry.env.example" "$tree/svc-registry.env.example"
    local service
    for service in "$@"; do
        install -m 0644 "$ROOT/packaging/$service" "$tree/$(basename "$service")"
    done
    install_docs "$tree"
    install -m 0644 "$ROOT/LICENSE" "$tree/LICENSE"

    mkdir -p "$RELEASE"
    local target="$RELEASE/$name.tar.gz"
    rm -f "$target"
    local flavour; flavour="$(tar --version 2>/dev/null || true)"
    if grep -q GNU <<<"$flavour"; then
        tar --owner=0 --group=0 --numeric-owner --sort=name --mtime=@0 \
            -C "$work" -cf - "$name" | gzip -9 -n > "$target"
    else
        COPYFILE_DISABLE=1 tar -C "$work" --uid 0 --gid 0 --uname root --gname wheel \
            -cf - "$name" | gzip -9 -n > "$target"
    fi
    rm -rf "$work"

    local listing; listing="$(tar -tzf "$target")"
    grep -qxF "$name/$PROGRAM" <<<"$listing" || die "$target does not hold $name/$PROGRAM"
    grep -qxF "$name/web/en/login.html" <<<"$listing" || die "$target does not hold the interface"
    ls -l "$target"
}
