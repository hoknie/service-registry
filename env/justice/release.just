# The version in internal/app/version.go, the one a release tag must name
[group('release')]
version:
    @packaging/release.sh version

# Check that a tag (vX.Y.Z) names the version in internal/app/version.go
[group('release')]
check-tag tag:
    packaging/release.sh check-tag {{ tag }}

# SHA256SUMS over the packages in dist/release
[group('release')]
checksums:
    packaging/release.sh checksums

# CHANGELOG.md anew from the commits between v* tags
[group('release')]
changelog:
    packaging/changelog.sh rebuild

# The version the next release takes (auto from the commits, or patch/minor/major)
[group('release')]
next-version level='auto':
    @packaging/changelog.sh next {{ level }}

# Bump internal/app/version.go unless it is already past the last tag, then write its CHANGELOG.md section
[group('release')]
release level='auto':
    packaging/changelog.sh release {{ level }}
