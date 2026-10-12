#!/usr/bin/env bash
# What a release needs besides the archives.
#   packaging/release.sh version          the version in internal/app/version.go
#   packaging/release.sh check-tag vX.Y.Z  the tag names that version
#   packaging/release.sh checksums        SHA256SUMS over dist/release
#   packaging/release.sh notes            the CHANGELOG.md section of this version, else one written from git

set -euo pipefail
# shellcheck source=packaging/common.sh
source "$(dirname "$0")/common.sh"

sum() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi
}

check_tag() {
    local tag="${1:-}"
    [ -n "$tag" ] || die "usage: packaging/release.sh check-tag vX.Y.Z"
    tag="${tag#refs/tags/}"
    if [ "$tag" != "v$VERSION" ]; then
        echo "release: the tag is $tag and internal/app/version.go says $VERSION." >&2
        echo "release: set const Version = \"${tag#v}\" in internal/app/version.go and tag that commit." >&2
        exit 1
    fi
    echo "$VERSION"
}

checksums() {
    [ -d "$RELEASE" ] || die "$RELEASE does not exist"
    cd "$RELEASE"
    rm -f SHA256SUMS
    local assets=()
    while IFS= read -r asset; do assets+=("${asset#./}"); done \
        < <(find . -maxdepth 1 -type f -name "${PROGRAM}_${VERSION}.*" | sort)
    [ "${#assets[@]}" -gt 0 ] || die "no ${PROGRAM}_${VERSION}.* asset in $RELEASE"
    sum "${assets[@]}" > SHA256SUMS
    sum -c SHA256SUMS >/dev/null
    cat SHA256SUMS
}

notes() {
    local section=""
    if [ -f "$ROOT/CHANGELOG.md" ]; then
        section="$(awk -v want="## [$VERSION]" '
            /^## / { if (inside) exit; if (index($0, want) == 1) { inside = 1; next } }
            inside { print }
        ' "$ROOT/CHANGELOG.md")"
    fi
    if [ -z "${section//[[:space:]]/}" ]; then
        section="$("$ROOT/packaging/changelog.sh" section "$VERSION" | tail -n +2)"
    fi
    printf '%s\n' "$section" | sed '/./,$!d'
}

case "${1:-}" in
version)   echo "$VERSION" ;;
check-tag) check_tag "${2:-}" ;;
checksums) checksums ;;
notes)     notes ;;
*) echo "usage: packaging/release.sh {version|check-tag vX.Y.Z|checksums|notes}" >&2; exit 2 ;;
esac
