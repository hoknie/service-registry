# Static Linux binaries (amd64, arm64) and their archives in dist/release; TARGET=amd64 or arm64 for one
[group('package')]
package:
    #!/usr/bin/env bash
    set -euo pipefail
    for target in ${TARGET:-amd64 arm64}; do TARGET="$target" packaging/package.sh all; done

# The deb of a built Linux binary, made in debian:12 (Docker); TARGET as for package (default amd64)
[group('package')]
package-deb:
    docker run --rm --volume "$PWD":/work --workdir /work --env TARGET="${TARGET:-amd64}" \
        debian:12 bash -c './packaging/package.sh deb && chown -R "$(stat -c %u:%g /work)" dist'

# The rpm of a built Linux binary, made in almalinux:9 (Docker); TARGET as for package (default amd64)
[group('package')]
package-rpm:
    docker run --rm --volume "$PWD":/work --workdir /work --env TARGET="${TARGET:-amd64}" \
        almalinux:9 bash -c 'dnf -y -q install rpm-build >/dev/null && ./packaging/package.sh rpm && chown -R "$(stat -c %u:%g /work)" dist'

# Install the deb or the rpm on a clean system next to Postgres, run it, sign admin in, remove it (Docker)
[group('package')]
package-check kind:
    packaging/package-check.sh {{ kind }}

# A universal macOS binary, its archive and the .pkg, checked, in dist/release (macOS only)
[group('package')]
package-macos:
    packaging/package-macos.sh all
