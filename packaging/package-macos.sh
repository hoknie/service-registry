#!/usr/bin/env bash
# A universal macOS binary (arm64 and amd64 glued with lipo), its archive, and a .pkg that installs
# it and sets the service up for the user signed in (see packaging/macos/setup).
#   packaging/package-macos.sh [binaries|archive|pkg|check|all]
# check runs the service from the payload against CHECK_DATABASE_URL, or a throwaway Postgres in
# Docker when it is unset.

# verdict() evaluates its checks in single quotes, and they read the local variables set before them.
# shellcheck disable=SC2016,SC2034

set -euo pipefail
# shellcheck source=packaging/common.sh
source "$(dirname "$0")/common.sh"

BINARY="$OUT/bin/darwin-universal/$PROGRAM"
IDENTIFIER="io.github.hoknie.svc-registry"
PKG="$RELEASE/${PROGRAM}_${VERSION}.macos.universal.pkg"
MACOS="$ROOT/packaging/macos"
GO="${GO:-go}"

WORK=()
cleanup() { [ "${#WORK[@]}" -eq 0 ] || rm -rf "${WORK[@]}"; [ -z "${PG_CONTAINER:-}" ] || docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

[ "$(uname -s)" = Darwin ] || die "lipo, pkgbuild and productbuild only exist on macOS"

binaries() {
    require_interface
    local arch
    for arch in arm64 amd64; do
        say "building $PROGRAM $VERSION for darwin/$arch"
        mkdir -p "$OUT/bin/darwin-$arch"
        GOOS=darwin GOARCH="$arch" CGO_ENABLED=0 "$GO" build -trimpath -ldflags='-s -w' \
            -o "$OUT/bin/darwin-$arch/$PROGRAM" ./cmd/svc-registry
    done
    say "gluing the two into a universal binary"
    mkdir -p "$(dirname "$BINARY")"
    lipo -create "$OUT/bin/darwin-arm64/$PROGRAM" "$OUT/bin/darwin-amd64/$PROGRAM" -output "$BINARY"
    chmod 0755 "$BINARY"
    local architectures; architectures="$(lipo -archs "$BINARY")"
    for arch in arm64 x86_64; do
        grep -qw "$arch" <<<"$architectures" || die "$BINARY holds $architectures, not both arm64 and x86_64"
    done
    lipo -info "$BINARY"
}

built() {
    [ -x "$BINARY" ] || die "$BINARY is missing; run 'packaging/package-macos.sh binaries' first"
}

package() {
    built
    say "the archive for macOS"
    archive "$BINARY" macos universal macos/launch macos/setup macos/uninstall
}

stage() {
    local tree="$1"
    built
    install -d -m 0755 "$tree/usr/local/bin" "$tree/usr/local/libexec/svc-registry" \
        "$tree/usr/local/share/svc-registry" "$tree/usr/local/share/doc/svc-registry"
    install -m 0755 "$BINARY" "$tree/usr/local/bin/$PROGRAM"
    install -m 0755 "$ROOT/packaging/provision" "$tree/usr/local/libexec/svc-registry/provision"
    local helper
    for helper in launch setup uninstall; do
        install -m 0755 "$MACOS/$helper" "$tree/usr/local/libexec/svc-registry/$helper"
    done
    install -m 0644 "$ROOT/packaging/svc-registry.env.example" "$tree/usr/local/share/svc-registry/svc-registry.env.example"
    install_interface "$tree/usr/local/share/svc-registry/web"
    install_docs "$tree/usr/local/share/doc/svc-registry"
    install -m 0644 "$ROOT/LICENSE" "$tree/usr/local/share/doc/svc-registry/LICENSE"
    xattr -rc "$tree" 2>/dev/null || true
}

pkg() {
    local work; work="$(mktemp -d)"; WORK+=("$work")
    say "staging the install tree"
    stage "$work/root"

    say "pkgbuild"
    COPYFILE_DISABLE=1 pkgbuild \
        --root "$work/root" \
        --identifier "$IDENTIFIER" \
        --version "$VERSION" \
        --scripts "$MACOS/scripts" \
        --ownership recommended \
        --install-location / \
        "$work/svc-registry-component.pkg"

    sed -e "s/@VERSION@/$VERSION/g" -e "s/@IDENTIFIER@/$IDENTIFIER/g" \
        "$MACOS/distribution.xml" > "$work/distribution.xml"

    say "productbuild"
    mkdir -p "$RELEASE"
    rm -f "$PKG"
    productbuild \
        --distribution "$work/distribution.xml" \
        --resources "$MACOS/resources" \
        --package-path "$work" \
        "$PKG"
    ls -l "$PKG"
}

database() {
    if [ -n "${CHECK_DATABASE_URL:-}" ]; then
        DATABASE="$CHECK_DATABASE_URL"
        return
    fi
    command -v docker >/dev/null || die "set CHECK_DATABASE_URL or install Docker for a throwaway Postgres"
    PG_CONTAINER="svc-registry-macos-check-$$"
    docker run -d --name "$PG_CONTAINER" -p 127.0.0.1::5432 \
        -e POSTGRES_USER=registry -e POSTGRES_PASSWORD=registry -e POSTGRES_DB=registry \
        "${POSTGRES_IMAGE:-postgres:17-alpine}" >/dev/null
    local ready=0
    for _ in $(seq 1 60); do
        if docker exec "$PG_CONTAINER" pg_isready -q -h 127.0.0.1 -U registry -d registry; then ready=1; break; fi
        sleep 0.5
    done
    [ "$ready" = 1 ] || die "the throwaway Postgres did not start"
    local port; port="$(docker port "$PG_CONTAINER" 5432/tcp | head -1 | awk -F: '{ print $NF }')"
    DATABASE="postgres://registry:registry@127.0.0.1:$port/registry?sslmode=disable"
}

check() {
    [ -f "$PKG" ] || die "$PKG is missing; run 'packaging/package-macos.sh pkg' first"
    local work; work="$(mktemp -d)"; WORK+=("$work")
    local failed=0
    verdict() {
        if eval "$1"; then printf '  ok    %s\n' "$2"; else printf '  FAIL  %s\n' "$2"; failed=1; fi
    }

    say "what $(basename "$PKG") holds"
    pkgutil --expand "$PKG" "$work/expanded"
    local component="$work/expanded/svc-registry-component.pkg"
    mkdir -p "$work/payload"
    (cd "$work/payload" && gzip -dc "$component/Payload" | cpio -i --quiet)
    lsbom -p MUGf "$component/Bom" | awk -F'\t' '$4 !~ /\/\._/' > "$work/bom"
    local root="$work/payload/usr/local"
    mode() { awk -F'\t' -v path="./$1" '{ sub(/ +$/, "", $1) } $4 == path { print $1 "\t" $2 "/" $3 }' "$work/bom"; }

    local program
    for program in bin/svc-registry libexec/svc-registry/provision libexec/svc-registry/launch \
        libexec/svc-registry/setup libexec/svc-registry/uninstall; do
        verdict "[ \"\$(mode usr/local/$program)\" = \"-rwxr-xr-x	root/wheel\" ]" "$program is root's, 0755"
    done
    verdict '[ "$(lipo -archs "$root/bin/svc-registry")" = "x86_64 arm64" ]' "the binary is universal"
    verdict '[ "$("$root/bin/svc-registry" version)" = "svc-registry $VERSION" ]' "svc-registry version says $VERSION"
    verdict '[ -f "$root/share/svc-registry/svc-registry.env.example" ]' "the example settings are shipped"
    verdict '[ -f "$root/share/svc-registry/web/en/login.html" ]' "the interface is shipped in share/svc-registry/web"
    verdict '[ -x "$component/Scripts/postinstall" ] && sh -n "$component/Scripts/postinstall"' "postinstall is there and parses"
    verdict 'grep -q "conclusion.html" "$work/expanded/Distribution"' "the installer ends on the page that names DATABASE_URL and the password"

    say "setup for a user, run from the payload against a scratch home"
    local home="$work/home"
    mkdir -p "$home"
    env -u XDG_CONFIG_HOME HOME="$home" SVC_REGISTRY_PREFIX="$root" SVC_REGISTRY_SETUP_NO_START=1 \
        sh "$root/libexec/svc-registry/setup" >/dev/null
    local folder="$home/.config/svc-registry" data="$home/Library/Application Support/svc-registry"
    local settings="$folder/svc-registry.env"
    local plist="$home/Library/LaunchAgents/$IDENTIFIER.plist"
    verdict '[ "$(stat -f %Lp "$settings")" = 640 ]' "svc-registry.env is 0640"
    verdict '[ "$(stat -f %Lp "$folder/initial-password")" = 600 ]' "initial-password is 0600"
    verdict 'grep -qx "UPLOADS_DIR=$data/uploads" "$settings"' "uploads go to Application Support"
    verdict 'grep -q "^SECRETS_KEYS=k1:." "$settings"' "SECRETS_KEYS is generated"
    verdict 'plutil -lint "$plist" >/dev/null' "the LaunchAgent is a valid property list"
    verdict '[ "$(plutil -extract KeepAlive.SuccessfulExit raw "$plist")" = false ]' "launchd does not restart a clean exit"
    local before; before="$(shasum -a 256 < "$settings")"
    env -u XDG_CONFIG_HOME HOME="$home" SVC_REGISTRY_PREFIX="$root" SVC_REGISTRY_SETUP_NO_START=1 \
        sh "$root/libexec/svc-registry/setup" >/dev/null
    verdict '[ "$(shasum -a 256 < "$settings")" = "$before" ]' "running setup again keeps the settings"

    say "launch without DATABASE_URL exits cleanly"
    local message status
    message="$(SVC_REGISTRY_PREFIX="$root" sh "$root/libexec/svc-registry/launch" "$settings" 2>&1)" && status=0 || status=$?
    verdict '[ "$status" = 0 ] && grep -q DATABASE_URL <<<"$message"' "launch exits 0 and names DATABASE_URL"

    say "the service starts and admin signs in"
    local port=18082
    database
    sed -i '' -e "s|^DATABASE_URL=.*|DATABASE_URL=$DATABASE|" -e "s|^HTTP_ADDR=.*|HTTP_ADDR=127.0.0.1:$port|" "$settings"
    (cd "$data" && SVC_REGISTRY_PREFIX="$root" exec sh "$root/libexec/svc-registry/launch" "$settings") > "$work/service.log" 2>&1 &
    local service=$!
    for _ in $(seq 1 60); do
        curl -fsS "http://127.0.0.1:$port/api/ready" >/dev/null 2>&1 && break
        kill -0 "$service" 2>/dev/null || sleep 1
        sleep 0.5
    done
    verdict 'curl -fsS "http://127.0.0.1:$port/api/ready" >/dev/null' "GET /api/ready answers 200 after db:migrate"
    verdict '[ "$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$port/en/login")" = 200 ]' "GET /en/login answers the interface from share/svc-registry/web"
    local body password; password="$(cat "$folder/initial-password")"
    body="$(printf '{"email":"admin@example.com","password":"%s"}' "$password")"
    status="$(curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "$body" \
        "http://127.0.0.1:$port/api/v1/auth/login" || true)"
    verdict '[ "$status" = 200 ]' "admin@example.com signs in with initial-password (HTTP $status)"
    pkill -P "$service" 2>/dev/null || true
    kill "$service" 2>/dev/null || true
    wait "$service" 2>/dev/null || true
    [ "$failed" -eq 0 ] || { cat "$work/service.log"; die "the package is not what it should be"; }
    say "the .pkg is what it should be"
}

case "${1:-all}" in
binaries) binaries ;;
archive)  package ;;
pkg)      pkg ;;
check)    check ;;
all)      binaries; package; pkg; check ;;
*) echo "usage: packaging/package-macos.sh [binaries|archive|pkg|check|all]" >&2; exit 2 ;;
esac
