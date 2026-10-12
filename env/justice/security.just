# Pinned security and CI tools (ADR-0069); `go run` fetches the Go ones into the module cache once.
GOVULNCHECK := 'golang.org/x/vuln/cmd/govulncheck@v1.8.0'
GOSEC := 'github.com/securego/gosec/v2/cmd/gosec@v2.29.0'
ACTIONLINT := 'github.com/rhysd/actionlint/cmd/actionlint@v1.7.12'
SHELLCHECK_IMAGE := 'koalaman/shellcheck:v0.11.0'
HADOLINT_IMAGE := 'hadolint/hadolint:v2.15.1'

# Known vulnerabilities reachable from the code, in its modules and in the Go standard library
[group('security')]
vuln:
    GOTOOLCHAIN="$({{ GO }} env GOVERSION)" {{ GO }} run {{ GOVULNCHECK }} ./...

# Static security analysis of the Go code (exceptions: `#nosec G<n> -- <reason>` in the code).
# G115 (integer conversions) is off: every converted value is bounded by config validation, page
# limits or Postgres column types before it is converted.
[group('security')]
sast:
    {{ GO }} run {{ GOSEC }} -quiet -exclude-generated -exclude-dir=tests -exclude-dir=internal/testsupport -exclude=G115 ./...

# Known vulnerabilities of the web's production dependencies, high and above
[group('security')]
web-audit:
    cd web && {{ PNPM }} audit --prod --audit-level high

# Lint the workflows (actionlint), the shell scripts (shellcheck) and env/Dockerfile (hadolint)
[group('security')]
ci-lint:
    #!/usr/bin/env bash
    set -euo pipefail
    {{ GO }} run {{ ACTIONLINT }}
    scripts=(); while IFS= read -r f; do scripts+=("$f"); done < <(grep -rlE '^#!(/usr/bin/env (ba)?sh|/bin/(ba)?sh)' env/scripts scripts packaging 2>/dev/null | sort)
    if command -v shellcheck >/dev/null; then
        shellcheck -x "${scripts[@]}"
    else
        docker run --rm -v "$PWD":/mnt:ro -w /mnt {{ SHELLCHECK_IMAGE }} -x "${scripts[@]}"
    fi
    echo "shellcheck: ${#scripts[@]} scripts"
    if command -v hadolint >/dev/null; then
        hadolint env/Dockerfile
    else
        docker run --rm -i {{ HADOLINT_IMAGE }} hadolint - < env/Dockerfile
    fi
    echo "hadolint: env/Dockerfile"

# Every security and CI check: vuln, sast, web-audit, ci-lint
[group('security')]
security: vuln sast web-audit ci-lint
