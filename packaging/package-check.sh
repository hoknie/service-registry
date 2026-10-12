#!/usr/bin/env bash
# Installs the deb (in debian:12) or the rpm (in almalinux:9) of this version and TARGET on a clean
# system next to a throwaway Postgres, checks what the installation made, runs the service the way
# its systemd unit does (as the user svc-registry), signs in as the first superadmin with the
# password the installation wrote, reinstalls over changed settings, then removes the package.
#   packaging/package-check.sh deb|rpm           on the host: Docker network, Postgres, clean system
#   packaging/package-check.sh inside deb|rpm    inside the clean system (run by the first form)

# verdict() evaluates its checks in single quotes, and they read the local variables set before them.
# shellcheck disable=SC2016,SC2034

set -euo pipefail
# shellcheck source=packaging/common.sh
source "$(dirname "$0")/common.sh"

TARGET="${TARGET:-amd64}"
case "$TARGET" in
amd64) ARCH=x86_64; DEB_ARCH=amd64 ;;
arm64) ARCH=aarch64; DEB_ARCH=arm64 ;;
*) die "TARGET is amd64 or arm64, not $TARGET" ;;
esac
DEB="$RELEASE/${PROGRAM}_${VERSION}.debian.${DEB_ARCH}.deb"
RPM="$RELEASE/${PROGRAM}_${VERSION}.el.${ARCH}.rpm"
PORT=8080
POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:17-alpine}"

outside() {
    local kind="$1" image package
    case "$kind" in
    deb) image=debian:12; package="$DEB" ;;
    rpm) image=almalinux:9; package="$RPM" ;;
    *) die "usage: packaging/package-check.sh deb|rpm" ;;
    esac
    [ -f "$package" ] || die "$package is missing; run 'just package-$kind' first"
    command -v docker >/dev/null || die "package-check needs Docker"

    NAME="svc-registry-check-$$"
    local name="$NAME"
    cleanup() { docker rm -f "$NAME-pg" >/dev/null 2>&1 || true; docker network rm "$NAME" >/dev/null 2>&1 || true; }
    trap cleanup EXIT
    docker network create "$name" >/dev/null
    docker run -d --name "$name-pg" --network "$name" --network-alias pg \
        -e POSTGRES_USER=registry -e POSTGRES_PASSWORD=registry -e POSTGRES_DB=registry \
        "$POSTGRES_IMAGE" >/dev/null
    say "checking $(basename "$package") on a clean $image"
    docker run --rm --network "$name" --platform "linux/$TARGET" \
        --volume "$ROOT":/work:ro --workdir /work --env TARGET --env VERSION \
        "$image" bash ./packaging/package-check.sh inside "$kind"
}

FAILED=0
verdict() {
    if eval "$1"; then printf '  ok    %s\n' "$2"; else printf '  FAIL  %s\n' "$2"; FAILED=1; fi
}

http() {
    local method="$1" path="$2" body="${3:-}"
    exec 3<>"/dev/tcp/127.0.0.1/$PORT"
    printf '%s %s HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\nContent-Type: application/json\r\nContent-Length: %s\r\n\r\n%s' \
        "$method" "$path" "${#body}" "$body" >&3
    cat <&3
    exec 3<&-
}

status_of() { http "$@" 2>/dev/null | tr -d '\000' | head -1 | awk '{ print $2 }'; }

unit_lines() { awk -F= -v key="$1" '$1 == key { sub(/^[^=]*=/, ""); print }' /usr/lib/systemd/system/svc-registry.service; }

as_service() {
    setpriv --reuid svc-registry --regid svc-registry --init-groups \
        env -i PATH=/usr/bin:/bin bash -c 'set -a; . /etc/svc-registry/svc-registry.env; set +a; cd /var/lib/svc-registry; exec "$@"' sh "$@"
}

install_package() {
    case "$KIND" in
    deb) apt-get install -y -qq "$DEB" >/tmp/install.log 2>&1 ;;
    rpm) dnf install -y -q "$RPM" >/tmp/install.log 2>&1 ;;
    esac
}

inside() {
    KIND="$1"
    local log=/tmp/serve.log
    case "$KIND" in
    deb) export DEBIAN_FRONTEND=noninteractive; apt-get update -qq >/dev/null ;;
    rpm) ;;
    *) die "usage: packaging/package-check.sh inside deb|rpm" ;;
    esac

    echo "installation"
    install_package || { cat /tmp/install.log; die "the package does not install"; }
    verdict '[ "$(/usr/bin/svc-registry version)" = "svc-registry $VERSION" ]' "svc-registry version says $VERSION"
    verdict 'getent passwd svc-registry >/dev/null' "the user svc-registry exists"
    verdict '[ "$(stat -c %a:%U:%G /etc/svc-registry)" = 750:root:svc-registry ]' "/etc/svc-registry is 0750 root:svc-registry"
    verdict '[ "$(stat -c %a:%U:%G /etc/svc-registry/svc-registry.env)" = 640:root:svc-registry ]' "svc-registry.env is 0640 root:svc-registry"
    verdict '[ "$(stat -c %a:%U /etc/svc-registry/initial-password)" = 600:root ]' "initial-password is 0600 root"
    verdict '[ "$(stat -c %U /var/lib/svc-registry/uploads)" = svc-registry ]' "/var/lib/svc-registry/uploads belongs to svc-registry"
    verdict '[ -f /usr/share/svc-registry/web/en/login.html ]' "the interface is in /usr/share/svc-registry/web"
    verdict '[ -f /usr/lib/systemd/system/svc-registry.service ]' "the unit svc-registry.service is installed"
    verdict 'grep -q "^SECRETS_KEYS=k1:." /etc/svc-registry/svc-registry.env' "SECRETS_KEYS is generated"
    verdict 'grep -qx "BOOTSTRAP_ADMIN_PASSWORD=$(cat /etc/svc-registry/initial-password)" /etc/svc-registry/svc-registry.env' "the first password is in svc-registry.env and initial-password"

    echo "without DATABASE_URL"
    local pre; read -r -a pre < <(unit_lines ExecStartPre | head -1)
    verdict '! as_service "${pre[@]}" >/tmp/pre.log 2>&1 && grep -q DATABASE_URL /tmp/pre.log' "the unit refuses to start and names DATABASE_URL"

    echo "with Postgres"
    sed -i 's|^DATABASE_URL=.*|DATABASE_URL=postgres://registry:registry@pg:5432/registry?sslmode=disable|' /etc/svc-registry/svc-registry.env
    local ready=0
    for _ in $(seq 1 30); do
        ready=1
        while read -r -a pre; do
            as_service "${pre[@]}" >/tmp/migrate.log 2>&1 || { ready=0; break; }
        done < <(unit_lines ExecStartPre)
        [ "$ready" = 0 ] || break
        sleep 1
    done
    verdict '[ "$ready" = 1 ]' "every ExecStartPre passes, the database is migrated"
    [ "$ready" = 1 ] || cat /tmp/migrate.log
    local start; read -r -a start < <(unit_lines ExecStart)
    as_service "${start[@]}" >"$log" 2>&1 &
    local server=$!
    local up=0
    for _ in $(seq 1 30); do
        if [ "$(status_of GET /api/ready)" = 200 ]; then up=1; break; fi
        sleep 1
    done
    verdict '[ "$up" = 1 ]' "serve answers /api/ready with 200"
    verdict '[ "$(status_of GET /en/login)" = 200 ]' "GET /en/login is 200"
    local password; password="$(cat /etc/svc-registry/initial-password)"
    verdict '[ "$(status_of POST /api/v1/auth/login "{\"email\":\"admin@example.com\",\"password\":\"$password\"}")" = 200 ]' "admin@example.com signs in with initial-password"
    kill "$server" 2>/dev/null || true
    wait "$server" 2>/dev/null || true
    [ "$FAILED" = 0 ] || tail -n 20 "$log"

    echo "reinstallation"
    echo "# changed by the administrator" >> /etc/svc-registry/svc-registry.env
    local before; before="$(md5sum < /etc/svc-registry/svc-registry.env)"
    case "$KIND" in
    deb) apt-get install -y -qq --reinstall "$DEB" >/tmp/install.log 2>&1 ;;
    rpm) dnf reinstall -y -q "$RPM" >/tmp/install.log 2>&1 ;;
    esac
    verdict '[ "$(md5sum < /etc/svc-registry/svc-registry.env)" = "$before" ]' "svc-registry.env is left as the administrator changed it"

    echo "removal"
    case "$KIND" in
    deb) apt-get remove -y -qq svc-registry >/tmp/remove.log 2>&1 ;;
    rpm) dnf remove -y -q svc-registry >/tmp/remove.log 2>&1 ;;
    esac
    verdict '[ ! -e /usr/bin/svc-registry ] && [ ! -e /usr/lib/systemd/system/svc-registry.service ]' "the binary and the unit are gone"
    verdict '[ -f /etc/svc-registry/svc-registry.env ] && [ -d /var/lib/svc-registry ]' "settings and data stay"

    [ "$FAILED" = 0 ] || die "the $KIND check failed"
    echo "the $KIND check passed"
}

case "${1:-}" in
inside) inside "${2:-}" ;;
*) outside "${1:-}" ;;
esac
