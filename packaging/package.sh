#!/usr/bin/env bash
# Static Linux binaries (Go, CGO_ENABLED=0), their archive, a deb and an rpm, for TARGET:
#   amd64 (default) or arm64.
# The binaries cross-compile anywhere Go runs. The deb needs dpkg-deb (debian:12) and the rpm needs
# rpmbuild (almalinux:9); both only package the binary built before, so they run in those images
# without Go.
#   packaging/package.sh [binaries|archive|deb|rpm|all]

set -euo pipefail
# shellcheck source=packaging/common.sh
source "$(dirname "$0")/common.sh"

TARGET="${TARGET:-amd64}"
case "$TARGET" in
amd64) ARCH=x86_64; DEB_ARCH=amd64 ;;
arm64) ARCH=aarch64; DEB_ARCH=arm64 ;;
*) die "TARGET is amd64 or arm64, not $TARGET" ;;
esac
BINARY="$OUT/bin/linux-$TARGET/$PROGRAM"
GO="${GO:-go}"

WORK=()
cleanup() { [ "${#WORK[@]}" -eq 0 ] || rm -rf "${WORK[@]}"; }
trap cleanup EXIT

binaries() {
    require_interface
    say "building $PROGRAM $VERSION for linux/$TARGET"
    mkdir -p "$(dirname "$BINARY")"
    GOOS=linux GOARCH="$TARGET" CGO_ENABLED=0 "$GO" build -trimpath -ldflags='-s -w' \
        -o "$BINARY" ./cmd/svc-registry
    if command -v file >/dev/null 2>&1; then
        local kind; kind="$(file "$BINARY")"
        printf '%s\n' "$kind"
        grep -q 'statically linked\|static-pie' <<<"$kind" || die "$BINARY is not statically linked"
    fi
}

built() {
    [ -x "$BINARY" ] || die "$BINARY is missing; run 'TARGET=$TARGET packaging/package.sh binaries' first"
}

package() {
    built
    say "the archive for linux/$TARGET"
    archive "$BINARY" linux "$ARCH" linux/svc-registry.service
}

stage() {
    local tree="$1"
    built
    install -d -m 0755 "$tree/usr/bin" "$tree/usr/lib/svc-registry" "$tree/usr/share/svc-registry" \
        "$tree/usr/lib/systemd/system" "$tree/usr/lib/sysusers.d" "$tree/usr/lib/tmpfiles.d" \
        "$tree/usr/share/doc/svc-registry"
    install -m 0755 "$BINARY" "$tree/usr/bin/$PROGRAM"
    install -m 0755 "$ROOT/packaging/provision" "$tree/usr/lib/svc-registry/provision"
    install -m 0755 "$ROOT/packaging/linux/require-database" "$tree/usr/lib/svc-registry/require-database"
    install -m 0755 "$ROOT/packaging/linux/after-install" "$tree/usr/lib/svc-registry/after-install"
    install -m 0755 "$ROOT/packaging/linux/before-remove" "$tree/usr/lib/svc-registry/before-remove"
    install -m 0644 "$ROOT/packaging/linux/svc-registry.service" "$tree/usr/lib/systemd/system/svc-registry.service"
    install -m 0644 "$ROOT/packaging/linux/sysusers.conf" "$tree/usr/lib/sysusers.d/svc-registry.conf"
    install -m 0644 "$ROOT/packaging/linux/tmpfiles.conf" "$tree/usr/lib/tmpfiles.d/svc-registry.conf"
    install -m 0644 "$ROOT/packaging/svc-registry.env.example" "$tree/usr/share/svc-registry/svc-registry.env.example"
    install_interface "$tree/usr/share/svc-registry/web"
    install_docs "$tree/usr/share/doc/svc-registry"
    install -m 0644 "$ROOT/LICENSE" "$tree/usr/share/doc/svc-registry/copyright"
}

deb() {
    command -v dpkg-deb >/dev/null || die "no dpkg-deb here; run this in debian:12 (just package-deb)"
    local tree; tree="$(mktemp -d)"; WORK+=("$tree")
    say "the deb for linux/$TARGET"
    stage "$tree"
    install -d -m 0755 "$tree/DEBIAN"
    local size; size="$(du -k -s "$tree" | cut -f1)"
    sed -e "s/@VERSION@/${VERSION//-/\~}/" -e "s/@ARCH@/$DEB_ARCH/" -e "s/@INSTALLED_SIZE@/$size/" \
        "$ROOT/packaging/deb/control.in" > "$tree/DEBIAN/control"
    local script
    for script in postinst prerm postrm; do
        install -m 0755 "$ROOT/packaging/deb/$script" "$tree/DEBIAN/$script"
    done
    ( cd "$tree" && find . -path ./DEBIAN -prune -o -type f -print0 \
        | xargs -0 md5sum | sed 's| \./| |' > DEBIAN/md5sums )
    chmod 0644 "$tree/DEBIAN/md5sums"

    mkdir -p "$RELEASE"
    local target="$RELEASE/${PROGRAM}_${VERSION}.debian.${DEB_ARCH}.deb"
    rm -f "$target"
    dpkg-deb --root-owner-group -Zxz --build "$tree" "$target"
    dpkg-deb --info "$target"
    ls -l "$target"
}

rpm() {
    command -v rpmbuild >/dev/null || die "no rpmbuild here; run this in almalinux:9 (just package-rpm)"
    local tree; tree="$(mktemp -d)"
    local build; build="$(mktemp -d)"
    WORK+=("$tree" "$build")
    say "the rpm for linux/$TARGET"
    stage "$tree"
    local version="${VERSION//-/\~}"
    rpmbuild -bb "$ROOT/packaging/rpm/svc-registry.spec" \
        --target "$ARCH" \
        --define "registry_version $version" \
        --define "_sourcedir $tree" \
        --define "_topdir $build" \
        --define "_rpmdir $build/rpms" \
        --define "_build_id_links none" \
        --define "dist %{nil}"

    local made; made="$(find "$build/rpms" -name "${PROGRAM}-${version}-1.${ARCH}.rpm" -print -quit)"
    [ -n "$made" ] || die "rpmbuild produced no ${PROGRAM}-${version}-1.${ARCH}.rpm"
    mkdir -p "$RELEASE"
    local target="$RELEASE/${PROGRAM}_${VERSION}.el.${ARCH}.rpm"
    install -m 0644 "$made" "$target"
    command rpm -qpi "$target"
    ls -l "$target"
}

case "${1:-all}" in
binaries) binaries ;;
archive)  package ;;
deb)      deb ;;
rpm)      rpm ;;
all)      binaries; package ;;
*) echo "usage: packaging/package.sh [binaries|archive|deb|rpm|all]" >&2; exit 2 ;;
esac
