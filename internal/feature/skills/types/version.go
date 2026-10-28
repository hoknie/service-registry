package types

import (
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const FirstVersion = "0.1.0"

func ParseVersion(raw string) (string, error) {
	v, err := semver.StrictNewVersion(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if err != nil || len(v.String()) > 128 {
		return "", InvalidVersion
	}
	plain, err := v.SetMetadata("")
	if err != nil {
		return "", InvalidVersion
	}
	return plain.String(), nil
}

type Parts struct {
	Major, Minor, Patch int64
	Prerelease          string
}

func PartsOf(version string) Parts {
	v, err := semver.StrictNewVersion(version)
	if err != nil {
		return Parts{}
	}
	return Parts{Major: int64(v.Major()), Minor: int64(v.Minor()), Patch: int64(v.Patch()), Prerelease: v.Prerelease()}
}

func Compare(a, b string) int {
	va, errA := semver.StrictNewVersion(a)
	vb, errB := semver.StrictNewVersion(b)
	switch {
	case errA != nil && errB != nil:
		return strings.Compare(a, b)
	case errA != nil:
		return -1
	case errB != nil:
		return 1
	}
	return va.Compare(vb)
}

func SortDesc(versions []Version) {
	slices.SortStableFunc(versions, func(a, b Version) int { return Compare(b.Version, a.Version) })
}

func NextPatch(existing []string) string {
	var top *semver.Version
	for _, raw := range existing {
		v, err := semver.StrictNewVersion(raw)
		if err == nil && (top == nil || v.GreaterThan(top)) {
			top = v
		}
	}
	if top == nil {
		return FirstVersion
	}
	if top.Prerelease() != "" {
		release, _ := top.SetPrerelease("")
		return release.String()
	}
	return top.IncPatch().String()
}

func ResolveVersion(form, manifest string, existing []string) (string, bool, error) {
	for _, explicit := range []string{form, manifest} {
		if strings.TrimSpace(explicit) != "" {
			v, err := ParseVersion(explicit)
			return v, true, err
		}
	}
	return NextPatch(existing), false, nil
}

func IsPrerelease(version string) bool {
	v, err := semver.StrictNewVersion(version)
	return err == nil && v.Prerelease() != ""
}
